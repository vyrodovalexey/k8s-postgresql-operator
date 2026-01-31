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

	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"
)

// RetryConfig holds configuration for exponential backoff retry logic
type RetryConfig struct {
	// InitialInterval is the initial retry interval (default: 500ms)
	InitialInterval time.Duration
	// MaxInterval is the maximum retry interval (default: 30s)
	MaxInterval time.Duration
	// MaxElapsedTime is the maximum total time for all retries (default: 5m)
	MaxElapsedTime time.Duration
	// Multiplier is the factor by which the interval increases (default: 2.0)
	Multiplier float64
	// RandomizationFactor adds jitter to prevent thundering herd (default: 0.5)
	RandomizationFactor float64
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		InitialInterval:     500 * time.Millisecond,
		MaxInterval:         30 * time.Second,
		MaxElapsedTime:      5 * time.Minute,
		Multiplier:          2.0,
		RandomizationFactor: 0.5,
	}
}

// NewRetryConfigFromParams creates a RetryConfig from legacy parameters
// This provides backward compatibility with existing code
func NewRetryConfigFromParams(retries int, retryDelay time.Duration) RetryConfig {
	cfg := DefaultRetryConfig()
	cfg.InitialInterval = retryDelay
	// Calculate max elapsed time based on retries and delay
	// Use a reasonable multiplier to allow for exponential growth
	cfg.MaxElapsedTime = time.Duration(retries) * retryDelay * 3
	if cfg.MaxElapsedTime < 30*time.Second {
		cfg.MaxElapsedTime = 30 * time.Second
	}
	return cfg
}

// newExponentialBackOff creates a new exponential backoff with the given configuration
func newExponentialBackOff(cfg RetryConfig) *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = cfg.InitialInterval
	b.MaxInterval = cfg.MaxInterval
	b.MaxElapsedTime = cfg.MaxElapsedTime
	b.Multiplier = cfg.Multiplier
	b.RandomizationFactor = cfg.RandomizationFactor
	b.Reset()
	return b
}

// ContextAwareSleep sleeps for the specified duration but respects context cancellation
// Returns nil if sleep completed, or context error if cancelled
func ContextAwareSleep(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// ExecuteOperationWithRetry executes a PostgreSQL operation with exponential backoff retry logic
// The function respects context cancellation and will return early if the context is cancelled
func ExecuteOperationWithRetry(
	ctx context.Context, operation func() error, log *zap.SugaredLogger,
	retries int, retryDelay time.Duration, operationName string) error {
	// Convert legacy parameters to RetryConfig
	cfg := NewRetryConfigFromParams(retries, retryDelay)
	return ExecuteOperationWithRetryConfig(ctx, operation, log, cfg, operationName)
}

// ExecuteOperationWithRetryConfig executes a PostgreSQL operation with configurable exponential backoff
func ExecuteOperationWithRetryConfig(
	ctx context.Context, operation func() error, log *zap.SugaredLogger,
	cfg RetryConfig, operationName string) error {
	b := newExponentialBackOff(cfg)
	attempt := 0

	for {
		attempt++

		// Check context before attempting operation
		if err := ctx.Err(); err != nil {
			log.Warnw("Context cancelled before retry attempt",
				"operation", operationName, "attempt", attempt, "error", err)
			return fmt.Errorf("operation %s cancelled: %w", operationName, err)
		}

		log.Debugw("Attempting PostgreSQL operation",
			"operation", operationName, "attempt", attempt)

		err := operation()
		if err == nil {
			// Success!
			log.Debugw("PostgreSQL operation successful",
				"operation", operationName, "attempt", attempt)
			return nil
		}

		// Operation failed, calculate next backoff interval
		nextBackoff := b.NextBackOff()
		if nextBackoff == backoff.Stop {
			log.Errorw("PostgreSQL operation failed, max elapsed time reached",
				"operation", operationName, "attempt", attempt, "error", err,
				"maxElapsedTime", cfg.MaxElapsedTime)
			return fmt.Errorf("postgreSQL operation %s failed after %d attempts (max elapsed time %v): %w",
				operationName, attempt, cfg.MaxElapsedTime, err)
		}

		log.Warnw("PostgreSQL operation failed, will retry",
			"operation", operationName, "attempt", attempt, "error", err,
			"nextRetryIn", nextBackoff)

		// Context-aware sleep
		if sleepErr := ContextAwareSleep(ctx, nextBackoff); sleepErr != nil {
			log.Warnw("Context cancelled during retry backoff",
				"operation", operationName, "attempt", attempt, "error", sleepErr)
			return fmt.Errorf("operation %s cancelled during backoff: %w", operationName, sleepErr)
		}
	}
}
