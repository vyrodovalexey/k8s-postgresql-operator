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
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ExecuteOperationWithRetry executes a PostgreSQL operation with retry logic
func ExecuteOperationWithRetry(
	ctx context.Context, operation func() error, log *zap.SugaredLogger,
	retries int, retryDelay time.Duration, operationName string) error {
	var lastErr error
	for attempt := 1; attempt <= retries; attempt++ {
		log.Debugw("Attempting PostgreSQL operation", "operation", operationName, "attempt", attempt, "maxRetries", retries)

		err := operation()
		if err != nil {
			lastErr = err
			log.Warnw("PostgreSQL operation failed", "operation", operationName, "attempt", attempt, "error", err)
			if attempt < retries {
				log.Debugw("Retrying PostgreSQL operation",
					"operation", operationName, "attempt", attempt, "nextAttempt", attempt+1, "delay", retryDelay)
				time.Sleep(retryDelay)
			}
			continue
		}

		// Success!
		log.Debugw("PostgreSQL operation successful", "operation", operationName, "attempt", attempt)
		return nil
	}

	// All retries failed
	return fmt.Errorf("postgreSQL operation %s failed after %d attempts: %w", operationName, retries, lastErr)
}
