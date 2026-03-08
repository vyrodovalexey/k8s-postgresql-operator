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

// SchemaReconciler reconciles a Schema object
type SchemaReconciler struct {
	BaseReconcilerConfig
}

// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=schemas,verbs=get;list;watch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=schemas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=schemas/finalizers,verbs=update
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=postgresqls,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *SchemaReconciler) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	ctx, span := otel.Tracer("controller").Start(ctx,
		"SchemaReconciler.Reconcile",
		trace.WithAttributes(
			attribute.String("controller", "schema"),
			attribute.String("k8s.resource.name", req.Name),
			attribute.String("k8s.resource.namespace",
				req.Namespace),
		))
	defer span.End()

	startTime := time.Now()
	metrics.IncReconcileTotal("schema")
	defer func() {
		metrics.ObserveReconcileDuration("schema", time.Since(startTime))
	}()

	schema := &instancev1alpha1.Schema{}
	if err := r.Get(ctx, req.NamespacedName, schema); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Errorw("Failed to get Schema", "error", err)
		metrics.IncReconcileErrors("schema")
		return ctrl.Result{}, err
	}

	connInfo, reason, msg := r.resolvePostgresqlConnection(
		ctx, schema.Spec.PostgresqlID,
	)
	if connInfo == nil {
		controllerhelpers.UpdateSchemaCondition(
			schema, "Ready", metav1.ConditionFalse, reason, msg,
		)
		if err := r.Status().Update(ctx, schema); err != nil {
			r.Log.Errorw("Failed to update Schema status", "error", err)
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Resolve admin credentials (non-fatal if Vault unavailable)
	_ = r.resolveAdminCredentials(ctx, connInfo)

	return r.syncSchema(ctx, schema, connInfo)
}

// syncSchema creates or updates the schema in PostgreSQL
// nolint:unparam // ctrl.Result is kept for Reconcile signature compatibility and future extensibility
func (r *SchemaReconciler) syncSchema(
	ctx context.Context,
	schema *instancev1alpha1.Schema,
	info *connectionInfo,
) (ctrl.Result, error) {
	err := pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.CreateOrUpdateSchema(
			ctx, info.ExternalInstance.Address, info.Port,
			info.Username, info.Password, info.SSLMode,
			pg.DefaultDB, schema.Spec.Schema, schema.Spec.Owner,
		)
	}, r.Log, r.PostgresqlConnectionRetries,
		r.PostgresqlConnectionTimeout, "createOrUpdateSchema")

	if err != nil {
		r.Log.Errorw("Failed to create/update schema in PostgreSQL",
			"error", err, "schema", schema.Spec.Schema)
		schema.Status.Created = false
		controllerhelpers.UpdateSchemaCondition(
			schema, "Ready", metav1.ConditionFalse, "CreateFailed",
			fmt.Sprintf("Failed to create schema: %v", err))
		r.Recorder.Event(schema, corev1.EventTypeWarning,
			"CreateFailed",
			fmt.Sprintf("Failed to create schema %s: %v",
				schema.Spec.Schema, err))
		metrics.IncReconcileErrors("schema")
	} else {
		schema.Status.Created = true
		controllerhelpers.UpdateSchemaCondition(
			schema, "Ready", metav1.ConditionTrue, "Created",
			fmt.Sprintf("Schema %s successfully created",
				schema.Spec.Schema))
		r.Log.Infow("Schema successfully created in PostgreSQL",
			"PostgresqlID", schema.Spec.PostgresqlID,
			"schema", schema.Spec.Schema)
		r.Recorder.Event(schema, corev1.EventTypeNormal,
			"Created",
			fmt.Sprintf("Successfully created schema %s",
				schema.Spec.Schema))
	}

	now := metav1.Now()
	schema.Status.LastSyncAttempt = &now

	if err := r.Status().Update(ctx, schema); err != nil {
		r.Log.Errorw("Failed to update Schema status", "error", err)
		return ctrl.Result{}, err
	}

	r.Log.Infow("Successfully reconciled Schema",
		"schema", schema.Spec.Schema)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchemaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1alpha1.Schema{}).
		Named("schema").
		WithEventFilter(controllerhelpers.IgnoreStatusUpdatesPredicate()).
		Complete(r)
}
