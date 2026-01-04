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

package webhook

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// checkVaultAvailability checks if Vault is available with retry logic
func checkVaultAvailability(
	ctx context.Context, vaultClient *vault.Client, log *zap.SugaredLogger,
	retries int, retryDelay time.Duration) error {
	if vaultClient == nil {
		return fmt.Errorf("vault client is not configured")
	}

	var lastErr error
	for attempt := 1; attempt <= retries; attempt++ {
		log.Debugw("Checking Vault availability", "attempt", attempt, "maxRetries", retries)

		// Use the vault client's health check method
		err := vaultClient.CheckHealth(ctx)
		if err != nil {
			lastErr = fmt.Errorf("vault health check failed: %w", err)
			log.Warnw("Vault availability check failed", "attempt", attempt, "error", lastErr)
			if attempt < retries {
				log.Debugw("Retrying Vault availability check", "attempt", attempt, "nextAttempt", attempt+1, "delay", retryDelay)
				time.Sleep(retryDelay)
			}
			continue
		}

		// Health check successful
		log.Debugw("Vault is available", "attempt", attempt)
		return nil
	}

	// All retries failed
	return fmt.Errorf("vault is not available after %d attempts: %w", retries, lastErr)
}
