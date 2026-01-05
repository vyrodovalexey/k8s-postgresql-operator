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
	"fmt"
	"math/big"
	"time"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	k8sclient "github.com/vyrodovalexey/k8s-postgresql-operator/internal/k8s"
	pg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	BaseReconcilerConfig
}

// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=users,verbs=get;list;watch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=users/finalizers,verbs=update
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=postgresqls,verbs=get;list;watch

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
	postgresql, err := k8sclient.FindPostgresqlByID(ctx, r.Client, user.Spec.PostgresqlID)
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
		sslMode = pg.DefaultSSLMode
	}

	var postgresUsername, postgresPassword, dbPassword string

	// Get admin credentials from Vault if available
	if r.VaultClient != nil {
		vaultUsername, vaultPassword, err := getVaultCredentialsWithRetry(
			ctx, r.VaultClient, user.Spec.PostgresqlID, r.Log,
			r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay)
		if err != nil {
			r.Log.Error(err, "Failed to get credentials from Vault", "postgresqlID", user.Spec.PostgresqlID)
		} else {
			postgresUsername = vaultUsername
			postgresPassword = vaultPassword
			r.Log.Info("Credentials retrieved from Vault by user controller, ", "postgresqlID: ", user.Spec.PostgresqlID)
		}
	} else {
		r.Log.Info("Vault client not available")
	}

	// Get user password from Vault or generate/store it
	if r.VaultClient == nil {
		// Use password from spec if Vault is not available
		r.Log.Info("Vault client not available")
	} else {
		dbPassword = r.handleVaultUserPassword(ctx, user)
		if dbPassword == "" {
			return ctrl.Result{}, fmt.Errorf("failed to handle user password from Vault")
		}
	}

	// Create or update user in PostgreSQL with retry logic
	err = pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.CreateOrUpdateUser(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
			user.Spec.Username, dbPassword)
	}, r.Log, r.PostgresqlConnectionRetries, r.PostgresqlConnectionTimeout, "createOrUpdateUser")
	if err != nil {
		r.Log.Error(err, "Failed to create/update user in PostgreSQL, ", "username: ", user.Spec.Username)
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

	r.Log.Info("Successfully reconciled User with ", "username: ", user.Spec.Username)
	return ctrl.Result{}, nil
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

// updateUserCondition updates or adds a condition to the User status using meta.SetStatusCondition
// nolint:unparam // conditionType parameter is kept for API consistency and future extensibility
func updateUserCondition(
	user *instancev1alpha1.User, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: user.Generation,
	}
	meta.SetStatusCondition(&user.Status.Conditions, condition)
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

// handleVaultUserPassword handles password retrieval/generation from Vault
func (r *UserReconciler) handleVaultUserPassword(ctx context.Context, user *instancev1alpha1.User) string {
	vaultUserPassword, err := getVaultUserCredentialsWithRetry(
		ctx, r.VaultClient, user.Spec.PostgresqlID, user.Spec.Username, r.Log,
		r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay)
	credentialsExist := err == nil && vaultUserPassword != ""

	// Generate new password if:
	// 1. Credentials don't exist in Vault (regardless of updatePassword option)
	// 2. Credentials exist and updatePassword is true
	shouldGeneratePassword := !credentialsExist || user.Spec.UpdatePassword

	if !shouldGeneratePassword {
		// Use existing password from Vault
		r.Log.Info("User credentials retrieved from Vault",
			"postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
		return vaultUserPassword
	}

	// Generate new password
	if !credentialsExist {
		r.Log.Info("User credentials not found in Vault, generating new password",
			"postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username, "error", err)
	} else if user.Spec.UpdatePassword {
		r.Log.Info("updatePassword is true, regenerating password",
			"postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
	}

	// Generate random password
	generatedPassword, err := generateRandomPassword(32)
	if err != nil {
		r.Log.Error(err, "Failed to generate random password",
			"postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
		return ""
	}
	r.Log.Info("Generated random password for user", "postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)

	// Store/update password in Vault
	err = storeVaultUserCredentialsWithRetry(
		ctx, r.VaultClient, user.Spec.PostgresqlID, user.Spec.Username, generatedPassword, r.Log,
		r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay)
	if err != nil {
		r.Log.Error(err, "Failed to store user credentials in Vault",
			"postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
		// Continue with reconciliation even if Vault storage fails
		return generatedPassword
	}

	if credentialsExist {
		r.Log.Info("User credentials updated in Vault", "postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
	} else {
		r.Log.Info("User credentials stored in Vault", "postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
	}

	return generatedPassword
}
