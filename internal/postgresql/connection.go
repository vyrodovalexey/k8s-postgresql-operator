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

	"github.com/cenkalti/backoff/v4"
	_ "github.com/lib/pq" // PostgreSQL driver
	"go.uber.org/zap"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/constants"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

const (
	DefaultSSLMode        = "require"
	DefaultPort           = 5432
	DefaultConnectTimeout = 5
)

// DefaultDB is the default PostgreSQL database name (uses constants.DefaultDatabaseName)
var DefaultDB = constants.DefaultDatabaseName

// ConnectionConfig holds all parameters needed to build a PostgreSQL connection string
type ConnectionConfig struct {
	Host           string
	Port           int32
	Username       string
	Password       string
	Database       string
	SSLMode        string
	ConnectTimeout int
}

// NewConnectionConfig creates a new ConnectionConfig with default values
func NewConnectionConfig(host string, port int32, username, password, database, sslMode string) *ConnectionConfig {
	if port == 0 {
		port = DefaultPort
	}
	if database == "" {
		database = DefaultDB
	}
	if sslMode == "" {
		sslMode = DefaultSSLMode
	}
	return &ConnectionConfig{
		Host:           host,
		Port:           port,
		Username:       username,
		Password:       password,
		Database:       database,
		SSLMode:        sslMode,
		ConnectTimeout: DefaultConnectTimeout,
	}
}

// WithConnectTimeout sets a custom connect timeout
func (c *ConnectionConfig) WithConnectTimeout(timeout int) *ConnectionConfig {
	c.ConnectTimeout = timeout
	return c
}

// BuildConnectionString builds a PostgreSQL connection string from the configuration
func (c *ConnectionConfig) BuildConnectionString() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		c.Host, c.Port, c.Username, c.Password, c.Database, c.SSLMode, c.ConnectTimeout)
}

// TestConnection tests the connection to a PostgreSQL instance with exponential backoff retry logic
// The function respects context cancellation and will return early if the context is cancelled
func TestConnection(
	ctx context.Context, host string, port int32, database, username, password, sslMode string,
	log *zap.SugaredLogger, retries int, retryTimeout time.Duration) (bool, error) {
	// Convert legacy parameters to RetryConfig
	cfg := NewRetryConfigFromParams(retries, retryTimeout)
	return TestConnectionWithConfig(ctx, host, port, database, username, password, sslMode, log, cfg)
}

// TestConnectionWithConfig tests the connection to a PostgreSQL instance with configurable exponential backoff
func TestConnectionWithConfig(
	ctx context.Context, host string, port int32, database, username, password, sslMode string,
	log *zap.SugaredLogger, cfg RetryConfig) (bool, error) {
	b := newExponentialBackOff(cfg)
	attempt := 0

	for {
		attempt++

		// Check context before attempting connection
		if err := ctx.Err(); err != nil {
			log.Warnw("Context cancelled before connection attempt",
				"host", host, "port", port, "attempt", attempt, "error", err)
			return false, fmt.Errorf("connection test cancelled: %w", err)
		}

		log.Debugw("Attempting PostgreSQL connection test",
			"host", host, "port", port, "attempt", attempt)

		// Build connection string using ConnectionConfig
		connCfg := NewConnectionConfig(host, port, username, password, database, sslMode)
		connStr := connCfg.BuildConnectionString()

		// Create a context with timeout for the connection test
		connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)

		// Open connection
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			cancel()
			log.Warnw("PostgreSQL connection attempt failed - failed to open connection",
				"host", host, "port", port, "attempt", attempt, "error", err)

			nextBackoff := b.NextBackOff()
			if nextBackoff == backoff.Stop {
				return false, fmt.Errorf("failed to connect to PostgreSQL after %d attempts (max elapsed time %v): %w",
					attempt, cfg.MaxElapsedTime, err)
			}

			log.Debugw("Will retry connection", "nextRetryIn", nextBackoff)
			if sleepErr := ContextAwareSleep(ctx, nextBackoff); sleepErr != nil {
				return false, fmt.Errorf("connection test cancelled during backoff: %w", sleepErr)
			}
			continue
		}

		// Test the connection with a simple query
		var result int
		err = db.QueryRowContext(connCtx, "SELECT 1").Scan(&result)
		cancel()

		// Close the connection
		if closeErr := db.Close(); closeErr != nil {
			log.Warnw("Failed to close database connection", "error", closeErr)
		}

		if err != nil {
			log.Warnw("PostgreSQL connection attempt failed - query failed",
				"host", host, "port", port, "attempt", attempt, "error", err)

			nextBackoff := b.NextBackOff()
			if nextBackoff == backoff.Stop {
				return false, fmt.Errorf("failed to connect to PostgreSQL after %d attempts (max elapsed time %v): %w",
					attempt, cfg.MaxElapsedTime, err)
			}

			log.Debugw("Will retry connection", "nextRetryIn", nextBackoff)
			if sleepErr := ContextAwareSleep(ctx, nextBackoff); sleepErr != nil {
				return false, fmt.Errorf("connection test cancelled during backoff: %w", sleepErr)
			}
			continue
		}

		// Verify we got the expected result
		if result != 1 {
			err := fmt.Errorf("unexpected query result: %d", result)
			log.Warnw("PostgreSQL connection attempt failed - unexpected result",
				"host", host, "port", port, "attempt", attempt, "error", err)

			nextBackoff := b.NextBackOff()
			if nextBackoff == backoff.Stop {
				return false, fmt.Errorf("failed to connect to PostgreSQL after %d attempts (max elapsed time %v): %w",
					attempt, cfg.MaxElapsedTime, err)
			}

			log.Debugw("Will retry connection", "nextRetryIn", nextBackoff)
			if sleepErr := ContextAwareSleep(ctx, nextBackoff); sleepErr != nil {
				return false, fmt.Errorf("connection test cancelled during backoff: %w", sleepErr)
			}
			continue
		}

		// Success!
		log.Infow("PostgreSQL connection test successful", "host", host, "port", port, "attempt", attempt)
		return true, nil
	}
}

// TestConnectionFromPostgresql tests the connection to a PostgreSQL instance from a Postgresql CRD
func TestConnectionFromPostgresql(
	ctx context.Context, postgresql *instancev1alpha1.Postgresql, vaultClient vault.ClientInterface,
	log *zap.SugaredLogger, retries int, retryTimeout time.Duration) error {
	if postgresql.Spec.ExternalInstance == nil {
		return fmt.Errorf("PostgreSQL instance has no external instance configuration")
	}

	externalInstance := postgresql.Spec.ExternalInstance

	// Set default port if not specified
	port := externalInstance.Port
	if port == 0 {
		port = DefaultPort
	}

	// Set default SSL mode if not specified
	sslMode := externalInstance.SSLMode
	if sslMode == "" {
		sslMode = DefaultSSLMode
	}

	// Set default database if not specified
	database := DefaultDB

	// Get credentials from Vault if available
	var username, password string
	if vaultClient != nil {
		vaultUsername, vaultPassword, err := vaultClient.GetPostgresqlCredentials(ctx, externalInstance.PostgresqlID)
		if err != nil {
			log.Warnw("Failed to get credentials from Vault, connection test may fail",
				"postgresqlID", externalInstance.PostgresqlID, "error", err)
			return fmt.Errorf("failed to get credentials from Vault: %w", err)
		}
		username = vaultUsername
		password = vaultPassword
		log.Debugw("Credentials retrieved from Vault", "postgresqlID", externalInstance.PostgresqlID)
	} else {
		// Vault client not configured, skip connection test
		log.Warnw("Vault client not configured, skipping PostgreSQL connection test",
			"postgresqlID", externalInstance.PostgresqlID)
		return nil
	}

	// If no credentials from Vault, we cannot test the connection
	if username == "" || password == "" {
		return fmt.Errorf("no credentials available to test connection (credentials not found in Vault)")
	}

	_, err := TestConnection(ctx, externalInstance.Address, port, database, username, password, sslMode,
		log, retries, retryTimeout)
	return err
}
