//go:build functional

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

package functional_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vyrodovalexey/k8s-postgresql-operator/test/helpers"
)

func newVaultClient(t *testing.T) *helpers.VaultTestClient {
	t.Helper()
	helpers.SkipIfEnvNotSet(t, "TEST_VAULT_ADDR", "TEST_VAULT_TOKEN")

	addr := helpers.GetEnvOrDefault("TEST_VAULT_ADDR", "http://localhost:8200")
	token := helpers.GetEnvOrDefault("TEST_VAULT_TOKEN", "myroot")

	return helpers.NewVaultTestClient(addr, token)
}

func TestFunctional_VaultHealthCheck(t *testing.T) {
	client := newVaultClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.CheckHealth(ctx)
	require.NoError(t, err, "Vault health check should succeed")
}

func TestFunctional_VaultKVWrite(t *testing.T) {
	client := newVaultClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	secretPath := fmt.Sprintf("functional-test/write-%d", time.Now().UnixNano())
	data := map[string]interface{}{
		"username": "testuser",
		"password": "testpass123",
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = client.DeleteSecret(cleanupCtx, "secret", secretPath)
	})

	err := client.WriteSecret(ctx, "secret", secretPath, data)
	require.NoError(t, err, "Writing secret to Vault should succeed")

	// Verify the secret was written by reading it back
	readData, err := client.ReadSecret(ctx, "secret", secretPath)
	require.NoError(t, err, "Reading back written secret should succeed")
	assert.Equal(t, "testuser", readData["username"])
	assert.Equal(t, "testpass123", readData["password"])
}

func TestFunctional_VaultKVRead(t *testing.T) {
	client := newVaultClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	secretPath := fmt.Sprintf("functional-test/read-%d", time.Now().UnixNano())
	data := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = client.DeleteSecret(cleanupCtx, "secret", secretPath)
	})

	// Write a secret first
	err := client.WriteSecret(ctx, "secret", secretPath, data)
	require.NoError(t, err, "Writing secret should succeed")

	// Read the secret
	readData, err := client.ReadSecret(ctx, "secret", secretPath)
	require.NoError(t, err, "Reading secret should succeed")
	assert.Equal(t, "value1", readData["key1"])
	assert.Equal(t, "value2", readData["key2"])
}

func TestFunctional_VaultKVOverwrite(t *testing.T) {
	client := newVaultClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	secretPath := fmt.Sprintf("functional-test/overwrite-%d", time.Now().UnixNano())

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = client.DeleteSecret(cleanupCtx, "secret", secretPath)
	})

	// Write initial secret
	initialData := map[string]interface{}{
		"password": "initial-password",
	}
	err := client.WriteSecret(ctx, "secret", secretPath, initialData)
	require.NoError(t, err, "Writing initial secret should succeed")

	// Overwrite with new data
	updatedData := map[string]interface{}{
		"password": "updated-password",
	}
	err = client.WriteSecret(ctx, "secret", secretPath, updatedData)
	require.NoError(t, err, "Overwriting secret should succeed")

	// Read and verify the updated data
	readData, err := client.ReadSecret(ctx, "secret", secretPath)
	require.NoError(t, err, "Reading overwritten secret should succeed")
	assert.Equal(t, "updated-password", readData["password"],
		"Secret should contain the updated password")
}

func TestFunctional_VaultKVReadNonExistent(t *testing.T) {
	client := newVaultClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	secretPath := fmt.Sprintf("functional-test/nonexistent-%d", time.Now().UnixNano())

	_, err := client.ReadSecret(ctx, "secret", secretPath)
	require.Error(t, err, "Reading non-existent secret should return an error")
	assert.Contains(t, err.Error(), "not found",
		"Error should indicate the secret was not found")
}
