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
	assert.Equal(t, "http://0.0.0.0:8200", cfg.VaultAddr)
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
