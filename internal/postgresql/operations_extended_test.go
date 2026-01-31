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
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// ============================================================================
// DeleteUser Extended Tests - Testing actual function behavior
// ============================================================================

// TestDeleteUser_ActualFunction tests the actual DeleteUser function
// These tests will fail at connection time but still cover the initial code paths
func TestDeleteUser_ActualFunction(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		port          int32
		adminUser     string
		adminPassword string
		sslMode       string
		username      string
		expectError   bool
	}{
		{
			name:          "Invalid host - connection fails",
			host:          "invalid-host-that-does-not-exist",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			username:      "testuser",
			expectError:   true,
		},
		{
			name:          "Empty host - connection fails",
			host:          "",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			username:      "testuser",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := DeleteUser(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode, tt.username)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestDeleteUser_WithMockedDB tests DeleteUser logic with mocked database
func TestDeleteUser_WithMockedDB(t *testing.T) {
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
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_user WHERE usename = \$1\)`).
					WithArgs("testuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`DROP USER "testuser"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:     "User does not exist - no action",
			username: "nonexistent",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_user WHERE usename = \$1\)`).
					WithArgs("nonexistent").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			},
			expectError: false,
		},
		{
			name:     "Check user exists - SQL error",
			username: "testuser",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_user WHERE usename = \$1\)`).
					WithArgs("testuser").
					WillReturnError(errors.New("connection refused"))
			},
			expectError:   true,
			errorContains: "failed to check if user exists",
		},
		{
			name:     "Drop user - SQL error",
			username: "testuser",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_user WHERE usename = \$1\)`).
					WithArgs("testuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`DROP USER "testuser"`).
					WillReturnError(errors.New("user has dependent objects"))
			},
			expectError:   true,
			errorContains: "failed to drop user",
		},
		{
			name:     "User with special characters",
			username: "test-user_123",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_user WHERE usename = \$1\)`).
					WithArgs("test-user_123").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`DROP USER "test-user_123"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:     "User with quotes in name - SQL injection prevention",
			username: `user"injection`,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_user WHERE usename = \$1\)`).
					WithArgs(`user"injection`).
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				// pq.QuoteIdentifier escapes double quotes by doubling them
				m.ExpectExec(`DROP USER "user""injection"`).
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

			err = deleteUserWithDBExtended(ctx, db, tt.username)

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

// deleteUserWithDBExtended mirrors DeleteUser but uses pq.QuoteIdentifier like the actual function
func deleteUserWithDBExtended(ctx context.Context, db *sql.DB, username string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedUsername := pq.QuoteIdentifier(username)

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

// ============================================================================
// CreateOrUpdateDatabase Extended Tests
// ============================================================================

// TestCreateOrUpdateDatabase_ActualFunction tests the actual function
func TestCreateOrUpdateDatabase_ActualFunction(t *testing.T) {
	tests := []struct {
		name             string
		host             string
		port             int32
		adminUser        string
		adminPassword    string
		sslMode          string
		databaseName     string
		owner            string
		schemaName       string
		templateDatabase string
		expectError      bool
	}{
		{
			name:          "Invalid host - connection fails",
			host:          "invalid-host-that-does-not-exist",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			owner:         "testowner",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := CreateOrUpdateDatabase(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode,
				tt.databaseName, tt.owner, tt.schemaName, tt.templateDatabase)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCreateOrUpdateDatabase_WithMockedDB tests with mocked database
func TestCreateOrUpdateDatabase_WithMockedDB(t *testing.T) {
	tests := []struct {
		name             string
		databaseName     string
		owner            string
		schemaName       string
		templateDatabase string
		setupMock        func(sqlmock.Sqlmock)
		setupSchemaMock  func(sqlmock.Sqlmock)
		expectError      bool
		errorContains    string
	}{
		{
			name:         "Database exists - update owner successfully",
			databaseName: "testdb",
			owner:        "newowner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("testdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`ALTER DATABASE "testdb" OWNER TO "newowner"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Database does not exist - create new",
			databaseName: "newdb",
			owner:        "owner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("newdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE DATABASE "newdb" OWNER "owner"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:             "Create database with template",
			databaseName:     "newdb",
			owner:            "owner",
			templateDatabase: "template1",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("newdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE DATABASE "newdb" OWNER "owner" TEMPLATE "template1"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Create database with schema - schema exists",
			databaseName: "newdb",
			owner:        "owner",
			schemaName:   "myschema",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("newdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE DATABASE "newdb" OWNER "owner"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			setupSchemaMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM information_schema.schemata WHERE schema_name = \$1\)`).
					WithArgs("myschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`ALTER SCHEMA "myschema" OWNER TO "owner"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Create database with schema - schema does not exist",
			databaseName: "newdb",
			owner:        "owner",
			schemaName:   "myschema",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("newdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE DATABASE "newdb" OWNER "owner"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			setupSchemaMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM information_schema.schemata WHERE schema_name = \$1\)`).
					WithArgs("myschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			},
			expectError: false,
		},
		{
			name:         "Create database with public schema - skip schema handling",
			databaseName: "newdb",
			owner:        "owner",
			schemaName:   "public",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("newdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE DATABASE "newdb" OWNER "owner"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Check database exists - SQL error",
			databaseName: "testdb",
			owner:        "owner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("testdb").
					WillReturnError(errors.New("connection refused"))
			},
			expectError:   true,
			errorContains: "checkDatabaseExists",
		},
		{
			name:         "Update database owner - SQL error",
			databaseName: "testdb",
			owner:        "newowner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("testdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`ALTER DATABASE "testdb" OWNER TO "newowner"`).
					WillReturnError(errors.New("permission denied"))
			},
			expectError:   true,
			errorContains: "updateDatabaseOwner",
		},
		{
			name:         "Create database - SQL error",
			databaseName: "newdb",
			owner:        "owner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("newdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE DATABASE "newdb" OWNER "owner"`).
					WillReturnError(errors.New("database already exists"))
			},
			expectError:   true,
			errorContains: "createDatabase",
		},
		{
			name:         "Check schema exists - SQL error",
			databaseName: "newdb",
			owner:        "owner",
			schemaName:   "myschema",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("newdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE DATABASE "newdb" OWNER "owner"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			setupSchemaMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM information_schema.schemata WHERE schema_name = \$1\)`).
					WithArgs("myschema").
					WillReturnError(errors.New("connection refused"))
			},
			expectError:   true,
			errorContains: "checkSchemaExists",
		},
		{
			name:         "Update schema owner - SQL error",
			databaseName: "newdb",
			owner:        "owner",
			schemaName:   "myschema",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("newdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE DATABASE "newdb" OWNER "owner"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			setupSchemaMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM information_schema.schemata WHERE schema_name = \$1\)`).
					WithArgs("myschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`ALTER SCHEMA "myschema" OWNER TO "owner"`).
					WillReturnError(errors.New("permission denied"))
			},
			expectError:   true,
			errorContains: "updateSchemaOwner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			var dbSchema *sql.DB
			var mockSchema sqlmock.Sqlmock
			if tt.setupSchemaMock != nil {
				dbSchema, mockSchema, err = sqlmock.New()
				require.NoError(t, err)
				defer dbSchema.Close()
				tt.setupSchemaMock(mockSchema)
			}

			ctx := context.Background()

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			err = createOrUpdateDatabaseWithDBExtended(ctx, db, dbSchema, tt.databaseName, tt.owner, tt.schemaName, tt.templateDatabase)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
			if mockSchema != nil {
				assert.NoError(t, mockSchema.ExpectationsWereMet())
			}
		})
	}
}

// createOrUpdateDatabaseWithDBExtended mirrors CreateOrUpdateDatabase but accepts *sql.DB
func createOrUpdateDatabaseWithDBExtended(ctx context.Context, db *sql.DB, dbSchema *sql.DB, databaseName, owner, schemaName, templateDatabase string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedDatabaseName := pq.QuoteIdentifier(databaseName)
	escapedOwner := pq.QuoteIdentifier(owner)

	var exists bool
	err := db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", databaseName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("CreateOrUpdateDatabase.checkDatabaseExists: %w", err)
	}

	if exists {
		query := fmt.Sprintf("ALTER DATABASE %s OWNER TO %s", escapedDatabaseName, escapedOwner)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("CreateOrUpdateDatabase.updateDatabaseOwner: %w", err)
		}
		return nil
	}

	var query string
	if templateDatabase != "" {
		escapedTemplate := pq.QuoteIdentifier(templateDatabase)
		query = fmt.Sprintf("CREATE DATABASE %s OWNER %s TEMPLATE %s", escapedDatabaseName, escapedOwner, escapedTemplate)
	} else {
		query = fmt.Sprintf("CREATE DATABASE %s OWNER %s", escapedDatabaseName, escapedOwner)
	}
	_, err = db.ExecContext(connCtx, query)
	if err != nil {
		return fmt.Errorf("CreateOrUpdateDatabase.createDatabase: %w", err)
	}

	if schemaName == "" || schemaName == "public" {
		return nil
	}

	if dbSchema == nil {
		return nil
	}

	escapedSchemaName := pq.QuoteIdentifier(schemaName)

	var schemaExists bool
	err = dbSchema.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)",
		schemaName).Scan(&schemaExists)
	if err != nil {
		return fmt.Errorf("CreateOrUpdateDatabase.checkSchemaExists: %w", err)
	}

	if !schemaExists {
		return nil
	}

	query = fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s", escapedSchemaName, escapedOwner)
	_, err = dbSchema.ExecContext(connCtx, query)
	if err != nil {
		return fmt.Errorf("CreateOrUpdateDatabase.updateSchemaOwner: %w", err)
	}

	return nil
}

// ============================================================================
// CreateOrUpdateUser Extended Tests
// ============================================================================

// TestCreateOrUpdateUser_ActualFunction tests the actual function
func TestCreateOrUpdateUser_ActualFunction(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		port          int32
		adminUser     string
		adminPassword string
		sslMode       string
		username      string
		password      string
		expectError   bool
	}{
		{
			name:          "Invalid host - connection fails",
			host:          "invalid-host-that-does-not-exist",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			username:      "testuser",
			password:      "testpass",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := CreateOrUpdateUser(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode, tt.username, tt.password)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCreateOrUpdateUser_WithMockedDB tests with mocked database
func TestCreateOrUpdateUser_WithMockedDB(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		password      string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:     "User exists - update password",
			username: "testuser",
			password: "newpassword",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_user WHERE usename = \$1\)`).
					WithArgs("testuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`ALTER USER "testuser" WITH PASSWORD`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:     "User does not exist - create new",
			username: "newuser",
			password: "password",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_user WHERE usename = \$1\)`).
					WithArgs("newuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE USER "newuser" WITH PASSWORD`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:     "Check user exists - SQL error",
			username: "testuser",
			password: "password",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_user WHERE usename = \$1\)`).
					WithArgs("testuser").
					WillReturnError(errors.New("connection refused"))
			},
			expectError:   true,
			errorContains: "checkUserExists",
		},
		{
			name:     "Update password - SQL error",
			username: "testuser",
			password: "newpassword",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_user WHERE usename = \$1\)`).
					WithArgs("testuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`ALTER USER "testuser" WITH PASSWORD`).
					WillReturnError(errors.New("permission denied"))
			},
			expectError:   true,
			errorContains: "updateUserPassword",
		},
		{
			name:     "Create user - SQL error",
			username: "newuser",
			password: "password",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_user WHERE usename = \$1\)`).
					WithArgs("newuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE USER "newuser" WITH PASSWORD`).
					WillReturnError(errors.New("role already exists"))
			},
			expectError:   true,
			errorContains: "createUser",
		},
		{
			name:     "Password with special characters",
			username: "testuser",
			password: "p@ss'word\"with$pecial",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_user WHERE usename = \$1\)`).
					WithArgs("testuser").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE USER "testuser" WITH PASSWORD`).
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

			err = createOrUpdateUserWithDBExtended(ctx, db, tt.username, tt.password)

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

// createOrUpdateUserWithDBExtended mirrors CreateOrUpdateUser but accepts *sql.DB
func createOrUpdateUserWithDBExtended(ctx context.Context, db *sql.DB, username, password string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedUsername := pq.QuoteIdentifier(username)
	escapedPassword := pq.QuoteLiteral(password)

	var exists bool
	err := db.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM pg_user WHERE usename = $1)", username).Scan(&exists)
	if err != nil {
		return fmt.Errorf("CreateOrUpdateUser.checkUserExists: %w", err)
	}

	if exists {
		query := fmt.Sprintf("ALTER USER %s WITH PASSWORD %s", escapedUsername, escapedPassword)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("CreateOrUpdateUser.updateUserPassword: %w", err)
		}
	} else {
		query := fmt.Sprintf("CREATE USER %s WITH PASSWORD %s", escapedUsername, escapedPassword)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("CreateOrUpdateUser.createUser: %w", err)
		}
	}

	return nil
}

// ============================================================================
// CreateOrUpdateRoleGroup Extended Tests
// ============================================================================

// TestCreateOrUpdateRoleGroup_ActualFunction tests the actual function
func TestCreateOrUpdateRoleGroup_ActualFunction(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		port          int32
		adminUser     string
		adminPassword string
		sslMode       string
		groupRole     string
		memberRoles   []string
		expectError   bool
	}{
		{
			name:          "Invalid host - connection fails",
			host:          "invalid-host-that-does-not-exist",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			groupRole:     "testgroup",
			memberRoles:   []string{"member1"},
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := CreateOrUpdateRoleGroup(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode, tt.groupRole, tt.memberRoles)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCreateOrUpdateRoleGroup_WithMockedDB tests with mocked database
func TestCreateOrUpdateRoleGroup_WithMockedDB(t *testing.T) {
	tests := []struct {
		name          string
		groupRole     string
		memberRoles   []string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:        "Group does not exist - create and add members",
			groupRole:   "testgroup",
			memberRoles: []string{"member1", "member2"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE ROLE "testgroup"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery(`SELECT r.rolname`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}))
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("member1").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`GRANT "testgroup" TO "member1"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("member2").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`GRANT "testgroup" TO "member2"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:        "Group exists - add new members and remove old",
			groupRole:   "testgroup",
			memberRoles: []string{"member2"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery(`SELECT r.rolname`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}).AddRow("member1"))
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("member2").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`GRANT "testgroup" TO "member2"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(`REVOKE "testgroup" FROM "member1"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:        "Skip non-existent member",
			groupRole:   "testgroup",
			memberRoles: []string{"nonexistent"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery(`SELECT r.rolname`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}))
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("nonexistent").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			},
			expectError: false,
		},
		{
			name:        "Check group exists - SQL error",
			groupRole:   "testgroup",
			memberRoles: []string{},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("testgroup").
					WillReturnError(errors.New("connection refused"))
			},
			expectError:   true,
			errorContains: "checkGroupRoleExists",
		},
		{
			name:        "Create group - SQL error",
			groupRole:   "testgroup",
			memberRoles: []string{},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE ROLE "testgroup"`).
					WillReturnError(errors.New("role already exists"))
			},
			expectError:   true,
			errorContains: "createGroupRole",
		},
		{
			name:        "Query members - SQL error",
			groupRole:   "testgroup",
			memberRoles: []string{},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery(`SELECT r.rolname`).
					WithArgs("testgroup").
					WillReturnError(errors.New("query error"))
			},
			expectError:   true,
			errorContains: "queryCurrentMembers",
		},
		{
			name:        "Check member exists - SQL error",
			groupRole:   "testgroup",
			memberRoles: []string{"member1"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery(`SELECT r.rolname`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}))
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("member1").
					WillReturnError(errors.New("check error"))
			},
			expectError:   true,
			errorContains: "checkMemberRoleExists",
		},
		{
			name:        "Add member - SQL error",
			groupRole:   "testgroup",
			memberRoles: []string{"member1"},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery(`SELECT r.rolname`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}))
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("member1").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`GRANT "testgroup" TO "member1"`).
					WillReturnError(errors.New("grant error"))
			},
			expectError:   true,
			errorContains: "addMemberToGroup",
		},
		{
			name:        "Remove member - SQL error",
			groupRole:   "testgroup",
			memberRoles: []string{},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectQuery(`SELECT r.rolname`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"rolname"}).AddRow("oldmember"))
				m.ExpectExec(`REVOKE "testgroup" FROM "oldmember"`).
					WillReturnError(errors.New("revoke error"))
			},
			expectError:   true,
			errorContains: "failed to remove member",
		},
		{
			name:        "Scan member error",
			groupRole:   "testgroup",
			memberRoles: []string{},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_roles WHERE rolname = \$1\)`).
					WithArgs("testgroup").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				rows := sqlmock.NewRows([]string{"rolname"}).AddRow("member1")
				rows = rows.RowError(0, errors.New("scan error"))
				m.ExpectQuery(`SELECT r.rolname`).
					WithArgs("testgroup").
					WillReturnRows(rows)
			},
			expectError:   true,
			errorContains: "error iterating members",
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

			err = createOrUpdateRoleGroupWithDBExtended(ctx, db, tt.groupRole, tt.memberRoles)

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

// createOrUpdateRoleGroupWithDBExtended mirrors CreateOrUpdateRoleGroup but accepts *sql.DB
func createOrUpdateRoleGroupWithDBExtended(ctx context.Context, db *sql.DB, groupRole string, memberRoles []string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedGroupRole := pq.QuoteIdentifier(groupRole)

	var exists bool
	err := db.QueryRowContext(connCtx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", groupRole).Scan(&exists)
	if err != nil {
		return fmt.Errorf("CreateOrUpdateRoleGroup.checkGroupRoleExists: %w", err)
	}

	if !exists {
		query := fmt.Sprintf("CREATE ROLE %s", escapedGroupRole)
		_, err = db.ExecContext(connCtx, query)
		if err != nil {
			return fmt.Errorf("CreateOrUpdateRoleGroup.createGroupRole: %w", err)
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
		return fmt.Errorf("CreateOrUpdateRoleGroup.queryCurrentMembers: %w", err)
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
				return fmt.Errorf("CreateOrUpdateRoleGroup.checkMemberRoleExists: %w", err)
			}
			if !memberExists {
				continue
			}

			escapedMemberRole := pq.QuoteIdentifier(memberRole)
			query := fmt.Sprintf("GRANT %s TO %s", escapedGroupRole, escapedMemberRole)
			_, err = db.ExecContext(connCtx, query)
			if err != nil {
				return fmt.Errorf("CreateOrUpdateRoleGroup.addMemberToGroup(%s): %w", memberRole, err)
			}
		}
	}

	for member := range currentMembers {
		if !desiredMembers[member] {
			escapedMemberRole := pq.QuoteIdentifier(member)
			query := fmt.Sprintf("REVOKE %s FROM %s", escapedGroupRole, escapedMemberRole)
			_, err = db.ExecContext(connCtx, query)
			if err != nil {
				return fmt.Errorf("failed to remove member %s from group role: %w", member, err)
			}
		}
	}

	return nil
}

// ============================================================================
// CreateOrUpdateSchema Extended Tests
// ============================================================================

// TestCreateOrUpdateSchema_ActualFunction tests the actual function
func TestCreateOrUpdateSchema_ActualFunction(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		port          int32
		adminUser     string
		adminPassword string
		sslMode       string
		databaseName  string
		schemaName    string
		owner         string
		expectError   bool
	}{
		{
			name:          "Invalid host - connection fails",
			host:          "invalid-host-that-does-not-exist",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			schemaName:    "testschema",
			owner:         "testowner",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := CreateOrUpdateSchema(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode, tt.databaseName, tt.schemaName, tt.owner)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCreateOrUpdateSchema_WithMockedDB tests with mocked database
func TestCreateOrUpdateSchema_WithMockedDB(t *testing.T) {
	tests := []struct {
		name          string
		schemaName    string
		owner         string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:       "Schema exists - update owner",
			schemaName: "testschema",
			owner:      "newowner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM information_schema.schemata WHERE schema_name = \$1\)`).
					WithArgs("testschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`ALTER SCHEMA "testschema" OWNER TO "newowner"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:       "Schema does not exist - create new",
			schemaName: "newschema",
			owner:      "owner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM information_schema.schemata WHERE schema_name = \$1\)`).
					WithArgs("newschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE SCHEMA "newschema" AUTHORIZATION "owner"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:       "Check schema exists - SQL error",
			schemaName: "testschema",
			owner:      "owner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM information_schema.schemata WHERE schema_name = \$1\)`).
					WithArgs("testschema").
					WillReturnError(errors.New("connection refused"))
			},
			expectError:   true,
			errorContains: "failed to check if schema exists",
		},
		{
			name:       "Update schema owner - SQL error",
			schemaName: "testschema",
			owner:      "newowner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM information_schema.schemata WHERE schema_name = \$1\)`).
					WithArgs("testschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`ALTER SCHEMA "testschema" OWNER TO "newowner"`).
					WillReturnError(errors.New("permission denied"))
			},
			expectError:   true,
			errorContains: "failed to update schema owner",
		},
		{
			name:       "Create schema - SQL error",
			schemaName: "newschema",
			owner:      "owner",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM information_schema.schemata WHERE schema_name = \$1\)`).
					WithArgs("newschema").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectExec(`CREATE SCHEMA "newschema" AUTHORIZATION "owner"`).
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

			err = createOrUpdateSchemaWithDBExtended(ctx, db, tt.schemaName, tt.owner)

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

// createOrUpdateSchemaWithDBExtended mirrors CreateOrUpdateSchema but accepts *sql.DB
func createOrUpdateSchemaWithDBExtended(ctx context.Context, db *sql.DB, schemaName, owner string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedSchemaName := pq.QuoteIdentifier(schemaName)
	escapedOwner := pq.QuoteIdentifier(owner)

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

// ============================================================================
// ApplyGrants Extended Tests
// ============================================================================

// TestApplyGrants_ActualFunction tests the actual function
func TestApplyGrants_ActualFunction(t *testing.T) {
	tests := []struct {
		name              string
		host              string
		port              int32
		adminUser         string
		adminPassword     string
		sslMode           string
		databaseName      string
		roleName          string
		grants            []instancev1alpha1.GrantItem
		defaultPrivileges []instancev1alpha1.DefaultPrivilegeItem
		expectError       bool
		errorContains     string
	}{
		{
			name:          "Invalid privilege - validation fails before connection",
			host:          "invalid-host",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			roleName:      "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeDatabase,
					Privileges: []string{"INVALID_PRIVILEGE"},
				},
			},
			expectError:   true,
			errorContains: "invalid privilege",
		},
		{
			name:          "Invalid default privilege - validation fails before connection",
			host:          "invalid-host",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			roleName:      "testrole",
			defaultPrivileges: []instancev1alpha1.DefaultPrivilegeItem{
				{
					Schema:     "public",
					ObjectType: "tables",
					Privileges: []string{"INVALID_PRIVILEGE"},
				},
			},
			expectError:   true,
			errorContains: "invalid privilege",
		},
		{
			name:          "Empty grants - no connection needed",
			host:          "invalid-host",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			roleName:      "testrole",
			grants:        []instancev1alpha1.GrantItem{},
			expectError:   false,
		},
		{
			name:          "Database grant - connection fails",
			host:          "invalid-host-that-does-not-exist",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			roleName:      "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeDatabase,
					Privileges: []string{"CONNECT"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := ApplyGrants(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode,
				tt.databaseName, tt.roleName, tt.grants, tt.defaultPrivileges)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestApplyGrants_WithMockedDB tests with mocked database
func TestApplyGrants_WithMockedDB(t *testing.T) {
	tests := []struct {
		name              string
		databaseName      string
		roleName          string
		grants            []instancev1alpha1.GrantItem
		defaultPrivileges []instancev1alpha1.DefaultPrivilegeItem
		setupMock         func(sqlmock.Sqlmock)
		expectError       bool
		errorContains     string
	}{
		{
			name:         "Database grant - success",
			databaseName: "testdb",
			roleName:     "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeDatabase,
					Privileges: []string{"CONNECT", "CREATE"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec(`GRANT CONNECT, CREATE ON DATABASE "testdb" TO "testrole"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Schema grant - success",
			databaseName: "testdb",
			roleName:     "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeSchema,
					Schema:     "public",
					Privileges: []string{"USAGE", "CREATE"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec(`GRANT USAGE, CREATE ON SCHEMA "public" TO "testrole"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Table grant - success",
			databaseName: "testdb",
			roleName:     "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeTable,
					Schema:     "public",
					Table:      "users",
					Privileges: []string{"SELECT", "INSERT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec(`GRANT SELECT, INSERT ON TABLE "public"."users" TO "testrole"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Sequence grant - success",
			databaseName: "testdb",
			roleName:     "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeSequence,
					Schema:     "public",
					Sequence:   "users_id_seq",
					Privileges: []string{"USAGE", "SELECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec(`GRANT USAGE, SELECT ON SEQUENCE "public"."users_id_seq" TO "testrole"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Function grant - success",
			databaseName: "testdb",
			roleName:     "testrole",
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
				m.ExpectExec(`GRANT EXECUTE ON FUNCTION "public"."my_function" TO "testrole"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "All tables grant - success",
			databaseName: "testdb",
			roleName:     "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeAllTables,
					Schema:     "public",
					Privileges: []string{"SELECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec(`GRANT SELECT ON ALL TABLES IN SCHEMA "public" TO "testrole"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "All sequences grant - success",
			databaseName: "testdb",
			roleName:     "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeAllSequences,
					Schema:     "public",
					Privileges: []string{"USAGE"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec(`GRANT USAGE ON ALL SEQUENCES IN SCHEMA "public" TO "testrole"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Default privileges for tables - success",
			databaseName: "testdb",
			roleName:     "testrole",
			defaultPrivileges: []instancev1alpha1.DefaultPrivilegeItem{
				{
					Schema:     "public",
					ObjectType: "tables",
					Privileges: []string{"SELECT", "INSERT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec(`ALTER DEFAULT PRIVILEGES IN SCHEMA "public" GRANT SELECT, INSERT ON TABLES TO "testrole"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Default privileges for sequences - success",
			databaseName: "testdb",
			roleName:     "testrole",
			defaultPrivileges: []instancev1alpha1.DefaultPrivilegeItem{
				{
					Schema:     "public",
					ObjectType: "sequences",
					Privileges: []string{"USAGE"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec(`ALTER DEFAULT PRIVILEGES IN SCHEMA "public" GRANT USAGE ON SEQUENCES TO "testrole"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Default privileges for functions - success",
			databaseName: "testdb",
			roleName:     "testrole",
			defaultPrivileges: []instancev1alpha1.DefaultPrivilegeItem{
				{
					Schema:     "public",
					ObjectType: "functions",
					Privileges: []string{"EXECUTE"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec(`ALTER DEFAULT PRIVILEGES IN SCHEMA "public" GRANT EXECUTE ON FUNCTIONS TO "testrole"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Default privileges for types - success",
			databaseName: "testdb",
			roleName:     "testrole",
			defaultPrivileges: []instancev1alpha1.DefaultPrivilegeItem{
				{
					Schema:     "public",
					ObjectType: "types",
					Privileges: []string{"USAGE"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec(`ALTER DEFAULT PRIVILEGES IN SCHEMA "public" GRANT USAGE ON TYPES TO "testrole"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Database grant - ping error",
			databaseName: "testdb",
			roleName:     "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeDatabase,
					Privileges: []string{"CONNECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing().WillReturnError(errors.New("connection refused"))
			},
			expectError:   true,
			errorContains: "failed to ping postgres database",
		},
		{
			name:         "Database grant - exec error",
			databaseName: "testdb",
			roleName:     "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeDatabase,
					Privileges: []string{"CONNECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
				m.ExpectExec(`GRANT CONNECT ON DATABASE "testdb" TO "testrole"`).
					WillReturnError(errors.New("permission denied"))
			},
			expectError:   true,
			errorContains: "failed to apply database grant",
		},
		{
			name:         "Schema grant - ping error",
			databaseName: "testdb",
			roleName:     "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeSchema,
					Schema:     "public",
					Privileges: []string{"USAGE"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing().WillReturnError(errors.New("connection refused"))
			},
			expectError:   true,
			errorContains: "failed to ping database",
		},
		{
			name:         "Grant item error - missing schema",
			databaseName: "testdb",
			roleName:     "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeSchema,
					Privileges: []string{"USAGE"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
			},
			expectError:   true,
			errorContains: "schema is required",
		},
		{
			name:         "Default privilege error - invalid object type",
			databaseName: "testdb",
			roleName:     "testrole",
			defaultPrivileges: []instancev1alpha1.DefaultPrivilegeItem{
				{
					Schema:     "public",
					ObjectType: "invalid",
					Privileges: []string{"SELECT"},
				},
			},
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectPing()
			},
			expectError:   true,
			errorContains: "unknown default privilege object type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
			require.NoError(t, err)
			defer db.Close()

			ctx := context.Background()

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			err = applyGrantsWithDBExtended(ctx, db, tt.databaseName, tt.roleName, tt.grants, tt.defaultPrivileges)

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

// applyGrantsWithDBExtended mirrors ApplyGrants but accepts *sql.DB
func applyGrantsWithDBExtended(
	ctx context.Context, db *sql.DB, databaseName, roleName string,
	grants []instancev1alpha1.GrantItem, defaultPrivileges []instancev1alpha1.DefaultPrivilegeItem) error {
	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	escapedRole := pq.QuoteIdentifier(roleName)
	escapedDBName := pq.QuoteIdentifier(databaseName)

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

// ============================================================================
// DeleteDatabase Extended Tests
// ============================================================================

// TestDeleteDatabase_ActualFunction tests the actual function
func TestDeleteDatabase_ActualFunction(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		port          int32
		adminUser     string
		adminPassword string
		sslMode       string
		databaseName  string
		expectError   bool
	}{
		{
			name:          "Invalid host - connection fails at CloseAllSessions",
			host:          "invalid-host-that-does-not-exist",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := DeleteDatabase(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode, tt.databaseName)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestDeleteDatabase_WithMockedDB tests with mocked database
func TestDeleteDatabase_WithMockedDB(t *testing.T) {
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
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("testdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`DROP DATABASE "testdb"`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:         "Database does not exist - no action",
			databaseName: "nonexistent",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("nonexistent").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			},
			expectError: false,
		},
		{
			name:         "Check database exists - SQL error",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("testdb").
					WillReturnError(errors.New("connection refused"))
			},
			expectError:   true,
			errorContains: "failed to check if database exists",
		},
		{
			name:         "Drop database - SQL error",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_database WHERE datname = \$1\)`).
					WithArgs("testdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec(`DROP DATABASE "testdb"`).
					WillReturnError(errors.New("database is being accessed by other users"))
			},
			expectError:   true,
			errorContains: "failed to drop database",
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

			err = deleteDatabaseWithDBExtended(ctx, db, tt.databaseName)

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

// deleteDatabaseWithDBExtended mirrors DeleteDatabase but accepts *sql.DB
func deleteDatabaseWithDBExtended(ctx context.Context, db *sql.DB, databaseName string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedDatabaseName := pq.QuoteIdentifier(databaseName)

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

// ============================================================================
// CloseAllSessions Extended Tests
// ============================================================================

// TestCloseAllSessions_ActualFunction tests the actual function
func TestCloseAllSessions_ActualFunction(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		port          int32
		adminUser     string
		adminPassword string
		sslMode       string
		databaseName  string
		expectError   bool
	}{
		{
			name:          "Invalid host - connection fails",
			host:          "invalid-host-that-does-not-exist",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := CloseAllSessions(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode, tt.databaseName)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCloseAllSessions_WithMockedDB tests with mocked database
func TestCloseAllSessions_WithMockedDB(t *testing.T) {
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
				m.ExpectExec(`SELECT pg_terminate_backend`).
					WithArgs("testdb").
					WillReturnResult(sqlmock.NewResult(0, 5))
			},
			expectError: false,
		},
		{
			name:         "No sessions to terminate",
			databaseName: "emptydb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(`SELECT pg_terminate_backend`).
					WithArgs("emptydb").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: false,
		},
		{
			name:         "Terminate sessions - SQL error",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(`SELECT pg_terminate_backend`).
					WithArgs("testdb").
					WillReturnError(errors.New("permission denied"))
			},
			expectError:   true,
			errorContains: "failed to terminate connections",
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

			err = closeAllSessionsWithDBExtended(ctx, db, tt.databaseName)

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

// closeAllSessionsWithDBExtended mirrors CloseAllSessions but accepts *sql.DB
func closeAllSessionsWithDBExtended(ctx context.Context, db *sql.DB, databaseName string) error {
	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = $1
		AND pid <> pg_backend_pid()`

	_, err := db.ExecContext(connCtx, query, databaseName)
	if err != nil {
		return fmt.Errorf("failed to terminate connections to database: %w", err)
	}

	return nil
}
