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

package controller

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// getVaultCredentialsWithRetry retrieves PostgreSQL credentials from Vault with retry logic
func getVaultCredentialsWithRetry(
	ctx context.Context, vaultClient *vault.Client, postgresqlID string,
	log *zap.SugaredLogger, retries int, retryDelay time.Duration) (username, password string, err error) {
	if vaultClient == nil {
		return "", "", fmt.Errorf("vault client is not configured")
	}

	var lastErr error
	for attempt := 1; attempt <= retries; attempt++ {
		log.Debugw("Attempting to get credentials from Vault",
			"postgresqlID", postgresqlID, "attempt", attempt, "maxRetries", retries)

		username, password, err := vaultClient.GetPostgresqlCredentials(ctx, postgresqlID)
		if err != nil {
			lastErr = err
			log.Warnw("Failed to get credentials from Vault",
				"postgresqlID", postgresqlID, "attempt", attempt, "error", err)
			if attempt < retries {
				log.Debugw("Retrying Vault credentials retrieval",
					"attempt", attempt, "nextAttempt", attempt+1, "delay", retryDelay)
				time.Sleep(retryDelay)
			}
			continue
		}

		// Success!
		log.Debugw("Credentials retrieved from Vault", "postgresqlID", postgresqlID, "attempt", attempt)
		return username, password, nil
	}

	// All retries failed
	return "", "", fmt.Errorf("failed to get credentials from Vault after %d attempts: %w", retries, lastErr)
}

// getVaultUserCredentialsWithRetry retrieves PostgreSQL user credentials from Vault with retry logic
func getVaultUserCredentialsWithRetry(
	ctx context.Context, vaultClient *vault.Client, postgresqlID, username string,
	log *zap.SugaredLogger, retries int, retryDelay time.Duration) (password string, err error) {
	if vaultClient == nil {
		return "", fmt.Errorf("vault client is not configured")
	}

	var lastErr error
	for attempt := 1; attempt <= retries; attempt++ {
		log.Debugw("Attempting to get user credentials from Vault",
			"postgresqlID", postgresqlID, "username", username, "attempt", attempt, "maxRetries", retries)

		password, err := vaultClient.GetPostgresqlUserCredentials(ctx, postgresqlID, username)
		if err != nil {
			lastErr = err
			log.Warnw("Failed to get user credentials from Vault",
				"postgresqlID", postgresqlID, "username", username, "attempt", attempt, "error", err)
			if attempt < retries {
				log.Debugw("Retrying Vault user credentials retrieval",
					"attempt", attempt, "nextAttempt", attempt+1, "delay", retryDelay)
				time.Sleep(retryDelay)
			}
			continue
		}

		// Success!
		log.Debugw("User credentials retrieved from Vault",
			"postgresqlID", postgresqlID, "username", username, "attempt", attempt)
		return password, nil
	}

	// All retries failed
	return "", fmt.Errorf("failed to get user credentials from Vault after %d attempts: %w", retries, lastErr)
}

// storeVaultUserCredentialsWithRetry stores PostgreSQL user credentials in Vault with retry logic
func storeVaultUserCredentialsWithRetry(
	ctx context.Context, vaultClient *vault.Client, postgresqlID, username, password string,
	log *zap.SugaredLogger, retries int, retryDelay time.Duration) error {
	if vaultClient == nil {
		return fmt.Errorf("vault client is not configured")
	}

	var lastErr error
	for attempt := 1; attempt <= retries; attempt++ {
		log.Debugw("Attempting to store user credentials in Vault",
			"postgresqlID", postgresqlID, "username", username, "attempt", attempt, "maxRetries", retries)

		err := vaultClient.StorePostgresqlUserCredentials(ctx, postgresqlID, username, password)
		if err != nil {
			lastErr = err
			log.Warnw("Failed to store user credentials in Vault",
				"postgresqlID", postgresqlID, "username", username, "attempt", attempt, "error", err)
			if attempt < retries {
				log.Debugw("Retrying Vault user credentials storage",
					"attempt", attempt, "nextAttempt", attempt+1, "delay", retryDelay)
				time.Sleep(retryDelay)
			}
			continue
		}

		// Success!
		log.Debugw("User credentials stored in Vault", "postgresqlID", postgresqlID, "username", username, "attempt", attempt)
		return nil
	}

	// All retries failed
	return fmt.Errorf("failed to store user credentials in Vault after %d attempts: %w", retries, lastErr)
}
