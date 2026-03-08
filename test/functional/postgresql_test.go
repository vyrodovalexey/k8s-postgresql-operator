//go:build functional

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

package functional_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
	"github.com/vyrodovalexey/k8s-postgresql-operator/test/helpers"
)

func newPGConfig(t *testing.T) *helpers.PostgreSQLTestConfig {
	t.Helper()
	helpers.SkipIfEnvNotSet(t, "TEST_POSTGRES_HOST", "TEST_POSTGRES_USER", "TEST_POSTGRES_PASSWORD")
	return helpers.NewPostgreSQLTestConfig()
}

func testLogger() *zap.SugaredLogger {
	logger, _ := zap.NewDevelopment()
	return logger.Sugar()
}

func TestFunctional_PostgreSQLConnection(t *testing.T) {
	pgCfg := newPGConfig(t)
	log := testLogger()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	connected, err := postgresql.TestConnection(
		ctx, pgCfg.Host, pgCfg.Port, "postgres",
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		log, 3, 2*time.Second,
	)

	require.NoError(t, err, "Connection to PostgreSQL should succeed")
	assert.True(t, connected, "Should be connected to PostgreSQL")
}

func TestFunctional_PostgreSQLCreateDatabase(t *testing.T) {
	pgCfg := newPGConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbName := fmt.Sprintf("func_test_db_%d", time.Now().UnixNano())

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = pgCfg.DropTestDatabase(cleanupCtx, dbName)
	})

	// Create the database (owner = pgCfg.User since it's the admin)
	err := postgresql.CreateOrUpdateDatabase(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		dbName, pgCfg.User, "", "",
	)
	require.NoError(t, err, "Creating database should succeed")

	// Verify the database exists
	exists, err := pgCfg.DatabaseExists(ctx, dbName)
	require.NoError(t, err)
	assert.True(t, exists, "Database should exist after creation")

	// Test idempotency: creating the same database again should succeed
	err = postgresql.CreateOrUpdateDatabase(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		dbName, pgCfg.User, "", "",
	)
	require.NoError(t, err, "Creating the same database again should succeed (idempotent)")
}

func TestFunctional_PostgreSQLCreateUser(t *testing.T) {
	pgCfg := newPGConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	username := fmt.Sprintf("func_test_user_%d", time.Now().UnixNano())
	password := "test_password_123"

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = pgCfg.DropTestUser(cleanupCtx, username)
	})

	err := postgresql.CreateOrUpdateUser(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		username, password,
	)
	require.NoError(t, err, "Creating user should succeed")

	// Verify the user exists
	exists, err := pgCfg.UserExists(ctx, username)
	require.NoError(t, err)
	assert.True(t, exists, "User should exist after creation")

	// Test idempotency: updating the same user should succeed
	err = postgresql.CreateOrUpdateUser(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		username, "new_password_456",
	)
	require.NoError(t, err, "Updating user password should succeed (idempotent)")
}

func TestFunctional_PostgreSQLCreateSchema(t *testing.T) {
	pgCfg := newPGConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbName := fmt.Sprintf("func_test_schema_db_%d", time.Now().UnixNano())
	schemaName := "test_schema"

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = pgCfg.DropTestDatabase(cleanupCtx, dbName)
	})

	// Create the database first
	err := postgresql.CreateOrUpdateDatabase(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		dbName, pgCfg.User, "", "",
	)
	require.NoError(t, err, "Creating database should succeed")

	// Create the schema
	err = postgresql.CreateOrUpdateSchema(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		dbName, schemaName, pgCfg.User,
	)
	require.NoError(t, err, "Creating schema should succeed")

	// Verify the schema exists
	exists, err := pgCfg.SchemaExists(ctx, dbName, schemaName)
	require.NoError(t, err)
	assert.True(t, exists, "Schema should exist after creation")

	// Test idempotency
	err = postgresql.CreateOrUpdateSchema(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		dbName, schemaName, pgCfg.User,
	)
	require.NoError(t, err, "Creating the same schema again should succeed (idempotent)")
}

func TestFunctional_PostgreSQLCreateRoleGroup(t *testing.T) {
	pgCfg := newPGConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	suffix := time.Now().UnixNano()
	groupRole := fmt.Sprintf("func_test_group_%d", suffix)
	member1 := fmt.Sprintf("func_test_member1_%d", suffix)
	member2 := fmt.Sprintf("func_test_member2_%d", suffix)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		// Revoke memberships before dropping roles
		_ = pgCfg.DropTestUser(cleanupCtx, member1)
		_ = pgCfg.DropTestUser(cleanupCtx, member2)
		_ = pgCfg.DropTestUser(cleanupCtx, groupRole)
	})

	// Create member users first
	err := postgresql.CreateOrUpdateUser(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		member1, "pass1",
	)
	require.NoError(t, err, "Creating member1 should succeed")

	err = postgresql.CreateOrUpdateUser(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		member2, "pass2",
	)
	require.NoError(t, err, "Creating member2 should succeed")

	// Create role group with members
	err = postgresql.CreateOrUpdateRoleGroup(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		groupRole, []string{member1, member2},
	)
	require.NoError(t, err, "Creating role group should succeed")

	// Verify group role exists
	exists, err := pgCfg.UserExists(ctx, groupRole)
	require.NoError(t, err)
	assert.True(t, exists, "Group role should exist")

	// Verify memberships
	hasMember1, err := pgCfg.RoleGroupHasMember(ctx, groupRole, member1)
	require.NoError(t, err)
	assert.True(t, hasMember1, "Group should have member1")

	hasMember2, err := pgCfg.RoleGroupHasMember(ctx, groupRole, member2)
	require.NoError(t, err)
	assert.True(t, hasMember2, "Group should have member2")
}

func TestFunctional_PostgreSQLApplyGrants(t *testing.T) {
	pgCfg := newPGConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	suffix := time.Now().UnixNano()
	dbName := fmt.Sprintf("func_test_grants_db_%d", suffix)
	roleName := fmt.Sprintf("func_test_grants_role_%d", suffix)
	schemaName := "test_grants_schema"

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = pgCfg.DropTestDatabase(cleanupCtx, dbName)
		_ = pgCfg.DropTestUser(cleanupCtx, roleName)
	})

	// Create user
	err := postgresql.CreateOrUpdateUser(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		roleName, "grantpass",
	)
	require.NoError(t, err)

	// Create database
	err = postgresql.CreateOrUpdateDatabase(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		dbName, pgCfg.User, "", "",
	)
	require.NoError(t, err)

	// Create schema
	err = postgresql.CreateOrUpdateSchema(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		dbName, schemaName, pgCfg.User,
	)
	require.NoError(t, err)

	// Apply grants
	grants := []instancev1alpha1.GrantItem{
		{
			Type:       instancev1alpha1.GrantTypeDatabase,
			Privileges: []string{"CONNECT"},
		},
		{
			Type:       instancev1alpha1.GrantTypeSchema,
			Schema:     schemaName,
			Privileges: []string{"USAGE"},
		},
		{
			Type:       instancev1alpha1.GrantTypeAllTables,
			Schema:     schemaName,
			Privileges: []string{"SELECT"},
		},
	}

	defaultPrivileges := []instancev1alpha1.DefaultPrivilegeItem{
		{
			ObjectType: "tables",
			Schema:     schemaName,
			Privileges: []string{"SELECT", "INSERT"},
		},
	}

	err = postgresql.ApplyGrants(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		dbName, roleName, grants, defaultPrivileges,
	)
	require.NoError(t, err, "Applying grants should succeed")

	// Verify database CONNECT privilege
	hasConnect, err := pgCfg.RoleHasPrivilege(ctx, dbName, roleName, "CONNECT")
	require.NoError(t, err)
	assert.True(t, hasConnect, "Role should have CONNECT privilege on database")
}

func TestFunctional_PostgreSQLDeleteDatabase(t *testing.T) {
	pgCfg := newPGConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbName := fmt.Sprintf("func_test_delete_db_%d", time.Now().UnixNano())

	// Create the database
	err := postgresql.CreateOrUpdateDatabase(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		dbName, pgCfg.User, "", "",
	)
	require.NoError(t, err, "Creating database should succeed")

	// Verify it exists
	exists, err := pgCfg.DatabaseExists(ctx, dbName)
	require.NoError(t, err)
	require.True(t, exists, "Database should exist before deletion")

	// Delete the database
	err = postgresql.DeleteDatabase(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		dbName,
	)
	require.NoError(t, err, "Deleting database should succeed")

	// Verify it no longer exists
	exists, err = pgCfg.DatabaseExists(ctx, dbName)
	require.NoError(t, err)
	assert.False(t, exists, "Database should not exist after deletion")

	// Test idempotency: deleting a non-existent database should succeed
	err = postgresql.DeleteDatabase(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		dbName,
	)
	require.NoError(t, err, "Deleting non-existent database should succeed (idempotent)")
}

func TestFunctional_PostgreSQLDatabaseWithTemplate(t *testing.T) {
	pgCfg := newPGConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	suffix := time.Now().UnixNano()
	templateDB := fmt.Sprintf("func_test_template_%d", suffix)
	newDB := fmt.Sprintf("func_test_from_template_%d", suffix)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = pgCfg.DropTestDatabase(cleanupCtx, newDB)
		_ = pgCfg.DropTestDatabase(cleanupCtx, templateDB)
	})

	// Create the template database
	err := postgresql.CreateOrUpdateDatabase(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		templateDB, pgCfg.User, "", "",
	)
	require.NoError(t, err, "Creating template database should succeed")

	// Close all connections to the template database (required for using it as template)
	err = postgresql.CloseAllSessions(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		templateDB,
	)
	require.NoError(t, err, "Closing sessions should succeed")

	// Create database from template
	err = postgresql.CreateOrUpdateDatabase(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		newDB, pgCfg.User, "", templateDB,
	)
	require.NoError(t, err, "Creating database from template should succeed")

	// Verify the new database exists
	exists, err := pgCfg.DatabaseExists(ctx, newDB)
	require.NoError(t, err)
	assert.True(t, exists, "Database created from template should exist")
}
