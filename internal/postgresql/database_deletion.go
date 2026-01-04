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
	"fmt"
	"time"

	"github.com/lib/pq"
)

// CloseAllSessions closes all active sessions/connections to a database
func CloseAllSessions(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, databaseName string) error {
	// Connect to PostgreSQL as admin user (must connect to postgres database, not the target database)
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, sslMode)

	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	// Terminate all connections to the database
	// PostgreSQL requires terminating connections before dropping a database
	// Use parameterized query for database name
	query := `
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = $1
		AND pid <> pg_backend_pid()`

	_, err = db.ExecContext(connCtx, query, databaseName)
	if err != nil {
		return fmt.Errorf("failed to terminate connections to database: %w", err)
	}

	// Wait a bit for connections to close
	time.Sleep(1 * time.Second)

	return nil
}

// DeleteDatabase deletes a PostgreSQL database
func DeleteDatabase(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, databaseName string) error {
	// First, close all sessions to the database
	if err := CloseAllSessions(ctx, host, port, adminUser, adminPassword, sslMode, databaseName); err != nil {
		return fmt.Errorf("failed to close sessions before deletion: %w", err)
	}

	// Connect to PostgreSQL as admin user (must connect to postgres database, not the target database)
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s connect_timeout=5",
		host, port, adminUser, adminPassword, sslMode)

	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	// Escape database name for SQL identifier
	escapedDatabaseName := pq.QuoteIdentifier(databaseName)

	// Check if database exists
	var exists bool
	err = db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", databaseName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if !exists {
		// Database doesn't exist, nothing to delete
		return nil
	}

	// Drop the database
	query := fmt.Sprintf("DROP DATABASE %s", escapedDatabaseName)
	_, err = db.ExecContext(connCtx, query)
	if err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	return nil
}
