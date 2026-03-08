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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	controllerhelpers "github.com/vyrodovalexey/k8s-postgresql-operator/internal/controller/helpers"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/metrics"
	pg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
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
func (r *UserReconciler) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	ctx, span := otel.Tracer("controller").Start(ctx,
		"UserReconciler.Reconcile",
		trace.WithAttributes(
			attribute.String("controller", "user"),
			attribute.String("k8s.resource.name", req.Name),
			attribute.String("k8s.resource.namespace",
				req.Namespace),
		))
	defer span.End()

	startTime := time.Now()
	metrics.IncReconcileTotal("user")
	defer func() {
		metrics.ObserveReconcileDuration("user", time.Since(startTime))
	}()

	user := &instancev1alpha1.User{}
	if err := r.Get(ctx, req.NamespacedName, user); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Errorw("Failed to get User", "error", err)
		metrics.IncReconcileErrors("user")
		return ctrl.Result{}, err
	}

	connInfo, reason, msg := r.resolvePostgresqlConnection(
		ctx, user.Spec.PostgresqlID,
	)
	if connInfo == nil {
		controllerhelpers.UpdateUserCondition(
			user, "Ready", metav1.ConditionFalse, reason, msg,
		)
		if err := r.Status().Update(ctx, user); err != nil {
			r.Log.Errorw("Failed to update User status", "error", err)
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Resolve admin credentials (non-fatal if Vault unavailable)
	_ = r.resolveAdminCredentials(ctx, connInfo)

	dbPassword := r.resolveUserPassword(ctx, user)
	if dbPassword == "" && r.VaultClient != nil {
		return ctrl.Result{},
			fmt.Errorf("failed to handle user password from Vault")
	}

	return r.syncUser(ctx, user, connInfo, dbPassword)
}

// resolveUserPassword retrieves or generates the user password
func (r *UserReconciler) resolveUserPassword(
	ctx context.Context, user *instancev1alpha1.User,
) string {
	if r.VaultClient == nil {
		r.Log.Infow("Vault client not available")
		return ""
	}
	return r.handleVaultUserPassword(ctx, user)
}

// syncUser creates or updates the user in PostgreSQL
// nolint:unparam // ctrl.Result is kept for Reconcile signature compatibility and future extensibility
func (r *UserReconciler) syncUser(
	ctx context.Context,
	user *instancev1alpha1.User,
	info *connectionInfo,
	dbPassword string,
) (ctrl.Result, error) {
	err := pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.CreateOrUpdateUser(
			ctx, info.ExternalInstance.Address, info.Port,
			info.Username, info.Password, info.SSLMode,
			user.Spec.Username, dbPassword,
		)
	}, r.Log, r.PostgresqlConnectionRetries,
		r.PostgresqlConnectionTimeout, "createOrUpdateUser")

	if err != nil {
		r.Log.Errorw("Failed to create/update user in PostgreSQL",
			"error", err, "username", user.Spec.Username)
		user.Status.Created = false
		controllerhelpers.UpdateUserCondition(
			user, "Ready", metav1.ConditionFalse, "CreateFailed",
			fmt.Sprintf("Failed to create user: %v", err))
		r.Recorder.Event(user, corev1.EventTypeWarning,
			"CreateFailed",
			fmt.Sprintf("Failed to create user %s: %v",
				user.Spec.Username, err))
		metrics.IncReconcileErrors("user")
	} else {
		user.Status.Created = true
		controllerhelpers.UpdateUserCondition(
			user, "Ready", metav1.ConditionTrue, "Created",
			fmt.Sprintf("User %s successfully created in PostgreSQL",
				user.Spec.Username))
		r.Recorder.Event(user, corev1.EventTypeNormal,
			"Created",
			fmt.Sprintf("Successfully created user %s",
				user.Spec.Username))
		if dbPassword != "" {
			user.Status.LastAppliedPasswordHash = computePasswordHash(dbPassword)
		}
	}

	now := metav1.Now()
	user.Status.LastSyncAttempt = &now

	if err := r.Status().Update(ctx, user); err != nil {
		r.Log.Errorw("Failed to update User status", "error", err)
		return ctrl.Result{}, err
	}

	r.Log.Infow("Successfully reconciled User",
		"username", user.Spec.Username)
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

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1alpha1.User{}).
		Named("user").
		WithEventFilter(controllerhelpers.IgnoreStatusUpdatesPredicate()).
		Complete(r)
}

// computePasswordHash computes SHA-256 hash of a password
func computePasswordHash(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// handleVaultUserPassword handles password retrieval/generation from Vault
func (r *UserReconciler) handleVaultUserPassword(ctx context.Context, user *instancev1alpha1.User) string {
	vaultUserPassword, err := getVaultUserCredentialsWithRetry(
		ctx, r.VaultClient, user.Spec.PostgresqlID, user.Spec.Username, r.Log,
		r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay)
	credentialsExist := err == nil && vaultUserPassword != ""

	// If credentials exist in Vault and updatePassword is false, check for external changes
	if credentialsExist && !user.Spec.UpdatePassword {
		currentHash := computePasswordHash(vaultUserPassword)
		if currentHash == user.Status.LastAppliedPasswordHash {
			// Password unchanged - no PG update needed
			r.Log.Infow("User credentials retrieved from Vault, password unchanged",
				"postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
			return vaultUserPassword
		}
		// Password changed externally in Vault - PG will be updated in syncUser
		r.Log.Infow("External Vault password change detected for user",
			"postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
		return vaultUserPassword
	}

	// Generate new password if:
	// 1. Credentials don't exist in Vault (regardless of updatePassword option)
	// 2. Credentials exist and updatePassword is true
	shouldGeneratePassword := !credentialsExist || user.Spec.UpdatePassword

	if !shouldGeneratePassword {
		// Use existing password from Vault
		r.Log.Infow("User credentials retrieved from Vault",
			"postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
		return vaultUserPassword
	}

	// Generate new password
	if !credentialsExist {
		r.Log.Infow("User credentials not found in Vault, generating new password",
			"postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username, "error", err)
	} else if user.Spec.UpdatePassword {
		r.Log.Infow("updatePassword is true, regenerating password",
			"postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
	}

	// Generate random password
	generatedPassword, err := generateRandomPassword(32)
	if err != nil {
		r.Log.Errorw("Failed to generate random password",
			"error", err, "postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
		return ""
	}
	r.Log.Infow("Generated random password for user",
		"postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)

	// Store/update password in Vault
	err = storeVaultUserCredentialsWithRetry(
		ctx, r.VaultClient, user.Spec.PostgresqlID, user.Spec.Username, generatedPassword, r.Log,
		r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay)
	if err != nil {
		r.Log.Errorw("Failed to store user credentials in Vault",
			"error", err, "postgresqlID", user.Spec.PostgresqlID, "user", user.Spec.Username)
		// Continue with reconciliation even if Vault storage fails
		return generatedPassword
	}

	if credentialsExist {
		r.Log.Infow("User credentials updated in Vault",
			"postgresqlID", user.Spec.PostgresqlID,
			"user", user.Spec.Username)
	} else {
		r.Log.Infow("User credentials stored in Vault",
			"postgresqlID", user.Spec.PostgresqlID,
			"user", user.Spec.Username)
	}

	return generatedPassword
}
