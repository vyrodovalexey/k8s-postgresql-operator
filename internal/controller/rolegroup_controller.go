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
func (r *RoleGroupReconciler) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	ctx, span := otel.Tracer("controller").Start(ctx,
		"RoleGroupReconciler.Reconcile",
		trace.WithAttributes(
			attribute.String("controller", "rolegroup"),
			attribute.String("k8s.resource.name", req.Name),
			attribute.String("k8s.resource.namespace",
				req.Namespace),
		))
	defer span.End()

	startTime := time.Now()
	metrics.IncReconcileTotal("rolegroup")
	defer func() {
		metrics.ObserveReconcileDuration("rolegroup", time.Since(startTime))
	}()

	roleGroup := &instancev1alpha1.RoleGroup{}
	if err := r.Get(ctx, req.NamespacedName, roleGroup); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Errorw("Failed to get RoleGroup", "error", err)
		metrics.IncReconcileErrors("rolegroup")
		return ctrl.Result{}, err
	}

	connInfo, reason, msg := r.resolvePostgresqlConnection(
		ctx, roleGroup.Spec.PostgresqlID,
	)
	if connInfo == nil {
		controllerhelpers.UpdateRoleGroupCondition(
			roleGroup, "Ready", metav1.ConditionFalse, reason, msg,
		)
		if err := r.Status().Update(ctx, roleGroup); err != nil {
			r.Log.Errorw("Failed to update RoleGroup status", "error", err)
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if err := r.resolveAdminCredentials(ctx, connInfo); err != nil {
		controllerhelpers.UpdateRoleGroupCondition(
			roleGroup, "Ready", metav1.ConditionFalse, "VaultError",
			fmt.Sprintf("Failed to get credentials from Vault: %v", err),
		)
		if err := r.Status().Update(ctx, roleGroup); err != nil {
			r.Log.Errorw("Failed to update RoleGroup status", "error", err)
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return r.syncRoleGroup(ctx, roleGroup, connInfo)
}

// syncRoleGroup creates or updates the role group in PostgreSQL
// nolint:unparam // ctrl.Result is kept for Reconcile signature compatibility and future extensibility
func (r *RoleGroupReconciler) syncRoleGroup(
	ctx context.Context,
	roleGroup *instancev1alpha1.RoleGroup,
	info *connectionInfo,
) (ctrl.Result, error) {
	err := pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.CreateOrUpdateRoleGroup(
			ctx, info.ExternalInstance.Address, info.Port,
			info.Username, info.Password, info.SSLMode,
			roleGroup.Spec.GroupRole, roleGroup.Spec.MemberRoles,
		)
	}, r.Log, r.PostgresqlConnectionRetries,
		r.PostgresqlConnectionTimeout, "createOrUpdateRoleGroup")

	if err != nil {
		r.Log.Errorw("Failed to create/update role group in PostgreSQL",
			"error", err, "groupRole", roleGroup.Spec.GroupRole)
		roleGroup.Status.Created = false
		controllerhelpers.UpdateRoleGroupCondition(
			roleGroup, "Ready", metav1.ConditionFalse, "CreateFailed",
			fmt.Sprintf("Failed to create role group: %v", err))
		r.Recorder.Event(roleGroup, corev1.EventTypeWarning,
			"CreateFailed",
			fmt.Sprintf("Failed to create role group %s: %v",
				roleGroup.Spec.GroupRole, err))
		metrics.IncReconcileErrors("rolegroup")
	} else {
		roleGroup.Status.Created = true
		controllerhelpers.UpdateRoleGroupCondition(
			roleGroup, "Ready", metav1.ConditionTrue, "Created",
			fmt.Sprintf("Role group %s successfully created in PostgreSQL",
				roleGroup.Spec.GroupRole))
		r.Log.Infow("Role group successfully created in PostgreSQL",
			"PostgresqlID", roleGroup.Spec.PostgresqlID,
			"groupRole", roleGroup.Spec.GroupRole)
		r.Recorder.Event(roleGroup, corev1.EventTypeNormal,
			"Created",
			fmt.Sprintf("Successfully created role group %s",
				roleGroup.Spec.GroupRole))
	}

	now := metav1.Now()
	roleGroup.Status.LastSyncAttempt = &now

	if err := r.Status().Update(ctx, roleGroup); err != nil {
		r.Log.Errorw("Failed to update RoleGroup status", "error", err)
		return ctrl.Result{}, err
	}

	r.Log.Infow("Successfully reconciled RoleGroup",
		"groupRole", roleGroup.Spec.GroupRole)
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
