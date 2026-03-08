//go:build integration

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

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
	"github.com/vyrodovalexey/k8s-postgresql-operator/test/helpers"
)

func newVaultClient(t *testing.T) *helpers.VaultTestClient {
	t.Helper()
	helpers.SkipIfEnvNotSet(t, "TEST_VAULT_ADDR", "TEST_VAULT_TOKEN")

	addr := helpers.GetEnvOrDefault("TEST_VAULT_ADDR", "http://localhost:8200")
	token := helpers.GetEnvOrDefault("TEST_VAULT_TOKEN", "myroot")

	return helpers.NewVaultTestClient(addr, token)
}

func newPGConfig(t *testing.T) *helpers.PostgreSQLTestConfig {
	t.Helper()
	helpers.SkipIfEnvNotSet(t, "TEST_POSTGRES_HOST", "TEST_POSTGRES_USER", "TEST_POSTGRES_PASSWORD")
	return helpers.NewPostgreSQLTestConfig()
}

func testLogger() *zap.SugaredLogger {
	logger, _ := zap.NewDevelopment()
	return logger.Sugar()
}

func TestIntegration_VaultCredentialFlow(t *testing.T) {
	vaultClient := newVaultClient(t)
	pgCfg := newPGConfig(t)
	log := testLogger()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	suffix := time.Now().UnixNano()
	postgresqlID := fmt.Sprintf("integ-test-%d", suffix)
	username := fmt.Sprintf("integ_vault_user_%d", suffix)
	password := "vault_cred_flow_pass"

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = vaultClient.DeleteSecret(cleanupCtx, "secret", fmt.Sprintf("pdb/%s/admin", postgresqlID))
		_ = pgCfg.DropTestUser(cleanupCtx, username)
	})

	// Step 1: Store admin credentials in Vault
	adminData := map[string]interface{}{
		"admin_username": pgCfg.User,
		"admin_password": pgCfg.Password,
	}
	err := vaultClient.WriteSecret(ctx, "secret", fmt.Sprintf("pdb/%s/admin", postgresqlID), adminData)
	require.NoError(t, err, "Storing admin credentials in Vault should succeed")

	// Step 2: Retrieve credentials from Vault
	readData, err := vaultClient.ReadSecret(ctx, "secret", fmt.Sprintf("pdb/%s/admin", postgresqlID))
	require.NoError(t, err, "Reading admin credentials from Vault should succeed")

	adminUser, ok := readData["admin_username"].(string)
	require.True(t, ok, "admin_username should be a string")
	adminPass, ok := readData["admin_password"].(string)
	require.True(t, ok, "admin_password should be a string")

	assert.Equal(t, pgCfg.User, adminUser)
	assert.Equal(t, pgCfg.Password, adminPass)

	// Step 3: Use retrieved credentials to connect to PostgreSQL
	connected, err := postgresql.TestConnection(
		ctx, pgCfg.Host, pgCfg.Port, "postgres",
		adminUser, adminPass, pgCfg.SSLMode,
		log, 3, 2*time.Second,
	)
	require.NoError(t, err, "Connecting to PostgreSQL with Vault credentials should succeed")
	assert.True(t, connected)

	// Step 4: Create a user using the retrieved admin credentials
	err = postgresql.CreateOrUpdateUser(
		ctx, pgCfg.Host, pgCfg.Port,
		adminUser, adminPass, pgCfg.SSLMode,
		username, password,
	)
	require.NoError(t, err, "Creating user with Vault-retrieved credentials should succeed")

	// Step 5: Verify the user exists
	exists, err := pgCfg.UserExists(ctx, username)
	require.NoError(t, err)
	assert.True(t, exists, "User created via Vault credential flow should exist")
}

func TestIntegration_VaultUserCredentialLifecycle(t *testing.T) {
	vaultClient := newVaultClient(t)
	pgCfg := newPGConfig(t)
	log := testLogger()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	suffix := time.Now().UnixNano()
	postgresqlID := fmt.Sprintf("integ-user-lifecycle-%d", suffix)
	username := fmt.Sprintf("integ_lifecycle_user_%d", suffix)
	password := "lifecycle_pass_123"

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = vaultClient.DeleteSecret(cleanupCtx, "secret", fmt.Sprintf("pdb/%s/%s", postgresqlID, username))
		_ = pgCfg.DropTestUser(cleanupCtx, username)
	})

	// Step 1: Store user password in Vault
	userData := map[string]interface{}{
		"password": password,
	}
	err := vaultClient.WriteSecret(ctx, "secret", fmt.Sprintf("pdb/%s/%s", postgresqlID, username), userData)
	require.NoError(t, err, "Storing user password in Vault should succeed")

	// Step 2: Retrieve user password from Vault
	readData, err := vaultClient.ReadSecret(ctx, "secret", fmt.Sprintf("pdb/%s/%s", postgresqlID, username))
	require.NoError(t, err, "Reading user password from Vault should succeed")

	retrievedPassword, ok := readData["password"].(string)
	require.True(t, ok, "password should be a string")
	assert.Equal(t, password, retrievedPassword)

	// Step 3: Create PostgreSQL user with the retrieved password
	err = postgresql.CreateOrUpdateUser(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		username, retrievedPassword,
	)
	require.NoError(t, err, "Creating PG user with Vault password should succeed")

	// Step 4: Verify login with the created user
	// Note: CreateOrUpdateUser uses pq.QuoteLiteral for the password, which properly
	// escapes it as a SQL string literal. The actual password stored in PostgreSQL
	// is the plain password value (no extra quoting).
	connected, err := postgresql.TestConnection(
		ctx, pgCfg.Host, pgCfg.Port, "postgres",
		username, retrievedPassword, pgCfg.SSLMode,
		log, 3, 2*time.Second,
	)
	require.NoError(t, err, "Login with created user should succeed")
	assert.True(t, connected)

	// Step 5: Update password in Vault
	newPassword := "updated_lifecycle_pass_456"
	updatedData := map[string]interface{}{
		"password": newPassword,
	}
	err = vaultClient.WriteSecret(ctx, "secret", fmt.Sprintf("pdb/%s/%s", postgresqlID, username), updatedData)
	require.NoError(t, err, "Updating user password in Vault should succeed")

	// Step 6: Update PostgreSQL user with new password
	err = postgresql.CreateOrUpdateUser(
		ctx, pgCfg.Host, pgCfg.Port,
		pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
		username, newPassword,
	)
	require.NoError(t, err, "Updating PG user password should succeed")

	// Step 7: Verify login with new password
	connected, err = postgresql.TestConnection(
		ctx, pgCfg.Host, pgCfg.Port, "postgres",
		username, newPassword, pgCfg.SSLMode,
		log, 3, 2*time.Second,
	)
	require.NoError(t, err, "Login with updated password should succeed")
	assert.True(t, connected)
}

func TestIntegration_VaultAdminCredentials(t *testing.T) {
	vaultClient := newVaultClient(t)
	pgCfg := newPGConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	suffix := time.Now().UnixNano()
	postgresqlID := fmt.Sprintf("integ-admin-%d", suffix)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = vaultClient.DeleteSecret(cleanupCtx, "secret", fmt.Sprintf("pdb/%s/admin", postgresqlID))
	})

	// Store admin credentials in the format expected by vault.Client.GetPostgresqlCredentials
	adminData := map[string]interface{}{
		"admin_username": pgCfg.User,
		"admin_password": pgCfg.Password,
	}
	err := vaultClient.WriteSecret(ctx, "secret", fmt.Sprintf("pdb/%s/admin", postgresqlID), adminData)
	require.NoError(t, err, "Storing admin credentials should succeed")

	// Read back and verify the format matches what GetPostgresqlCredentials expects
	readData, err := vaultClient.ReadSecret(ctx, "secret", fmt.Sprintf("pdb/%s/admin", postgresqlID))
	require.NoError(t, err, "Reading admin credentials should succeed")

	// Verify the keys match what vault.Client.GetPostgresqlCredentials looks for
	adminUsername, ok := readData["admin_username"].(string)
	require.True(t, ok, "admin_username key should exist and be a string")
	assert.NotEmpty(t, adminUsername, "admin_username should not be empty")

	adminPassword, ok := readData["admin_password"].(string)
	require.True(t, ok, "admin_password key should exist and be a string")
	assert.NotEmpty(t, adminPassword, "admin_password should not be empty")

	assert.Equal(t, pgCfg.User, adminUsername)
	assert.Equal(t, pgCfg.Password, adminPassword)
}
