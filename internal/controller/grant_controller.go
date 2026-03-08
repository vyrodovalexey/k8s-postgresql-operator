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

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	controllerhelpers "github.com/vyrodovalexey/k8s-postgresql-operator/internal/controller/helpers"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/metrics"
	pg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
	corev1 "k8s.io/api/core/v1"
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
func (r *GrantReconciler) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	startTime := time.Now()
	metrics.IncReconcileTotal("grant")
	defer func() {
		metrics.ObserveReconcileDuration("grant", time.Since(startTime))
	}()

	grant := &instancev1alpha1.Grant{}
	if err := r.Get(ctx, req.NamespacedName, grant); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Errorw("Failed to get Grant", "error", err)
		metrics.IncReconcileErrors("grant")
		return ctrl.Result{}, err
	}

	connInfo, reason, msg := r.resolvePostgresqlConnection(
		ctx, grant.Spec.PostgresqlID,
	)
	if connInfo == nil {
		controllerhelpers.UpdateGrantCondition(
			grant, "Ready", metav1.ConditionFalse, reason, msg,
		)
		if err := r.Status().Update(ctx, grant); err != nil {
			r.Log.Errorw("Failed to update Grant status", "error", err)
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if err := r.resolveAdminCredentials(ctx, connInfo); err != nil {
		controllerhelpers.UpdateGrantCondition(
			grant, "Ready", metav1.ConditionFalse, "VaultError",
			fmt.Sprintf("Failed to get credentials from Vault: %v", err),
		)
		if err := r.Status().Update(ctx, grant); err != nil {
			r.Log.Errorw("Failed to update Grant status", "error", err)
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return r.syncGrants(ctx, grant, connInfo)
}

// syncGrants applies grants in PostgreSQL and updates status
// nolint:unparam // ctrl.Result is kept for Reconcile signature compatibility and future extensibility
func (r *GrantReconciler) syncGrants(
	ctx context.Context,
	grant *instancev1alpha1.Grant,
	info *connectionInfo,
) (ctrl.Result, error) {
	err := pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.ApplyGrants(
			ctx, info.ExternalInstance.Address, info.Port,
			info.Username, info.Password, info.SSLMode,
			grant.Spec.Database, grant.Spec.Role,
			grant.Spec.Grants, grant.Spec.DefaultPrivileges,
		)
	}, r.Log, r.PostgresqlConnectionRetries,
		r.PostgresqlConnectionTimeout, "applyGrants")

	if err != nil {
		r.Log.Errorw("Failed to apply grants in PostgreSQL",
			"error", err, "database", grant.Spec.Database, "role", grant.Spec.Role)
		grant.Status.Applied = false
		controllerhelpers.UpdateGrantCondition(
			grant, "Ready", metav1.ConditionFalse, "ApplyFailed",
			fmt.Sprintf("Failed to apply grants: %v", err))
		r.Recorder.Event(grant, corev1.EventTypeWarning,
			"ApplyFailed",
			fmt.Sprintf("Failed to apply grants for role %s on database %s: %v",
				grant.Spec.Role, grant.Spec.Database, err))
		metrics.IncReconcileErrors("grant")
	} else {
		grant.Status.Applied = true
		controllerhelpers.UpdateGrantCondition(
			grant, "Ready", metav1.ConditionTrue, "Applied",
			fmt.Sprintf(
				"Grants successfully applied to role %s on database %s",
				grant.Spec.Role, grant.Spec.Database))
		r.Log.Infow("Grants successfully applied in PostgreSQL",
			"PostgresqlID", grant.Spec.PostgresqlID,
			"database", grant.Spec.Database,
			"role", grant.Spec.Role)
		r.Recorder.Event(grant, corev1.EventTypeNormal,
			"Applied",
			fmt.Sprintf("Successfully applied grants for role %s on database %s",
				grant.Spec.Role, grant.Spec.Database))
	}

	now := metav1.Now()
	grant.Status.LastSyncAttempt = &now

	if err := r.Status().Update(ctx, grant); err != nil {
		r.Log.Errorw("Failed to update Grant status", "error", err)
		return ctrl.Result{}, err
	}

	r.Log.Infow("Successfully reconciled Grant",
		"database", grant.Spec.Database, "role", grant.Spec.Role)
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
