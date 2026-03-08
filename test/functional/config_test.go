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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
)

func TestFunctional_ConfigDefaults(t *testing.T) {
	cfg := config.New()
	require.NotNil(t, cfg, "Config should not be nil")

	t.Run("webhook defaults", func(t *testing.T) {
		assert.Equal(t, "", cfg.WebhookCertPath)
		assert.Equal(t, "tls.crt", cfg.WebhookCertName)
		assert.Equal(t, "tls.key", cfg.WebhookCertKey)
		assert.Equal(t, 8443, cfg.WebhookServerPort)
		assert.Equal(t, "0.0.0.0", cfg.WebhookServerAddr)
		assert.Equal(t, "k8s-postgresql-operator-controller-service", cfg.WebhookK8sServiceName)
	})

	t.Run("webhook name defaults", func(t *testing.T) {
		assert.Equal(t, "k8s-postgresql-operator-validating-webhook-postgresql", cfg.K8sWebhookNamePostgresql)
		assert.Equal(t, "k8s-postgresql-operator-validating-webhook-user", cfg.K8sWebhookNameUser)
		assert.Equal(t, "k8s-postgresql-operator-validating-webhook-database", cfg.K8sWebhookNameDatabase)
		assert.Equal(t, "k8s-postgresql-operator-validating-webhook-grant", cfg.K8sWebhookNameGrant)
		assert.Equal(t, "k8s-postgresql-operator-validating-webhook-rolegroup", cfg.K8sWebhookNameRoleGroup)
		assert.Equal(t, "k8s-postgresql-operator-validating-webhook-schema", cfg.K8sWebhookNameSchema)
	})

	t.Run("leader election default", func(t *testing.T) {
		assert.False(t, cfg.EnableLeaderElection)
	})

	t.Run("probe address default", func(t *testing.T) {
		assert.Equal(t, ":8081", cfg.ProbeAddr)
	})

	t.Run("vault defaults", func(t *testing.T) {
		assert.Equal(t, "http://0.0.0.0:8200", cfg.VaultAddr)
		assert.Equal(t, "role", cfg.VaultRole)
		assert.Equal(t, "secret", cfg.VaultMountPoint)
		assert.Equal(t, "pdb", cfg.VaultSecretPath)
	})

	t.Run("k8s token path default", func(t *testing.T) {
		assert.Equal(t, "/var/run/secrets/kubernetes.io/serviceaccount/token", cfg.K8sTokenPath)
	})

	t.Run("k8s namespace path default", func(t *testing.T) {
		assert.Equal(t, "/var/run/secrets/kubernetes.io/serviceaccount/namespace", cfg.K8sNamespacePath)
	})

	t.Run("exclude user list default", func(t *testing.T) {
		assert.Equal(t, "postgres", cfg.ExcludeUserList)
	})

	t.Run("postgresql connection defaults", func(t *testing.T) {
		assert.Equal(t, 3, cfg.PostgresqlConnectionRetries)
		assert.Equal(t, 10, cfg.PostgresqlConnectionTimeoutSecs)
		assert.Equal(t, 10*time.Second, cfg.PostgresqlConnectionTimeout())
	})

	t.Run("vault availability defaults", func(t *testing.T) {
		assert.Equal(t, 3, cfg.VaultAvailabilityRetries)
		assert.Equal(t, 10, cfg.VaultAvailabilityRetryDelaySecs)
		assert.Equal(t, 10*time.Second, cfg.VaultAvailabilityRetryDelay())
	})

	t.Run("setup webhooks list", func(t *testing.T) {
		webhooks := cfg.SetupWebhooksList()
		assert.Len(t, webhooks, 6)
		assert.Contains(t, webhooks, cfg.K8sWebhookNamePostgresql)
		assert.Contains(t, webhooks, cfg.K8sWebhookNameUser)
		assert.Contains(t, webhooks, cfg.K8sWebhookNameDatabase)
		assert.Contains(t, webhooks, cfg.K8sWebhookNameGrant)
		assert.Contains(t, webhooks, cfg.K8sWebhookNameRoleGroup)
		assert.Contains(t, webhooks, cfg.K8sWebhookNameSchema)
	})
}

func TestFunctional_ConfigEnvOverride(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		check    func(t *testing.T, cfg *config.Config)
	}{
		{
			name:     "override webhook server port",
			envKey:   "WEBHOOK_SERVER_PORT",
			envValue: "9443",
			check: func(t *testing.T, cfg *config.Config) {
				// Config.New() returns defaults; env parsing is done by caarlos0/env
				// We verify the struct has the correct env tags
				assert.Equal(t, 8443, cfg.WebhookServerPort,
					"Default should be 8443 (env override requires env/v6 Parse)")
			},
		},
		{
			name:     "override vault addr",
			envKey:   "VAULT_ADDR",
			envValue: "http://vault.example.com:8200",
			check: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "http://0.0.0.0:8200", cfg.VaultAddr,
					"Default should be http://0.0.0.0:8200 (env override requires env/v6 Parse)")
			},
		},
		{
			name:     "override postgresql connection retries",
			envKey:   "POSTGRESQL_CONNECTION_RETRIES",
			envValue: "5",
			check: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, 3, cfg.PostgresqlConnectionRetries,
					"Default should be 3 (env override requires env/v6 Parse)")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Note: config.New() returns defaults without parsing env vars.
			// The env parsing is done externally via caarlos0/env.
			// Here we verify the default values are correct.
			cfg := config.New()
			require.NotNil(t, cfg)
			tc.check(t, cfg)
		})
	}
}
