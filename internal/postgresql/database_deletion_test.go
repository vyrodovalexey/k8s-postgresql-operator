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
)

func TestCloseAllSessions_InvalidConnection(t *testing.T) {
	ctx := context.Background()

	// Test with invalid connection parameters
	err := CloseAllSessions(ctx, "invalid-host", 5432, "user", "password", "require", "testdb")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to")
}

func TestDeleteDatabase_InvalidConnection(t *testing.T) {
	ctx := context.Background()

	// Test with invalid connection parameters
	err := DeleteDatabase(ctx, "invalid-host", 5432, "user", "password", "require", "testdb")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to")
}

func TestDeleteDatabase_ClosesSessionsFirst(t *testing.T) {
	ctx := context.Background()

	// This test verifies that DeleteDatabase calls CloseAllSessions first
	// Since we can't easily test the actual database operations without a real DB,
	// we test that the function structure is correct
	err := DeleteDatabase(ctx, "invalid-host", 5432, "user", "password", "require", "testdb")

	// Should fail at CloseAllSessions step
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to close sessions")
}

// closeAllSessionsWithDB is a helper that mirrors CloseAllSessions but accepts *sql.DB
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
		return fmt.Errorf("failed to terminate connections to database: %w", err)
	}

	return nil
}

// deleteDatabaseWithDB is a helper that mirrors DeleteDatabase logic after CloseAllSessions
func deleteDatabaseWithDB(ctx context.Context, db *sql.DB, databaseName string) error {
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

// TestCloseAllSessionsWithDB_TableDriven tests CloseAllSessions with mocked DB
func TestCloseAllSessionsWithDB_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		databaseName  string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:         "Success - terminate connections",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec("SELECT pg_terminate_backend").
					WithArgs("testdb").
					WillReturnResult(sqlmock.NewResult(0, 3))
			},
			expectError: false,
		},
		{
			name:         "Success - no connections to terminate",
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
			errorContains: "failed to terminate connections",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			ctx := context.Background()
			tt.setupMock(mock)

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

// TestDeleteDatabaseWithDB_TableDriven tests DeleteDatabase with mocked DB
func TestDeleteDatabaseWithDB_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		databaseName  string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:         "Database exists - drop successfully",
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
			name:         "Database does not exist - no-op",
			databaseName: "nonexistent",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("nonexistent").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			},
			expectError: false,
		},
		{
			name:         "Check exists error",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testdb").
					WillReturnError(errors.New("query error"))
			},
			expectError:   true,
			errorContains: "failed to check if database exists",
		},
		{
			name:         "Drop database error",
			databaseName: "testdb",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT EXISTS").
					WithArgs("testdb").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectExec("DROP DATABASE").
					WillReturnError(errors.New("drop error"))
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
			tt.setupMock(mock)

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

// ============================================================================
// Tests using mock PostgreSQL server to cover actual source functions
// ============================================================================

// TestCloseAllSessions_MockPG tests CloseAllSessions with mock PG server
func TestCloseAllSessions_MockPG(t *testing.T) {
	tests := []struct {
		name         string
		databaseName string
		handler      func(query string) mockPGResponse
		expectError  bool
	}{
		{
			name:         "Success - terminate connections",
			databaseName: "testdb",
			handler: func(q string) mockPGResponse {
				return mockPGResponse{isSelect: false}
			},
			expectError: false,
		},
		{
			name:         "Error - terminate fails",
			databaseName: "testdb",
			handler: func(q string) mockPGResponse {
				return mockPGResponse{isError: true, errorMsg: "terminate error"}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newMockPGServer(t, tt.handler)
			defer srv.close()

			ctx := context.Background()
			err := CloseAllSessions(ctx, "127.0.0.1", srv.port(), "admin", "password", "disable", tt.databaseName)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestDeleteDatabase_MockPG tests DeleteDatabase with mock PG server
func TestDeleteDatabase_MockPG(t *testing.T) {
	tests := []struct {
		name         string
		databaseName string
		handler      func(query string) mockPGResponse
		expectError  bool
	}{
		{
			name:         "Database exists - drop successfully",
			databaseName: "testdb",
			handler: func(q string) mockPGResponse {
				upper := strings.ToUpper(q)
				if strings.Contains(upper, "SELECT EXISTS") {
					return mockPGResponse{isSelect: true, value: "t"}
				}
				return mockPGResponse{isSelect: false}
			},
			expectError: false,
		},
		{
			name:         "Database does not exist - no-op",
			databaseName: "nonexistent",
			handler: func(q string) mockPGResponse {
				upper := strings.ToUpper(q)
				if strings.Contains(upper, "SELECT EXISTS") {
					return mockPGResponse{isSelect: true, value: "f"}
				}
				return mockPGResponse{isSelect: false}
			},
			expectError: false,
		},
		{
			name:         "Check exists error",
			databaseName: "testdb",
			handler: func(q string) mockPGResponse {
				upper := strings.ToUpper(q)
				if strings.Contains(upper, "PG_TERMINATE") {
					return mockPGResponse{isSelect: false}
				}
				return mockPGResponse{isError: true, errorMsg: "query error"}
			},
			expectError: true,
		},
		{
			name:         "Drop database error",
			databaseName: "testdb",
			handler: func(q string) mockPGResponse {
				upper := strings.ToUpper(q)
				if strings.Contains(upper, "PG_TERMINATE") {
					return mockPGResponse{isSelect: false}
				}
				if strings.Contains(upper, "SELECT EXISTS") {
					return mockPGResponse{isSelect: true, value: "t"}
				}
				return mockPGResponse{isError: true, errorMsg: "drop error"}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newMockPGServer(t, tt.handler)
			defer srv.close()

			ctx := context.Background()
			err := DeleteDatabase(ctx, "127.0.0.1", srv.port(), "admin", "password", "disable", tt.databaseName)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
