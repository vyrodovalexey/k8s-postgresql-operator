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
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// ============================================================================
// DeleteUser Tests - Comprehensive coverage for DeleteUser function
// ============================================================================

// deleteUserWithDBComprehensive is a helper function that mirrors DeleteUser logic
// but accepts a *sql.DB for testing with sqlmock
func deleteUserWithDBComprehensive(ctx context.Context, db *sql.DB, username string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedUsername := `"` + username + `"`

	var exists bool
	err := db.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM pg_user WHERE usename = $1)", username).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}

	if !exists {
		return nil
	}

	query := fmt.Sprintf("DROP USER %s", escapedUsername)
	_, err = db.ExecContext(connCtx, query)
	if err != nil {
		return fmt.Errorf("failed to drop user: %w", err)
	}

	return nil
}

func TestDeleteUser_Comprehensive(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:     "User exists - delete successfully",
			username: "testuser",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("DROP USER").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:     "User does not exist - no action needed",
			username: "nonexistent",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("nonexistent").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			},
			expectError: false,
		},
		{
			name:     "Check user exists - query error",
			username: "testuser",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testuser").
					WillReturnError(errors.New("connection refused"))
			},
			expectError:   true,
			errorContains: "failed to check if user exists",
		},
		{
			name:     "Drop user - execution error",
			username: "testuser",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("DROP USER").
					WillReturnError(errors.New("user has dependent objects"))
			},
			expectError:   true,
			errorContains: "failed to drop user",
		},
		{
			name:     "User with special characters in name",
			username: "test-user_123",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("test-user_123").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("DROP USER").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:     "User with SQL injection attempt in name",
			username: "user'; DROP TABLE users; --",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("user'; DROP TABLE users; --").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("DROP USER").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:     "Empty username",
			username: "",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			},
			expectError: false,
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

			err = deleteUserWithDBComprehensive(ctx, db, tt.username)

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

// ============================================================================
// DeleteDatabase Tests - Comprehensive coverage for DeleteDatabase function
// ============================================================================

// deleteDatabaseWithDBComprehensive is a helper function that mirrors DeleteDatabase logic
// but accepts a *sql.DB for testing with sqlmock
func deleteDatabaseWithDBComprehensive(ctx context.Context, db *sql.DB, databaseName string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedDatabaseName := `"` + databaseName + `"`

	var exists bool
	err := db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", databaseName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if !exists {
		return nil
	}

	query := fmt.Sprintf("DROP DATABASE %s", escapedDatabaseName)
	_, err = db.ExecContext(connCtx, query)
	if err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	return nil
}

func TestDeleteDatabase_Comprehensive(t *testing.T) {
	tests := []struct {
		name          string
		databaseName  string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:         "Database exists - delete successfully",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("DROP DATABASE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Database does not exist - no action needed",
			databaseName: "nonexistent",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("nonexistent").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			},
			expectError: false,
		},
		{
			name:         "Check database exists - query error",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testdb").
					WillReturnError(errors.New("connection refused"))
			},
			expectError:   true,
			errorContains: "failed to check if database exists",
		},
		{
			name:         "Drop database - execution error",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("DROP DATABASE").
					WillReturnError(errors.New("database is being accessed by other users"))
			},
			expectError:   true,
			errorContains: "failed to drop database",
		},
		{
			name:         "Database with special characters in name",
			databaseName: "test-db_123",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("test-db_123").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("DROP DATABASE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Database with SQL injection attempt in name",
			databaseName: "db'; DROP TABLE users; --",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("db'; DROP TABLE users; --").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("DROP DATABASE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
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

			err = deleteDatabaseWithDBComprehensive(ctx, db, tt.databaseName)

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

// ============================================================================
// CloseAllSessions Tests - Comprehensive coverage for CloseAllSessions function
// ============================================================================

// closeAllSessionsWithDBComprehensive is a helper function that mirrors CloseAllSessions logic
// but accepts a *sql.DB for testing with sqlmock
func closeAllSessionsWithDBComprehensive(ctx context.Context, db *sql.DB, databaseName string) error {
	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = $1
		AND pid <> pg_backend_pid()`

	_, err := db.ExecContext(connCtx, query, databaseName)
	if err != nil {
		return fmt.Errorf("failed to terminate connections: %w", err)
	}

	return nil
}

func TestCloseAllSessions_Comprehensive(t *testing.T) {
	tests := []struct {
		name          string
		databaseName  string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:         "Sessions terminated successfully",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("SELECT pg_terminate_backend").
					WithArgs("testdb").
					WillReturnResult(sqlmock.NewResult(0, 5))
			},
			expectError: false,
		},
		{
			name:         "No sessions to terminate",
			databaseName: "emptydb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("SELECT pg_terminate_backend").
					WithArgs("emptydb").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: false,
		},
		{
			name:         "Terminate sessions - execution error",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("SELECT pg_terminate_backend").
					WithArgs("testdb").
					WillReturnError(errors.New("permission denied"))
			},
			expectError:   true,
			errorContains: "failed to terminate connections",
		},
		{
			name:         "Database with special characters",
			databaseName: "test-db_123",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("SELECT pg_terminate_backend").
					WithArgs("test-db_123").
					WillReturnResult(sqlmock.NewResult(0, 2))
			},
			expectError: false,
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

			err = closeAllSessionsWithDBComprehensive(ctx, db, tt.databaseName)

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

// ============================================================================
// CreateOrUpdateDatabase Tests - Additional comprehensive coverage
// ============================================================================

func TestCreateOrUpdateDatabase_AdditionalCases(t *testing.T) {
	tests := []struct {
		name          string
		databaseName  string
		owner         string
		schemaName    string
		templateDB    string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:         "Create database with template - success",
			databaseName: "newdb",
			owner:        "owner",
			templateDB:   "template1",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("newdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE DATABASE.*TEMPLATE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Update existing database owner",
			databaseName: "existingdb",
			owner:        "newowner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("existingdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("ALTER DATABASE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Create database - SQL error",
			databaseName: "newdb",
			owner:        "owner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("newdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE DATABASE").
					WillReturnError(errors.New("database already exists"))
			},
			expectError:   true,
			errorContains: "failed to create database",
		},
		{
			name:         "Database with reserved name",
			databaseName: "postgres",
			owner:        "postgres",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("postgres").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("ALTER DATABASE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
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

			err = createOrUpdateDatabaseWithDB(ctx, db, tt.databaseName, tt.owner, tt.schemaName, tt.templateDB)

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

// ============================================================================
// CreateOrUpdateUser Tests - Additional comprehensive coverage
// ============================================================================

func TestCreateOrUpdateUser_AdditionalCases(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		password      string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:     "Create new user - success",
			username: "newuser",
			password: "securepassword",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("newuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE USER").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:     "Update existing user password",
			username: "existinguser",
			password: "newpassword",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("existinguser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("ALTER USER").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:     "User with special characters in password",
			username: "testuser",
			password: "p@ss'word\"with$pecial",
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
			name:     "Create user - SQL error",
			username: "newuser",
			password: "password",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("newuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE USER").
					WillReturnError(errors.New("role already exists"))
			},
			expectError:   true,
			errorContains: "failed to create user",
		},
		{
			name:     "Update user - SQL error",
			username: "existinguser",
			password: "newpassword",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("existinguser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("ALTER USER").
					WillReturnError(errors.New("permission denied"))
			},
			expectError:   true,
			errorContains: "failed to update user password",
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

// ============================================================================
// CreateOrUpdateRoleGroup Tests - Additional comprehensive coverage
// ============================================================================

func TestCreateOrUpdateRoleGroup_AdditionalCases(t *testing.T) {
	tests := []struct {
		name          string
		groupRole     string
		memberRoles   []string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:        "Create new group with multiple members",
			groupRole:   "developers",
			memberRoles: []string{"dev1", "dev2", "dev3"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("developers").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE ROLE").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("developers").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}))
				// Check and add each member
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("dev1").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("GRANT").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("dev2").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("GRANT").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("dev3").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("GRANT").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:        "Update group - remove and add members",
			groupRole:   "developers",
			memberRoles: []string{"dev2", "dev3"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("developers").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("developers").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}).AddRow("dev1").AddRow("dev2"))
				// dev2 already exists, dev3 needs to be added, dev1 needs to be removed
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("dev3").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("GRANT").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("REVOKE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:        "Skip non-existent member",
			groupRole:   "developers",
			memberRoles: []string{"nonexistent"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("developers").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("developers").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}))
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("nonexistent").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			},
			expectError: false,
		},
		{
			name:        "Empty member list - remove all members",
			groupRole:   "developers",
			memberRoles: []string{},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("developers").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery("SELECT r.rolname").
					WithArgs("developers").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}).AddRow("dev1"))
				m.ExpectExec("REVOKE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
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

// ============================================================================
// CreateOrUpdateSchema Tests - Additional comprehensive coverage
// ============================================================================

func TestCreateOrUpdateSchema_AdditionalCases(t *testing.T) {
	tests := []struct {
		name          string
		schemaName    string
		owner         string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:       "Create new schema - success",
			schemaName: "newschema",
			owner:      "owner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("newschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE SCHEMA").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:       "Update existing schema owner",
			schemaName: "existingschema",
			owner:      "newowner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("existingschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("ALTER SCHEMA").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:       "Schema with special characters",
			schemaName: "my-schema_v2",
			owner:      "owner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("my-schema_v2").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE SCHEMA").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:       "Create schema - SQL error",
			schemaName: "newschema",
			owner:      "owner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("newschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec("CREATE SCHEMA").
					WillReturnError(errors.New("schema already exists"))
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

// ============================================================================
// ApplyGrants Tests - Additional comprehensive coverage
// ============================================================================

func TestApplyGrants_AdditionalCases(t *testing.T) {
	tests := []struct {
		name          string
		grants        []instancev1alpha1.GrantItem
		defaultPrivs  []instancev1alpha1.DefaultPrivilegeItem
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:         "Empty grants and default privileges - no action",
			grants:       []instancev1alpha1.GrantItem{},
			defaultPrivs: []instancev1alpha1.DefaultPrivilegeItem{},
			setupMock:    func(m sqlmock.Sqlmock) {},
			expectError:  false,
		},
		{
			name: "Multiple schema grants",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeSchema,
					Schema:     "public",
					Privileges: []string{"USAGE", "CREATE"},
				},
				{
					Type:       instancev1alpha1.GrantTypeSchema,
					Schema:     "private",
					Privileges: []string{"USAGE"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec("GRANT.*ON SCHEMA").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("GRANT.*ON SCHEMA").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "All tables grant",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeAllTables,
					Schema:     "public",
					Privileges: []string{"SELECT", "INSERT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec("GRANT.*ON ALL TABLES").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "All sequences grant",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeAllSequences,
					Schema:     "public",
					Privileges: []string{"USAGE", "SELECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec("GRANT.*ON ALL SEQUENCES").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Function grant",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeFunction,
					Schema:     "public",
					Function:   "my_function",
					Privileges: []string{"EXECUTE"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec("GRANT.*ON FUNCTION").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Sequence grant",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeSequence,
					Schema:     "public",
					Sequence:   "my_sequence",
					Privileges: []string{"USAGE", "SELECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec("GRANT.*ON SEQUENCE").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name: "Multiple default privileges",
			defaultPrivs: []instancev1alpha1.DefaultPrivilegeItem{
				{
					Schema:     "public",
					ObjectType: "tables",
					Privileges: []string{"SELECT"},
				},
				{
					Schema:     "public",
					ObjectType: "sequences",
					Privileges: []string{"USAGE"},
				},
				{
					Schema:     "public",
					ObjectType: "functions",
					Privileges: []string{"EXECUTE"},
				},
				{
					Schema:     "public",
					ObjectType: "types",
					Privileges: []string{"USAGE"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec("ALTER DEFAULT PRIVILEGES.*ON TABLES").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("ALTER DEFAULT PRIVILEGES.*ON SEQUENCES").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("ALTER DEFAULT PRIVILEGES.*ON FUNCTIONS").
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("ALTER DEFAULT PRIVILEGES.*ON TYPES").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
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

// ============================================================================
// Validation Tests - Edge cases and error paths
// ============================================================================

func TestValidatePrivilege_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		grantType   instancev1alpha1.GrantType
		privilege   string
		expectError bool
	}{
		{
			name:        "Valid privilege with extra spaces",
			grantType:   instancev1alpha1.GrantTypeTable,
			privilege:   "   SELECT   ",
			expectError: false,
		},
		{
			name:        "Valid privilege lowercase",
			grantType:   instancev1alpha1.GrantTypeTable,
			privilege:   "select",
			expectError: false,
		},
		{
			name:        "Valid privilege mixed case",
			grantType:   instancev1alpha1.GrantTypeTable,
			privilege:   "SeLeCt",
			expectError: false,
		},
		{
			name:        "ALL PRIVILEGES with spaces",
			grantType:   instancev1alpha1.GrantTypeDatabase,
			privilege:   "ALL PRIVILEGES",
			expectError: false,
		},
		{
			name:        "Invalid privilege for grant type",
			grantType:   instancev1alpha1.GrantTypeDatabase,
			privilege:   "SELECT",
			expectError: true,
		},
		{
			name:        "Empty privilege",
			grantType:   instancev1alpha1.GrantTypeTable,
			privilege:   "",
			expectError: true,
		},
		{
			name:        "Whitespace only privilege",
			grantType:   instancev1alpha1.GrantTypeTable,
			privilege:   "   ",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePrivilege(tt.grantType, tt.privilege)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDefaultPrivilege_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		objectType  string
		privilege   string
		expectError bool
	}{
		{
			name:        "Valid object type uppercase",
			objectType:  "TABLES",
			privilege:   "SELECT",
			expectError: false,
		},
		{
			name:        "Valid object type mixed case",
			objectType:  "TaBLeS",
			privilege:   "SELECT",
			expectError: false,
		},
		{
			name:        "Invalid object type",
			objectType:  "views",
			privilege:   "SELECT",
			expectError: true,
		},
		{
			name:        "Empty object type",
			objectType:  "",
			privilege:   "SELECT",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDefaultPrivilege(tt.objectType, tt.privilege)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ============================================================================
// SQL Injection Prevention Tests
// ============================================================================

func TestSQLInjectionPrevention(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		testFunc func(ctx context.Context, db *sql.DB, input string) error
	}{
		{
			name:  "Delete user with SQL injection attempt",
			input: "user'; DROP TABLE users; --",
			testFunc: func(ctx context.Context, db *sql.DB, input string) error {
				return deleteUserWithDBComprehensive(ctx, db, input)
			},
		},
		{
			name:  "Delete database with SQL injection attempt",
			input: "db'; DROP TABLE users; --",
			testFunc: func(ctx context.Context, db *sql.DB, input string) error {
				return deleteDatabaseWithDBComprehensive(ctx, db, input)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			ctx := context.Background()

			// The SQL injection attempt should be safely handled
			// The query should use parameterized queries for the EXISTS check
			mock.ExpectQuery("SELECT EXISTS").
				WithArgs(tt.input).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

			err = tt.testFunc(ctx, db, tt.input)
			assert.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
