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
	"fmt"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/constants"
	controllerhelpers "github.com/vyrodovalexey/k8s-postgresql-operator/internal/controller/helpers"
	k8sclient "github.com/vyrodovalexey/k8s-postgresql-operator/internal/k8s"
	pg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// GrantReconciler reconciles a Grant object
type GrantReconciler struct {
	BaseReconcilerConfig
}

// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=grants,verbs=get;list;watch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=grants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=grants/finalizers,verbs=update
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=postgresqls,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *GrantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the Grant instance
	grant := &instancev1alpha1.Grant{}
	if err := r.Get(ctx, req.NamespacedName, grant); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Errorw("Failed to get Grant",
			"name", req.Name, "namespace", req.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := k8sclient.FindPostgresqlByID(ctx, r.Client, grant.Spec.PostgresqlID)
	if err != nil {
		r.Log.Errorw("Failed to find PostgreSQL instance",
			"name", grant.Name, "namespace", grant.Namespace,
			"postgresqlID", grant.Spec.PostgresqlID, "error", err)
		controllerhelpers.UpdateGrantCondition(grant, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonPostgresqlNotFound,
			fmt.Sprintf("PostgreSQL instance with ID %s not found: %v", grant.Spec.PostgresqlID, err))
		if err := r.Status().Update(ctx, grant); err != nil {
			r.Log.Errorw("Failed to update Grant status",
				"name", grant.Name, "namespace", grant.Namespace, "error", err)
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		r.Log.Infow("PostgreSQL instance is not connected, waiting",
			"name", grant.Name, "namespace", grant.Namespace, "postgresqlID", grant.Spec.PostgresqlID)
		controllerhelpers.UpdateGrantCondition(grant, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonPostgresqlNotConnected,
			fmt.Sprintf("PostgreSQL instance with ID %s is not connected", grant.Spec.PostgresqlID))
		if err := r.Status().Update(ctx, grant); err != nil {
			r.Log.Errorw("Failed to update Grant status",
				"name", grant.Name, "namespace", grant.Namespace, "error", err)
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Errorw("PostgreSQL instance has no external instance configuration",
			"name", grant.Name, "namespace", grant.Namespace, "postgresqlID", grant.Spec.PostgresqlID)
		controllerhelpers.UpdateGrantCondition(grant, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonInvalidConfiguration,
			"PostgreSQL instance has no external instance configuration")
		if err := r.Status().Update(ctx, grant); err != nil {
			r.Log.Errorw("Failed to update Grant status",
				"name", grant.Name, "namespace", grant.Namespace, "error", err)
		}
		return ctrl.Result{}, nil
	}

	// Get connection defaults
	port, sslMode := controllerhelpers.GetConnectionDefaults(externalInstance.Port, externalInstance.SSLMode)

	var postgresUsername, postgresPassword string
	if r.VaultClient == nil {
		r.Log.Infow("Vault client not available",
			"name", grant.Name, "namespace", grant.Namespace)
		controllerhelpers.UpdateGrantCondition(grant, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonVaultNotAvailable,
			"Vault client is not available")
		if err := r.Status().Update(ctx, grant); err != nil {
			r.Log.Errorw("Failed to update Grant status",
				"name", grant.Name, "namespace", grant.Namespace, "error", err)
		}
		return ctrl.Result{}, nil
	}

	vaultUsername, vaultPassword, err := getVaultCredentialsWithRetryAndRateLimit(
		ctx, r.VaultClient, externalInstance.PostgresqlID, r.Log,
		r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay, r.VaultRateLimiter)
	if err != nil {
		r.Log.Errorw("Failed to get credentials from Vault",
			"name", grant.Name, "namespace", grant.Namespace,
			"postgresqlID", externalInstance.PostgresqlID, "error", err)
		controllerhelpers.UpdateGrantCondition(grant, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonVaultError,
			fmt.Sprintf("Failed to get credentials from Vault: %v", err))
		if err := r.Status().Update(ctx, grant); err != nil {
			r.Log.Errorw("Failed to update Grant status",
				"name", grant.Name, "namespace", grant.Namespace, "error", err)
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}
	postgresUsername = vaultUsername
	postgresPassword = vaultPassword
	r.Log.Debugw("Credentials retrieved from Vault",
		"name", grant.Name, "namespace", grant.Namespace, "postgresqlID", externalInstance.PostgresqlID)

	// Apply rate limiting before PostgreSQL operation
	if r.PostgresqlRateLimiter != nil {
		if err := r.PostgresqlRateLimiter.Wait(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	// Apply grants in PostgreSQL with retry logic
	err = pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.ApplyGrants(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
			grant.Spec.Database, grant.Spec.Role, grant.Spec.Grants, grant.Spec.DefaultPrivileges)
	}, r.Log, r.PostgresqlConnectionRetries, r.PostgresqlConnectionTimeout, "applyGrants")
	if err != nil {
		r.Log.Errorw("Failed to apply grants in PostgreSQL",
			"name", grant.Name, "namespace", grant.Namespace,
			"database", grant.Spec.Database, "role", grant.Spec.Role, "operation", "applyGrants", "error", err)
		grant.Status.Applied = false
		controllerhelpers.UpdateGrantCondition(grant, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonApplyFailed,
			fmt.Sprintf("Failed to apply grants: %v", err))
		r.EventRecorder.RecordApplyFailed(grant, err.Error())
	} else {
		grant.Status.Applied = true
		controllerhelpers.UpdateGrantCondition(grant, constants.ConditionTypeReady,
			metav1.ConditionTrue, constants.ConditionReasonApplied,
			fmt.Sprintf("Grants successfully applied to role %s on database %s",
				grant.Spec.Role, grant.Spec.Database))
		r.EventRecorder.RecordApplied(grant, fmt.Sprintf("Grants applied to role %s on database %s",
			grant.Spec.Role, grant.Spec.Database))
		r.Log.Infow("Grants successfully applied in PostgreSQL",
			"name", grant.Name, "namespace", grant.Namespace,
			"PostgresqlID", grant.Spec.PostgresqlID, "database", grant.Spec.Database, "role", grant.Spec.Role)
	}

	now := metav1.Now()
	grant.Status.LastSyncAttempt = &now

	// Update the status
	if err := r.Status().Update(ctx, grant); err != nil {
		r.Log.Errorw("Failed to update Grant status",
			"name", grant.Name, "namespace", grant.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	r.Log.Infow("Successfully reconciled Grant",
		"name", grant.Name, "namespace", grant.Namespace, "database", grant.Spec.Database, "role", grant.Spec.Role)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1alpha1.Grant{}).
		Named("grant").
		WithEventFilter(controllerhelpers.IgnoreStatusUpdatesPredicate()).
		Complete(r)
}
