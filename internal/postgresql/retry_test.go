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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// testRetryConfig returns a fast retry config for testing
func testRetryConfig() RetryConfig {
	return RetryConfig{
		InitialInterval:     1 * time.Millisecond,
		MaxInterval:         10 * time.Millisecond,
		MaxElapsedTime:      100 * time.Millisecond,
		Multiplier:          1.5,
		RandomizationFactor: 0.0, // No jitter for predictable tests
	}
}

func TestExecuteOperationWithRetry_Success(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	operation := func() error {
		return nil
	}

	err := ExecuteOperationWithRetry(ctx, operation, logger, 3, 1*time.Second, "testOperation")

	assert.NoError(t, err)
}

func TestExecuteOperationWithRetry_SuccessAfterRetry(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary error")
		}
		return nil
	}

	cfg := testRetryConfig()
	err := ExecuteOperationWithRetryConfig(ctx, operation, logger, cfg, "testOperation")

	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestExecuteOperationWithRetry_AllRetriesFail(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	operation := func() error {
		return errors.New("persistent error")
	}

	cfg := testRetryConfig()
	cfg.MaxElapsedTime = 50 * time.Millisecond // Short timeout for fast test
	err := ExecuteOperationWithRetryConfig(ctx, operation, logger, cfg, "testOperation")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after")
	assert.Contains(t, err.Error(), "persistent error")
}

func TestExecuteOperationWithRetry_ZeroRetries(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	operation := func() error {
		return errors.New("error")
	}

	// With zero retries, the max elapsed time should be very short
	cfg := RetryConfig{
		InitialInterval:     1 * time.Millisecond,
		MaxInterval:         1 * time.Millisecond,
		MaxElapsedTime:      1 * time.Nanosecond, // Effectively no retries
		Multiplier:          1.0,
		RandomizationFactor: 0.0,
	}
	err := ExecuteOperationWithRetryConfig(ctx, operation, logger, cfg, "testOperation")

	assert.Error(t, err)
}

func TestExecuteOperationWithRetry_OneRetry(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 2 {
			return errors.New("first attempt fails")
		}
		return nil
	}

	cfg := testRetryConfig()
	err := ExecuteOperationWithRetryConfig(ctx, operation, logger, cfg, "testOperation")

	// Should try once, fail, then try again and succeed
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestExecuteOperationWithRetry_ContextCancellation(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	operation := func() error {
		attempts++
		if attempts == 1 {
			cancel() // Cancel context after first attempt
		}
		return errors.New("error")
	}

	cfg := testRetryConfig()
	cfg.MaxElapsedTime = 1 * time.Second // Long enough to allow retries
	err := ExecuteOperationWithRetryConfig(ctx, operation, logger, cfg, "testOperation")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestExecuteOperationWithRetry_ContextAlreadyCancelled(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	operation := func() error {
		return nil
	}

	cfg := testRetryConfig()
	err := ExecuteOperationWithRetryConfig(ctx, operation, logger, cfg, "testOperation")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestContextAwareSleep(t *testing.T) {
	t.Run("completes normally", func(t *testing.T) {
		ctx := context.Background()
		err := ContextAwareSleep(ctx, 1*time.Millisecond)
		assert.NoError(t, err)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := ContextAwareSleep(ctx, 1*time.Hour) // Long sleep
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("zero duration returns immediately", func(t *testing.T) {
		ctx := context.Background()
		err := ContextAwareSleep(ctx, 0)
		assert.NoError(t, err)
	})

	t.Run("negative duration returns immediately", func(t *testing.T) {
		ctx := context.Background()
		err := ContextAwareSleep(ctx, -1*time.Second)
		assert.NoError(t, err)
	})
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	assert.Equal(t, 500*time.Millisecond, cfg.InitialInterval)
	assert.Equal(t, 30*time.Second, cfg.MaxInterval)
	assert.Equal(t, 5*time.Minute, cfg.MaxElapsedTime)
	assert.Equal(t, 2.0, cfg.Multiplier)
	assert.Equal(t, 0.5, cfg.RandomizationFactor)
}

func TestNewRetryConfigFromParams(t *testing.T) {
	cfg := NewRetryConfigFromParams(3, 5*time.Second)

	assert.Equal(t, 5*time.Second, cfg.InitialInterval)
	// MaxElapsedTime should be at least 30 seconds
	assert.GreaterOrEqual(t, cfg.MaxElapsedTime, 30*time.Second)
}
