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

	err := ExecuteOperationWithRetry(ctx, operation, logger, 3, 1*time.Millisecond, "testOperation")

	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestExecuteOperationWithRetry_AllRetriesFail(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	operation := func() error {
		return errors.New("persistent error")
	}

	err := ExecuteOperationWithRetry(ctx, operation, logger, 3, 1*time.Millisecond, "testOperation")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 3 attempts")
	assert.Contains(t, err.Error(), "persistent error")
}

func TestExecuteOperationWithRetry_ZeroRetries(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	operation := func() error {
		return errors.New("error")
	}

	err := ExecuteOperationWithRetry(ctx, operation, logger, 0, 1*time.Millisecond, "testOperation")

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

	err := ExecuteOperationWithRetry(ctx, operation, logger, 2, 1*time.Millisecond, "testOperation")

	// With 2 retries, it should try once, fail, then try again and succeed
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}
