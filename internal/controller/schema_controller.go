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
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	_ "github.com/lib/pq"
	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// SchemaReconciler reconciles a Schema object
type SchemaReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	VaultClient *vault.Client
	Log         *zap.SugaredLogger
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
		r.Log.Error(err, "Failed to get Schema")
		return ctrl.Result{}, err
	}

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := r.findPostgresqlByID(ctx, schema.Spec.PostgresqlID)
	if err != nil {
		r.Log.Error(err, "Failed to find PostgreSQL instance", "postgresqlID", schema.Spec.PostgresqlID)
		updateSchemaCondition(schema, "Ready", metav1.ConditionFalse, "PostgresqlNotFound",
			fmt.Sprintf("PostgreSQL instance with ID %s not found: %v", schema.Spec.PostgresqlID, err))
		if err := r.Status().Update(ctx, schema); err != nil {
			r.Log.Error(err, "Failed to update Schema status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		r.Log.Infow("PostgreSQL instance is not connected, waiting", "postgresqlID", schema.Spec.PostgresqlID)
		updateSchemaCondition(schema, "Ready", metav1.ConditionFalse, "PostgresqlNotConnected",
			fmt.Sprintf("PostgreSQL instance with ID %s is not connected", schema.Spec.PostgresqlID))
		if err := r.Status().Update(ctx, schema); err != nil {
			r.Log.Error(err, "Failed to update Schema status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Error(nil, "PostgreSQL instance has no external instance configuration", "postgresqlID", schema.Spec.PostgresqlID)
		updateSchemaCondition(schema, "Ready", metav1.ConditionFalse, "InvalidConfiguration",
			"PostgreSQL instance has no external instance configuration")
		if err := r.Status().Update(ctx, schema); err != nil {
			r.Log.Error(err, "Failed to update Schema status")
		}
		return ctrl.Result{}, nil
	}

	port := externalInstance.Port
	if port == 0 {
		port = 5432
	}

	sslMode := externalInstance.SSLMode
	if sslMode == "" {
		sslMode = defaultSSLMode
	}

	var postgresUsername, postgresPassword string
	if r.VaultClient != nil {
		vaultUsername, vaultPassword, err := r.VaultClient.GetPostgresqlCredentials(ctx, externalInstance.PostgresqlID)
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

	// Create or update schema in PostgreSQL
	// Connect to postgres database as default
	err = r.createOrUpdateSchema(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
		"postgres", schema.Spec.Schema, schema.Spec.Owner)
	if err != nil {
		r.Log.Error(err, "Failed to create/update schema in PostgreSQL", "schema", schema.Spec.Schema)
		schema.Status.Created = false
		updateSchemaCondition(schema, "Ready", metav1.ConditionFalse, "CreateFailed",
			fmt.Sprintf("Failed to create schema: %v", err))
	} else {
		schema.Status.Created = true
		updateSchemaCondition(schema, "Ready", metav1.ConditionTrue, "Created",
			fmt.Sprintf("Schema %s successfully created", schema.Spec.Schema))
		r.Log.Infow("Schema successfully created in PostgreSQL", "PostgresqlID", schema.Spec.PostgresqlID, "schema", schema.Spec.Schema)
	}

	now := metav1.Now()
	schema.Status.LastSyncAttempt = &now

	// Update the status
	if err := r.Status().Update(ctx, schema); err != nil {
		r.Log.Error(err, "Failed to update Schema status")
		return ctrl.Result{}, err
	}

	r.Log.Infow("Successfully reconciled Schema", "schema", schema.Spec.Schema)
	return ctrl.Result{}, nil
}

// findPostgresqlByID finds a PostgreSQL instance by its PostgresqlID
func (r *SchemaReconciler) findPostgresqlByID(ctx context.Context, postgresqlID string) (*instancev1alpha1.Postgresql, error) {
	postgresqlList := &instancev1alpha1.PostgresqlList{}
	if err := r.List(ctx, postgresqlList); err != nil {
		return nil, fmt.Errorf("failed to list PostgreSQL instances: %w", err)
	}

	for i := range postgresqlList.Items {
		if postgresqlList.Items[i].Spec.ExternalInstance != nil &&
			postgresqlList.Items[i].Spec.ExternalInstance.PostgresqlID == postgresqlID {
			return &postgresqlList.Items[i], nil
		}
	}

	return nil, fmt.Errorf("PostgreSQL instance with ID %s not found", postgresqlID)
}

// createOrUpdateSchema creates or updates a PostgreSQL schema
func (r *SchemaReconciler) createOrUpdateSchema(ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, databaseName, schemaName, owner string) error {
	// Connect to the specific database
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, databaseName, sslMode)

	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	// Escape schema name and owner for SQL identifiers (PostgreSQL uses double quotes)
	escapedSchemaName := fmt.Sprintf(`"%s"`, strings.ReplaceAll(schemaName, `"`, `""`))
	escapedOwner := fmt.Sprintf(`"%s"`, strings.ReplaceAll(owner, `"`, `""`))

	// Check if schema exists
	var exists bool
	err = db.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)", schemaName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if schema exists: %w", err)
	}

	if exists {
		// Update schema owner
		query := fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s", escapedSchemaName, escapedOwner)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to update schema owner: %w", err)
		}
	} else {
		// Create new schema with owner
		query := fmt.Sprintf("CREATE SCHEMA %s AUTHORIZATION %s", escapedSchemaName, escapedOwner)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}
	}

	return nil
}

// updateSchemaCondition updates or adds a condition to the Schema status
func updateSchemaCondition(schema *instancev1alpha1.Schema, conditionType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		ObservedGeneration: schema.Generation,
	}

	found := false
	for i, c := range schema.Status.Conditions {
		if c.Type == conditionType {
			schema.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		schema.Status.Conditions = append(schema.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchemaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ignoreStatusUpdates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj := e.ObjectOld.(*instancev1alpha1.Schema)
			newObj := e.ObjectNew.(*instancev1alpha1.Schema)

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
		For(&instancev1alpha1.Schema{}).
		Named("schema").
		WithEventFilter(ignoreStatusUpdates).
		Complete(r)
}
