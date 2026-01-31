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

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/constants"
	controllerhelpers "github.com/vyrodovalexey/k8s-postgresql-operator/internal/controller/helpers"
	k8sclient "github.com/vyrodovalexey/k8s-postgresql-operator/internal/k8s"
	pg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	userFinalizerName = "user.postgresql-operator.vyrodovalexey.github.com/finalizer"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	BaseReconcilerConfig
}

// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=users,verbs=get;list;watch;update
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
		r.Log.Errorw("Failed to get User",
			"name", req.Name, "namespace", req.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if user.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, user)
	}

	// Add finalizer if DeleteFromCRD is true
	if user.Spec.DeleteFromCRD {
		if !controllerhelpers.ContainsString(user.Finalizers, userFinalizerName) {
			user.Finalizers = append(user.Finalizers, userFinalizerName)
			if err := r.Update(ctx, user); err != nil {
				r.Log.Errorw("Failed to add finalizer",
					"name", user.Name, "namespace", user.Namespace, "error", err)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := k8sclient.FindPostgresqlByID(ctx, r.Client, user.Spec.PostgresqlID)
	if err != nil {
		r.Log.Errorw("Failed to find PostgreSQL instance",
			"name", user.Name, "namespace", user.Namespace,
			"postgresqlID", user.Spec.PostgresqlID, "error", err)
		controllerhelpers.UpdateUserCondition(user, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonPostgresqlNotFound,
			fmt.Sprintf("PostgreSQL instance with ID %s not found: %v", user.Spec.PostgresqlID, err))
		if err := r.Status().Update(ctx, user); err != nil {
			r.Log.Errorw("Failed to update User status",
				"name", user.Name, "namespace", user.Namespace, "error", err)
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		r.Log.Infow("PostgreSQL instance is not connected, waiting",
			"name", user.Name, "namespace", user.Namespace, "postgresqlID", user.Spec.PostgresqlID)
		controllerhelpers.UpdateUserCondition(user, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonPostgresqlNotConnected,
			fmt.Sprintf("PostgreSQL instance with ID %s is not connected", user.Spec.PostgresqlID))
		if err := r.Status().Update(ctx, user); err != nil {
			r.Log.Errorw("Failed to update User status",
				"name", user.Name, "namespace", user.Namespace, "error", err)
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Errorw("PostgreSQL instance has no external instance configuration",
			"name", user.Name, "namespace", user.Namespace, "postgresqlID", user.Spec.PostgresqlID)
		controllerhelpers.UpdateUserCondition(user, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonInvalidConfiguration,
			"PostgreSQL instance has no external instance configuration")
		if err := r.Status().Update(ctx, user); err != nil {
			r.Log.Errorw("Failed to update User status",
				"name", user.Name, "namespace", user.Namespace, "error", err)
		}
		return ctrl.Result{}, nil
	}

	// Get connection defaults
	port, sslMode := controllerhelpers.GetConnectionDefaults(externalInstance.Port, externalInstance.SSLMode)

	var postgresUsername, postgresPassword, dbPassword string

	// Get admin credentials from Vault if available
	if r.VaultClient != nil {
		vaultUsername, vaultPassword, err := getVaultCredentialsWithRetryAndRateLimit(
			ctx, r.VaultClient, user.Spec.PostgresqlID, r.Log,
			r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay, r.VaultRateLimiter)
		if err != nil {
			r.Log.Errorw("Failed to get credentials from Vault",
				"name", user.Name, "namespace", user.Namespace,
				"postgresqlID", user.Spec.PostgresqlID, "error", err)
		} else {
			postgresUsername = vaultUsername
			postgresPassword = vaultPassword
			r.Log.Debugw("Credentials retrieved from Vault by user controller",
				"name", user.Name, "namespace", user.Namespace, "postgresqlID", user.Spec.PostgresqlID)
		}
	} else {
		r.Log.Infow("Vault client not available",
			"name", user.Name, "namespace", user.Namespace)
	}

	// Get user password from Vault or generate/store it
	if r.VaultClient == nil {
		// Use password from spec if Vault is not available
		r.Log.Infow("Vault client not available")
	} else {
		dbPassword = r.handleVaultUserPassword(ctx, user)
		if dbPassword == "" {
			return ctrl.Result{}, fmt.Errorf("failed to handle user password from Vault")
		}
	}

	// Apply rate limiting before PostgreSQL operation
	if r.PostgresqlRateLimiter != nil {
		if err := r.PostgresqlRateLimiter.Wait(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	// Create or update user in PostgreSQL with retry logic
	err = pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.CreateOrUpdateUser(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
			user.Spec.Username, dbPassword)
	}, r.Log, r.PostgresqlConnectionRetries, r.PostgresqlConnectionTimeout, "createOrUpdateUser")
	if err != nil {
		r.Log.Errorw("Failed to create/update user in PostgreSQL",
			"name", user.Name, "namespace", user.Namespace,
			"operation", "createOrUpdateUser", "error", err)
		user.Status.Created = false
		controllerhelpers.UpdateUserCondition(user, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonCreateFailed,
			fmt.Sprintf("Failed to create user: %v", err))
		r.EventRecorder.RecordCreateFailed(user, "User", user.Spec.Username, err.Error())
	} else {
		user.Status.Created = true
		controllerhelpers.UpdateUserCondition(user, constants.ConditionTypeReady,
			metav1.ConditionTrue, constants.ConditionReasonCreated,
			fmt.Sprintf("User %s successfully created in PostgreSQL", user.Spec.Username))
		r.EventRecorder.RecordCreated(user, "User", user.Spec.Username)
	}

	now := metav1.Now()
	user.Status.LastSyncAttempt = &now

	// Update the status
	if err := r.Status().Update(ctx, user); err != nil {
		r.Log.Errorw("Failed to update User status",
			"name", user.Name, "namespace", user.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	r.Log.Infow("Successfully reconciled User",
		"name", user.Name, "namespace", user.Namespace)
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

// handleVaultUserPassword handles password retrieval/generation from Vault
func (r *UserReconciler) handleVaultUserPassword(ctx context.Context, user *instancev1alpha1.User) string {
	vaultUserPassword, err := getVaultUserCredentialsWithRetryAndRateLimit(
		ctx, r.VaultClient, user.Spec.PostgresqlID, user.Spec.Username, r.Log,
		r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay, r.VaultRateLimiter)
	credentialsExist := err == nil && vaultUserPassword != ""

	// Generate new password if:
	// 1. Credentials don't exist in Vault (regardless of updatePassword option)
	// 2. Credentials exist and updatePassword is true
	shouldGeneratePassword := !credentialsExist || user.Spec.UpdatePassword

	if !shouldGeneratePassword {
		// Use existing password from Vault
		r.Log.Debugw("User credentials retrieved from Vault",
			"postgresqlID", user.Spec.PostgresqlID)
		return vaultUserPassword
	}

	// Generate new password
	if !credentialsExist {
		r.Log.Debugw("User credentials not found in Vault, generating new password",
			"postgresqlID", user.Spec.PostgresqlID, "error", err)
	} else if user.Spec.UpdatePassword {
		r.Log.Debugw("updatePassword is true, regenerating password",
			"postgresqlID", user.Spec.PostgresqlID)
	}

	// Generate random password
	generatedPassword, err := generateRandomPassword(constants.DefaultPasswordLength)
	if err != nil {
		r.Log.Errorw("Failed to generate random password",
			"postgresqlID", user.Spec.PostgresqlID, "error", err)
		return ""
	}
	r.Log.Debugw("Generated random password for user", "postgresqlID", user.Spec.PostgresqlID)

	// Store/update password in Vault
	err = storeVaultUserCredentialsWithRetryAndRateLimit(
		ctx, r.VaultClient, user.Spec.PostgresqlID, user.Spec.Username, generatedPassword, r.Log,
		r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay, r.VaultRateLimiter)
	if err != nil {
		r.Log.Errorw("Failed to store user credentials in Vault",
			"postgresqlID", user.Spec.PostgresqlID, "error", err)
		// Continue with reconciliation even if Vault storage fails
		return generatedPassword
	}

	if credentialsExist {
		r.Log.Debugw("User credentials updated in Vault", "postgresqlID", user.Spec.PostgresqlID)
	} else {
		r.Log.Debugw("User credentials stored in Vault", "postgresqlID", user.Spec.PostgresqlID)
	}

	return generatedPassword
}

// handleDeletion handles the deletion of a User resource
func (r *UserReconciler) handleDeletion(ctx context.Context, user *instancev1alpha1.User) (ctrl.Result, error) {
	// Only delete from PostgreSQL if DeleteFromCRD is true
	if !user.Spec.DeleteFromCRD {
		// Remove finalizer if it exists
		if controllerhelpers.ContainsString(user.Finalizers, userFinalizerName) {
			user.Finalizers = controllerhelpers.RemoveString(user.Finalizers, userFinalizerName)
			if err := r.Update(ctx, user); err != nil {
				r.Log.Errorw("Failed to remove finalizer",
					"name", user.Name, "namespace", user.Namespace, "error", err)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Check if finalizer is present
	if !controllerhelpers.ContainsString(user.Finalizers, userFinalizerName) {
		// Finalizer already removed, nothing to do
		return ctrl.Result{}, nil
	}

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := k8sclient.FindPostgresqlByID(ctx, r.Client, user.Spec.PostgresqlID)
	if err != nil {
		r.Log.Warnw("Failed to find PostgreSQL instance during deletion, proceeding with finalizer removal",
			"name", user.Name, "namespace", user.Namespace,
			"postgresqlID", user.Spec.PostgresqlID, "error", err)
		// Remove finalizer even if PostgreSQL not found
		user.Finalizers = controllerhelpers.RemoveString(user.Finalizers, userFinalizerName)
		if err := r.Update(ctx, user); err != nil {
			r.Log.Errorw("Failed to remove finalizer",
				"name", user.Name, "namespace", user.Namespace, "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		r.Log.Warnw("PostgreSQL instance is not connected during deletion, proceeding with finalizer removal",
			"name", user.Name, "namespace", user.Namespace, "postgresqlID", user.Spec.PostgresqlID)
		// Remove finalizer even if PostgreSQL not connected
		user.Finalizers = controllerhelpers.RemoveString(user.Finalizers, userFinalizerName)
		if err := r.Update(ctx, user); err != nil {
			r.Log.Errorw("Failed to remove finalizer",
				"name", user.Name, "namespace", user.Namespace, "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Warnw(
			"PostgreSQL instance has no external instance configuration during deletion, proceeding with finalizer removal",
			"name", user.Name, "namespace", user.Namespace, "postgresqlID", user.Spec.PostgresqlID)
		// Remove finalizer
		user.Finalizers = controllerhelpers.RemoveString(user.Finalizers, userFinalizerName)
		if err := r.Update(ctx, user); err != nil {
			r.Log.Errorw("Failed to remove finalizer",
				"name", user.Name, "namespace", user.Namespace, "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Get connection defaults
	port, sslMode := controllerhelpers.GetConnectionDefaults(externalInstance.Port, externalInstance.SSLMode)

	var postgresUsername, postgresPassword string
	if r.VaultClient == nil {
		r.Log.Warnw("Vault client not available during deletion, cannot delete user from PostgreSQL",
			"name", user.Name, "namespace", user.Namespace)
		// Remove finalizer even if Vault not available
		user.Finalizers = controllerhelpers.RemoveString(user.Finalizers, userFinalizerName)
		if err := r.Update(ctx, user); err != nil {
			r.Log.Errorw("Failed to remove finalizer",
				"name", user.Name, "namespace", user.Namespace, "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	vaultUsername, vaultPassword, err := getVaultCredentialsWithRetryAndRateLimit(
		ctx, r.VaultClient, externalInstance.PostgresqlID, r.Log,
		r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay, r.VaultRateLimiter)
	if err != nil {
		r.Log.Errorw("Failed to get credentials from Vault during deletion",
			"name", user.Name, "namespace", user.Namespace,
			"postgresqlID", externalInstance.PostgresqlID, "error", err)
		// Remove finalizer even if credentials not available
		user.Finalizers = controllerhelpers.RemoveString(user.Finalizers, userFinalizerName)
		if err := r.Update(ctx, user); err != nil {
			r.Log.Errorw("Failed to remove finalizer",
				"name", user.Name, "namespace", user.Namespace, "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	postgresUsername = vaultUsername
	postgresPassword = vaultPassword

	// Apply rate limiting before PostgreSQL operation
	if r.PostgresqlRateLimiter != nil {
		if err := r.PostgresqlRateLimiter.Wait(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	// Delete user from PostgreSQL with retry logic
	err = pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.DeleteUser(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
			user.Spec.Username)
	}, r.Log, r.PostgresqlConnectionRetries, r.PostgresqlConnectionTimeout, "deleteUser")
	if err != nil {
		r.Log.Errorw("Failed to delete user from PostgreSQL",
			"name", user.Name, "namespace", user.Namespace,
			"operation", "deleteUser", "error", err)
		controllerhelpers.UpdateUserCondition(user, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonDeleteFailed,
			fmt.Sprintf("Failed to delete user: %v", err))
		r.EventRecorder.RecordDeleteFailed(user, "User", user.Spec.Username, err.Error())
		if err := r.Status().Update(ctx, user); err != nil {
			r.Log.Errorw("Failed to update User status",
				"name", user.Name, "namespace", user.Namespace, "error", err)
		}
		// Requeue to retry deletion
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}

	r.EventRecorder.RecordDeleted(user, "User", user.Spec.Username)
	r.Log.Infow("User successfully deleted from PostgreSQL",
		"name", user.Name, "namespace", user.Namespace,
		"PostgresqlID", user.Spec.PostgresqlID)

	// Remove finalizer
	user.Finalizers = controllerhelpers.RemoveString(user.Finalizers, userFinalizerName)
	if err := r.Update(ctx, user); err != nil {
		r.Log.Errorw("Failed to remove finalizer",
			"name", user.Name, "namespace", user.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
