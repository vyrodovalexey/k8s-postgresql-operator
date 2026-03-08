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

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cfg := New()

	assert.NotNil(t, cfg)
	assert.Equal(t, defaultWebhookCertPath, cfg.WebhookCertPath)
	assert.Equal(t, defaultWebhookCertName, cfg.WebhookCertName)
	assert.Equal(t, defaultWebhookCertKey, cfg.WebhookCertKey)
	assert.Equal(t, defaultWebhookServerPort, cfg.WebhookServerPort)
	assert.Equal(t, defaultWebhookServerAddr, cfg.WebhookServerAddr)
	assert.Equal(t, defaultWebhookK8sServiceName, cfg.WebhookK8sServiceName)
	assert.Equal(t, defaultK8sWebhookNamePostgresql, cfg.K8sWebhookNamePostgresql)
	assert.Equal(t, defaultK8sWebhookNameUser, cfg.K8sWebhookNameUser)
	assert.Equal(t, defaultK8sWebhookNameDatabase, cfg.K8sWebhookNameDatabase)
	assert.Equal(t, defaultK8sWebhookNameGrant, cfg.K8sWebhookNameGrant)
	assert.Equal(t, defaultK8sWebhookNameRoleGroup, cfg.K8sWebhookNameRoleGroup)
	assert.Equal(t, defaultK8sWebhookNameSchema, cfg.K8sWebhookNameSchema)
	assert.Equal(t, defaultEnableLeaderElection, cfg.EnableLeaderElection)
	assert.Equal(t, defaultProbeAddr, cfg.ProbeAddr)
	assert.Equal(t, defaultVaultAddr, cfg.VaultAddr)
	assert.Equal(t, defaultVaultRole, cfg.VaultRole)
	assert.Equal(t, defaultVaultMountPoint, cfg.VaultMountPoint)
	assert.Equal(t, defaultVaultSecretPath, cfg.VaultSecretPath)
	assert.Equal(t, defaultK8sTokenPath, cfg.K8sTokenPath)
	assert.Equal(t, defaultK8SNamespacePath, cfg.K8sNamespacePath)
	assert.Equal(t, defaultExcludeUserList, cfg.ExcludeUserList)
	assert.Equal(t, defaultPostgresqlConnectionRetries, cfg.PostgresqlConnectionRetries)
	assert.Equal(t, defaultPostgresqlConnectionTimeoutSecs, cfg.PostgresqlConnectionTimeoutSecs)
	assert.Equal(t, defaultVaultAvailabilityRetries, cfg.VaultAvailabilityRetries)
	assert.Equal(t, defaultVaultAvailabilityRetryDelaySecs, cfg.VaultAvailabilityRetryDelaySecs)
}

func TestNew_MultipleInstances(t *testing.T) {
	cfg1 := New()
	cfg2 := New()

	// Each call should return a new instance
	assert.NotSame(t, cfg1, cfg2)
	assert.Equal(t, cfg1.WebhookCertPath, cfg2.WebhookCertPath)
}

func TestConfig_DefaultValues(t *testing.T) {
	cfg := New()

	// Verify default values match constants
	assert.Equal(t, "", cfg.WebhookCertPath)
	assert.Equal(t, "tls.crt", cfg.WebhookCertName)
	assert.Equal(t, "tls.key", cfg.WebhookCertKey)
	assert.Equal(t, 8443, cfg.WebhookServerPort)
	assert.Equal(t, "0.0.0.0", cfg.WebhookServerAddr)
	assert.Equal(t, "k8s-postgresql-operator-controller-service", cfg.WebhookK8sServiceName)
	assert.Equal(t, "k8s-postgresql-operator-validating-webhook-postgresql", cfg.K8sWebhookNamePostgresql)
	assert.Equal(t, "k8s-postgresql-operator-validating-webhook-user", cfg.K8sWebhookNameUser)
	assert.Equal(t, "k8s-postgresql-operator-validating-webhook-database", cfg.K8sWebhookNameDatabase)
	assert.Equal(t, "k8s-postgresql-operator-validating-webhook-grant", cfg.K8sWebhookNameGrant)
	assert.Equal(t, "k8s-postgresql-operator-validating-webhook-rolegroup", cfg.K8sWebhookNameRoleGroup)
	assert.Equal(t, "k8s-postgresql-operator-validating-webhook-schema", cfg.K8sWebhookNameSchema)
	assert.False(t, cfg.EnableLeaderElection)
	assert.Equal(t, ":8081", cfg.ProbeAddr)
	assert.Equal(t, "", cfg.VaultAddr)
	assert.Equal(t, "role", cfg.VaultRole)
	assert.Equal(t, "secret", cfg.VaultMountPoint)
	assert.Equal(t, "pdb", cfg.VaultSecretPath)
	assert.Equal(t, "/var/run/secrets/kubernetes.io/serviceaccount/token", cfg.K8sTokenPath)
	assert.Equal(t, "/var/run/secrets/kubernetes.io/serviceaccount/namespace", cfg.K8sNamespacePath)
	assert.Equal(t, "postgres", cfg.ExcludeUserList)
	assert.Equal(t, 3, cfg.PostgresqlConnectionRetries)
	assert.Equal(t, 10, cfg.PostgresqlConnectionTimeoutSecs)
	assert.Equal(t, 10*time.Second, cfg.PostgresqlConnectionTimeout())
	assert.Equal(t, 3, cfg.VaultAvailabilityRetries)
	assert.Equal(t, 10, cfg.VaultAvailabilityRetryDelaySecs)
	assert.Equal(t, 10*time.Second, cfg.VaultAvailabilityRetryDelay())
}

func TestSetupWebhooksList_DefaultConfig(t *testing.T) {
	// Arrange
	cfg := New()

	// Act
	webhooks := cfg.SetupWebhooksList()

	// Assert
	assert.Len(t, webhooks, 6)
	assert.Equal(t, []string{
		defaultK8sWebhookNamePostgresql,
		defaultK8sWebhookNameUser,
		defaultK8sWebhookNameDatabase,
		defaultK8sWebhookNameGrant,
		defaultK8sWebhookNameRoleGroup,
		defaultK8sWebhookNameSchema,
	}, webhooks)
}

func TestSetupWebhooksList_CustomConfig(t *testing.T) {
	// Arrange
	cfg := New()
	cfg.K8sWebhookNamePostgresql = "custom-postgresql"
	cfg.K8sWebhookNameUser = "custom-user"
	cfg.K8sWebhookNameDatabase = "custom-database"
	cfg.K8sWebhookNameGrant = "custom-grant"
	cfg.K8sWebhookNameRoleGroup = "custom-rolegroup"
	cfg.K8sWebhookNameSchema = "custom-schema"

	// Act
	webhooks := cfg.SetupWebhooksList()

	// Assert
	assert.Len(t, webhooks, 6)
	assert.Equal(t, []string{
		"custom-postgresql",
		"custom-user",
		"custom-database",
		"custom-grant",
		"custom-rolegroup",
		"custom-schema",
	}, webhooks)
}

func TestSetupWebhooksList_ContainsAllWebhookNames(t *testing.T) {
	// Arrange
	cfg := New()

	// Act
	webhooks := cfg.SetupWebhooksList()

	// Assert - verify each webhook name is present in the list
	assert.Contains(t, webhooks, cfg.K8sWebhookNamePostgresql)
	assert.Contains(t, webhooks, cfg.K8sWebhookNameUser)
	assert.Contains(t, webhooks, cfg.K8sWebhookNameDatabase)
	assert.Contains(t, webhooks, cfg.K8sWebhookNameGrant)
	assert.Contains(t, webhooks, cfg.K8sWebhookNameRoleGroup)
	assert.Contains(t, webhooks, cfg.K8sWebhookNameSchema)
}

func TestSetupWebhooksList_Order(t *testing.T) {
	// Arrange
	cfg := New()

	// Act
	webhooks := cfg.SetupWebhooksList()

	// Assert - verify the order matches the expected order
	assert.Equal(t, cfg.K8sWebhookNamePostgresql, webhooks[0])
	assert.Equal(t, cfg.K8sWebhookNameUser, webhooks[1])
	assert.Equal(t, cfg.K8sWebhookNameDatabase, webhooks[2])
	assert.Equal(t, cfg.K8sWebhookNameGrant, webhooks[3])
	assert.Equal(t, cfg.K8sWebhookNameRoleGroup, webhooks[4])
	assert.Equal(t, cfg.K8sWebhookNameSchema, webhooks[5])
}

func TestPostgresqlConnectionTimeout_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		timeoutSecs int
		expected    time.Duration
	}{
		{
			name:        "default timeout",
			timeoutSecs: 10,
			expected:    10 * time.Second,
		},
		{
			name:        "zero timeout",
			timeoutSecs: 0,
			expected:    0,
		},
		{
			name:        "one second timeout",
			timeoutSecs: 1,
			expected:    1 * time.Second,
		},
		{
			name:        "large timeout",
			timeoutSecs: 300,
			expected:    300 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := New()
			cfg.PostgresqlConnectionTimeoutSecs = tt.timeoutSecs

			// Act
			result := cfg.PostgresqlConnectionTimeout()

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_New_VaultPKIDefaults(t *testing.T) {
	cfg := New()

	assert.True(t, cfg.VaultPKIEnabled)
	assert.Equal(t, "pki", cfg.VaultPKIMountPath)
	assert.Equal(t, "webhook-cert", cfg.VaultPKIRole)
	assert.Equal(t, "720h", cfg.VaultPKITTL)
	assert.Equal(t, "24h", cfg.VaultPKIRenewalBuffer)
}

func TestConfig_VaultPKIEnabled_EnvOverride(t *testing.T) {
	// Verify that the struct fields can be set (simulating env override)
	cfg := New()

	// Override PKI fields as env parsing would
	cfg.VaultPKIEnabled = false
	cfg.VaultPKIMountPath = "custom-pki"
	cfg.VaultPKIRole = "custom-role"
	cfg.VaultPKITTL = "48h"
	cfg.VaultPKIRenewalBuffer = "12h"

	assert.False(t, cfg.VaultPKIEnabled)
	assert.Equal(t, "custom-pki", cfg.VaultPKIMountPath)
	assert.Equal(t, "custom-role", cfg.VaultPKIRole)
	assert.Equal(t, "48h", cfg.VaultPKITTL)
	assert.Equal(t, "12h", cfg.VaultPKIRenewalBuffer)
}

func TestMetricsCollectionInterval_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		intervalSec int
		expected    time.Duration
	}{
		{
			name:        "default interval",
			intervalSec: 30,
			expected:    30 * time.Second,
		},
		{
			name:        "zero interval",
			intervalSec: 0,
			expected:    0,
		},
		{
			name:        "one second interval",
			intervalSec: 1,
			expected:    1 * time.Second,
		},
		{
			name:        "large interval",
			intervalSec: 600,
			expected:    600 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := New()
			cfg.MetricsCollectionIntervalSecs = tt.intervalSec

			// Act
			result := cfg.MetricsCollectionInterval()

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMetricsCollectionInterval_DefaultValue(t *testing.T) {
	// Arrange
	cfg := New()

	// Act
	result := cfg.MetricsCollectionInterval()

	// Assert
	assert.Equal(t, 30*time.Second, result)
	assert.Equal(t, defaultMetricsCollectionIntervalSecs, cfg.MetricsCollectionIntervalSecs)
}

func TestVaultAvailabilityRetryDelay_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		delaySec int
		expected time.Duration
	}{
		{
			name:     "default delay",
			delaySec: 10,
			expected: 10 * time.Second,
		},
		{
			name:     "zero delay",
			delaySec: 0,
			expected: 0,
		},
		{
			name:     "one second delay",
			delaySec: 1,
			expected: 1 * time.Second,
		},
		{
			name:     "large delay",
			delaySec: 600,
			expected: 600 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := New()
			cfg.VaultAvailabilityRetryDelaySecs = tt.delaySec

			// Act
			result := cfg.VaultAvailabilityRetryDelay()

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}
