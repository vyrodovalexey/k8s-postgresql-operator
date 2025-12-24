/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
	"go.uber.org/zap"
	"math/big"
	"strings"
	"time"

	_ "github.com/lib/pq"
	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	VaultClient *vault.Client
	Log         *zap.SugaredLogger
}

// +kubebuilder:rbac:groups=instance.alexvyrodov.example,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=instance.alexvyrodov.example,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=instance.alexvyrodov.example,resources=users/finalizers,verbs=update
// +kubebuilder:rbac:groups=instance.alexvyrodov.example,resources=postgresqls,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// Fetch the User instance
	user := &instancev1alpha1.User{}
	if err := r.Get(ctx, req.NamespacedName, user); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "Failed to get User")
		return ctrl.Result{}, err
	}

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := r.findPostgresqlByID(ctx, user.Spec.PostgresqlID)
	if err != nil {
		r.Log.Error(err, "Failed to find PostgreSQL instance", "postgresqlID", user.Spec.PostgresqlID)
		updateUserCondition(user, "Ready", metav1.ConditionFalse, "PostgresqlNotFound",
			fmt.Sprintf("PostgreSQL instance with ID %s not found: %v", user.Spec.PostgresqlID, err))
		if err := r.Status().Update(ctx, user); err != nil {
			r.Log.Error(err, "Failed to update User status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		r.Log.Info("PostgreSQL instance is not connected, waiting", "postgresqlID", user.Spec.PostgresqlID)
		updateUserCondition(user, "Ready", metav1.ConditionFalse, "PostgresqlNotConnected",
			fmt.Sprintf("PostgreSQL instance with ID %s is not connected", user.Spec.PostgresqlID))
		if err := r.Status().Update(ctx, user); err != nil {
			r.Log.Error(err, "Failed to update User status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Error(nil, "PostgreSQL instance has no external instance configuration", "postgresqlID", user.Spec.PostgresqlID)
		updateUserCondition(user, "Ready", metav1.ConditionFalse, "InvalidConfiguration",
			"PostgreSQL instance has no external instance configuration")
		if err := r.Status().Update(ctx, user); err != nil {
			r.Log.Error(err, "Failed to update User status")
		}
		return ctrl.Result{}, nil
	}

	port := externalInstance.Port
	if port == 0 {
		port = 5432
	}

	sslMode := externalInstance.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}

	var postgresUsername, postgresPassword, dbPassword string

	// Get admin credentials from Vault if available
	if r.VaultClient != nil {
		vaultUsername, vaultPassword, err := r.VaultClient.GetPostgresqlCredentials(ctx, user.Spec.PostgresqlID)
		if err != nil {
			r.Log.Error(err, "Failed to get credentials from Vault", "postgresqlID", user.Spec.PostgresqlID)
		} else {
			postgresUsername = vaultUsername
			postgresPassword = vaultPassword
			r.Log.Info("Credentials retrieved from Vault by user controller", "postgresqlID", user.Spec.PostgresqlID)
		}
	} else {
		r.Log.Info("Vault client not available")
	}

	// Get user password from Vault or generate/store it
	if r.VaultClient != nil {
		vaultUserPassword, err := r.VaultClient.GetPostgresqlUserCredentials(ctx, user.Spec.PostgresqlID, user.Spec.Username)
		if err != nil || vaultUserPassword == "" {
			r.Log.Info("User credentials not found in Vault, generating new password", "postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username, "error", err)
			// Generate random password

			// If no password in spec, generate a secure random one
			generatedPassword, err := generateRandomPassword(32)
			if err != nil {
				r.Log.Error(err, "Failed to generate random password", "postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
				return ctrl.Result{}, fmt.Errorf("failed to generate random password: %w", err)
			}
			dbPassword = generatedPassword
			r.Log.Info("Generated random password for user", "postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)

			err = r.VaultClient.StorePostgresqlUserCredentials(ctx, user.Spec.PostgresqlID, user.Spec.Username, dbPassword)
			if err != nil {
				r.Log.Error(err, "Failed to store user credentials in Vault", "postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
				// Continue with reconciliation even if Vault storage fails
			} else {
				r.Log.Info("User credentials stored in Vault", "postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
			}
		} else {
			dbPassword = vaultUserPassword
			r.Log.Info("User credentials retrieved from Vault", "postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
		}
	} else {
		// Use password from spec if Vault is not available
		r.Log.Info("Vault client not available")
	}

	// Create or update user in PostgreSQL
	err = r.createOrUpdateUser(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
		user.Spec.Username, dbPassword)
	if err != nil {
		r.Log.Error(err, "Failed to create/update user in PostgreSQL", "username", user.Spec.Username)
		user.Status.Created = false
		updateUserCondition(user, "Ready", metav1.ConditionFalse, "CreateFailed",
			fmt.Sprintf("Failed to create user: %v", err))
	} else {
		user.Status.Created = true
		updateUserCondition(user, "Ready", metav1.ConditionTrue, "Created",
			fmt.Sprintf("User %s successfully created in PostgreSQL", user.Spec.Username))
	}

	now := metav1.Now()
	user.Status.LastSyncAttempt = &now

	// Update the status
	if err := r.Status().Update(ctx, user); err != nil {
		r.Log.Error(err, "Failed to update User status")
		return ctrl.Result{}, err
	}

	r.Log.Info("Successfully reconciled User", "username", user.Spec.Username)
	return ctrl.Result{}, nil
}

// findPostgresqlByID finds a PostgreSQL instance by its PostgresqlID
func (r *UserReconciler) findPostgresqlByID(ctx context.Context, postgresqlID string) (*instancev1alpha1.Postgresql, error) {
	postgresqlList := &instancev1alpha1.PostgresqlList{}
	if err := r.List(ctx, postgresqlList); err != nil {
		return nil, fmt.Errorf("failed to list PostgreSQL instances: %w", err)
	}

	for i := range postgresqlList.Items {
		if postgresqlList.Items[i].Spec.ExternalInstance != nil &&
			postgresqlList.Items[i].Spec.ExternalInstance.PostgresqlID == postgresqlID {
			return &postgresqlList.Items[i], nil
		}
	}

	return nil, fmt.Errorf("PostgreSQL instance with ID %s not found", postgresqlID)
}

// createOrUpdateUser creates or updates a PostgreSQL user
func (r *UserReconciler) createOrUpdateUser(ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, username, password string) error {
	// Connect to PostgreSQL as admin user
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, sslMode)

	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	// Escape username for SQL identifier (PostgreSQL uses double quotes)
	// Replace double quotes with two double quotes to escape them
	escapedUsername := fmt.Sprintf(`"%s"`, strings.ReplaceAll(username, `"`, `""`))

	// Escape password for SQL string literal (replace single quotes with two single quotes)
	escapedPassword := strings.ReplaceAll(password, `'`, `''`)

	// Check if user exists
	var exists bool
	err = db.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM pg_user WHERE usename = $1)", username).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}

	if exists {
		// Update user password - PostgreSQL doesn't support parameters in ALTER USER
		query := fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s'", escapedUsername, escapedPassword)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to update user password: %w", err)
		}
	} else {
		// Create new user - PostgreSQL doesn't support parameters in CREATE USER
		query := fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", escapedUsername, escapedPassword)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
	}

	return nil
}

// generateRandomPassword generates a secure random password of the specified length
// The password includes uppercase, lowercase, numbers, and special characters
func generateRandomPassword(length int) (string, error) {
	const (
		lowercase = "abcdefghijklmnopqrstuvwxyz"
		uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		numbers   = "0123456789"
		special   = "!@#$%^&*()_+-=[]{}|;:,.<>?"
	)
	charset := lowercase + uppercase + numbers + special

	password := make([]byte, length)
	for i := range password {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		password[i] = charset[num.Int64()]
	}

	return string(password), nil
}

// updateUserCondition updates or adds a condition to the User status
func updateUserCondition(user *instancev1alpha1.User, conditionType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		ObservedGeneration: user.Generation,
	}

	found := false
	for i, c := range user.Status.Conditions {
		if c.Type == conditionType {
			user.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		user.Status.Conditions = append(user.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ignoreStatusUpdates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj := e.ObjectOld.(*instancev1alpha1.User)
			newObj := e.ObjectNew.(*instancev1alpha1.User)

			if oldObj.Generation != newObj.Generation {
				return true
			}

			if oldObj.DeletionTimestamp != newObj.DeletionTimestamp {
				return true
			}

			return false
		},
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return true },
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1alpha1.User{}).
		Named("user").
		WithEventFilter(ignoreStatusUpdates).
		Complete(r)
}
