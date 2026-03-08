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

const (
	databaseFinalizerName = "database.postgresql-operator.vyrodovalexey.github.com/finalizer"
)

// DatabaseReconciler reconciles a Database object
type DatabaseReconciler struct {
	BaseReconcilerConfig
}

// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=databases,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=databases/status,verbs=get;update;patch
//nolint:lll // kubebuilder directive cannot be split
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=databases/finalizers,verbs=update
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=postgresqls,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *DatabaseReconciler) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	ctx, span := otel.Tracer("controller").Start(ctx,
		"DatabaseReconciler.Reconcile",
		trace.WithAttributes(
			attribute.String("controller", "database"),
			attribute.String("k8s.resource.name", req.Name),
			attribute.String("k8s.resource.namespace",
				req.Namespace),
		))
	defer span.End()

	startTime := time.Now()
	metrics.IncReconcileTotal("database")
	defer func() {
		metrics.ObserveReconcileDuration("database", time.Since(startTime))
	}()

	database := &instancev1alpha1.Database{}
	if err := r.Get(ctx, req.NamespacedName, database); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Errorw("Failed to get Database", "error", err)
		metrics.IncReconcileErrors("database")
		return ctrl.Result{}, err
	}

	if database.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, database)
	}

	if result, done := r.ensureFinalizer(ctx, database); done {
		return result, nil
	}

	connInfo, reason, msg := r.resolvePostgresqlConnection(
		ctx, database.Spec.PostgresqlID,
	)
	if connInfo == nil {
		controllerhelpers.UpdateDatabaseCondition(
			database, "Ready", metav1.ConditionFalse, reason, msg,
		)
		if err := r.Status().Update(ctx, database); err != nil {
			r.Log.Errorw("Failed to update Database status", "error", err)
		}
		r.Recorder.Event(database, corev1.EventTypeWarning,
			"ConnectionFailed", msg)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Resolve admin credentials (non-fatal if Vault unavailable)
	_ = r.resolveAdminCredentials(ctx, connInfo)

	return ctrl.Result{}, r.syncDatabase(ctx, database, connInfo)
}

// ensureFinalizer adds the finalizer if DeleteFromCRD is true.
// Returns (result, true) if the caller should return early.
func (r *DatabaseReconciler) ensureFinalizer(
	ctx context.Context, database *instancev1alpha1.Database,
) (ctrl.Result, bool) {
	if !database.Spec.DeleteFromCRD {
		return ctrl.Result{}, false
	}
	if controllerhelpers.ContainsString(
		database.Finalizers, databaseFinalizerName,
	) {
		return ctrl.Result{}, false
	}
	database.Finalizers = append(
		database.Finalizers, databaseFinalizerName,
	)
	if err := r.Update(ctx, database); err != nil {
		r.Log.Errorw("Failed to add finalizer", "error", err)
		return ctrl.Result{}, false
	}
	return ctrl.Result{Requeue: true}, true
}

// syncDatabase creates or updates the database in PostgreSQL
func (r *DatabaseReconciler) syncDatabase(
	ctx context.Context,
	database *instancev1alpha1.Database,
	info *connectionInfo,
) error {
	schemaName := database.Spec.Schema
	if schemaName == "" {
		schemaName = "public"
	}

	err := pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.CreateOrUpdateDatabase(
			ctx, info.ExternalInstance.Address, info.Port,
			info.Username, info.Password, info.SSLMode,
			database.Spec.Database, database.Spec.Owner,
			schemaName, database.Spec.DBTemplate,
		)
	}, r.Log, r.PostgresqlConnectionRetries,
		r.PostgresqlConnectionTimeout, "createOrUpdateDatabase")

	if err != nil {
		r.Log.Errorw("Failed to create/update database in PostgreSQL",
			"error", err, "database", database.Spec.Database)
		database.Status.Created = false
		controllerhelpers.UpdateDatabaseCondition(
			database, "Ready", metav1.ConditionFalse,
			"CreateFailed",
			fmt.Sprintf("Failed to create database: %v", err))
		r.Recorder.Event(database, corev1.EventTypeWarning,
			"CreateFailed",
			fmt.Sprintf("Failed to create database %s: %v",
				database.Spec.Database, err))
		metrics.IncReconcileErrors("database")
	} else {
		database.Status.Created = true
		controllerhelpers.UpdateDatabaseCondition(
			database, "Ready", metav1.ConditionTrue,
			"Created",
			fmt.Sprintf("Database %s successfully created in PostgreSQL",
				database.Spec.Database))
		r.Log.Infow("Database successfully created in PostgreSQL",
			"PostgresqlID", database.Spec.PostgresqlID,
			"database", database.Spec.Database, "schema", schemaName)
		r.Recorder.Event(database, corev1.EventTypeNormal,
			"Created",
			fmt.Sprintf("Successfully created database %s",
				database.Spec.Database))
	}

	now := metav1.Now()
	database.Status.LastSyncAttempt = &now

	if err := r.Status().Update(ctx, database); err != nil {
		r.Log.Errorw("Failed to update Database status", "error", err)
		return err
	}

	r.Log.Infow("Successfully reconciled Database",
		"database", database.Spec.Database)
	return nil
}

// removeDBFinalizer removes the database finalizer and updates the resource
func (r *DatabaseReconciler) removeDBFinalizer(
	ctx context.Context, database *instancev1alpha1.Database,
) (ctrl.Result, error) {
	database.Finalizers = controllerhelpers.RemoveString(
		database.Finalizers, databaseFinalizerName,
	)
	if err := r.Update(ctx, database); err != nil {
		r.Log.Errorw("Failed to remove finalizer", "error", err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// resolveDeletionConnection resolves connection info for deletion.
// On failure, it removes the finalizer and returns true to signal
// the caller should return the provided result.
func (r *DatabaseReconciler) resolveDeletionConnection(
	ctx context.Context, database *instancev1alpha1.Database,
) (*connectionInfo, *ctrl.Result, error) {
	connInfo, _, _ := r.resolvePostgresqlConnection(
		ctx, database.Spec.PostgresqlID,
	)
	if connInfo == nil {
		r.Log.Warnw(
			"Cannot resolve PostgreSQL during deletion, "+
				"proceeding with finalizer removal",
			"postgresqlID", database.Spec.PostgresqlID)
		result, err := r.removeDBFinalizer(ctx, database)
		return nil, &result, err
	}

	if r.VaultClient == nil {
		r.Log.Warnw(
			"Vault client not available during deletion, " +
				"cannot delete database from PostgreSQL")
		result, err := r.removeDBFinalizer(ctx, database)
		return nil, &result, err
	}

	username, password, err := getVaultCredentialsWithRetry(
		ctx, r.VaultClient,
		connInfo.ExternalInstance.PostgresqlID, r.Log,
		r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay,
	)
	if err != nil {
		r.Log.Errorw(
			"Failed to get credentials from Vault during deletion",
			"error", err, "postgresqlID", connInfo.ExternalInstance.PostgresqlID)
		result, err := r.removeDBFinalizer(ctx, database)
		return nil, &result, err
	}
	connInfo.Username = username
	connInfo.Password = password

	return connInfo, nil, nil
}

// handleDeletion handles the deletion of a Database resource
func (r *DatabaseReconciler) handleDeletion(
	ctx context.Context, database *instancev1alpha1.Database,
) (ctrl.Result, error) {
	if !database.Spec.DeleteFromCRD {
		if controllerhelpers.ContainsString(
			database.Finalizers, databaseFinalizerName,
		) {
			return r.removeDBFinalizer(ctx, database)
		}
		return ctrl.Result{}, nil
	}

	if !controllerhelpers.ContainsString(
		database.Finalizers, databaseFinalizerName,
	) {
		return ctrl.Result{}, nil
	}

	connInfo, earlyResult, earlyErr := r.resolveDeletionConnection(
		ctx, database,
	)
	if earlyResult != nil {
		return *earlyResult, earlyErr
	}

	err := pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.DeleteDatabase(
			ctx, connInfo.ExternalInstance.Address,
			connInfo.Port, connInfo.Username, connInfo.Password,
			connInfo.SSLMode, database.Spec.Database,
		)
	}, r.Log, r.PostgresqlConnectionRetries,
		r.PostgresqlConnectionTimeout, "deleteDatabase")

	if err != nil {
		r.Log.Errorw("Failed to delete database from PostgreSQL",
			"error", err, "database", database.Spec.Database)
		controllerhelpers.UpdateDatabaseCondition(
			database, "Ready", metav1.ConditionFalse,
			"DeleteFailed",
			fmt.Sprintf("Failed to delete database: %v", err))
		if err := r.Status().Update(ctx, database); err != nil {
			r.Log.Errorw("Failed to update Database status", "error", err)
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	r.Log.Infow("Database successfully deleted from PostgreSQL",
		"PostgresqlID", database.Spec.PostgresqlID,
		"database", database.Spec.Database)

	return r.removeDBFinalizer(ctx, database)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1alpha1.Database{}).
		Named("database").
		WithEventFilter(controllerhelpers.IgnoreStatusUpdatesPredicate()).
		Complete(r)
}
