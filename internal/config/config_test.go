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

	// Vault PKI defaults
	assert.Equal(t, defaultVaultPKIEnabled, cfg.VaultPKIEnabled)
	assert.Equal(t, defaultVaultPKIMountPath, cfg.VaultPKIMountPath)
	assert.Equal(t, defaultVaultPKIRole, cfg.VaultPKIRole)
	assert.Equal(t, defaultVaultPKITTL, cfg.VaultPKITTL)
	assert.Equal(t, defaultVaultPKIRenewalBuffer, cfg.VaultPKIRenewalBuffer)
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

	// Vault PKI defaults
	assert.False(t, cfg.VaultPKIEnabled)
	assert.Equal(t, "pki", cfg.VaultPKIMountPath)
	assert.Equal(t, "webhook-cert", cfg.VaultPKIRole)
	assert.Equal(t, "720h", cfg.VaultPKITTL)
	assert.Equal(t, "24h", cfg.VaultPKIRenewalBuffer)
	assert.Equal(t, 720*time.Hour, cfg.VaultPKITTLDuration())
	assert.Equal(t, 24*time.Hour, cfg.VaultPKIRenewalBufferDuration())
}

func TestConfig_VaultPKITTLDuration(t *testing.T) {
	tests := []struct {
		name     string
		ttl      string
		expected time.Duration
	}{
		{
			name:     "valid duration - hours",
			ttl:      "720h",
			expected: 720 * time.Hour,
		},
		{
			name:     "valid duration - minutes",
			ttl:      "60m",
			expected: 60 * time.Minute,
		},
		{
			name:     "valid duration - seconds",
			ttl:      "3600s",
			expected: 3600 * time.Second,
		},
		{
			name:     "invalid duration - returns default",
			ttl:      "invalid",
			expected: 720 * time.Hour, // Default
		},
		{
			name:     "empty duration - returns default",
			ttl:      "",
			expected: 720 * time.Hour, // Default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.VaultPKITTL = tt.ttl
			assert.Equal(t, tt.expected, cfg.VaultPKITTLDuration())
		})
	}
}

func TestConfig_VaultPKIRenewalBufferDuration(t *testing.T) {
	tests := []struct {
		name     string
		buffer   string
		expected time.Duration
	}{
		{
			name:     "valid duration - hours",
			buffer:   "24h",
			expected: 24 * time.Hour,
		},
		{
			name:     "valid duration - minutes",
			buffer:   "30m",
			expected: 30 * time.Minute,
		},
		{
			name:     "valid duration - seconds",
			buffer:   "1800s",
			expected: 1800 * time.Second,
		},
		{
			name:     "invalid duration - returns default",
			buffer:   "invalid",
			expected: 24 * time.Hour, // Default
		},
		{
			name:     "empty duration - returns default",
			buffer:   "",
			expected: 24 * time.Hour, // Default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.VaultPKIRenewalBuffer = tt.buffer
			assert.Equal(t, tt.expected, cfg.VaultPKIRenewalBufferDuration())
		})
	}
}

func TestConfig_SetupWebhooksList(t *testing.T) {
	cfg := New()
	webhooks := cfg.SetupWebhooksList()

	assert.Len(t, webhooks, 6)
	assert.Contains(t, webhooks, cfg.K8sWebhookNamePostgresql)
	assert.Contains(t, webhooks, cfg.K8sWebhookNameUser)
	assert.Contains(t, webhooks, cfg.K8sWebhookNameDatabase)
	assert.Contains(t, webhooks, cfg.K8sWebhookNameGrant)
	assert.Contains(t, webhooks, cfg.K8sWebhookNameRoleGroup)
	assert.Contains(t, webhooks, cfg.K8sWebhookNameSchema)
}

func TestConfig_VaultPKIFields(t *testing.T) {
	cfg := New()

	// Test that Vault PKI fields can be modified
	cfg.VaultPKIEnabled = true
	cfg.VaultPKIMountPath = "custom-pki"
	cfg.VaultPKIRole = "custom-role"
	cfg.VaultPKITTL = "48h"
	cfg.VaultPKIRenewalBuffer = "12h"

	assert.True(t, cfg.VaultPKIEnabled)
	assert.Equal(t, "custom-pki", cfg.VaultPKIMountPath)
	assert.Equal(t, "custom-role", cfg.VaultPKIRole)
	assert.Equal(t, "48h", cfg.VaultPKITTL)
	assert.Equal(t, "12h", cfg.VaultPKIRenewalBuffer)
	assert.Equal(t, 48*time.Hour, cfg.VaultPKITTLDuration())
	assert.Equal(t, 12*time.Hour, cfg.VaultPKIRenewalBufferDuration())
}

// TestConfig_VaultAuthTimeout tests VaultAuthTimeout method
func TestConfig_VaultAuthTimeout(t *testing.T) {
	tests := []struct {
		name            string
		timeoutSecs     int
		expectedTimeout time.Duration
	}{
		{
			name:            "default timeout",
			timeoutSecs:     30,
			expectedTimeout: 30 * time.Second,
		},
		{
			name:            "custom timeout",
			timeoutSecs:     60,
			expectedTimeout: 60 * time.Second,
		},
		{
			name:            "zero timeout",
			timeoutSecs:     0,
			expectedTimeout: 0,
		},
		{
			name:            "large timeout",
			timeoutSecs:     300,
			expectedTimeout: 300 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.VaultAuthTimeoutSecs = tt.timeoutSecs
			assert.Equal(t, tt.expectedTimeout, cfg.VaultAuthTimeout())
		})
	}
}

// TestConfig_RetryInitialInterval tests RetryInitialInterval method
func TestConfig_RetryInitialInterval(t *testing.T) {
	tests := []struct {
		name             string
		intervalMs       int
		expectedInterval time.Duration
	}{
		{
			name:             "default interval",
			intervalMs:       500,
			expectedInterval: 500 * time.Millisecond,
		},
		{
			name:             "custom interval",
			intervalMs:       1000,
			expectedInterval: 1000 * time.Millisecond,
		},
		{
			name:             "zero interval",
			intervalMs:       0,
			expectedInterval: 0,
		},
		{
			name:             "small interval",
			intervalMs:       100,
			expectedInterval: 100 * time.Millisecond,
		},
		{
			name:             "large interval",
			intervalMs:       5000,
			expectedInterval: 5000 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.RetryInitialIntervalMs = tt.intervalMs
			assert.Equal(t, tt.expectedInterval, cfg.RetryInitialInterval())
		})
	}
}

// TestConfig_RetryMaxInterval tests RetryMaxInterval method
func TestConfig_RetryMaxInterval(t *testing.T) {
	tests := []struct {
		name             string
		intervalSecs     int
		expectedInterval time.Duration
	}{
		{
			name:             "default interval",
			intervalSecs:     30,
			expectedInterval: 30 * time.Second,
		},
		{
			name:             "custom interval",
			intervalSecs:     60,
			expectedInterval: 60 * time.Second,
		},
		{
			name:             "zero interval",
			intervalSecs:     0,
			expectedInterval: 0,
		},
		{
			name:             "small interval",
			intervalSecs:     5,
			expectedInterval: 5 * time.Second,
		},
		{
			name:             "large interval",
			intervalSecs:     300,
			expectedInterval: 300 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.RetryMaxIntervalSecs = tt.intervalSecs
			assert.Equal(t, tt.expectedInterval, cfg.RetryMaxInterval())
		})
	}
}

// TestConfig_RetryMaxElapsedTime tests RetryMaxElapsedTime method
func TestConfig_RetryMaxElapsedTime(t *testing.T) {
	tests := []struct {
		name         string
		elapsedSecs  int
		expectedTime time.Duration
	}{
		{
			name:         "default elapsed time",
			elapsedSecs:  300,
			expectedTime: 300 * time.Second,
		},
		{
			name:         "custom elapsed time",
			elapsedSecs:  600,
			expectedTime: 600 * time.Second,
		},
		{
			name:         "zero elapsed time",
			elapsedSecs:  0,
			expectedTime: 0,
		},
		{
			name:         "small elapsed time",
			elapsedSecs:  30,
			expectedTime: 30 * time.Second,
		},
		{
			name:         "large elapsed time",
			elapsedSecs:  3600,
			expectedTime: 3600 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.RetryMaxElapsedTimeSecs = tt.elapsedSecs
			assert.Equal(t, tt.expectedTime, cfg.RetryMaxElapsedTime())
		})
	}
}

// TestConfig_RetryDefaults tests that retry defaults are set correctly
func TestConfig_RetryDefaults(t *testing.T) {
	cfg := New()

	assert.Equal(t, 500, cfg.RetryInitialIntervalMs)
	assert.Equal(t, 30, cfg.RetryMaxIntervalSecs)
	assert.Equal(t, 300, cfg.RetryMaxElapsedTimeSecs)
	assert.Equal(t, 2.0, cfg.RetryMultiplier)
	assert.Equal(t, 0.5, cfg.RetryRandomizationFactor)

	// Test duration methods
	assert.Equal(t, 500*time.Millisecond, cfg.RetryInitialInterval())
	assert.Equal(t, 30*time.Second, cfg.RetryMaxInterval())
	assert.Equal(t, 300*time.Second, cfg.RetryMaxElapsedTime())
}

// TestConfig_RateLimitDefaults tests that rate limit defaults are set correctly
func TestConfig_RateLimitDefaults(t *testing.T) {
	cfg := New()

	assert.Equal(t, 10.0, cfg.PostgresqlRateLimitPerSecond)
	assert.Equal(t, 20, cfg.PostgresqlRateLimitBurst)
	assert.Equal(t, 10.0, cfg.VaultRateLimitPerSecond)
	assert.Equal(t, 20, cfg.VaultRateLimitBurst)
}

// TestConfig_LogLevelDefault tests that log level default is set correctly
func TestConfig_LogLevelDefault(t *testing.T) {
	cfg := New()
	assert.Equal(t, "info", cfg.LogLevel)
}

// TestConfig_AllDurationMethods tests all duration conversion methods
func TestConfig_AllDurationMethods(t *testing.T) {
	cfg := New()

	// Set custom values
	cfg.PostgresqlConnectionTimeoutSecs = 15
	cfg.VaultAvailabilityRetryDelaySecs = 20
	cfg.VaultAuthTimeoutSecs = 45
	cfg.RetryInitialIntervalMs = 750
	cfg.RetryMaxIntervalSecs = 45
	cfg.RetryMaxElapsedTimeSecs = 450

	// Verify all duration methods
	assert.Equal(t, 15*time.Second, cfg.PostgresqlConnectionTimeout())
	assert.Equal(t, 20*time.Second, cfg.VaultAvailabilityRetryDelay())
	assert.Equal(t, 45*time.Second, cfg.VaultAuthTimeout())
	assert.Equal(t, 750*time.Millisecond, cfg.RetryInitialInterval())
	assert.Equal(t, 45*time.Second, cfg.RetryMaxInterval())
	assert.Equal(t, 450*time.Second, cfg.RetryMaxElapsedTime())
}

// TestConfig_ModifyAndVerify tests that config values can be modified
func TestConfig_ModifyAndVerify(t *testing.T) {
	cfg := New()

	// Modify values
	cfg.WebhookServerPort = 9443
	cfg.EnableLeaderElection = true
	cfg.VaultAddr = "https://vault.example.com:8200"
	cfg.PostgresqlConnectionRetries = 5
	cfg.LogLevel = "debug"

	// Verify modifications
	assert.Equal(t, 9443, cfg.WebhookServerPort)
	assert.True(t, cfg.EnableLeaderElection)
	assert.Equal(t, "https://vault.example.com:8200", cfg.VaultAddr)
	assert.Equal(t, 5, cfg.PostgresqlConnectionRetries)
	assert.Equal(t, "debug", cfg.LogLevel)
}

// TestConfig_Validate tests the Validate method
func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		modifyFunc  func(*Config)
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid default config",
			modifyFunc:  func(_ *Config) {},
			expectError: false,
		},
		{
			name: "invalid webhook port - too low",
			modifyFunc: func(c *Config) {
				c.WebhookServerPort = 0
			},
			expectError: true,
			errorMsg:    "webhook server port must be between 1 and 65535",
		},
		{
			name: "invalid webhook port - too high",
			modifyFunc: func(c *Config) {
				c.WebhookServerPort = 70000
			},
			expectError: true,
			errorMsg:    "webhook server port must be between 1 and 65535",
		},
		{
			name: "negative postgresql connection retries",
			modifyFunc: func(c *Config) {
				c.PostgresqlConnectionRetries = -1
			},
			expectError: true,
			errorMsg:    "postgresql connection retries must be non-negative",
		},
		{
			name: "negative postgresql rate limit per second",
			modifyFunc: func(c *Config) {
				c.PostgresqlRateLimitPerSecond = 0
			},
			expectError: true,
			errorMsg:    "postgresql rate limit per second must be positive",
		},
		{
			name: "negative postgresql rate limit burst",
			modifyFunc: func(c *Config) {
				c.PostgresqlRateLimitBurst = 0
			},
			expectError: true,
			errorMsg:    "postgresql rate limit burst must be positive",
		},
		{
			name: "invalid retry multiplier",
			modifyFunc: func(c *Config) {
				c.RetryMultiplier = 0.5
			},
			expectError: true,
			errorMsg:    "retry multiplier must be at least 1.0",
		},
		{
			name: "invalid retry randomization factor - too high",
			modifyFunc: func(c *Config) {
				c.RetryRandomizationFactor = 1.5
			},
			expectError: true,
			errorMsg:    "retry randomization factor must be between 0 and 1",
		},
		{
			name: "invalid log level",
			modifyFunc: func(c *Config) {
				c.LogLevel = "invalid"
			},
			expectError: true,
			errorMsg:    "log level must be one of: debug, info, warn, error",
		},
		{
			name: "valid log level - debug",
			modifyFunc: func(c *Config) {
				c.LogLevel = "debug"
			},
			expectError: false,
		},
		{
			name: "valid log level - warn",
			modifyFunc: func(c *Config) {
				c.LogLevel = "warn"
			},
			expectError: false,
		},
		{
			name: "invalid vault PKI TTL when enabled",
			modifyFunc: func(c *Config) {
				c.VaultPKIEnabled = true
				c.VaultPKITTL = "invalid"
			},
			expectError: true,
			errorMsg:    "vault PKI TTL must be a valid duration",
		},
		{
			name: "invalid vault PKI renewal buffer when enabled",
			modifyFunc: func(c *Config) {
				c.VaultPKIEnabled = true
				c.VaultPKIRenewalBuffer = "invalid"
			},
			expectError: true,
			errorMsg:    "vault PKI renewal buffer must be a valid duration",
		},
		{
			name: "valid vault PKI config when enabled",
			modifyFunc: func(c *Config) {
				c.VaultPKIEnabled = true
				c.VaultPKITTL = "720h"
				c.VaultPKIRenewalBuffer = "24h"
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			tt.modifyFunc(cfg)
			err := cfg.Validate()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestConfigValidationError tests the ConfigValidationError type
func TestConfigValidationError(t *testing.T) {
	t.Run("single error", func(t *testing.T) {
		err := &ConfigValidationError{Errors: []string{"single error"}}
		assert.Equal(t, "configuration validation error: single error", err.Error())
	})

	t.Run("multiple errors", func(t *testing.T) {
		err := &ConfigValidationError{Errors: []string{"error 1", "error 2"}}
		errStr := err.Error()
		assert.Contains(t, errStr, "configuration validation errors:")
		assert.Contains(t, errStr, "error 1")
		assert.Contains(t, errStr, "error 2")
	})
}
