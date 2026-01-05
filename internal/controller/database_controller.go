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
	k8sclient "github.com/vyrodovalexey/k8s-postgresql-operator/internal/k8s"
	pg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	databaseFinalizerName = "database.postgresql-operator.vyrodovalexey.github.com/finalizer"
)

// DatabaseReconciler reconciles a Database object
type DatabaseReconciler struct {
	BaseReconcilerConfig
}

// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=databases,verbs=get;list;watch
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
		r.Log.Error(err, "Failed to get Database")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if database.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, database)
	}

	// Add finalizer if DeleteFromCRD is true
	if database.Spec.DeleteFromCRD {
		if !containsString(database.Finalizers, databaseFinalizerName) {
			database.Finalizers = append(database.Finalizers, databaseFinalizerName)
			if err := r.Update(ctx, database); err != nil {
				r.Log.Error(err, "Failed to add finalizer")
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := k8sclient.FindPostgresqlByID(ctx, r.Client, database.Spec.PostgresqlID)
	if err != nil {
		r.Log.Error(err, "Failed to find PostgreSQL instance", "postgresqlID", database.Spec.PostgresqlID)
		updateDatabaseCondition(database, "Ready", metav1.ConditionFalse, "PostgresqlNotFound",
			fmt.Sprintf("PostgreSQL instance with ID %s not found: %v", database.Spec.PostgresqlID, err))
		if err := r.Status().Update(ctx, database); err != nil {
			r.Log.Error(err, "Failed to update Database status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		r.Log.Infow("PostgreSQL instance is not connected, waiting", "postgresqlID", database.Spec.PostgresqlID)
		updateDatabaseCondition(database, "Ready", metav1.ConditionFalse, "PostgresqlNotConnected",
			fmt.Sprintf("PostgreSQL instance with ID %s is not connected", database.Spec.PostgresqlID))
		if err := r.Status().Update(ctx, database); err != nil {
			r.Log.Error(err, "Failed to update Database status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Error(nil, "PostgreSQL instance has no external instance configuration ",
			"postgresqlID: ", database.Spec.PostgresqlID)
		updateDatabaseCondition(database, "Ready", metav1.ConditionFalse, "InvalidConfiguration",
			"PostgreSQL instance has no external instance configuration")
		if err := r.Status().Update(ctx, database); err != nil {
			r.Log.Error(err, "Failed to update Database status")
		}
		return ctrl.Result{}, nil
	}

	port := externalInstance.Port
	if port == 0 {
		port = 5432
	}

	sslMode := externalInstance.SSLMode
	if sslMode == "" {
		sslMode = pg.DefaultSSLMode
	}

	var postgresUsername, postgresPassword string
	if r.VaultClient != nil {
		vaultUsername, vaultPassword, err := getVaultCredentialsWithRetry(
			ctx, r.VaultClient, externalInstance.PostgresqlID, r.Log,
			r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay)
		if err != nil {
			r.Log.Error(err, "Failed to get credentials from Vault", "postgresqlID", externalInstance.PostgresqlID)
		} else {
			postgresUsername = vaultUsername
			postgresPassword = vaultPassword
			r.Log.Infow("Credentials retrieved from Vault", "postgresqlID", externalInstance.PostgresqlID)
		}
	} else {
		// Use password from spec if Vault is not available
		r.Log.Infow("Vault client not available")
	}

	// Determine schema name - default to "public" if not specified
	schemaName := database.Spec.Schema
	if schemaName == "" {
		schemaName = "public"
	}

	// Create or update database in PostgreSQL with retry logic
	err = pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.CreateOrUpdateDatabase(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
			database.Spec.Database, database.Spec.Owner, schemaName, database.Spec.DBTemplate)
	}, r.Log, r.PostgresqlConnectionRetries, r.PostgresqlConnectionTimeout, "createOrUpdateDatabase")
	if err != nil {
		r.Log.Error(err, "Failed to create/update database in PostgreSQL", "database", database.Spec.Database)
		database.Status.Created = false
		updateDatabaseCondition(database, "Ready", metav1.ConditionFalse, "CreateFailed",
			fmt.Sprintf("Failed to create database: %v", err))
	} else {
		database.Status.Created = true
		updateDatabaseCondition(database, "Ready", metav1.ConditionTrue, "Created",
			fmt.Sprintf("Database %s successfully created in PostgreSQL", database.Spec.Database))
		r.Log.Infow("Database successfully created in PostgreSQL",
			"PostgresqlID", database.Spec.PostgresqlID, "database", database.Spec.Database, "schema", schemaName)
	}

	now := metav1.Now()
	database.Status.LastSyncAttempt = &now

	// Update the status
	if err := r.Status().Update(ctx, database); err != nil {
		r.Log.Error(err, "Failed to update Database status")
		return ctrl.Result{}, err
	}

	r.Log.Infow("Successfully reconciled Database", "database", database.Spec.Database)
	return ctrl.Result{}, nil
}

// handleDeletion handles the deletion of a Database resource
func (r *DatabaseReconciler) handleDeletion(
	ctx context.Context, database *instancev1alpha1.Database) (ctrl.Result, error) {
	// Only delete from PostgreSQL if DeleteFromCRD is true
	if !database.Spec.DeleteFromCRD {
		// Remove finalizer if it exists
		if containsString(database.Finalizers, databaseFinalizerName) {
			database.Finalizers = removeString(database.Finalizers, databaseFinalizerName)
			if err := r.Update(ctx, database); err != nil {
				r.Log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Check if finalizer is present
	if !containsString(database.Finalizers, databaseFinalizerName) {
		// Finalizer already removed, nothing to do
		return ctrl.Result{}, nil
	}

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := k8sclient.FindPostgresqlByID(ctx, r.Client, database.Spec.PostgresqlID)
	if err != nil {
		r.Log.Warnw("Failed to find PostgreSQL instance during deletion, proceeding with finalizer removal",
			"postgresqlID", database.Spec.PostgresqlID, "error", err)
		// Remove finalizer even if PostgreSQL not found
		database.Finalizers = removeString(database.Finalizers, databaseFinalizerName)
		if err := r.Update(ctx, database); err != nil {
			r.Log.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		r.Log.Warnw("PostgreSQL instance is not connected during deletion, proceeding with finalizer removal",
			"postgresqlID", database.Spec.PostgresqlID)
		// Remove finalizer even if PostgreSQL not connected
		database.Finalizers = removeString(database.Finalizers, databaseFinalizerName)
		if err := r.Update(ctx, database); err != nil {
			r.Log.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Warnw(
			"PostgreSQL instance has no external instance configuration during deletion, proceeding with finalizer removal",
			"postgresqlID", database.Spec.PostgresqlID)
		// Remove finalizer
		database.Finalizers = removeString(database.Finalizers, databaseFinalizerName)
		if err := r.Update(ctx, database); err != nil {
			r.Log.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	port := externalInstance.Port
	if port == 0 {
		port = 5432
	}

	sslMode := externalInstance.SSLMode
	if sslMode == "" {
		sslMode = pg.DefaultSSLMode
	}

	var postgresUsername, postgresPassword string
	if r.VaultClient == nil {
		r.Log.Warnw("Vault client not available during deletion, cannot delete database from PostgreSQL")
		// Remove finalizer even if Vault not available
		database.Finalizers = removeString(database.Finalizers, databaseFinalizerName)
		if err := r.Update(ctx, database); err != nil {
			r.Log.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	vaultUsername, vaultPassword, err := getVaultCredentialsWithRetry(
		ctx, r.VaultClient, externalInstance.PostgresqlID, r.Log,
		r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay)
	if err != nil {
		r.Log.Error(err, "Failed to get credentials from Vault during deletion",
			"postgresqlID", externalInstance.PostgresqlID)
		// Remove finalizer even if credentials not available
		database.Finalizers = removeString(database.Finalizers, databaseFinalizerName)
		if err := r.Update(ctx, database); err != nil {
			r.Log.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	postgresUsername = vaultUsername
	postgresPassword = vaultPassword

	// Delete database from PostgreSQL with retry logic
	err = pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.DeleteDatabase(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
			database.Spec.Database)
	}, r.Log, r.PostgresqlConnectionRetries, r.PostgresqlConnectionTimeout, "deleteDatabase")
	if err != nil {
		r.Log.Error(err, "Failed to delete database from PostgreSQL", "database", database.Spec.Database)
		updateDatabaseCondition(database, "Ready", metav1.ConditionFalse, "DeleteFailed",
			fmt.Sprintf("Failed to delete database: %v", err))
		if err := r.Status().Update(ctx, database); err != nil {
			r.Log.Error(err, "Failed to update Database status")
		}
		// Requeue to retry deletion
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	r.Log.Infow("Database successfully deleted from PostgreSQL",
		"PostgresqlID", database.Spec.PostgresqlID, "database", database.Spec.Database)

	// Remove finalizer
	database.Finalizers = removeString(database.Finalizers, databaseFinalizerName)
	if err := r.Update(ctx, database); err != nil {
		r.Log.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// containsString checks if a string slice contains a specific string
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// removeString removes a string from a string slice
//
//nolint:unparam // s parameter is kept for reusability as a generic utility function
func removeString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// updateDatabaseCondition updates or adds a condition to the Database status
// nolint:unparam // conditionType parameter is kept for API consistency and future extensibility
func updateDatabaseCondition(
	database *instancev1alpha1.Database, conditionType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		ObservedGeneration: database.Generation,
	}

	found := false
	for i, c := range database.Status.Conditions {
		if c.Type == conditionType {
			database.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		database.Status.Conditions = append(database.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ignoreStatusUpdates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj := e.ObjectOld.(*instancev1alpha1.Database)
			newObj := e.ObjectNew.(*instancev1alpha1.Database)

			if oldObj.Generation != newObj.Generation {
				return true
			}

			if oldObj.DeletionTimestamp != newObj.DeletionTimestamp {
				return true
			}

			return false
		},
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return true },
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1alpha1.Database{}).
		Named("database").
		WithEventFilter(ignoreStatusUpdates).
		Complete(r)
}
