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

package helpers

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgreSQLTestConfig holds configuration for connecting to a test PostgreSQL instance.
type PostgreSQLTestConfig struct {
	Host     string
	Port     int32
	User     string
	Password string
	SSLMode  string
}

// NewPostgreSQLTestConfig creates a PostgreSQLTestConfig from environment variables.
func NewPostgreSQLTestConfig() *PostgreSQLTestConfig {
	port, err := strconv.Atoi(GetEnvOrDefault("TEST_POSTGRES_PORT", "5432"))
	if err != nil {
		port = 5432
	}

	return &PostgreSQLTestConfig{
		Host:     GetEnvOrDefault("TEST_POSTGRES_HOST", "localhost"),
		Port:     int32(port),
		User:     GetEnvOrDefault("TEST_POSTGRES_USER", "postgres"),
		Password: GetEnvOrDefault("TEST_POSTGRES_PASSWORD", "postgres"),
		SSLMode:  GetEnvOrDefault("TEST_POSTGRES_SSLMODE", "disable"),
	}
}

// ConnStr returns a connection string for the given database.
func (c *PostgreSQLTestConfig) ConnStr(database string) string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
		c.Host, c.Port, c.User, c.Password, database, c.SSLMode)
}

// OpenDB opens a connection to the specified database.
func (c *PostgreSQLTestConfig) OpenDB(ctx context.Context, database string) (*sql.DB, error) {
	db, err := sql.Open("postgres", c.ConnStr(database))
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(connCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// CreateTestDatabase creates a test database.
func (c *PostgreSQLTestConfig) CreateTestDatabase(ctx context.Context, name string) error {
	db, err := c.OpenDB(ctx, "postgres")
	if err != nil {
		return err
	}
	defer db.Close()

	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if database exists
	var exists bool
	err = db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if !exists {
		_, err = db.ExecContext(connCtx, fmt.Sprintf("CREATE DATABASE %q", name))
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
	}

	return nil
}

// DropTestDatabase drops a test database.
func (c *PostgreSQLTestConfig) DropTestDatabase(ctx context.Context, name string) error {
	db, err := c.OpenDB(ctx, "postgres")
	if err != nil {
		return err
	}
	defer db.Close()

	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Terminate connections
	_, _ = db.ExecContext(connCtx,
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()", name)

	time.Sleep(500 * time.Millisecond)

	_, err = db.ExecContext(connCtx, fmt.Sprintf("DROP DATABASE IF EXISTS %q", name))
	if err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	return nil
}

// UserExists checks if a PostgreSQL user/role exists.
func (c *PostgreSQLTestConfig) UserExists(ctx context.Context, username string) (bool, error) {
	db, err := c.OpenDB(ctx, "postgres")
	if err != nil {
		return false, err
	}
	defer db.Close()

	connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var exists bool
	err = db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if user exists: %w", err)
	}

	return exists, nil
}

// DatabaseExists checks if a PostgreSQL database exists.
func (c *PostgreSQLTestConfig) DatabaseExists(ctx context.Context, dbname string) (bool, error) {
	db, err := c.OpenDB(ctx, "postgres")
	if err != nil {
		return false, err
	}
	defer db.Close()

	connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var exists bool
	err = db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbname).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if database exists: %w", err)
	}

	return exists, nil
}

// SchemaExists checks if a PostgreSQL schema exists in a given database.
func (c *PostgreSQLTestConfig) SchemaExists(ctx context.Context, dbname, schemaName string) (bool, error) {
	db, err := c.OpenDB(ctx, dbname)
	if err != nil {
		return false, err
	}
	defer db.Close()

	connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var exists bool
	err = db.QueryRowContext(connCtx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)",
		schemaName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if schema exists: %w", err)
	}

	return exists, nil
}

// RoleGroupHasMember checks if a role group has a specific member.
func (c *PostgreSQLTestConfig) RoleGroupHasMember(ctx context.Context, groupRole, memberRole string) (bool, error) {
	db, err := c.OpenDB(ctx, "postgres")
	if err != nil {
		return false, err
	}
	defer db.Close()

	connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var exists bool
	err = db.QueryRowContext(connCtx, `
		SELECT EXISTS(
			SELECT 1 FROM pg_roles r
			JOIN pg_auth_members m ON r.oid = m.member
			JOIN pg_roles g ON g.oid = m.roleid
			WHERE g.rolname = $1 AND r.rolname = $2
		)`, groupRole, memberRole).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check role group membership: %w", err)
	}

	return exists, nil
}

// DropTestUser drops a test user/role.
func (c *PostgreSQLTestConfig) DropTestUser(ctx context.Context, username string) error {
	db, err := c.OpenDB(ctx, "postgres")
	if err != nil {
		return err
	}
	defer db.Close()

	connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = db.ExecContext(connCtx, fmt.Sprintf("DROP ROLE IF EXISTS %q", username))
	if err != nil {
		return fmt.Errorf("failed to drop user: %w", err)
	}

	return nil
}

// RoleHasPrivilege checks if a role has a specific privilege on a database.
func (c *PostgreSQLTestConfig) RoleHasPrivilege(ctx context.Context, dbname, roleName, privilege string) (bool, error) {
	db, err := c.OpenDB(ctx, "postgres")
	if err != nil {
		return false, err
	}
	defer db.Close()

	connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var hasPriv bool
	err = db.QueryRowContext(connCtx,
		"SELECT has_database_privilege($1, $2, $3)", roleName, dbname, privilege).Scan(&hasPriv)
	if err != nil {
		return false, fmt.Errorf("failed to check privilege: %w", err)
	}

	return hasPriv, nil
}
