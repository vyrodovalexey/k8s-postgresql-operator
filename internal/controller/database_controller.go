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

// DatabaseReconciler reconciles a Database object
type DatabaseReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	VaultClient *vault.Client
	Log         *zap.SugaredLogger
}

// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=databases,verbs=get;list;watch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=databases/status,verbs=get;update;patch
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

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := r.findPostgresqlByID(ctx, database.Spec.PostgresqlID)
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
		r.Log.Error(nil, "PostgreSQL instance has no external instance configuration ", "postgresqlID: ", database.Spec.PostgresqlID)
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

	// Determine schema name - default to "public" if not specified
	schemaName := database.Spec.Schema
	if schemaName == "" {
		schemaName = "public"
	}

	// Create or update database in PostgreSQL
	err = r.createOrUpdateDatabase(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
		database.Spec.Database, database.Spec.Owner, schemaName)
	if err != nil {
		r.Log.Error(err, "Failed to create/update database in PostgreSQL", "database", database.Spec.Database)
		database.Status.Created = false
		updateDatabaseCondition(database, "Ready", metav1.ConditionFalse, "CreateFailed",
			fmt.Sprintf("Failed to create database: %v", err))
	} else {
		database.Status.Created = true
		updateDatabaseCondition(database, "Ready", metav1.ConditionTrue, "Created",
			fmt.Sprintf("Database %s successfully created in PostgreSQL", database.Spec.Database))
		r.Log.Infow("Database successfully created in PostgreSQL", "PostgresqlID", database.Spec.PostgresqlID, "database", database.Spec.Database, "schema", schemaName)
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

// findPostgresqlByID finds a PostgreSQL instance by its PostgresqlID
func (r *DatabaseReconciler) findPostgresqlByID(ctx context.Context, postgresqlID string) (*instancev1alpha1.Postgresql, error) {
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

// createOrUpdateDatabase creates or updates a PostgreSQL database
func (r *DatabaseReconciler) createOrUpdateDatabase(ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, databaseName, owner, schemaName string) error {
	// Connect to PostgreSQL as admin user
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, sslMode)

	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	// Escape database name and owner for SQL identifiers (PostgreSQL uses double quotes)
	// Replace double quotes with two double quotes to escape them
	escapedDatabaseName := fmt.Sprintf(`"%s"`, strings.ReplaceAll(databaseName, `"`, `""`))
	escapedOwner := fmt.Sprintf(`"%s"`, strings.ReplaceAll(owner, `"`, `""`))

	// Check if database exists
	var exists bool
	err = db.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", databaseName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if exists {
		// Update database owner
		query := fmt.Sprintf("ALTER DATABASE %s OWNER TO %s", escapedDatabaseName, escapedOwner)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to update database owner: %w", err)
		}
	} else {
		// Create new database
		query := fmt.Sprintf("CREATE DATABASE %s OWNER %s", escapedDatabaseName, escapedOwner)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
	}

	// Create schema in the database if specified (and not "public" which is created by default)
	if schemaName != "" && schemaName != "public" {
		// Connect to the newly created database
		connStr = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
			host, port, adminUser, adminPassword, databaseName, sslMode)

		dbSchema, err := sql.Open("postgres", connStr)
		if err != nil {
			return fmt.Errorf("failed to open database connection to %s: %w", databaseName, err)
		}
		defer db.Close()

		// Escape schema name for SQL identifier
		escapedSchemaName := fmt.Sprintf(`"%s"`, strings.ReplaceAll(schemaName, `"`, `""`))

		// Check if schema exists
		var schemaExists bool
		err = dbSchema.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)", schemaName).Scan(&schemaExists)
		if err != nil {
			return fmt.Errorf("failed to check if schema exists: %w", err)
		}

		if schemaExists {
			// Update schema owner
			query := fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s", escapedSchemaName, escapedOwner)
			_, err = dbSchema.ExecContext(connCtx, query)
			if err != nil {
				return fmt.Errorf("failed to update schema owner: %w", err)
			}
		}
	}

	return nil
}

// updateDatabaseCondition updates or adds a condition to the Database status
func updateDatabaseCondition(database *instancev1alpha1.Database, conditionType string, status metav1.ConditionStatus, reason, message string) {
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
