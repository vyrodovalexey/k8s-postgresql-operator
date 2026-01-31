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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/events"
)

func TestBaseReconcilerConfig_Initialization(t *testing.T) {
	tests := []struct {
		name                            string
		postgresqlConnectionRetries     int
		postgresqlConnectionTimeoutSecs int
		vaultAvailabilityRetries        int
		vaultAvailabilityRetryDelaySecs int
		expectedPostgresqlRetries       int
		expectedPostgresqlTimeout       time.Duration
		expectedVaultRetries            int
		expectedVaultRetryDelay         time.Duration
	}{
		{
			name:                            "Default configuration values",
			postgresqlConnectionRetries:     3,
			postgresqlConnectionTimeoutSecs: 10,
			vaultAvailabilityRetries:        3,
			vaultAvailabilityRetryDelaySecs: 10,
			expectedPostgresqlRetries:       3,
			expectedPostgresqlTimeout:       10 * time.Second,
			expectedVaultRetries:            3,
			expectedVaultRetryDelay:         10 * time.Second,
		},
		{
			name:                            "Custom configuration values",
			postgresqlConnectionRetries:     5,
			postgresqlConnectionTimeoutSecs: 30,
			vaultAvailabilityRetries:        10,
			vaultAvailabilityRetryDelaySecs: 5,
			expectedPostgresqlRetries:       5,
			expectedPostgresqlTimeout:       30 * time.Second,
			expectedVaultRetries:            10,
			expectedVaultRetryDelay:         5 * time.Second,
		},
		{
			name:                            "Zero retries configuration",
			postgresqlConnectionRetries:     0,
			postgresqlConnectionTimeoutSecs: 0,
			vaultAvailabilityRetries:        0,
			vaultAvailabilityRetryDelaySecs: 0,
			expectedPostgresqlRetries:       0,
			expectedPostgresqlTimeout:       0,
			expectedVaultRetries:            0,
			expectedVaultRetryDelay:         0,
		},
		{
			name:                            "Large timeout values",
			postgresqlConnectionRetries:     100,
			postgresqlConnectionTimeoutSecs: 300,
			vaultAvailabilityRetries:        50,
			vaultAvailabilityRetryDelaySecs: 60,
			expectedPostgresqlRetries:       100,
			expectedPostgresqlTimeout:       300 * time.Second,
			expectedVaultRetries:            50,
			expectedVaultRetryDelay:         60 * time.Second,
		},
		{
			name:                            "Single retry configuration",
			postgresqlConnectionRetries:     1,
			postgresqlConnectionTimeoutSecs: 1,
			vaultAvailabilityRetries:        1,
			vaultAvailabilityRetryDelaySecs: 1,
			expectedPostgresqlRetries:       1,
			expectedPostgresqlTimeout:       1 * time.Second,
			expectedVaultRetries:            1,
			expectedVaultRetryDelay:         1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockClient := new(MockControllerClient)
			logger := zap.NewNop().Sugar()

			// Act
			baseCfg := BaseReconcilerConfig{
				Client:                      mockClient,
				VaultClient:                 nil,
				Log:                         logger,
				EventRecorder:               events.NewEventRecorder(nil),
				PostgresqlConnectionRetries: tt.postgresqlConnectionRetries,
				PostgresqlConnectionTimeout: time.Duration(tt.postgresqlConnectionTimeoutSecs) * time.Second,
				VaultAvailabilityRetries:    tt.vaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: time.Duration(tt.vaultAvailabilityRetryDelaySecs) * time.Second,
			}

			// Assert
			assert.NotNil(t, baseCfg.Client)
			assert.NotNil(t, baseCfg.Log)
			assert.Nil(t, baseCfg.VaultClient)
			assert.Equal(t, tt.expectedPostgresqlRetries, baseCfg.PostgresqlConnectionRetries)
			assert.Equal(t, tt.expectedPostgresqlTimeout, baseCfg.PostgresqlConnectionTimeout)
			assert.Equal(t, tt.expectedVaultRetries, baseCfg.VaultAvailabilityRetries)
			assert.Equal(t, tt.expectedVaultRetryDelay, baseCfg.VaultAvailabilityRetryDelay)
		})
	}
}

func TestBaseReconcilerConfig_WithVaultClient(t *testing.T) {
	// Arrange
	mockClient := new(MockControllerClient)
	logger := zap.NewNop().Sugar()

	// Act - test with nil vault client
	baseCfg := BaseReconcilerConfig{
		Client:                      mockClient,
		VaultClient:                 nil,
		Log:                         logger,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
	}

	// Assert
	assert.Nil(t, baseCfg.VaultClient)
	assert.NotNil(t, baseCfg.Client)
	assert.NotNil(t, baseCfg.Log)
}

func TestBaseReconcilerConfig_ConfigIntegration(t *testing.T) {
	tests := []struct {
		name            string
		configOverrides func(*config.Config)
		expectedRetries int
		expectedTimeout time.Duration
	}{
		{
			name:            "Default config values",
			configOverrides: nil,
			expectedRetries: 3,
			expectedTimeout: 10 * time.Second,
		},
		{
			name: "Custom config values",
			configOverrides: func(cfg *config.Config) {
				cfg.PostgresqlConnectionRetries = 5
				cfg.PostgresqlConnectionTimeoutSecs = 30
			},
			expectedRetries: 5,
			expectedTimeout: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := config.New()
			if tt.configOverrides != nil {
				tt.configOverrides(cfg)
			}

			mockClient := new(MockControllerClient)
			logger := zap.NewNop().Sugar()

			// Act
			baseCfg := BaseReconcilerConfig{
				Client:                      mockClient,
				VaultClient:                 nil,
				Log:                         logger,
				PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
				PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
				VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
			}

			// Assert
			assert.Equal(t, tt.expectedRetries, baseCfg.PostgresqlConnectionRetries)
			assert.Equal(t, tt.expectedTimeout, baseCfg.PostgresqlConnectionTimeout)
		})
	}
}

func TestControllerSetup_Structure(t *testing.T) {
	// Test the controllerSetup struct
	tests := []struct {
		name        string
		setupName   string
		setupFunc   func() error
		expectError bool
	}{
		{
			name:      "Setup with nil function",
			setupName: "TestController",
			setupFunc: func() error {
				return nil
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			setup := controllerSetup{
				name:  tt.setupName,
				setup: tt.setupFunc,
			}

			// Act
			err := setup.setup()

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.setupName, setup.name)
		})
	}
}
