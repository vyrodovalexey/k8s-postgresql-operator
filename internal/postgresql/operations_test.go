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

// Table-driven tests for PostgreSQL operations

// TestCreateOrUpdateDatabase_TableDriven tests CreateOrUpdateDatabase with various scenarios
func TestCreateOrUpdateDatabase_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		databaseName  string
		owner         string
		schemaName    string
		templateDB    string
		dbExists      bool
		schemaExists  bool
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:         "Database exists - update owner",
			databaseName: "testdb",
			owner:        "testowner",
			dbExists:     true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("ALTER DATABASE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Database does not exist - create new",
			databaseName: "testdb",
			owner:        "testowner",
			dbExists:     false,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE DATABASE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Create database with template",
			databaseName: "testdb",
			owner:        "testowner",
			templateDB:   "template1",
			dbExists:     false,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE DATABASE.*TEMPLATE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Create database with schema - schema exists",
			databaseName: "testdb",
			owner:        "testowner",
			schemaName:   "myschema",
			dbExists:     false,
			schemaExists: true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE DATABASE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Create database with public schema - should skip",
			databaseName: "testdb",
			owner:        "testowner",
			schemaName:   "public",
			dbExists:     false,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE DATABASE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Create database with empty schema - should skip",
			databaseName: "testdb",
			owner:        "testowner",
			schemaName:   "",
			dbExists:     false,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE DATABASE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Check database exists error",
			databaseName: "testdb",
			owner:        "testowner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WillReturnError(errors.New("query error"))
			},
			expectError:   true,
			errorContains: "failed to check if database exists",
		},
		{
			name:         "Update database owner error",
			databaseName: "testdb",
			owner:        "testowner",
			dbExists:     true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("ALTER DATABASE").
					WillReturnError(errors.New("alter error"))
			},
			expectError:   true,
			errorContains: "failed to update database owner",
		},
		{
			name:         "Create database error",
			databaseName: "testdb",
			owner:        "testowner",
			dbExists:     false,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE DATABASE").
					WillReturnError(errors.New("create error"))
			},
			expectError:   true,
			errorContains: "failed to create database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			ctx := context.Background()

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			var dbSchema *sql.DB
			var mockSchema sqlmock.Sqlmock
			if tt.schemaName != "" && tt.schemaName != "public" {
				dbSchema, mockSchema, err = sqlmock.New()
				require.NoError(t, err)
				defer dbSchema.Close()

				if tt.schemaExists {
					mockSchema.ExpectQuery("SELECT EXISTS").
						WithArgs(tt.schemaName).
						WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
					mockSchema.ExpectExec("ALTER SCHEMA").
						WillReturnResult(sqlmock.NewResult(0, 1))
				} else {
					mockSchema.ExpectQuery("SELECT EXISTS").
						WithArgs(tt.schemaName).
						WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				}

				err = createOrUpdateDatabaseWithDBAndSchemaDB(
					ctx, db, dbSchema, tt.databaseName, tt.owner, tt.schemaName, tt.templateDB)
			} else {
				err = createOrUpdateDatabaseWithDB(
					ctx, db, tt.databaseName, tt.owner, tt.schemaName, tt.templateDB)
			}

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
			if dbSchema != nil {
				assert.NoError(t, mockSchema.ExpectationsWereMet())
			}
		})
	}
}

// TestCreateOrUpdateDatabase_WithSchema_TableDriven tests schema handling scenarios
func TestCreateOrUpdateDatabase_WithSchema_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		schemaName      string
		schemaExists    bool
		schemaCheckErr  error
		schemaUpdateErr error
		expectError     bool
		errorContains   string
	}{
		{
			name:         "Schema exists - update owner",
			schemaName:   "myschema",
			schemaExists: true,
			expectError:  false,
		},
		{
			name:         "Schema does not exist - skip",
			schemaName:   "myschema",
			schemaExists: false,
			expectError:  false,
		},
		{
			name:           "Schema check error",
			schemaName:     "myschema",
			schemaCheckErr: errors.New("check error"),
			expectError:    true,
			errorContains:  "failed to check if schema exists",
		},
		{
			name:            "Schema update error",
			schemaName:      "myschema",
			schemaExists:    true,
			schemaUpdateErr: errors.New("update error"),
			expectError:     true,
			errorContains:   "failed to update schema owner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			dbSchema, mockSchema, err := sqlmock.New()
			require.NoError(t, err)
			defer dbSchema.Close()

			ctx := context.Background()
			databaseName := "testdb"
			owner := "testowner"

			mock.ExpectQuery("SELECT EXISTS").
				WithArgs(databaseName).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			mock.ExpectExec("CREATE DATABASE").
				WillReturnResult(sqlmock.NewResult(0, 1))

			if tt.schemaCheckErr != nil {
				mockSchema.ExpectQuery("SELECT EXISTS").
					WithArgs(tt.schemaName).
					WillReturnError(tt.schemaCheckErr)
			} else {
				mockSchema.ExpectQuery("SELECT EXISTS").
					WithArgs(tt.schemaName).
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tt.schemaExists))

				if tt.schemaExists {
					if tt.schemaUpdateErr != nil {
						mockSchema.ExpectExec("ALTER SCHEMA").
							WillReturnError(tt.schemaUpdateErr)
					} else {
						mockSchema.ExpectExec("ALTER SCHEMA").
							WillReturnResult(sqlmock.NewResult(0, 1))
					}
				}
			}

			err = createOrUpdateDatabaseWithDBAndSchemaDB(
				ctx, db, dbSchema, databaseName, owner, tt.schemaName, "")

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
			assert.NoError(t, mockSchema.ExpectationsWereMet())
		})
	}
}

// TestCreateOrUpdateUser_TableDriven tests CreateOrUpdateUser with various scenarios
func TestCreateOrUpdateUser_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		password      string
		userExists    bool
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:       "User exists - update password",
			username:   "testuser",
			password:   "newpass",
			userExists: true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("ALTER USER").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:       "User does not exist - create new",
			username:   "testuser",
			password:   "testpass",
			userExists: false,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE USER").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:     "Check user exists error",
			username: "testuser",
			password: "testpass",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testuser").
					WillReturnError(errors.New("query error"))
			},
			expectError:   true,
			errorContains: "failed to check if user exists",
		},
		{
			name:       "Update password error",
			username:   "testuser",
			password:   "newpass",
			userExists: true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("ALTER USER").
					WillReturnError(errors.New("alter error"))
			},
			expectError:   true,
			errorContains: "failed to update user password",
		},
		{
			name:       "Create user error",
			username:   "testuser",
			password:   "testpass",
			userExists: false,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE USER").
					WillReturnError(errors.New("create error"))
			},
			expectError:   true,
			errorContains: "failed to create user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			ctx := context.Background()

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			err = createOrUpdateUserWithDB(ctx, db, tt.username, tt.password)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestCreateOrUpdateRoleGroup_TableDriven tests CreateOrUpdateRoleGroup with various scenarios
func TestCreateOrUpdateRoleGroup_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		groupRole      string
		memberRoles    []string
		groupExists    bool
		currentMembers []string
		memberExists   map[string]bool
		setupMock      func(sqlmock.Sqlmock)
		expectError    bool
		errorContains  string
	}{
		{
			name:        "Group does not exist - create and add members",
			groupRole:   "testgroup",
			memberRoles: []string{"member1", "member2"},
			groupExists: false,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE ROLE").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}))
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("member1").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("GRANT").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("member2").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("GRANT").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:           "Group exists - add new members",
			groupRole:      "testgroup",
			memberRoles:    []string{"member1"},
			groupExists:    true,
			currentMembers: []string{},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}))
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("member1").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("GRANT").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:           "Remove members",
			groupRole:      "testgroup",
			memberRoles:    []string{},
			groupExists:    true,
			currentMembers: []string{"oldmember"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}).AddRow("oldmember"))
				m.ExpectExec("REVOKE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:        "Member does not exist - skip",
			groupRole:   "testgroup",
			memberRoles: []string{"nonexistent"},
			groupExists: true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}))
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("nonexistent").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			},
			expectError: false,
		},
		{
			name:        "Check group exists error",
			groupRole:   "testgroup",
			memberRoles: []string{},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testgroup").
					WillReturnError(errors.New("query error"))
			},
			expectError:   true,
			errorContains: "failed to check if group role exists",
		},
		{
			name:        "Create group error",
			groupRole:   "testgroup",
			memberRoles: []string{},
			groupExists: false,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE ROLE").
					WillReturnError(errors.New("create error"))
			},
			expectError:   true,
			errorContains: "failed to create group role",
		},
		{
			name:        "Query members error",
			groupRole:   "testgroup",
			memberRoles: []string{},
			groupExists: true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("testgroup").
					WillReturnError(errors.New("query error"))
			},
			expectError:   true,
			errorContains: "failed to query current members",
		},
		{
			name:        "Scan member error",
			groupRole:   "testgroup",
			memberRoles: []string{},
			groupExists: true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				rows := sqlmock.NewRows([]string{"rolname"}).AddRow("member1")
				rows = rows.RowError(0, errors.New("scan error"))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("testgroup").
					WillReturnRows(rows)
			},
			expectError:   true,
			errorContains: "error iterating members",
		},
		{
			name:        "Rows iteration error",
			groupRole:   "testgroup",
			memberRoles: []string{},
			groupExists: true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				rows := sqlmock.NewRows([]string{"rolname"}).AddRow("member1").
					CloseError(errors.New("rows error"))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("testgroup").
					WillReturnRows(rows)
			},
			expectError:   true,
			errorContains: "error iterating members",
		},
		{
			name:        "Check member exists error",
			groupRole:   "testgroup",
			memberRoles: []string{"member1"},
			groupExists: true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}))
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("member1").
					WillReturnError(errors.New("check error"))
			},
			expectError:   true,
			errorContains: "failed to check if member role exists",
		},
		{
			name:           "Add member error",
			groupRole:      "testgroup",
			memberRoles:    []string{"member1"},
			groupExists:    true,
			currentMembers: []string{},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}))
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("member1").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("GRANT").
					WillReturnError(errors.New("grant error"))
			},
			expectError:   true,
			errorContains: "failed to add member",
		},
		{
			name:           "Remove member error",
			groupRole:      "testgroup",
			memberRoles:    []string{},
			groupExists:    true,
			currentMembers: []string{"oldmember"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}).AddRow("oldmember"))
				m.ExpectExec("REVOKE").
					WillReturnError(errors.New("revoke error"))
			},
			expectError:   true,
			errorContains: "failed to remove member",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			ctx := context.Background()

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			err = createOrUpdateRoleGroupWithDB(ctx, db, tt.groupRole, tt.memberRoles)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestCreateOrUpdateSchema_TableDriven tests CreateOrUpdateSchema with various scenarios
func TestCreateOrUpdateSchema_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		schemaName    string
		owner         string
		schemaExists  bool
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:         "Schema exists - update owner",
			schemaName:   "testschema",
			owner:        "testowner",
			schemaExists: true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("ALTER SCHEMA").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Schema does not exist - create new",
			schemaName:   "testschema",
			owner:        "testowner",
			schemaExists: false,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE SCHEMA").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:       "Check schema exists error",
			schemaName: "testschema",
			owner:      "testowner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testschema").
					WillReturnError(errors.New("query error"))
			},
			expectError:   true,
			errorContains: "failed to check if schema exists",
		},
		{
			name:         "Update schema owner error",
			schemaName:   "testschema",
			owner:        "testowner",
			schemaExists: true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("ALTER SCHEMA").
					WillReturnError(errors.New("alter error"))
			},
			expectError:   true,
			errorContains: "failed to update schema owner",
		},
		{
			name:         "Create schema error",
			schemaName:   "testschema",
			owner:        "testowner",
			schemaExists: false,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE SCHEMA").
					WillReturnError(errors.New("create error"))
			},
			expectError:   true,
			errorContains: "failed to create schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			ctx := context.Background()

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			err = createOrUpdateSchemaWithDB(ctx, db, tt.schemaName, tt.owner)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestApplyGrantItem_TableDriven tests applyGrantItem with all grant types
func TestApplyGrantItem_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		grant         instancev1alpha1.GrantItem
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name: "Schema grant - success",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeSchema,
				Schema:     "public",
				Privileges: []string{"USAGE", "CREATE"},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("GRANT.*ON SCHEMA").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Table grant - success",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeTable,
				Schema:     "public",
				Table:      "users",
				Privileges: []string{"SELECT", "INSERT"},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("GRANT.*ON TABLE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Sequence grant - success",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeSequence,
				Schema:     "public",
				Sequence:   "myseq",
				Privileges: []string{"SELECT", "USAGE"},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("GRANT.*ON SEQUENCE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Function grant - success",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeFunction,
				Schema:     "public",
				Function:   "myfunc",
				Privileges: []string{"EXECUTE"},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("GRANT.*ON FUNCTION").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "All tables grant - success",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeAllTables,
				Schema:     "public",
				Privileges: []string{"SELECT", "INSERT"},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("GRANT.*ON ALL TABLES").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "All sequences grant - success",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeAllSequences,
				Schema:     "public",
				Privileges: []string{"SELECT", "USAGE"},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("GRANT.*ON ALL SEQUENCES").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Schema grant - missing schema",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeSchema,
				Privileges: []string{"USAGE"},
			},
			expectError:   true,
			errorContains: "schema is required",
		},
		{
			name: "Table grant - missing schema",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeTable,
				Table:      "users",
				Privileges: []string{"SELECT"},
			},
			expectError:   true,
			errorContains: "schema is required",
		},
		{
			name: "Table grant - missing table",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeTable,
				Schema:     "public",
				Privileges: []string{"SELECT"},
			},
			expectError:   true,
			errorContains: "table is required",
		},
		{
			name: "Sequence grant - missing schema",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeSequence,
				Sequence:   "myseq",
				Privileges: []string{"SELECT"},
			},
			expectError:   true,
			errorContains: "schema is required",
		},
		{
			name: "Sequence grant - missing sequence",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeSequence,
				Schema:     "public",
				Privileges: []string{"SELECT"},
			},
			expectError:   true,
			errorContains: "sequence is required",
		},
		{
			name: "Function grant - missing schema",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeFunction,
				Function:   "myfunc",
				Privileges: []string{"EXECUTE"},
			},
			expectError:   true,
			errorContains: "schema is required",
		},
		{
			name: "Function grant - missing function",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeFunction,
				Schema:     "public",
				Privileges: []string{"EXECUTE"},
			},
			expectError:   true,
			errorContains: "function is required",
		},
		{
			name: "All tables grant - missing schema",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeAllTables,
				Privileges: []string{"SELECT"},
			},
			expectError:   true,
			errorContains: "schema is required",
		},
		{
			name: "All sequences grant - missing schema",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeAllSequences,
				Privileges: []string{"SELECT"},
			},
			expectError:   true,
			errorContains: "schema is required",
		},
		{
			name: "Invalid grant type",
			grant: instancev1alpha1.GrantItem{
				Type: "invalid_type",
			},
			expectError:   true,
			errorContains: "unknown grant type",
		},
		{
			name: "Exec error",
			grant: instancev1alpha1.GrantItem{
				Type:       instancev1alpha1.GrantTypeSchema,
				Schema:     "public",
				Privileges: []string{"USAGE"},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("GRANT.*ON SCHEMA").
					WillReturnError(errors.New("exec error"))
			},
			expectError:   true,
			errorContains: "failed to execute grant query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			ctx := context.Background()
			escapedRole := "\"testrole\""

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			err = applyGrantItem(ctx, db, escapedRole, tt.grant)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestApplyDefaultPrivilege_TableDriven tests applyDefaultPrivilege with all object types
func TestApplyDefaultPrivilege_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		defaultPriv   instancev1alpha1.DefaultPrivilegeItem
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name: "Tables - success",
			defaultPriv: instancev1alpha1.DefaultPrivilegeItem{
				Schema:     "public",
				ObjectType: "tables",
				Privileges: []string{"SELECT", "INSERT"},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("ALTER DEFAULT PRIVILEGES.*ON TABLES").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Sequences - success",
			defaultPriv: instancev1alpha1.DefaultPrivilegeItem{
				Schema:     "public",
				ObjectType: "sequences",
				Privileges: []string{"SELECT", "USAGE"},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("ALTER DEFAULT PRIVILEGES.*ON SEQUENCES").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Functions - success",
			defaultPriv: instancev1alpha1.DefaultPrivilegeItem{
				Schema:     "public",
				ObjectType: "functions",
				Privileges: []string{"EXECUTE"},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("ALTER DEFAULT PRIVILEGES.*ON FUNCTIONS").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Types - success",
			defaultPriv: instancev1alpha1.DefaultPrivilegeItem{
				Schema:     "public",
				ObjectType: "types",
				Privileges: []string{"USAGE"},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("ALTER DEFAULT PRIVILEGES.*ON TYPES").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Invalid object type",
			defaultPriv: instancev1alpha1.DefaultPrivilegeItem{
				Schema:     "public",
				ObjectType: "invalid_type",
				Privileges: []string{"SELECT"},
			},
			expectError:   true,
			errorContains: "unknown default privilege object type",
		},
		{
			name: "Exec error",
			defaultPriv: instancev1alpha1.DefaultPrivilegeItem{
				Schema:     "public",
				ObjectType: "tables",
				Privileges: []string{"SELECT"},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("ALTER DEFAULT PRIVILEGES").
					WillReturnError(errors.New("exec error"))
			},
			expectError:   true,
			errorContains: "failed to execute default privilege query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			ctx := context.Background()
			escapedRole := "\"testrole\""

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			err = applyDefaultPrivilege(ctx, db, escapedRole, tt.defaultPriv)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestApplyGrants_TableDriven tests ApplyGrants with various scenarios
func TestApplyGrants_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		grants        []instancev1alpha1.GrantItem
		defaultPrivs  []instancev1alpha1.DefaultPrivilegeItem
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name: "Database grants only",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeDatabase,
					Privileges: []string{"CONNECT", "CREATE"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec("GRANT.*ON DATABASE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Other grants only",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeTable,
					Schema:     "public",
					Table:      "users",
					Privileges: []string{"SELECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec("GRANT.*ON TABLE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Default privileges only",
			defaultPrivs: []instancev1alpha1.DefaultPrivilegeItem{
				{
					Schema:     "public",
					ObjectType: "tables",
					Privileges: []string{"SELECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec("ALTER DEFAULT PRIVILEGES").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Mixed grants",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeDatabase,
					Privileges: []string{"CONNECT"},
				},
				{
					Type:       instancev1alpha1.GrantTypeTable,
					Schema:     "public",
					Table:      "users",
					Privileges: []string{"SELECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec("GRANT.*ON DATABASE").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectPing()
				m.ExpectExec("GRANT.*ON TABLE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Database grants ping error",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeDatabase,
					Privileges: []string{"CONNECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing().
					WillReturnError(errors.New("ping error"))
			},
			expectError:   true,
			errorContains: "failed to ping",
		},
		{
			name: "Database grant exec error",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeDatabase,
					Privileges: []string{"CONNECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec("GRANT.*ON DATABASE").
					WillReturnError(errors.New("grant error"))
			},
			expectError:   true,
			errorContains: "failed to apply database grant",
		},
		{
			name: "Other grants ping error",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeTable,
					Schema:     "public",
					Table:      "users",
					Privileges: []string{"SELECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing().
					WillReturnError(errors.New("ping error"))
			},
			expectError:   true,
			errorContains: "failed to ping",
		},
		{
			name: "Grant item error",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeTable,
					Schema:     "public",
					Table:      "users",
					Privileges: []string{"SELECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec("GRANT.*ON TABLE").
					WillReturnError(errors.New("grant error"))
			},
			expectError:   true,
			errorContains: "failed to apply grant",
		},
		{
			name: "Default privilege error",
			defaultPrivs: []instancev1alpha1.DefaultPrivilegeItem{
				{
					Schema:     "public",
					ObjectType: "tables",
					Privileges: []string{"SELECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec("ALTER DEFAULT PRIVILEGES").
					WillReturnError(errors.New("alter error"))
			},
			expectError:   true,
			errorContains: "failed to apply default privilege",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
			require.NoError(t, err)
			defer db.Close()

			ctx := context.Background()
			databaseName := "testdb"
			roleName := "testrole"

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			err = applyGrantsWithDB(ctx, db, databaseName, roleName, tt.grants, tt.defaultPrivs)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
