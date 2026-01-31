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

// SchemaReconciler reconciles a Schema object
type SchemaReconciler struct {
	BaseReconcilerConfig
}

// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=schemas,verbs=get;list;watch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=schemas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=schemas/finalizers,verbs=update
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=postgresqls,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *SchemaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the Schema instance
	schema := &instancev1alpha1.Schema{}
	if err := r.Get(ctx, req.NamespacedName, schema); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Errorw("Failed to get Schema",
			"name", req.Name, "namespace", req.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := k8sclient.FindPostgresqlByID(ctx, r.Client, schema.Spec.PostgresqlID)
	if err != nil {
		r.Log.Errorw("Failed to find PostgreSQL instance",
			"name", schema.Name, "namespace", schema.Namespace,
			"postgresqlID", schema.Spec.PostgresqlID, "error", err)
		controllerhelpers.UpdateSchemaCondition(schema, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonPostgresqlNotFound,
			fmt.Sprintf("PostgreSQL instance with ID %s not found: %v", schema.Spec.PostgresqlID, err))
		if err := r.Status().Update(ctx, schema); err != nil {
			r.Log.Errorw("Failed to update Schema status",
				"name", schema.Name, "namespace", schema.Namespace, "error", err)
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		r.Log.Infow("PostgreSQL instance is not connected, waiting",
			"name", schema.Name, "namespace", schema.Namespace, "postgresqlID", schema.Spec.PostgresqlID)
		controllerhelpers.UpdateSchemaCondition(schema, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonPostgresqlNotConnected,
			fmt.Sprintf("PostgreSQL instance with ID %s is not connected", schema.Spec.PostgresqlID))
		if err := r.Status().Update(ctx, schema); err != nil {
			r.Log.Errorw("Failed to update Schema status",
				"name", schema.Name, "namespace", schema.Namespace, "error", err)
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Errorw("PostgreSQL instance has no external instance configuration",
			"name", schema.Name, "namespace", schema.Namespace, "postgresqlID", schema.Spec.PostgresqlID)
		controllerhelpers.UpdateSchemaCondition(schema, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonInvalidConfiguration,
			"PostgreSQL instance has no external instance configuration")
		if err := r.Status().Update(ctx, schema); err != nil {
			r.Log.Errorw("Failed to update Schema status",
				"name", schema.Name, "namespace", schema.Namespace, "error", err)
		}
		return ctrl.Result{}, nil
	}

	// Get connection defaults
	port, sslMode := controllerhelpers.GetConnectionDefaults(externalInstance.Port, externalInstance.SSLMode)

	var postgresUsername, postgresPassword string
	if r.VaultClient != nil {
		vaultUsername, vaultPassword, err := getVaultCredentialsWithRetryAndRateLimit(
			ctx, r.VaultClient, externalInstance.PostgresqlID, r.Log,
			r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay, r.VaultRateLimiter)
		if err != nil {
			r.Log.Errorw("Failed to get credentials from Vault",
				"name", schema.Name, "namespace", schema.Namespace,
				"postgresqlID", externalInstance.PostgresqlID, "error", err)
		} else {
			postgresUsername = vaultUsername
			postgresPassword = vaultPassword
			r.Log.Debugw("Credentials retrieved from Vault",
				"name", schema.Name, "namespace", schema.Namespace, "postgresqlID", externalInstance.PostgresqlID)
		}
	} else {
		// Use password from spec if Vault is not available
		r.Log.Infow("Vault client not available",
			"name", schema.Name, "namespace", schema.Namespace)
	}

	// Get database name from spec
	database := schema.Spec.Database
	if database == "" {
		r.Log.Errorw("Database name not specified in Schema spec",
			"name", schema.Name, "namespace", schema.Namespace,
			"schema", schema.Spec.Schema, "postgresqlID", schema.Spec.PostgresqlID)
		controllerhelpers.UpdateSchemaCondition(schema, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonInvalidConfiguration,
			"Database name is required but not specified in Schema spec")
		if err := r.Status().Update(ctx, schema); err != nil {
			r.Log.Errorw("Failed to update Schema status",
				"name", schema.Name, "namespace", schema.Namespace, "error", err)
		}
		return ctrl.Result{}, nil
	}

	r.Log.Debugw("Creating/updating schema in PostgreSQL",
		"name", schema.Name, "namespace", schema.Namespace,
		"schema", schema.Spec.Schema, "database", database, "owner", schema.Spec.Owner)

	// Apply rate limiting before PostgreSQL operation
	if r.PostgresqlRateLimiter != nil {
		if err := r.PostgresqlRateLimiter.Wait(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	// Create or update schema in PostgreSQL with retry logic
	err = pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.CreateOrUpdateSchema(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
			database, schema.Spec.Schema, schema.Spec.Owner)
	}, r.Log, r.PostgresqlConnectionRetries, r.PostgresqlConnectionTimeout, "createOrUpdateSchema")
	if err != nil {
		r.Log.Errorw("Failed to create/update schema in PostgreSQL",
			"name", schema.Name, "namespace", schema.Namespace,
			"schema", schema.Spec.Schema, "operation", "createOrUpdateSchema", "error", err)
		schema.Status.Created = false
		controllerhelpers.UpdateSchemaCondition(schema, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonCreateFailed,
			fmt.Sprintf("Failed to create schema: %v", err))
		r.EventRecorder.RecordCreateFailed(schema, "Schema", schema.Spec.Schema, err.Error())
	} else {
		schema.Status.Created = true
		controllerhelpers.UpdateSchemaCondition(schema, constants.ConditionTypeReady,
			metav1.ConditionTrue, constants.ConditionReasonCreated,
			fmt.Sprintf("Schema %s successfully created", schema.Spec.Schema))
		r.EventRecorder.RecordCreated(schema, "Schema", schema.Spec.Schema)
		r.Log.Infow("Schema successfully created in PostgreSQL",
			"name", schema.Name, "namespace", schema.Namespace,
			"PostgresqlID", schema.Spec.PostgresqlID, "schema", schema.Spec.Schema)
	}

	now := metav1.Now()
	schema.Status.LastSyncAttempt = &now

	// Update the status
	if err := r.Status().Update(ctx, schema); err != nil {
		r.Log.Errorw("Failed to update Schema status",
			"name", schema.Name, "namespace", schema.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	r.Log.Infow("Successfully reconciled Schema",
		"name", schema.Name, "namespace", schema.Namespace, "schema", schema.Spec.Schema)
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
