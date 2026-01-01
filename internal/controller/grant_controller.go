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

// GrantReconciler reconciles a Grant object
type GrantReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	VaultClient *vault.Client
	Log         *zap.SugaredLogger
}

// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=grants,verbs=get;list;watch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=grants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=grants/finalizers,verbs=update
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=postgresqls,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *GrantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// Fetch the Grant instance
	grant := &instancev1alpha1.Grant{}
	if err := r.Get(ctx, req.NamespacedName, grant); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "Failed to get Grant")
		return ctrl.Result{}, err
	}

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := r.findPostgresqlByID(ctx, grant.Spec.PostgresqlID)
	if err != nil {
		r.Log.Error(err, "Failed to find PostgreSQL instance", "postgresqlID", grant.Spec.PostgresqlID)
		updateGrantCondition(grant, "Ready", metav1.ConditionFalse, "PostgresqlNotFound",
			fmt.Sprintf("PostgreSQL instance with ID %s not found: %v", grant.Spec.PostgresqlID, err))
		if err := r.Status().Update(ctx, grant); err != nil {
			r.Log.Error(err, "Failed to update Grant status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		r.Log.Infow("PostgreSQL instance is not connected, waiting", "postgresqlID", grant.Spec.PostgresqlID)
		updateGrantCondition(grant, "Ready", metav1.ConditionFalse, "PostgresqlNotConnected",
			fmt.Sprintf("PostgreSQL instance with ID %s is not connected", grant.Spec.PostgresqlID))
		if err := r.Status().Update(ctx, grant); err != nil {
			r.Log.Error(err, "Failed to update Grant status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Error(nil, "PostgreSQL instance has no external instance configuration", "postgresqlID", grant.Spec.PostgresqlID)
		updateGrantCondition(grant, "Ready", metav1.ConditionFalse, "InvalidConfiguration",
			"PostgreSQL instance has no external instance configuration")
		if err := r.Status().Update(ctx, grant); err != nil {
			r.Log.Error(err, "Failed to update Grant status")
		}
		return ctrl.Result{}, nil
	}

	port := externalInstance.Port
	if port == 0 {
		port = 5432
	}

	sslMode := externalInstance.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}

	var postgresUsername, postgresPassword string
	if r.VaultClient != nil {
		vaultUsername, vaultPassword, err := r.VaultClient.GetPostgresqlCredentials(ctx, externalInstance.PostgresqlID)
		if err != nil {
			r.Log.Error(err, "Failed to get credentials from Vault", "postgresqlID", externalInstance.PostgresqlID)
			updateGrantCondition(grant, "Ready", metav1.ConditionFalse, "VaultError",
				fmt.Sprintf("Failed to get credentials from Vault: %v", err))
			if err := r.Status().Update(ctx, grant); err != nil {
				r.Log.Error(err, "Failed to update Grant status")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		postgresUsername = vaultUsername
		postgresPassword = vaultPassword
		r.Log.Infow("Credentials retrieved from Vault", "postgresqlID", externalInstance.PostgresqlID)
	} else {
		r.Log.Infow("Vault client not available")
		updateGrantCondition(grant, "Ready", metav1.ConditionFalse, "VaultNotAvailable",
			"Vault client is not available")
		if err := r.Status().Update(ctx, grant); err != nil {
			r.Log.Error(err, "Failed to update Grant status")
		}
		return ctrl.Result{}, nil
	}

	// Apply grants in PostgreSQL
	err = r.applyGrants(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
		grant.Spec.Database, grant.Spec.Role, grant.Spec.Grants, grant.Spec.DefaultPrivileges)
	if err != nil {
		r.Log.Error(err, "Failed to apply grants in PostgreSQL", "database", grant.Spec.Database, "role", grant.Spec.Role)
		grant.Status.Applied = false
		updateGrantCondition(grant, "Ready", metav1.ConditionFalse, "ApplyFailed",
			fmt.Sprintf("Failed to apply grants: %v", err))
	} else {
		grant.Status.Applied = true
		updateGrantCondition(grant, "Ready", metav1.ConditionTrue, "Applied",
			fmt.Sprintf("Grants successfully applied to role %s on database %s", grant.Spec.Role, grant.Spec.Database))
		r.Log.Infow("Grants successfully applied in PostgreSQL", "PostgresqlID", grant.Spec.PostgresqlID, "database", grant.Spec.Database, "role", grant.Spec.Role)
	}

	now := metav1.Now()
	grant.Status.LastSyncAttempt = &now

	// Update the status
	if err := r.Status().Update(ctx, grant); err != nil {
		r.Log.Error(err, "Failed to update Grant status")
		return ctrl.Result{}, err
	}

	r.Log.Infow("Successfully reconciled Grant", "database", grant.Spec.Database, "role", grant.Spec.Role)
	return ctrl.Result{}, nil
}

// findPostgresqlByID finds a PostgreSQL instance by its PostgresqlID
func (r *GrantReconciler) findPostgresqlByID(ctx context.Context, postgresqlID string) (*instancev1alpha1.Postgresql, error) {
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

// applyGrants applies grants to a PostgreSQL role on a database
func (r *GrantReconciler) applyGrants(ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, databaseName, roleName string, grants []instancev1alpha1.GrantItem, defaultPrivileges []instancev1alpha1.DefaultPrivilegeItem) error {
	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Escape role name for SQL identifier
	escapedRole := fmt.Sprintf(`"%s"`, strings.ReplaceAll(roleName, `"`, `""`))
	escapedDbName := fmt.Sprintf(`"%s"`, strings.ReplaceAll(databaseName, `"`, `""`))

	// Separate database grants from other grants
	var databaseGrants []instancev1alpha1.GrantItem
	var otherGrants []instancev1alpha1.GrantItem
	for _, grant := range grants {
		if grant.Type == instancev1alpha1.GrantTypeDatabase {
			databaseGrants = append(databaseGrants, grant)
		} else {
			otherGrants = append(otherGrants, grant)
		}
	}

	// Apply database grants - must be done from postgres database
	if len(databaseGrants) > 0 {
		connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s connect_timeout=5",
			host, port, adminUser, adminPassword, sslMode)

		db, err := sql.Open("postgres", connStr)
		if err != nil {
			return fmt.Errorf("failed to open database connection to postgres: %w", err)
		}
		defer db.Close()

		// Test connection
		if err := db.PingContext(connCtx); err != nil {
			return fmt.Errorf("failed to ping postgres database: %w", err)
		}

		// Apply database grants
		for _, grant := range databaseGrants {
			privilegesStr := strings.Join(grant.Privileges, ", ")
			query := fmt.Sprintf("GRANT %s ON DATABASE %s TO %s", privilegesStr, escapedDbName, escapedRole)
			if _, err := db.ExecContext(connCtx, query); err != nil {
				return fmt.Errorf("failed to apply database grant: %w", err)
			}
		}
	}

	// Apply other grants and default privileges - must be done from the target database
	if len(otherGrants) > 0 || len(defaultPrivileges) > 0 {
		connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
			host, port, adminUser, adminPassword, databaseName, sslMode)

		db, err := sql.Open("postgres", connStr)
		if err != nil {
			return fmt.Errorf("failed to open database connection to %s: %w", databaseName, err)
		}
		defer db.Close()

		// Test connection
		if err := db.PingContext(connCtx); err != nil {
			return fmt.Errorf("failed to ping database %s: %w", databaseName, err)
		}

		// Apply regular grants
		for _, grant := range otherGrants {
			if err := r.applyGrantItem(connCtx, db, escapedRole, grant); err != nil {
				return fmt.Errorf("failed to apply grant %s: %w", grant.Type, err)
			}
		}

		// Apply default privileges
		for _, defaultPriv := range defaultPrivileges {
			if err := r.applyDefaultPrivilege(connCtx, db, escapedRole, defaultPriv); err != nil {
				return fmt.Errorf("failed to apply default privilege for %s: %w", defaultPriv.ObjectType, err)
			}
		}
	}

	return nil
}

// applyGrantItem applies a single grant item
func (r *GrantReconciler) applyGrantItem(ctx context.Context, db *sql.DB, escapedRole string, grant instancev1alpha1.GrantItem) error {
	// Escape schema, table, sequence, function names
	escapedSchema := ""
	if grant.Schema != "" {
		escapedSchema = fmt.Sprintf(`"%s"`, strings.ReplaceAll(grant.Schema, `"`, `""`))
	}

	escapedTable := ""
	if grant.Table != "" {
		escapedTable = fmt.Sprintf(`"%s"`, strings.ReplaceAll(grant.Table, `"`, `""`))
	}

	escapedSequence := ""
	if grant.Sequence != "" {
		escapedSequence = fmt.Sprintf(`"%s"`, strings.ReplaceAll(grant.Sequence, `"`, `""`))
	}

	escapedFunction := ""
	if grant.Function != "" {
		escapedFunction = fmt.Sprintf(`"%s"`, strings.ReplaceAll(grant.Function, `"`, `""`))
	}

	// Build privileges string
	privilegesStr := strings.Join(grant.Privileges, ", ")

	var query string
	switch grant.Type {
	case instancev1alpha1.GrantTypeSchema:
		if grant.Schema == "" {
			return fmt.Errorf("schema is required for schema grant type")
		}
		// GRANT privileges ON SCHEMA schema_name TO role_name;
		query = fmt.Sprintf("GRANT %s ON SCHEMA %s TO %s", privilegesStr, escapedSchema, escapedRole)

	case instancev1alpha1.GrantTypeTable:
		if grant.Schema == "" {
			return fmt.Errorf("schema is required for table grant type")
		}
		if grant.Table == "" {
			return fmt.Errorf("table is required for table grant type")
		}
		// GRANT privileges ON TABLE schema_name.table_name TO role_name;
		query = fmt.Sprintf("GRANT %s ON TABLE %s.%s TO %s", privilegesStr, escapedSchema, escapedTable, escapedRole)

	case instancev1alpha1.GrantTypeSequence:
		if grant.Schema == "" {
			return fmt.Errorf("schema is required for sequence grant type")
		}
		if grant.Sequence == "" {
			return fmt.Errorf("sequence is required for sequence grant type")
		}
		// GRANT privileges ON SEQUENCE schema_name.sequence_name TO role_name;
		query = fmt.Sprintf("GRANT %s ON SEQUENCE %s.%s TO %s", privilegesStr, escapedSchema, escapedSequence, escapedRole)

	case instancev1alpha1.GrantTypeFunction:
		if grant.Schema == "" {
			return fmt.Errorf("schema is required for function grant type")
		}
		if grant.Function == "" {
			return fmt.Errorf("function is required for function grant type")
		}
		// GRANT privileges ON FUNCTION schema_name.function_name TO role_name;
		query = fmt.Sprintf("GRANT %s ON FUNCTION %s.%s TO %s", privilegesStr, escapedSchema, escapedFunction, escapedRole)

	case instancev1alpha1.GrantTypeAllTables:
		if grant.Schema == "" {
			return fmt.Errorf("schema is required for all_tables grant type")
		}
		// GRANT privileges ON ALL TABLES IN SCHEMA schema_name TO role_name;
		query = fmt.Sprintf("GRANT %s ON ALL TABLES IN SCHEMA %s TO %s", privilegesStr, escapedSchema, escapedRole)

	case instancev1alpha1.GrantTypeAllSequences:
		if grant.Schema == "" {
			return fmt.Errorf("schema is required for all_sequences grant type")
		}
		// GRANT privileges ON ALL SEQUENCES IN SCHEMA schema_name TO role_name;
		query = fmt.Sprintf("GRANT %s ON ALL SEQUENCES IN SCHEMA %s TO %s", privilegesStr, escapedSchema, escapedRole)

	default:
		return fmt.Errorf("unknown grant type: %s", grant.Type)
	}

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to execute grant query: %w", err)
	}

	return nil
}

// applyDefaultPrivilege applies default privileges for future objects
func (r *GrantReconciler) applyDefaultPrivilege(ctx context.Context, db *sql.DB, escapedRole string, defaultPriv instancev1alpha1.DefaultPrivilegeItem) error {
	escapedSchema := fmt.Sprintf(`"%s"`, strings.ReplaceAll(defaultPriv.Schema, `"`, `""`))
	privilegesStr := strings.Join(defaultPriv.Privileges, ", ")

	var query string
	switch defaultPriv.ObjectType {
	case "tables":
		// ALTER DEFAULT PRIVILEGES IN SCHEMA schema_name GRANT privileges ON TABLES TO role_name;
		query = fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT %s ON TABLES TO %s", escapedSchema, privilegesStr, escapedRole)
	case "sequences":
		// ALTER DEFAULT PRIVILEGES IN SCHEMA schema_name GRANT privileges ON SEQUENCES TO role_name;
		query = fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT %s ON SEQUENCES TO %s", escapedSchema, privilegesStr, escapedRole)
	case "functions":
		// ALTER DEFAULT PRIVILEGES IN SCHEMA schema_name GRANT privileges ON FUNCTIONS TO role_name;
		query = fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT %s ON FUNCTIONS TO %s", escapedSchema, privilegesStr, escapedRole)
	case "types":
		// ALTER DEFAULT PRIVILEGES IN SCHEMA schema_name GRANT privileges ON TYPES TO role_name;
		query = fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT %s ON TYPES TO %s", escapedSchema, privilegesStr, escapedRole)
	default:
		return fmt.Errorf("unknown default privilege object type: %s", defaultPriv.ObjectType)
	}

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to execute default privilege query: %w", err)
	}

	return nil
}

// updateGrantCondition updates or adds a condition to the Grant status
func updateGrantCondition(grant *instancev1alpha1.Grant, conditionType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		ObservedGeneration: grant.Generation,
	}

	found := false
	for i, c := range grant.Status.Conditions {
		if c.Type == conditionType {
			grant.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		grant.Status.Conditions = append(grant.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ignoreStatusUpdates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj := e.ObjectOld.(*instancev1alpha1.Grant)
			newObj := e.ObjectNew.(*instancev1alpha1.Grant)

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
		For(&instancev1alpha1.Grant{}).
		Named("grant").
		WithEventFilter(ignoreStatusUpdates).
		Complete(r)
}
