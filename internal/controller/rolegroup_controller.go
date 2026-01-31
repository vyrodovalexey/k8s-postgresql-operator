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

// RoleGroupReconciler reconciles a RoleGroup object
type RoleGroupReconciler struct {
	BaseReconcilerConfig
}

// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=rolegroups,verbs=get;list;watch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=rolegroups/status,verbs=get;update;patch
//nolint:lll // kubebuilder directive cannot be split
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=rolegroups/finalizers,verbs=update
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=postgresqls,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *RoleGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the RoleGroup instance
	roleGroup := &instancev1alpha1.RoleGroup{}
	if err := r.Get(ctx, req.NamespacedName, roleGroup); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Errorw("Failed to get RoleGroup",
			"name", req.Name, "namespace", req.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := k8sclient.FindPostgresqlByID(ctx, r.Client, roleGroup.Spec.PostgresqlID)
	if err != nil {
		r.Log.Errorw("Failed to find PostgreSQL instance",
			"name", roleGroup.Name, "namespace", roleGroup.Namespace,
			"postgresqlID", roleGroup.Spec.PostgresqlID, "error", err)
		controllerhelpers.UpdateRoleGroupCondition(roleGroup, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonPostgresqlNotFound,
			fmt.Sprintf("PostgreSQL instance with ID %s not found: %v", roleGroup.Spec.PostgresqlID, err))
		if err := r.Status().Update(ctx, roleGroup); err != nil {
			r.Log.Errorw("Failed to update RoleGroup status",
				"name", roleGroup.Name, "namespace", roleGroup.Namespace, "error", err)
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		r.Log.Infow("PostgreSQL instance is not connected, waiting",
			"name", roleGroup.Name, "namespace", roleGroup.Namespace, "postgresqlID", roleGroup.Spec.PostgresqlID)
		controllerhelpers.UpdateRoleGroupCondition(roleGroup, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonPostgresqlNotConnected,
			fmt.Sprintf("PostgreSQL instance with ID %s is not connected", roleGroup.Spec.PostgresqlID))
		if err := r.Status().Update(ctx, roleGroup); err != nil {
			r.Log.Errorw("Failed to update RoleGroup status",
				"name", roleGroup.Name, "namespace", roleGroup.Namespace, "error", err)
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Errorw("PostgreSQL instance has no external instance configuration",
			"name", roleGroup.Name, "namespace", roleGroup.Namespace, "postgresqlID", roleGroup.Spec.PostgresqlID)
		controllerhelpers.UpdateRoleGroupCondition(roleGroup, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonInvalidConfiguration,
			"PostgreSQL instance has no external instance configuration")
		if err := r.Status().Update(ctx, roleGroup); err != nil {
			r.Log.Errorw("Failed to update RoleGroup status",
				"name", roleGroup.Name, "namespace", roleGroup.Namespace, "error", err)
		}
		return ctrl.Result{}, nil
	}

	// Get connection defaults
	port, sslMode := controllerhelpers.GetConnectionDefaults(externalInstance.Port, externalInstance.SSLMode)

	var postgresUsername, postgresPassword string
	if r.VaultClient == nil {
		r.Log.Infow("Vault client not available",
			"name", roleGroup.Name, "namespace", roleGroup.Namespace)
		controllerhelpers.UpdateRoleGroupCondition(roleGroup, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonVaultNotAvailable,
			"Vault client is not available")
		if err := r.Status().Update(ctx, roleGroup); err != nil {
			r.Log.Errorw("Failed to update RoleGroup status",
				"name", roleGroup.Name, "namespace", roleGroup.Namespace, "error", err)
		}
		return ctrl.Result{}, nil
	}

	vaultUsername, vaultPassword, err := getVaultCredentialsWithRetryAndRateLimit(
		ctx, r.VaultClient, externalInstance.PostgresqlID, r.Log,
		r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay, r.VaultRateLimiter)
	if err != nil {
		r.Log.Errorw("Failed to get credentials from Vault",
			"name", roleGroup.Name, "namespace", roleGroup.Namespace,
			"postgresqlID", externalInstance.PostgresqlID, "error", err)
		controllerhelpers.UpdateRoleGroupCondition(roleGroup, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonVaultError,
			fmt.Sprintf("Failed to get credentials from Vault: %v", err))
		if err := r.Status().Update(ctx, roleGroup); err != nil {
			r.Log.Errorw("Failed to update RoleGroup status",
				"name", roleGroup.Name, "namespace", roleGroup.Namespace, "error", err)
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}
	postgresUsername = vaultUsername
	postgresPassword = vaultPassword
	r.Log.Debugw("Credentials retrieved from Vault",
		"name", roleGroup.Name, "namespace", roleGroup.Namespace, "postgresqlID", externalInstance.PostgresqlID)

	// Apply rate limiting before PostgreSQL operation
	if r.PostgresqlRateLimiter != nil {
		if err := r.PostgresqlRateLimiter.Wait(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	// Create or update role group in PostgreSQL
	err = pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.CreateOrUpdateRoleGroup(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
			roleGroup.Spec.GroupRole, roleGroup.Spec.MemberRoles)
	}, r.Log, r.PostgresqlConnectionRetries, r.PostgresqlConnectionTimeout, "createOrUpdateRoleGroup")
	if err != nil {
		r.Log.Errorw("Failed to create/update role group in PostgreSQL",
			"name", roleGroup.Name, "namespace", roleGroup.Namespace,
			"groupRole", roleGroup.Spec.GroupRole, "operation", "createOrUpdateRoleGroup", "error", err)
		roleGroup.Status.Created = false
		controllerhelpers.UpdateRoleGroupCondition(roleGroup, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonCreateFailed,
			fmt.Sprintf("Failed to create role group: %v", err))
		r.EventRecorder.RecordCreateFailed(roleGroup, "RoleGroup", roleGroup.Spec.GroupRole, err.Error())
	} else {
		roleGroup.Status.Created = true
		controllerhelpers.UpdateRoleGroupCondition(roleGroup, constants.ConditionTypeReady,
			metav1.ConditionTrue, constants.ConditionReasonCreated,
			fmt.Sprintf("Role group %s successfully created in PostgreSQL", roleGroup.Spec.GroupRole))
		r.EventRecorder.RecordCreated(roleGroup, "RoleGroup", roleGroup.Spec.GroupRole)
		r.Log.Infow("Role group successfully created in PostgreSQL",
			"name", roleGroup.Name, "namespace", roleGroup.Namespace,
			"PostgresqlID", roleGroup.Spec.PostgresqlID, "groupRole", roleGroup.Spec.GroupRole)
	}

	now := metav1.Now()
	roleGroup.Status.LastSyncAttempt = &now

	// Update the status
	if err := r.Status().Update(ctx, roleGroup); err != nil {
		r.Log.Errorw("Failed to update RoleGroup status",
			"name", roleGroup.Name, "namespace", roleGroup.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	r.Log.Infow("Successfully reconciled RoleGroup",
		"name", roleGroup.Name, "namespace", roleGroup.Namespace, "groupRole", roleGroup.Spec.GroupRole)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RoleGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1alpha1.RoleGroup{}).
		Named("rolegroup").
		WithEventFilter(controllerhelpers.IgnoreStatusUpdatesPredicate()).
		Complete(r)
}
