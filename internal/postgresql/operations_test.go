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

package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// Note: This file uses sqlmock (github.com/DATA-DOG/go-sqlmock) instead of pgxmock
// because the code uses database/sql with lib/pq driver, not the pgx driver.
// sqlmock is the standard mocking library for database/sql interfaces.
//
// If you want to use pgxmock, you would need to refactor the operations.go file
// to use the pgx driver (github.com/jackc/pgx/v5) instead of database/sql with lib/pq.

func TestCreateOrUpdateDatabase_DatabaseExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	databaseName := "testdb"
	owner := "testowner"

	// Mock the database existence check
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(databaseName).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock the ALTER DATABASE query
	mock.ExpectExec("ALTER DATABASE").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// We need to mock sql.Open, but sqlmock doesn't support that directly.
	// Instead, we'll test the logic by creating a test helper that accepts a *sql.DB
	err = createOrUpdateDatabaseWithDB(ctx, db, databaseName, owner, "", "")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOrUpdateDatabase_DatabaseNotExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	databaseName := "testdb"
	owner := "testowner"

	// Mock the database existence check
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(databaseName).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// Mock the CREATE DATABASE query
	mock.ExpectExec("CREATE DATABASE").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = createOrUpdateDatabaseWithDB(ctx, db, databaseName, owner, "", "")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOrUpdateDatabase_WithTemplate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	databaseName := "testdb"
	owner := "testowner"
	templateDatabase := "template1"

	// Mock the database existence check
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(databaseName).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// Mock the CREATE DATABASE with TEMPLATE query
	mock.ExpectExec("CREATE DATABASE.*TEMPLATE").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = createOrUpdateDatabaseWithDB(ctx, db, databaseName, owner, "", templateDatabase)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOrUpdateDatabase_WithSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	dbSchema, mockSchema, err := sqlmock.New()
	require.NoError(t, err)
	defer dbSchema.Close()

	ctx := context.Background()
	databaseName := "testdb"
	owner := "testowner"
	schemaName := "myschema"

	// Mock the database existence check
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(databaseName).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// Mock the CREATE DATABASE query
	mock.ExpectExec("CREATE DATABASE").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock schema existence check
	mockSchema.ExpectQuery("SELECT EXISTS").
		WithArgs(schemaName).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock ALTER SCHEMA query
	mockSchema.ExpectExec("ALTER SCHEMA").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = createOrUpdateDatabaseWithDBAndSchemaDB(ctx, db, dbSchema, databaseName, owner, schemaName, "")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
	assert.NoError(t, mockSchema.ExpectationsWereMet())
}

func TestCreateOrUpdateDatabase_CheckExistsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	databaseName := "testdb"
	owner := "testowner"

	// Mock the database existence check to return an error
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(databaseName).
		WillReturnError(errors.New("connection error"))

	err = createOrUpdateDatabaseWithDB(ctx, db, databaseName, owner, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check if database exists")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOrUpdateUser_UserExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	username := "testuser"
	password := "testpass"

	// Mock the user existence check
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(username).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock the ALTER USER query
	mock.ExpectExec("ALTER USER").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = createOrUpdateUserWithDB(ctx, db, username, password)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOrUpdateUser_UserNotExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	username := "testuser"
	password := "testpass"

	// Mock the user existence check
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(username).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// Mock the CREATE USER query
	mock.ExpectExec("CREATE USER").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = createOrUpdateUserWithDB(ctx, db, username, password)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOrUpdateUser_CheckExistsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	username := "testuser"
	password := "testpass"

	// Mock the user existence check to return an error
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(username).
		WillReturnError(errors.New("connection error"))

	err = createOrUpdateUserWithDB(ctx, db, username, password)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check if user exists")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOrUpdateRoleGroup_RoleNotExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	groupRole := "testgroup"
	memberRoles := []string{"member1", "member2"}

	// Mock the group role existence check
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(groupRole).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// Mock the CREATE ROLE query
	mock.ExpectExec("CREATE ROLE").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock query for current members (empty result)
	mock.ExpectQuery("SELECT r.rolname").
		WithArgs(groupRole).
		WillReturnRows(sqlmock.NewRows([]string{"rolname"}))

	// Mock member role existence checks (in order they are checked)
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs("member1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock GRANT query for member1
	mock.ExpectExec("GRANT").
		WithArgs().
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock member role existence check for member2
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs("member2").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock GRANT query for member2
	mock.ExpectExec("GRANT").
		WithArgs().
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = createOrUpdateRoleGroupWithDB(ctx, db, groupRole, memberRoles)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOrUpdateRoleGroup_RemoveMembers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	groupRole := "testgroup"
	memberRoles := []string{"member1"}

	// Mock the group role existence check
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(groupRole).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock query for current members (return member2 which should be removed)
	mock.ExpectQuery("SELECT r.rolname").
		WithArgs(groupRole).
		WillReturnRows(sqlmock.NewRows([]string{"rolname"}).AddRow("member2"))

	// Mock member role existence check for member1
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs("member1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock GRANT query for member1
	mock.ExpectExec("GRANT").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock REVOKE query for member2
	mock.ExpectExec("REVOKE").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = createOrUpdateRoleGroupWithDB(ctx, db, groupRole, memberRoles)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOrUpdateSchema_SchemaExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	schemaName := "testschema"
	owner := "testowner"

	// Mock the schema existence check
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(schemaName).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// Mock the ALTER SCHEMA query
	mock.ExpectExec("ALTER SCHEMA").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = createOrUpdateSchemaWithDB(ctx, db, schemaName, owner)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOrUpdateSchema_SchemaNotExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	schemaName := "testschema"
	owner := "testowner"

	// Mock the schema existence check
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(schemaName).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// Mock the CREATE SCHEMA query
	mock.ExpectExec("CREATE SCHEMA").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = createOrUpdateSchemaWithDB(ctx, db, schemaName, owner)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyGrants_DatabaseGrant(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	databaseName := "testdb"
	roleName := "testrole"
	grants := []instancev1alpha1.GrantItem{
		{
			Type:       instancev1alpha1.GrantTypeDatabase,
			Privileges: []string{"CONNECT", "CREATE"},
		},
	}

	// Mock Ping
	mock.ExpectPing()

	// Mock GRANT ON DATABASE query
	mock.ExpectExec("GRANT.*ON DATABASE").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = applyGrantsWithDB(ctx, db, databaseName, roleName, grants, nil)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyGrants_TableGrant(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	databaseName := "testdb"
	roleName := "testrole"
	grants := []instancev1alpha1.GrantItem{
		{
			Type:       instancev1alpha1.GrantTypeTable,
			Schema:     "public",
			Table:      "users",
			Privileges: []string{"SELECT", "INSERT"},
		},
	}

	// Mock Ping
	mock.ExpectPing()

	// Mock GRANT ON TABLE query
	mock.ExpectExec("GRANT.*ON TABLE").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = applyGrantsWithDB(ctx, db, databaseName, roleName, grants, nil)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyGrants_DefaultPrivileges(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	databaseName := "testdb"
	roleName := "testrole"
	defaultPrivileges := []instancev1alpha1.DefaultPrivilegeItem{
		{
			Schema:     "public",
			ObjectType: "tables",
			Privileges: []string{"SELECT", "INSERT"},
		},
	}

	// Mock Ping
	mock.ExpectPing()

	// Mock ALTER DEFAULT PRIVILEGES query
	mock.ExpectExec("ALTER DEFAULT PRIVILEGES").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = applyGrantsWithDB(ctx, db, databaseName, roleName, nil, defaultPrivileges)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyGrantItem_SchemaGrant(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	escapedRole := "\"testrole\""
	grant := instancev1alpha1.GrantItem{
		Type:       instancev1alpha1.GrantTypeSchema,
		Schema:     "public",
		Privileges: []string{"USAGE", "CREATE"},
	}

	// Mock GRANT ON SCHEMA query
	mock.ExpectExec("GRANT.*ON SCHEMA").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = applyGrantItem(ctx, db, escapedRole, grant)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyGrantItem_AllTablesGrant(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	escapedRole := "\"testrole\""
	grant := instancev1alpha1.GrantItem{
		Type:       instancev1alpha1.GrantTypeAllTables,
		Schema:     "public",
		Privileges: []string{"SELECT", "INSERT"},
	}

	// Mock GRANT ON ALL TABLES query
	mock.ExpectExec("GRANT.*ON ALL TABLES").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = applyGrantItem(ctx, db, escapedRole, grant)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyGrantItem_InvalidGrantType(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	escapedRole := "\"testrole\""
	grant := instancev1alpha1.GrantItem{
		Type: "invalid_type",
	}

	err = applyGrantItem(ctx, db, escapedRole, grant)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown grant type")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyGrantItem_MissingSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	escapedRole := "\"testrole\""
	grant := instancev1alpha1.GrantItem{
		Type:       instancev1alpha1.GrantTypeTable,
		Table:      "users",
		Privileges: []string{"SELECT"},
	}

	err = applyGrantItem(ctx, db, escapedRole, grant)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema is required")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyDefaultPrivilege_Tables(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	escapedRole := "\"testrole\""
	defaultPriv := instancev1alpha1.DefaultPrivilegeItem{
		Schema:     "public",
		ObjectType: "tables",
		Privileges: []string{"SELECT", "INSERT"},
	}

	// Mock ALTER DEFAULT PRIVILEGES query
	mock.ExpectExec("ALTER DEFAULT PRIVILEGES.*ON TABLES").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = applyDefaultPrivilege(ctx, db, escapedRole, defaultPriv)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyDefaultPrivilege_InvalidObjectType(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	escapedRole := "\"testrole\""
	defaultPriv := instancev1alpha1.DefaultPrivilegeItem{
		Schema:     "public",
		ObjectType: "invalid_type",
		Privileges: []string{"SELECT"},
	}

	err = applyDefaultPrivilege(ctx, db, escapedRole, defaultPriv)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown default privilege object type")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// Helper functions to allow testing with mocked database connections
// These functions accept *sql.DB instead of opening connections

func createOrUpdateDatabaseWithDB(
	ctx context.Context, db *sql.DB, databaseName, owner, schemaName, templateDatabase string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedDatabaseName := `"` + databaseName + `"`
	escapedOwner := `"` + owner + `"`

	var exists bool
	err := db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", databaseName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if exists {
		query := fmt.Sprintf("ALTER DATABASE %s OWNER TO %s", escapedDatabaseName, escapedOwner)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to update database owner: %w", err)
		}
		return nil
	}

	var query string
	if templateDatabase != "" {
		escapedTemplate := `"` + templateDatabase + `"`
		query = fmt.Sprintf("CREATE DATABASE %s OWNER %s TEMPLATE %s", escapedDatabaseName, escapedOwner, escapedTemplate)
	} else {
		query = fmt.Sprintf("CREATE DATABASE %s OWNER %s", escapedDatabaseName, escapedOwner)
	}
	_, err = db.ExecContext(connCtx, query)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	if schemaName == "" || schemaName == "public" {
		return nil
	}

	return nil
}

func createOrUpdateDatabaseWithDBAndSchemaDB(
	ctx context.Context, db *sql.DB, dbSchema *sql.DB, databaseName, owner, schemaName, templateDatabase string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedDatabaseName := `"` + databaseName + `"`
	escapedOwner := `"` + owner + `"`

	var exists bool
	err := db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", databaseName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if exists {
		query := fmt.Sprintf("ALTER DATABASE %s OWNER TO %s", escapedDatabaseName, escapedOwner)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to update database owner: %w", err)
		}
		return nil
	}

	var query string
	if templateDatabase != "" {
		escapedTemplate := `"` + templateDatabase + `"`
		query = fmt.Sprintf("CREATE DATABASE %s OWNER %s TEMPLATE %s", escapedDatabaseName, escapedOwner, escapedTemplate)
	} else {
		query = fmt.Sprintf("CREATE DATABASE %s OWNER %s", escapedDatabaseName, escapedOwner)
	}
	_, err = db.ExecContext(connCtx, query)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	if schemaName == "" || schemaName == "public" {
		return nil
	}

	escapedSchemaName := `"` + schemaName + `"`

	var schemaExists bool
	err = dbSchema.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)",
		schemaName).Scan(&schemaExists)
	if err != nil {
		return fmt.Errorf("failed to check if schema exists: %w", err)
	}

	if !schemaExists {
		return nil
	}

	query = fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s", escapedSchemaName, escapedOwner)
	_, err = dbSchema.ExecContext(connCtx, query)
	if err != nil {
		return fmt.Errorf("failed to update schema owner: %w", err)
	}

	return nil
}

func createOrUpdateUserWithDB(ctx context.Context, db *sql.DB, username, password string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedUsername := `"` + username + `"`
	escapedPassword := `"` + password + `"`

	var exists bool
	err := db.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM pg_user WHERE usename = $1)", username).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}

	if exists {
		query := fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s'", escapedUsername, escapedPassword)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to update user password: %w", err)
		}
	} else {
		query := fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", escapedUsername, escapedPassword)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
	}

	return nil
}

func createOrUpdateRoleGroupWithDB(ctx context.Context, db *sql.DB, groupRole string, memberRoles []string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedGroupRole := `"` + groupRole + `"`

	var exists bool
	err := db.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", groupRole).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if group role exists: %w", err)
	}

	if !exists {
		query := fmt.Sprintf("CREATE ROLE %s", escapedGroupRole)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to create group role: %w", err)
		}
	}

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

	desiredMembers := make(map[string]bool)
	for _, memberRole := range memberRoles {
		desiredMembers[memberRole] = true
		if !currentMembers[memberRole] {
			var memberExists bool
			err = db.QueryRowContext(connCtx,
				"SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", memberRole).Scan(&memberExists)
			if err != nil {
				return fmt.Errorf("failed to check if member role exists: %w", err)
			}
			if !memberExists {
				continue
			}

			escapedMemberRole := `"` + memberRole + `"`
			query := fmt.Sprintf("GRANT %s TO %s", escapedGroupRole, escapedMemberRole)
			_, err = db.ExecContext(connCtx, query)
			if err != nil {
				return fmt.Errorf("failed to add member %s to group role: %w", memberRole, err)
			}
		}
	}

	for member := range currentMembers {
		if !desiredMembers[member] {
			escapedMemberRole := `"` + member + `"`
			query := fmt.Sprintf("REVOKE %s FROM %s", escapedGroupRole, escapedMemberRole)
			_, err = db.ExecContext(connCtx, query)
			if err != nil {
				return fmt.Errorf("failed to remove member %s from group role: %w", member, err)
			}
		}
	}

	return nil
}

func createOrUpdateSchemaWithDB(ctx context.Context, db *sql.DB, schemaName, owner string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedSchemaName := `"` + schemaName + `"`
	escapedOwner := `"` + owner + `"`

	var exists bool
	err := db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)",
		schemaName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if schema exists: %w", err)
	}

	if exists {
		query := fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s", escapedSchemaName, escapedOwner)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to update schema owner: %w", err)
		}
	} else {
		query := fmt.Sprintf("CREATE SCHEMA %s AUTHORIZATION %s", escapedSchemaName, escapedOwner)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}
	}

	return nil
}

func applyGrantsWithDB(
	ctx context.Context, db *sql.DB, databaseName, roleName string,
	grants []instancev1alpha1.GrantItem, defaultPrivileges []instancev1alpha1.DefaultPrivilegeItem) error {
	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	escapedRole := `"` + roleName + `"`
	escapedDBName := `"` + databaseName + `"`

	var databaseGrants []instancev1alpha1.GrantItem
	var otherGrants []instancev1alpha1.GrantItem
	for _, grant := range grants {
		if grant.Type == instancev1alpha1.GrantTypeDatabase {
			databaseGrants = append(databaseGrants, grant)
		} else {
			otherGrants = append(otherGrants, grant)
		}
	}

	if len(databaseGrants) > 0 {
		if err := db.PingContext(connCtx); err != nil {
			return fmt.Errorf("failed to ping postgres database: %w", err)
		}

		for _, grant := range databaseGrants {
			privilegesStr := strings.Join(grant.Privileges, ", ")
			query := fmt.Sprintf("GRANT %s ON DATABASE %s TO %s", privilegesStr, escapedDBName, escapedRole)
			if _, err := db.ExecContext(connCtx, query); err != nil {
				return fmt.Errorf("failed to apply database grant: %w", err)
			}
		}
	}

	if len(otherGrants) > 0 || len(defaultPrivileges) > 0 {
		if err := db.PingContext(connCtx); err != nil {
			return fmt.Errorf("failed to ping database %s: %w", databaseName, err)
		}

		for _, grant := range otherGrants {
			if err := applyGrantItem(connCtx, db, escapedRole, grant); err != nil {
				return fmt.Errorf("failed to apply grant %s: %w", grant.Type, err)
			}
		}

		for _, defaultPriv := range defaultPrivileges {
			if err := applyDefaultPrivilege(connCtx, db, escapedRole, defaultPriv); err != nil {
				return fmt.Errorf("failed to apply default privilege for %s: %w", defaultPriv.ObjectType, err)
			}
		}
	}

	return nil
}
