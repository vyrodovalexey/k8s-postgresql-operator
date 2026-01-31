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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGetVaultCredentialsWithRetryAndRateLimit_NilClient(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	username, password, err := getVaultCredentialsWithRetryAndRateLimit(
		ctx, nil, "test-id", logger, 3, 100*time.Millisecond, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
	assert.Empty(t, username)
	assert.Empty(t, password)
}

func TestGetVaultUserCredentialsWithRetryAndRateLimit_NilClient(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	password, err := getVaultUserCredentialsWithRetryAndRateLimit(
		ctx, nil, "test-id", "user", logger, 3, 100*time.Millisecond, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
	assert.Empty(t, password)
}

func TestStoreVaultUserCredentialsWithRetryAndRateLimit_NilClient(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	err := storeVaultUserCredentialsWithRetryAndRateLimit(
		ctx, nil, "test-id", "user", "pass", logger, 3, 100*time.Millisecond, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// Note: Full retry testing with actual vault.Client requires integration tests
// or refactoring the functions to accept an interface instead of *vault.Client
