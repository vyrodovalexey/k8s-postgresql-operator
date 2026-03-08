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

func TestGetVaultCredentialsWithRetry_NilClient(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	username, password, err := getVaultCredentialsWithRetry(
		ctx, nil, "test-id", logger, 3, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
	assert.Empty(t, username)
	assert.Empty(t, password)
}

func TestGetVaultUserCredentialsWithRetry_NilClient(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	password, err := getVaultUserCredentialsWithRetry(
		ctx, nil, "test-id", "user", logger, 3, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
	assert.Empty(t, password)
}

func TestStoreVaultUserCredentialsWithRetry_NilClient(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	err := storeVaultUserCredentialsWithRetry(
		ctx, nil, "test-id", "user", "pass", logger, 3, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

func TestGetDefaultVaultCredentialsWithRetry_NilClient_Unit(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	login, password, err := getDefaultVaultCredentialsWithRetry(
		ctx, nil, logger, 3, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
	assert.Empty(t, login)
	assert.Empty(t, password)
}

func TestStoreVaultCredentialsWithRetry_NilClient_Unit(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	err := storeVaultCredentialsWithRetry(
		ctx, nil, "test-id", "admin", "pass",
		logger, 3, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

func TestGetInstanceAdminNewPasswordWithRetry_NilClient_Unit(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	password, err := getInstanceAdminNewPasswordWithRetry(
		ctx, nil, "test-id",
		logger, 3, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
	assert.Empty(t, password)
}

func TestRotateInstanceAdminPasswordWithRetry_NilClient_Unit(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	err := rotateInstanceAdminPasswordWithRetry(
		ctx, nil, "test-id", "newpass",
		logger, 3, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// Note: Full retry testing with actual vault.Client requires integration tests
// or refactoring the functions to accept an interface instead of *vault.Client
