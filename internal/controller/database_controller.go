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
func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the Database instance
	database := &instancev1alpha1.Database{}
	if err := r.Get(ctx, req.NamespacedName, database); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Errorw("Failed to get Database",
			"name", req.Name, "namespace", req.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if database.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, database)
	}

	// Add finalizer if DeleteFromCRD is true
	if database.Spec.DeleteFromCRD {
		if !controllerhelpers.ContainsString(database.Finalizers, databaseFinalizerName) {
			database.Finalizers = append(database.Finalizers, databaseFinalizerName)
			if err := r.Update(ctx, database); err != nil {
				r.Log.Errorw("Failed to add finalizer",
					"name", database.Name, "namespace", database.Namespace, "error", err)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := k8sclient.FindPostgresqlByID(ctx, r.Client, database.Spec.PostgresqlID)
	if err != nil {
		r.Log.Errorw("Failed to find PostgreSQL instance",
			"name", database.Name, "namespace", database.Namespace,
			"postgresqlID", database.Spec.PostgresqlID, "error", err)
		controllerhelpers.UpdateDatabaseCondition(database, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonPostgresqlNotFound,
			fmt.Sprintf("PostgreSQL instance with ID %s not found: %v", database.Spec.PostgresqlID, err))
		if err := r.Status().Update(ctx, database); err != nil {
			r.Log.Errorw("Failed to update Database status",
				"name", database.Name, "namespace", database.Namespace, "error", err)
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		r.Log.Infow("PostgreSQL instance is not connected, waiting",
			"name", database.Name, "namespace", database.Namespace, "postgresqlID", database.Spec.PostgresqlID)
		controllerhelpers.UpdateDatabaseCondition(database, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonPostgresqlNotConnected,
			fmt.Sprintf("PostgreSQL instance with ID %s is not connected", database.Spec.PostgresqlID))
		if err := r.Status().Update(ctx, database); err != nil {
			r.Log.Errorw("Failed to update Database status",
				"name", database.Name, "namespace", database.Namespace, "error", err)
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Errorw("PostgreSQL instance has no external instance configuration",
			"name", database.Name, "namespace", database.Namespace, "postgresqlID", database.Spec.PostgresqlID)
		controllerhelpers.UpdateDatabaseCondition(database, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonInvalidConfiguration,
			"PostgreSQL instance has no external instance configuration")
		if err := r.Status().Update(ctx, database); err != nil {
			r.Log.Errorw("Failed to update Database status",
				"name", database.Name, "namespace", database.Namespace, "error", err)
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
				"name", database.Name, "namespace", database.Namespace,
				"postgresqlID", externalInstance.PostgresqlID, "error", err)
		} else {
			postgresUsername = vaultUsername
			postgresPassword = vaultPassword
			r.Log.Debugw("Credentials retrieved from Vault",
				"name", database.Name, "namespace", database.Namespace, "postgresqlID", externalInstance.PostgresqlID)
		}
	} else {
		// Use password from spec if Vault is not available
		r.Log.Infow("Vault client not available",
			"name", database.Name, "namespace", database.Namespace)
	}

	// Determine schema name - default to DefaultSchemaName if not specified
	schemaName := database.Spec.Schema
	if schemaName == "" {
		schemaName = constants.DefaultSchemaName
	}

	// Apply rate limiting before PostgreSQL operation
	if r.PostgresqlRateLimiter != nil {
		if err := r.PostgresqlRateLimiter.Wait(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	// Create or update database in PostgreSQL with retry logic
	err = pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.CreateOrUpdateDatabase(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
			database.Spec.Database, database.Spec.Owner, schemaName, database.Spec.DBTemplate)
	}, r.Log, r.PostgresqlConnectionRetries, r.PostgresqlConnectionTimeout, "createOrUpdateDatabase")
	if err != nil {
		r.Log.Errorw("Failed to create/update database in PostgreSQL",
			"name", database.Name, "namespace", database.Namespace,
			"database", database.Spec.Database, "operation", "createOrUpdateDatabase", "error", err)
		database.Status.Created = false
		controllerhelpers.UpdateDatabaseCondition(database, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonCreateFailed,
			fmt.Sprintf("Failed to create database: %v", err))
		r.EventRecorder.RecordCreateFailed(database, "Database", database.Spec.Database, err.Error())
	} else {
		database.Status.Created = true
		controllerhelpers.UpdateDatabaseCondition(database, constants.ConditionTypeReady,
			metav1.ConditionTrue, constants.ConditionReasonCreated,
			fmt.Sprintf("Database %s successfully created in PostgreSQL", database.Spec.Database))
		r.EventRecorder.RecordCreated(database, "Database", database.Spec.Database)
		r.Log.Infow("Database successfully created in PostgreSQL",
			"name", database.Name, "namespace", database.Namespace,
			"PostgresqlID", database.Spec.PostgresqlID, "database", database.Spec.Database, "schema", schemaName)
	}

	now := metav1.Now()
	database.Status.LastSyncAttempt = &now

	// Update the status
	if err := r.Status().Update(ctx, database); err != nil {
		r.Log.Errorw("Failed to update Database status",
			"name", database.Name, "namespace", database.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	r.Log.Infow("Successfully reconciled Database",
		"name", database.Name, "namespace", database.Namespace, "database", database.Spec.Database)
	return ctrl.Result{}, nil
}

// handleDeletion handles the deletion of a Database resource
func (r *DatabaseReconciler) handleDeletion(
	ctx context.Context, database *instancev1alpha1.Database) (ctrl.Result, error) {
	// Only delete from PostgreSQL if DeleteFromCRD is true
	if !database.Spec.DeleteFromCRD {
		// Remove finalizer if it exists
		if controllerhelpers.ContainsString(database.Finalizers, databaseFinalizerName) {
			database.Finalizers = controllerhelpers.RemoveString(database.Finalizers, databaseFinalizerName)
			if err := r.Update(ctx, database); err != nil {
				r.Log.Errorw("Failed to remove finalizer",
					"name", database.Name, "namespace", database.Namespace, "error", err)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Check if finalizer is present
	if !controllerhelpers.ContainsString(database.Finalizers, databaseFinalizerName) {
		// Finalizer already removed, nothing to do
		return ctrl.Result{}, nil
	}

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := k8sclient.FindPostgresqlByID(ctx, r.Client, database.Spec.PostgresqlID)
	if err != nil {
		r.Log.Warnw("Failed to find PostgreSQL instance during deletion, proceeding with finalizer removal",
			"name", database.Name, "namespace", database.Namespace,
			"postgresqlID", database.Spec.PostgresqlID, "error", err)
		// Remove finalizer even if PostgreSQL not found
		database.Finalizers = controllerhelpers.RemoveString(database.Finalizers, databaseFinalizerName)
		if err := r.Update(ctx, database); err != nil {
			r.Log.Errorw("Failed to remove finalizer",
				"name", database.Name, "namespace", database.Namespace, "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		r.Log.Warnw("PostgreSQL instance is not connected during deletion, proceeding with finalizer removal",
			"name", database.Name, "namespace", database.Namespace, "postgresqlID", database.Spec.PostgresqlID)
		// Remove finalizer even if PostgreSQL not connected
		database.Finalizers = controllerhelpers.RemoveString(database.Finalizers, databaseFinalizerName)
		if err := r.Update(ctx, database); err != nil {
			r.Log.Errorw("Failed to remove finalizer",
				"name", database.Name, "namespace", database.Namespace, "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Warnw(
			"PostgreSQL instance has no external instance configuration during deletion, proceeding with finalizer removal",
			"name", database.Name, "namespace", database.Namespace, "postgresqlID", database.Spec.PostgresqlID)
		// Remove finalizer
		database.Finalizers = controllerhelpers.RemoveString(database.Finalizers, databaseFinalizerName)
		if err := r.Update(ctx, database); err != nil {
			r.Log.Errorw("Failed to remove finalizer",
				"name", database.Name, "namespace", database.Namespace, "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Get connection defaults
	port, sslMode := controllerhelpers.GetConnectionDefaults(externalInstance.Port, externalInstance.SSLMode)

	var postgresUsername, postgresPassword string
	if r.VaultClient == nil {
		r.Log.Warnw("Vault client not available during deletion, cannot delete database from PostgreSQL",
			"name", database.Name, "namespace", database.Namespace)
		// Remove finalizer even if Vault not available
		database.Finalizers = controllerhelpers.RemoveString(database.Finalizers, databaseFinalizerName)
		if err := r.Update(ctx, database); err != nil {
			r.Log.Errorw("Failed to remove finalizer",
				"name", database.Name, "namespace", database.Namespace, "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	vaultUsername, vaultPassword, err := getVaultCredentialsWithRetryAndRateLimit(
		ctx, r.VaultClient, externalInstance.PostgresqlID, r.Log,
		r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay, r.VaultRateLimiter)
	if err != nil {
		r.Log.Errorw("Failed to get credentials from Vault during deletion",
			"name", database.Name, "namespace", database.Namespace,
			"postgresqlID", externalInstance.PostgresqlID, "error", err)
		// Remove finalizer even if credentials not available
		database.Finalizers = controllerhelpers.RemoveString(database.Finalizers, databaseFinalizerName)
		if err := r.Update(ctx, database); err != nil {
			r.Log.Errorw("Failed to remove finalizer",
				"name", database.Name, "namespace", database.Namespace, "error", err)
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

	// Delete database from PostgreSQL with retry logic
	err = pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.DeleteDatabase(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
			database.Spec.Database)
	}, r.Log, r.PostgresqlConnectionRetries, r.PostgresqlConnectionTimeout, "deleteDatabase")
	if err != nil {
		r.Log.Errorw("Failed to delete database from PostgreSQL",
			"name", database.Name, "namespace", database.Namespace,
			"database", database.Spec.Database, "operation", "deleteDatabase", "error", err)
		controllerhelpers.UpdateDatabaseCondition(database, constants.ConditionTypeReady,
			metav1.ConditionFalse, constants.ConditionReasonDeleteFailed,
			fmt.Sprintf("Failed to delete database: %v", err))
		r.EventRecorder.RecordDeleteFailed(database, "Database", database.Spec.Database, err.Error())
		if err := r.Status().Update(ctx, database); err != nil {
			r.Log.Errorw("Failed to update Database status",
				"name", database.Name, "namespace", database.Namespace, "error", err)
		}
		// Requeue to retry deletion
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}

	r.EventRecorder.RecordDeleted(database, "Database", database.Spec.Database)
	r.Log.Infow("Database successfully deleted from PostgreSQL",
		"name", database.Name, "namespace", database.Namespace,
		"PostgresqlID", database.Spec.PostgresqlID, "database", database.Spec.Database)

	// Remove finalizer
	database.Finalizers = controllerhelpers.RemoveString(database.Finalizers, databaseFinalizerName)
	if err := r.Update(ctx, database); err != nil {
		r.Log.Errorw("Failed to remove finalizer",
			"name", database.Name, "namespace", database.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1alpha1.Database{}).
		Named("database").
		WithEventFilter(controllerhelpers.IgnoreStatusUpdatesPredicate()).
		Complete(r)
}
