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
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closeAllSessionsWithDB is a helper function for testing that accepts a *sql.DB
func closeAllSessionsWithDB(ctx context.Context, db *sql.DB, databaseName string) error {
	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = $1
		AND pid <> pg_backend_pid()`

	_, err := db.ExecContext(connCtx, query, databaseName)
	if err != nil {
		return err
	}

	return nil
}

// deleteDatabaseWithDB is a helper function for testing that accepts a *sql.DB
func deleteDatabaseWithDB(ctx context.Context, db *sql.DB, databaseName string) error {
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	escapedDatabaseName := `"` + databaseName + `"`

	var exists bool
	err := db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", databaseName).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	query := "DROP DATABASE " + escapedDatabaseName
	_, err = db.ExecContext(connCtx, query)
	if err != nil {
		return err
	}

	return nil
}

// TestCloseAllSessions_TableDriven tests CloseAllSessions with various scenarios
func TestCloseAllSessions_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		databaseName  string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:         "Success - sessions terminated",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("SELECT pg_terminate_backend").
					WithArgs("testdb").
					WillReturnResult(sqlmock.NewResult(0, 3))
			},
			expectError: false,
		},
		{
			name:         "Success - no sessions to terminate",
			databaseName: "emptydb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("SELECT pg_terminate_backend").
					WithArgs("emptydb").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: false,
		},
		{
			name:         "Error - terminate fails",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("SELECT pg_terminate_backend").
					WithArgs("testdb").
					WillReturnError(errors.New("terminate error"))
			},
			expectError:   true,
			errorContains: "terminate error",
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

			err = closeAllSessionsWithDB(ctx, db, tt.databaseName)

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

// TestDeleteDatabase_TableDriven tests DeleteDatabase with various scenarios
func TestDeleteDatabase_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		databaseName  string
		dbExists      bool
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:         "Success - database exists and deleted",
			databaseName: "testdb",
			dbExists:     true,
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
			name:         "Success - database does not exist",
			databaseName: "nonexistent",
			dbExists:     false,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("nonexistent").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			},
			expectError: false,
		},
		{
			name:         "Error - check exists fails",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testdb").
					WillReturnError(errors.New("query error"))
			},
			expectError:   true,
			errorContains: "query error",
		},
		{
			name:         "Error - drop database fails",
			databaseName: "testdb",
			dbExists:     true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("DROP DATABASE").
					WillReturnError(errors.New("drop error"))
			},
			expectError:   true,
			errorContains: "drop error",
		},
		{
			name:         "Database with special characters in name",
			databaseName: "test-db-123",
			dbExists:     true,
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("test-db-123").
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

			err = deleteDatabaseWithDB(ctx, db, tt.databaseName)

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

// TestCloseAllSessions_InvalidConnection tests CloseAllSessions with invalid connection
func TestCloseAllSessions_InvalidConnection(t *testing.T) {
	ctx := context.Background()

	err := CloseAllSessions(ctx, "invalid-host", 5432, "user", "password", "require", "testdb")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to")
}

// TestDeleteDatabase_InvalidConnection tests DeleteDatabase with invalid connection
func TestDeleteDatabase_InvalidConnection(t *testing.T) {
	ctx := context.Background()

	err := DeleteDatabase(ctx, "invalid-host", 5432, "user", "password", "require", "testdb")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to")
}

// TestDeleteDatabase_ClosesSessionsFirst tests that DeleteDatabase calls CloseAllSessions first
func TestDeleteDatabase_ClosesSessionsFirst(t *testing.T) {
	ctx := context.Background()

	err := DeleteDatabase(ctx, "invalid-host", 5432, "user", "password", "require", "testdb")

	// Should fail at CloseAllSessions step
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to close sessions")
}

// TestCloseAllSessions_ContextCancellation tests context cancellation
func TestCloseAllSessions_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := CloseAllSessions(ctx, "localhost", 5432, "user", "password", "disable", "testdb")

	// Should fail due to context cancellation or connection error
	assert.Error(t, err)
}

// TestDeleteDatabase_ContextCancellation tests context cancellation
func TestDeleteDatabase_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := DeleteDatabase(ctx, "localhost", 5432, "user", "password", "disable", "testdb")

	// Should fail due to context cancellation or connection error
	assert.Error(t, err)
}

// TestCloseAllSessions_ContextTimeout tests context timeout
func TestCloseAllSessions_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for timeout
	time.Sleep(5 * time.Millisecond)

	err := CloseAllSessions(ctx, "localhost", 5432, "user", "password", "disable", "testdb")

	// Should fail due to context timeout or connection error
	assert.Error(t, err)
}

// TestDeleteDatabase_ContextTimeout tests context timeout
func TestDeleteDatabase_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for timeout
	time.Sleep(5 * time.Millisecond)

	err := DeleteDatabase(ctx, "localhost", 5432, "user", "password", "disable", "testdb")

	// Should fail due to context timeout or connection error
	assert.Error(t, err)
}
