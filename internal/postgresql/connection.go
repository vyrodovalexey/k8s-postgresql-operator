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
	"crypto/rand"
	"database/sql"
	"fmt"
	"math"
	"math/big"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver registration for database/sql
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/telemetry"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

const (
	DefaultSSLMode = "require"
	DefaultPort    = 5432
	DefaultDB      = "postgres"
)

// maxConnectionRetryDelay is the upper bound for exponential backoff delay in connection retries
const maxConnectionRetryDelay = 60 * time.Second

// calculateConnectionBackoff returns the delay for the given attempt using exponential backoff with jitter
func calculateConnectionBackoff(baseDelay time.Duration, attempt int, maxDelay time.Duration) time.Duration {
	exp := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(baseDelay) * exp)
	if delay > maxDelay {
		delay = maxDelay
	}
	// Add jitter: 0-25% of the delay
	maxJitter := int64(delay/4) + 1
	n, err := rand.Int(rand.Reader, big.NewInt(maxJitter))
	if err != nil {
		return delay
	}
	jitter := time.Duration(n.Int64())
	return delay + jitter
}

// waitForRetry waits for the backoff delay or context cancellation.
// Returns an error if the context is canceled during the wait.
func waitForRetry(
	ctx context.Context, retryTimeout time.Duration, attempt int,
) error {
	delay := calculateConnectionBackoff(retryTimeout, attempt, maxConnectionRetryDelay)
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return fmt.Errorf("connection test canceled: %w", ctx.Err())
	}
}

// testSingleConnection performs a single connection test attempt.
// Returns (true, nil) on success, (false, error) on failure.
func testSingleConnection(
	ctx context.Context, host string, port int32,
	database, username, password, sslMode string,
	log *zap.SugaredLogger,
) (bool, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
		host, port, username, password, database, sslMode,
	)

	connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return false, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Warnw("Failed to close database connection", "error", closeErr)
		}
	}()

	var result int
	if err = db.QueryRowContext(connCtx, "SELECT 1").Scan(&result); err != nil {
		return false, fmt.Errorf("failed to query database: %w", err)
	}

	if result != 1 {
		return false, fmt.Errorf("unexpected query result: %d", result)
	}

	return true, nil
}

// TestConnection tests the connection to a PostgreSQL instance with retry logic
func TestConnection(
	ctx context.Context, host string, port int32,
	database, username, password, sslMode string,
	log *zap.SugaredLogger, retries int, retryTimeout time.Duration,
) (bool, error) {
	ctx, span := otel.Tracer("postgresql").Start(ctx,
		"postgresql.TestConnection",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("net.peer.name", host),
			attribute.Int("net.peer.port", int(port)),
			attribute.String(telemetry.AttrOperation,
				"test_connection"),
		))
	defer span.End()

	var lastErr error

	for attempt := 1; attempt <= retries; attempt++ {
		log.Debugw("Attempting PostgreSQL connection test",
			"host", host, "port", port, "attempt", attempt, "maxRetries", retries)

		ok, err := testSingleConnection(
			ctx, host, port, database, username, password, sslMode, log,
		)
		if err != nil {
			lastErr = err
			log.Warnw("PostgreSQL connection attempt failed",
				"host", host, "port", port, "attempt", attempt, "error", lastErr)
			if attempt < retries {
				if waitErr := waitForRetry(ctx, retryTimeout, attempt); waitErr != nil {
					return false, waitErr
				}
			}
			continue
		}

		if ok {
			log.Infow("PostgreSQL connection test successful",
				"host", host, "port", port, "attempt", attempt)
			return true, nil
		}
	}

	// All retries failed
	connErr := fmt.Errorf(
		"failed to connect to PostgreSQL after %d attempts: %w",
		retries, lastErr,
	)
	telemetry.RecordError(span, connErr)
	return false, connErr
}

// resolveVaultCredentials resolves credentials from Vault with fallback to default credentials.
// First tries instance_admin, then falls back to default credentials if not found.
func resolveVaultCredentials(
	ctx context.Context, vaultClient *vault.Client,
	postgresqlID string, log *zap.SugaredLogger,
) (login, password string, err error) {
	ctx, span := otel.Tracer("postgresql").Start(ctx,
		"postgresql.resolveVaultCredentials",
		trace.WithAttributes(
			attribute.String(telemetry.AttrPostgresqlID, postgresqlID),
			attribute.String(telemetry.AttrOperation,
				"resolve_vault_credentials"),
		))
	defer span.End()
	username, password, err := vaultClient.GetPostgresqlCredentials(ctx, postgresqlID)
	if err == nil {
		log.Debugw("Credentials retrieved from Vault",
			"postgresqlID", postgresqlID)
		return username, password, nil
	}

	// If instance_admin secret not found, try default credentials
	if vault.IsSecretNotFound(err) {
		log.Infow("Instance admin credentials not found, trying default credentials",
			"postgresqlID", postgresqlID)
		defaultLogin, defaultPassword, defaultErr := vaultClient.GetDefaultCredentials(ctx)
		if defaultErr != nil {
			log.Warnw("Failed to get default credentials from Vault",
				"postgresqlID", postgresqlID, "error", defaultErr)
			return "", "", fmt.Errorf(
				"no credentials available: instance_admin not found, "+
					"default credentials also unavailable: %w", defaultErr)
		}
		log.Infow("Using default credentials for connection test",
			"postgresqlID", postgresqlID)
		return defaultLogin, defaultPassword, nil
	}

	// Vault error (not a 404) - don't fallback
	log.Warnw("Failed to get credentials from Vault",
		"postgresqlID", postgresqlID, "error", err)
	return "", "", fmt.Errorf("failed to get credentials from Vault: %w", err)
}

// TestConnectionFromPostgresql tests the connection to a PostgreSQL instance from a Postgresql CRD
func TestConnectionFromPostgresql(
	ctx context.Context,
	postgresql *instancev1alpha1.Postgresql,
	vaultClient *vault.Client,
	log *zap.SugaredLogger,
	retries int, retryTimeout time.Duration,
) error {
	ctx, span := otel.Tracer("postgresql").Start(ctx,
		"postgresql.TestConnectionFromPostgresql",
		trace.WithAttributes(
			attribute.String(telemetry.AttrOperation,
				"test_connection_from_postgresql"),
		))
	defer span.End()
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
	if vaultClient == nil {
		log.Warnw("Vault client not configured, skipping PostgreSQL connection test",
			"postgresqlID", externalInstance.PostgresqlID)
		return nil
	}

	username, password, credErr := resolveVaultCredentials(
		ctx, vaultClient, externalInstance.PostgresqlID, log)
	if credErr != nil {
		return credErr
	}

	// If no credentials from Vault, we cannot test the connection
	if username == "" || password == "" {
		return fmt.Errorf("no credentials available to test connection (credentials not found in Vault)")
	}

	_, err := TestConnection(ctx, externalInstance.Address, port, database, username, password, sslMode,
		log, retries, retryTimeout)
	return err
}
