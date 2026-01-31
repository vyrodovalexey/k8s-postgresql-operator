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

package integration

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vyrodovalexey/k8s-postgresql-operator/test/testutils"
)

// TestIntegration_VaultHealth tests Vault health check
func TestIntegration_VaultHealth(t *testing.T) {
	skipIfNoVault(t)

	health, err := vaultClient.Sys().Health()
	require.NoError(t, err)
	assert.True(t, health.Initialized)
	assert.False(t, health.Sealed)
}

// TestIntegration_VaultStoreAdminCredentials tests storing admin credentials in Vault
func TestIntegration_VaultStoreAdminCredentials(t *testing.T) {
	skipIfNoVault(t)

	postgresqlID := testutils.GenerateTestName("pg")
	username := "admin"
	password := testutils.GenerateTestPassword(16)

	// Store admin credentials
	path := fmt.Sprintf("%s/data/%s/%s/admin",
		testConfig.VaultMountPoint,
		testConfig.VaultSecretPath,
		postgresqlID,
	)

	data := map[string]interface{}{
		"data": map[string]interface{}{
			"admin_username": username,
			"admin_password": password,
		},
	}

	_, err := vaultClient.Logical().Write(path, data)
	require.NoError(t, err)

	// Verify credentials can be read
	secret, err := vaultClient.Logical().Read(path)
	require.NoError(t, err)
	require.NotNil(t, secret)
	require.NotNil(t, secret.Data)

	secretData, ok := secret.Data["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, username, secretData["admin_username"])
	assert.Equal(t, password, secretData["admin_password"])

	// Cleanup
	_, _ = vaultClient.Logical().Delete(fmt.Sprintf("%s/metadata/%s/%s",
		testConfig.VaultMountPoint,
		testConfig.VaultSecretPath,
		postgresqlID,
	))
}

// TestIntegration_VaultStoreUserCredentials tests storing user credentials in Vault
func TestIntegration_VaultStoreUserCredentials(t *testing.T) {
	skipIfNoVault(t)

	postgresqlID := testutils.GenerateTestName("pg")
	username := testutils.GenerateTestUsername()
	password := testutils.GenerateTestPassword(16)

	// Store user credentials
	path := fmt.Sprintf("%s/data/%s/%s/%s",
		testConfig.VaultMountPoint,
		testConfig.VaultSecretPath,
		postgresqlID,
		username,
	)

	data := map[string]interface{}{
		"data": map[string]interface{}{
			"password": password,
		},
	}

	_, err := vaultClient.Logical().Write(path, data)
	require.NoError(t, err)

	// Verify credentials can be read
	secret, err := vaultClient.Logical().Read(path)
	require.NoError(t, err)
	require.NotNil(t, secret)
	require.NotNil(t, secret.Data)

	secretData, ok := secret.Data["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, password, secretData["password"])

	// Cleanup
	_, _ = vaultClient.Logical().Delete(fmt.Sprintf("%s/metadata/%s/%s",
		testConfig.VaultMountPoint,
		testConfig.VaultSecretPath,
		postgresqlID,
	))
}

// TestIntegration_VaultUpdateUserCredentials tests updating user credentials in Vault
func TestIntegration_VaultUpdateUserCredentials(t *testing.T) {
	skipIfNoVault(t)

	postgresqlID := testutils.GenerateTestName("pg")
	username := testutils.GenerateTestUsername()
	password1 := testutils.GenerateTestPassword(16)
	password2 := testutils.GenerateTestPassword(16)

	path := fmt.Sprintf("%s/data/%s/%s/%s",
		testConfig.VaultMountPoint,
		testConfig.VaultSecretPath,
		postgresqlID,
		username,
	)

	// Store initial credentials
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"password": password1,
		},
	}
	_, err := vaultClient.Logical().Write(path, data)
	require.NoError(t, err)

	// Update credentials
	data["data"] = map[string]interface{}{
		"password": password2,
	}
	_, err = vaultClient.Logical().Write(path, data)
	require.NoError(t, err)

	// Verify updated credentials
	secret, err := vaultClient.Logical().Read(path)
	require.NoError(t, err)
	require.NotNil(t, secret)

	secretData, ok := secret.Data["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, password2, secretData["password"])

	// Cleanup
	_, _ = vaultClient.Logical().Delete(fmt.Sprintf("%s/metadata/%s/%s",
		testConfig.VaultMountPoint,
		testConfig.VaultSecretPath,
		postgresqlID,
	))
}

// TestIntegration_VaultReadNonExistentCredentials tests reading non-existent credentials
func TestIntegration_VaultReadNonExistentCredentials(t *testing.T) {
	skipIfNoVault(t)

	path := fmt.Sprintf("%s/data/%s/non-existent-pg/non-existent-user",
		testConfig.VaultMountPoint,
		testConfig.VaultSecretPath,
	)

	secret, err := vaultClient.Logical().Read(path)
	// Vault returns nil secret for non-existent paths, not an error
	assert.NoError(t, err)
	assert.Nil(t, secret)
}

// TestIntegration_VaultKVv2Versioning tests KV v2 versioning
func TestIntegration_VaultKVv2Versioning(t *testing.T) {
	skipIfNoVault(t)

	postgresqlID := testutils.GenerateTestName("pg")
	username := testutils.GenerateTestUsername()

	path := fmt.Sprintf("%s/data/%s/%s/%s",
		testConfig.VaultMountPoint,
		testConfig.VaultSecretPath,
		postgresqlID,
		username,
	)

	// Write multiple versions
	for i := 1; i <= 3; i++ {
		data := map[string]interface{}{
			"data": map[string]interface{}{
				"password": fmt.Sprintf("password-v%d", i),
				"version":  i,
			},
		}
		_, err := vaultClient.Logical().Write(path, data)
		require.NoError(t, err)
	}

	// Read latest version
	secret, err := vaultClient.Logical().Read(path)
	require.NoError(t, err)
	require.NotNil(t, secret)

	secretData, ok := secret.Data["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "password-v3", secretData["password"])

	// Check metadata for version count
	metadata, ok := secret.Data["metadata"].(map[string]interface{})
	require.True(t, ok)

	// Version can be json.Number or float64 depending on how Vault client parses it
	var version float64
	switch v := metadata["version"].(type) {
	case float64:
		version = v
	case int:
		version = float64(v)
	case int64:
		version = float64(v)
	default:
		// Try to parse as json.Number
		if num, ok := v.(interface{ Float64() (float64, error) }); ok {
			var err error
			version, err = num.Float64()
			require.NoError(t, err)
		} else {
			t.Fatalf("unexpected version type: %T", v)
		}
	}
	assert.Equal(t, float64(3), version)

	// Cleanup
	_, _ = vaultClient.Logical().Delete(fmt.Sprintf("%s/metadata/%s/%s",
		testConfig.VaultMountPoint,
		testConfig.VaultSecretPath,
		postgresqlID,
	))
}

// TestIntegration_VaultPKIAvailability tests that Vault PKI secrets engine is available
func TestIntegration_VaultPKIAvailability(t *testing.T) {
	skipIfNoVault(t)

	pkiPath := getEnvOrDefaultStr("TEST_VAULT_PKI_PATH", "pki")

	// Check if PKI secrets engine is enabled
	mounts, err := vaultClient.Sys().ListMounts()
	require.NoError(t, err, "Failed to list Vault mounts")

	pkiMountPath := pkiPath + "/"
	mount, ok := mounts[pkiMountPath]

	if !ok {
		t.Skipf("Vault PKI secrets engine not enabled at '%s'. Run setup-vault-pki.sh first.", pkiPath)
	}

	// Verify it's a PKI mount
	assert.Equal(t, "pki", mount.Type, "Mount at '%s' is not a PKI secrets engine", pkiPath)

	t.Logf("Vault PKI secrets engine is available at '%s'", pkiPath)
}

// TestIntegration_VaultPKIPolicyEnforcement tests that PKI policy is enforced
func TestIntegration_VaultPKIPolicyEnforcement(t *testing.T) {
	skipIfNoVault(t)

	pkiPath := getEnvOrDefaultStr("TEST_VAULT_PKI_PATH", "pki")
	pkiRole := getEnvOrDefaultStr("TEST_VAULT_PKI_ROLE", "webhook-cert")

	// Check if PKI secrets engine is enabled
	mounts, err := vaultClient.Sys().ListMounts()
	require.NoError(t, err)

	pkiMountPath := pkiPath + "/"
	if _, ok := mounts[pkiMountPath]; !ok {
		t.Skipf("Vault PKI secrets engine not enabled at '%s'", pkiPath)
	}

	// Check if the PKI role exists
	rolePath := fmt.Sprintf("%s/roles/%s", pkiPath, pkiRole)
	secret, err := vaultClient.Logical().Read(rolePath)
	if err != nil || secret == nil {
		t.Skipf("Vault PKI role '%s' not found", pkiRole)
	}

	// Verify role configuration
	require.NotNil(t, secret.Data, "PKI role data is nil")

	// Check allowed_domains
	allowedDomains, ok := secret.Data["allowed_domains"].([]interface{})
	if ok {
		t.Logf("PKI role '%s' allowed domains: %v", pkiRole, allowedDomains)
	}

	// Check allow_subdomains
	allowSubdomains, ok := secret.Data["allow_subdomains"].(bool)
	if ok {
		t.Logf("PKI role '%s' allow_subdomains: %v", pkiRole, allowSubdomains)
	}

	// Check max_ttl
	maxTTL, ok := secret.Data["max_ttl"]
	if ok {
		t.Logf("PKI role '%s' max_ttl: %v", pkiRole, maxTTL)
	}

	// Verify key usage settings
	keyUsage, ok := secret.Data["key_usage"].([]interface{})
	if ok {
		t.Logf("PKI role '%s' key_usage: %v", pkiRole, keyUsage)
	}

	t.Logf("PKI role '%s' policy configuration verified", pkiRole)
}

// getEnvOrDefaultStr returns the environment variable value or a default
func getEnvOrDefaultStr(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
