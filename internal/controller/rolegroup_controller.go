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

// RoleGroupReconciler reconciles a RoleGroup object
type RoleGroupReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	VaultClient *vault.Client
	Log         *zap.SugaredLogger
}

// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=rolegroups,verbs=get;list;watch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=rolegroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=rolegroups/finalizers,verbs=update
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=postgresqls,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *RoleGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// Fetch the RoleGroup instance
	roleGroup := &instancev1alpha1.RoleGroup{}
	if err := r.Get(ctx, req.NamespacedName, roleGroup); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "Failed to get RoleGroup")
		return ctrl.Result{}, err
	}

	// Find PostgreSQL instance by PostgresqlID
	postgresql, err := r.findPostgresqlByID(ctx, roleGroup.Spec.PostgresqlID)
	if err != nil {
		r.Log.Error(err, "Failed to find PostgreSQL instance", "postgresqlID", roleGroup.Spec.PostgresqlID)
		updateRoleGroupCondition(roleGroup, "Ready", metav1.ConditionFalse, "PostgresqlNotFound",
			fmt.Sprintf("PostgreSQL instance with ID %s not found: %v", roleGroup.Spec.PostgresqlID, err))
		if err := r.Status().Update(ctx, roleGroup); err != nil {
			r.Log.Error(err, "Failed to update RoleGroup status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check if PostgreSQL instance is connected
	if !postgresql.Status.Connected {
		r.Log.Infow("PostgreSQL instance is not connected, waiting", "postgresqlID", roleGroup.Spec.PostgresqlID)
		updateRoleGroupCondition(roleGroup, "Ready", metav1.ConditionFalse, "PostgresqlNotConnected",
			fmt.Sprintf("PostgreSQL instance with ID %s is not connected", roleGroup.Spec.PostgresqlID))
		if err := r.Status().Update(ctx, roleGroup); err != nil {
			r.Log.Error(err, "Failed to update RoleGroup status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Get PostgreSQL connection details
	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Error(nil, "PostgreSQL instance has no external instance configuration", "postgresqlID", roleGroup.Spec.PostgresqlID)
		updateRoleGroupCondition(roleGroup, "Ready", metav1.ConditionFalse, "InvalidConfiguration",
			"PostgreSQL instance has no external instance configuration")
		if err := r.Status().Update(ctx, roleGroup); err != nil {
			r.Log.Error(err, "Failed to update RoleGroup status")
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
			updateRoleGroupCondition(roleGroup, "Ready", metav1.ConditionFalse, "VaultError",
				fmt.Sprintf("Failed to get credentials from Vault: %v", err))
			if err := r.Status().Update(ctx, roleGroup); err != nil {
				r.Log.Error(err, "Failed to update RoleGroup status")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		postgresUsername = vaultUsername
		postgresPassword = vaultPassword
		r.Log.Infow("Credentials retrieved from Vault", "postgresqlID", externalInstance.PostgresqlID)
	} else {
		r.Log.Infow("Vault client not available")
		updateRoleGroupCondition(roleGroup, "Ready", metav1.ConditionFalse, "VaultNotAvailable",
			"Vault client is not available")
		if err := r.Status().Update(ctx, roleGroup); err != nil {
			r.Log.Error(err, "Failed to update RoleGroup status")
		}
		return ctrl.Result{}, nil
	}

	// Create or update role group in PostgreSQL
	err = r.createOrUpdateRoleGroup(ctx, externalInstance.Address, port, postgresUsername, postgresPassword, sslMode,
		roleGroup.Spec.GroupRole, roleGroup.Spec.MemberRoles)
	if err != nil {
		r.Log.Error(err, "Failed to create/update role group in PostgreSQL", "groupRole", roleGroup.Spec.GroupRole)
		roleGroup.Status.Created = false
		updateRoleGroupCondition(roleGroup, "Ready", metav1.ConditionFalse, "CreateFailed",
			fmt.Sprintf("Failed to create role group: %v", err))
	} else {
		roleGroup.Status.Created = true
		updateRoleGroupCondition(roleGroup, "Ready", metav1.ConditionTrue, "Created",
			fmt.Sprintf("Role group %s successfully created in PostgreSQL", roleGroup.Spec.GroupRole))
		r.Log.Infow("Role group successfully created in PostgreSQL", "PostgresqlID", roleGroup.Spec.PostgresqlID, "groupRole", roleGroup.Spec.GroupRole)
	}

	now := metav1.Now()
	roleGroup.Status.LastSyncAttempt = &now

	// Update the status
	if err := r.Status().Update(ctx, roleGroup); err != nil {
		r.Log.Error(err, "Failed to update RoleGroup status")
		return ctrl.Result{}, err
	}

	r.Log.Infow("Successfully reconciled RoleGroup", "groupRole", roleGroup.Spec.GroupRole)
	return ctrl.Result{}, nil
}

// findPostgresqlByID finds a PostgreSQL instance by its PostgresqlID
func (r *RoleGroupReconciler) findPostgresqlByID(ctx context.Context, postgresqlID string) (*instancev1alpha1.Postgresql, error) {
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

// createOrUpdateRoleGroup creates or updates a PostgreSQL role group
func (r *RoleGroupReconciler) createOrUpdateRoleGroup(ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, groupRole string, memberRoles []string) error {
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

	// Escape group role name for SQL identifier
	escapedGroupRole := fmt.Sprintf(`"%s"`, strings.ReplaceAll(groupRole, `"`, `""`))

	// Check if group role exists
	var exists bool
	err = db.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", groupRole).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if group role exists: %w", err)
	}

	if !exists {
		// Create the group role (as a role, not a user, so it can be a group)
		query := fmt.Sprintf("CREATE ROLE %s", escapedGroupRole)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to create group role: %w", err)
		}
	}

	// Get current members of the group role
	currentMembers := make(map[string]bool)
	rows, err := db.QueryContext(connCtx, `
		SELECT r.rolname 
		FROM pg_roles r 
		JOIN pg_auth_members m ON r.oid = m.member 
		JOIN pg_roles g ON g.oid = m.roleid 
		WHERE g.rolname = $1`, groupRole)
	if err != nil {
		return fmt.Errorf("failed to query current members: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var member string
		if err := rows.Scan(&member); err != nil {
			return fmt.Errorf("failed to scan member: %w", err)
		}
		currentMembers[member] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating members: %w", err)
	}

	// Add new members that are not already members
	desiredMembers := make(map[string]bool)
	for _, memberRole := range memberRoles {
		desiredMembers[memberRole] = true
		if !currentMembers[memberRole] {
			// Check if member role exists
			var memberExists bool
			err = db.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", memberRole).Scan(&memberExists)
			if err != nil {
				return fmt.Errorf("failed to check if member role exists: %w", err)
			}
			if !memberExists {
				r.Log.Warnw("Member role does not exist, skipping", "memberRole", memberRole)
				continue
			}

			// Add member to group
			escapedMemberRole := fmt.Sprintf(`"%s"`, strings.ReplaceAll(memberRole, `"`, `""`))
			query := fmt.Sprintf("GRANT %s TO %s", escapedGroupRole, escapedMemberRole)
			_, err = db.ExecContext(connCtx, query)
			if err != nil {
				return fmt.Errorf("failed to add member %s to group role: %w", memberRole, err)
			}
		}
	}

	// Remove members that are no longer in the desired list
	for member := range currentMembers {
		if !desiredMembers[member] {
			escapedMemberRole := fmt.Sprintf(`"%s"`, strings.ReplaceAll(member, `"`, `""`))
			query := fmt.Sprintf("REVOKE %s FROM %s", escapedGroupRole, escapedMemberRole)
			_, err = db.ExecContext(connCtx, query)
			if err != nil {
				return fmt.Errorf("failed to remove member %s from group role: %w", member, err)
			}
		}
	}

	return nil
}

// updateRoleGroupCondition updates or adds a condition to the RoleGroup status
func updateRoleGroupCondition(roleGroup *instancev1alpha1.RoleGroup, conditionType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		ObservedGeneration: roleGroup.Generation,
	}

	found := false
	for i, c := range roleGroup.Status.Conditions {
		if c.Type == conditionType {
			roleGroup.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		roleGroup.Status.Conditions = append(roleGroup.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *RoleGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ignoreStatusUpdates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj := e.ObjectOld.(*instancev1alpha1.RoleGroup)
			newObj := e.ObjectNew.(*instancev1alpha1.RoleGroup)

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
		For(&instancev1alpha1.RoleGroup{}).
		Named("rolegroup").
		WithEventFilter(ignoreStatusUpdates).
		Complete(r)
}
