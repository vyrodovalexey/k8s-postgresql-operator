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

package testutils

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// AssertPostgresqlConnected asserts that a Postgresql CRD has connected status
func AssertPostgresqlConnected(t *testing.T, ctx context.Context, c client.Client, name, namespace string) {
	t.Helper()

	postgresql := &instancev1alpha1.Postgresql{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, postgresql)
	require.NoError(t, err, "Failed to get Postgresql")
	assert.True(t, postgresql.Status.Connected, "Postgresql should be connected")
}

// AssertPostgresqlNotConnected asserts that a Postgresql CRD has not connected status
func AssertPostgresqlNotConnected(t *testing.T, ctx context.Context, c client.Client, name, namespace string) {
	t.Helper()

	postgresql := &instancev1alpha1.Postgresql{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, postgresql)
	require.NoError(t, err, "Failed to get Postgresql")
	assert.False(t, postgresql.Status.Connected, "Postgresql should not be connected")
}

// AssertCondition asserts that a condition exists with the expected status
func AssertCondition(t *testing.T, conditions []metav1.Condition, conditionType string, expectedStatus metav1.ConditionStatus) {
	t.Helper()

	for _, condition := range conditions {
		if condition.Type == conditionType {
			assert.Equal(t, expectedStatus, condition.Status,
				"Condition %s should have status %s, got %s", conditionType, expectedStatus, condition.Status)
			return
		}
	}
	t.Errorf("Condition %s not found", conditionType)
}

// AssertConditionReason asserts that a condition exists with the expected reason
func AssertConditionReason(t *testing.T, conditions []metav1.Condition, conditionType, expectedReason string) {
	t.Helper()

	for _, condition := range conditions {
		if condition.Type == conditionType {
			assert.Equal(t, expectedReason, condition.Reason,
				"Condition %s should have reason %s, got %s", conditionType, expectedReason, condition.Reason)
			return
		}
	}
	t.Errorf("Condition %s not found", conditionType)
}

// AssertUserCreated asserts that a User CRD has created status
func AssertUserCreated(t *testing.T, ctx context.Context, c client.Client, name, namespace string) {
	t.Helper()

	user := &instancev1alpha1.User{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, user)
	require.NoError(t, err, "Failed to get User")
	assert.True(t, user.Status.Created, "User should be created")
}

// AssertUserNotCreated asserts that a User CRD has not created status
func AssertUserNotCreated(t *testing.T, ctx context.Context, c client.Client, name, namespace string) {
	t.Helper()

	user := &instancev1alpha1.User{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, user)
	require.NoError(t, err, "Failed to get User")
	assert.False(t, user.Status.Created, "User should not be created")
}

// AssertDatabaseCreated asserts that a Database CRD has created status
func AssertDatabaseCreated(t *testing.T, ctx context.Context, c client.Client, name, namespace string) {
	t.Helper()

	database := &instancev1alpha1.Database{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, database)
	require.NoError(t, err, "Failed to get Database")
	assert.True(t, database.Status.Created, "Database should be created")
}

// AssertDatabaseNotCreated asserts that a Database CRD has not created status
func AssertDatabaseNotCreated(t *testing.T, ctx context.Context, c client.Client, name, namespace string) {
	t.Helper()

	database := &instancev1alpha1.Database{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, database)
	require.NoError(t, err, "Failed to get Database")
	assert.False(t, database.Status.Created, "Database should not be created")
}

// AssertGrantApplied asserts that a Grant CRD has applied status
func AssertGrantApplied(t *testing.T, ctx context.Context, c client.Client, name, namespace string) {
	t.Helper()

	grant := &instancev1alpha1.Grant{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, grant)
	require.NoError(t, err, "Failed to get Grant")
	assert.True(t, grant.Status.Applied, "Grant should be applied")
}

// AssertGrantNotApplied asserts that a Grant CRD has not applied status
func AssertGrantNotApplied(t *testing.T, ctx context.Context, c client.Client, name, namespace string) {
	t.Helper()

	grant := &instancev1alpha1.Grant{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, grant)
	require.NoError(t, err, "Failed to get Grant")
	assert.False(t, grant.Status.Applied, "Grant should not be applied")
}

// AssertRoleGroupCreated asserts that a RoleGroup CRD has created status
func AssertRoleGroupCreated(t *testing.T, ctx context.Context, c client.Client, name, namespace string) {
	t.Helper()

	roleGroup := &instancev1alpha1.RoleGroup{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, roleGroup)
	require.NoError(t, err, "Failed to get RoleGroup")
	assert.True(t, roleGroup.Status.Created, "RoleGroup should be created")
}

// AssertSchemaCreated asserts that a Schema CRD has created status
func AssertSchemaCreated(t *testing.T, ctx context.Context, c client.Client, name, namespace string) {
	t.Helper()

	schema := &instancev1alpha1.Schema{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, schema)
	require.NoError(t, err, "Failed to get Schema")
	assert.True(t, schema.Status.Created, "Schema should be created")
}

// AssertPostgresUserExists asserts that a user exists in PostgreSQL
func AssertPostgresUserExists(t *testing.T, ctx context.Context, db *sql.DB, username string) {
	t.Helper()

	var exists bool
	err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_user WHERE usename = $1)", username).Scan(&exists)
	require.NoError(t, err, "Failed to check if user exists")
	assert.True(t, exists, "User %s should exist in PostgreSQL", username)
}

// AssertPostgresUserNotExists asserts that a user does not exist in PostgreSQL
func AssertPostgresUserNotExists(t *testing.T, ctx context.Context, db *sql.DB, username string) {
	t.Helper()

	var exists bool
	err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_user WHERE usename = $1)", username).Scan(&exists)
	require.NoError(t, err, "Failed to check if user exists")
	assert.False(t, exists, "User %s should not exist in PostgreSQL", username)
}

// AssertPostgresDatabaseExists asserts that a database exists in PostgreSQL
func AssertPostgresDatabaseExists(t *testing.T, ctx context.Context, db *sql.DB, dbName string) {
	t.Helper()

	var exists bool
	err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists)
	require.NoError(t, err, "Failed to check if database exists")
	assert.True(t, exists, "Database %s should exist in PostgreSQL", dbName)
}

// AssertPostgresDatabaseNotExists asserts that a database does not exist in PostgreSQL
func AssertPostgresDatabaseNotExists(t *testing.T, ctx context.Context, db *sql.DB, dbName string) {
	t.Helper()

	var exists bool
	err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists)
	require.NoError(t, err, "Failed to check if database exists")
	assert.False(t, exists, "Database %s should not exist in PostgreSQL", dbName)
}

// AssertPostgresRoleExists asserts that a role exists in PostgreSQL
func AssertPostgresRoleExists(t *testing.T, ctx context.Context, db *sql.DB, roleName string) {
	t.Helper()

	var exists bool
	err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", roleName).Scan(&exists)
	require.NoError(t, err, "Failed to check if role exists")
	assert.True(t, exists, "Role %s should exist in PostgreSQL", roleName)
}

// AssertPostgresSchemaExists asserts that a schema exists in a PostgreSQL database
func AssertPostgresSchemaExists(t *testing.T, ctx context.Context, db *sql.DB, schemaName string) {
	t.Helper()

	var exists bool
	err := db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)",
		schemaName).Scan(&exists)
	require.NoError(t, err, "Failed to check if schema exists")
	assert.True(t, exists, "Schema %s should exist in PostgreSQL", schemaName)
}

// WaitForCondition waits for a condition to be met with retries
func WaitForCondition(t *testing.T, timeout time.Duration, interval time.Duration, condition func() bool, message string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("Timeout waiting for condition: %s", message)
}

// AssertEventuallyTrue asserts that a condition becomes true within the timeout
func AssertEventuallyTrue(t *testing.T, timeout time.Duration, interval time.Duration, condition func() bool, msgAndArgs ...interface{}) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	assert.Fail(t, "Condition never became true", msgAndArgs...)
}

// AssertDatabaseOwner asserts that a database has the expected owner
func AssertDatabaseOwner(t *testing.T, ctx context.Context, db *sql.DB, dbName, expectedOwner string) {
	t.Helper()

	var owner string
	err := db.QueryRowContext(ctx, `
		SELECT pg_catalog.pg_get_userbyid(d.datdba) 
		FROM pg_catalog.pg_database d 
		WHERE d.datname = $1
	`, dbName).Scan(&owner)
	require.NoError(t, err, "Failed to get database owner")
	assert.Equal(t, expectedOwner, owner, "Database %s should be owned by %s", dbName, expectedOwner)
}

// AssertRoleMembership asserts that a role is a member of a group role
func AssertRoleMembership(t *testing.T, ctx context.Context, db *sql.DB, memberRole, groupRole string) {
	t.Helper()

	var isMember bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 
			FROM pg_roles r 
			JOIN pg_auth_members m ON r.oid = m.member 
			JOIN pg_roles g ON g.oid = m.roleid 
			WHERE r.rolname = $1 AND g.rolname = $2
		)
	`, memberRole, groupRole).Scan(&isMember)
	require.NoError(t, err, "Failed to check role membership")
	assert.True(t, isMember, "Role %s should be a member of %s", memberRole, groupRole)
}

// AssertHasPrivilege asserts that a role has a specific privilege on an object
func AssertHasPrivilege(t *testing.T, ctx context.Context, db *sql.DB, roleName, objectType, objectName, privilege string) {
	t.Helper()

	var hasPrivilege bool
	var query string

	switch objectType {
	case "database":
		query = fmt.Sprintf("SELECT has_database_privilege($1, $2, '%s')", privilege)
	case "schema":
		query = fmt.Sprintf("SELECT has_schema_privilege($1, $2, '%s')", privilege)
	case "table":
		query = fmt.Sprintf("SELECT has_table_privilege($1, $2, '%s')", privilege)
	default:
		t.Fatalf("Unknown object type: %s", objectType)
	}

	err := db.QueryRowContext(ctx, query, roleName, objectName).Scan(&hasPrivilege)
	require.NoError(t, err, "Failed to check privilege")
	assert.True(t, hasPrivilege, "Role %s should have %s privilege on %s %s", roleName, privilege, objectType, objectName)
}
