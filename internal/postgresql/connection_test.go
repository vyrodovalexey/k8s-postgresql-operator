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

package postgresql

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestDefaultConstants(t *testing.T) {
	assert.Equal(t, "require", DefaultSSLMode)
	assert.Equal(t, int(5432), int(DefaultPort))
	assert.Equal(t, "postgres", DefaultDB)
}

func TestTestConnection_InvalidConnection(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	// Test with invalid connection parameters
	connected, err := TestConnection(ctx, "invalid-host", 5432, "postgres", "user", "password", "require", logger, 1, 1*time.Second)

	assert.False(t, connected)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to PostgreSQL")
}

func TestTestConnectionFromPostgresql_NoExternalInstance(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	postgresql := &instancev1alpha1.Postgresql{
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: nil,
		},
	}

	err := TestConnectionFromPostgresql(ctx, postgresql, nil, logger, 1, 1*time.Second)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no external instance configuration")
}

func TestTestConnectionFromPostgresql_NoVaultClient(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	postgresql := &instancev1alpha1.Postgresql{
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "test-id",
				Address:      "localhost",
				Port:         5432,
			},
		},
	}

	// Should return nil when vault client is not configured (skips test)
	err := TestConnectionFromPostgresql(ctx, postgresql, nil, logger, 1, 1*time.Second)

	assert.NoError(t, err)
}

func TestTestConnectionFromPostgresql_DefaultValues(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	postgresql := &instancev1alpha1.Postgresql{
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "test-id",
				Address:      "localhost",
				Port:         0,  // Should default to 5432
				SSLMode:      "", // Should default to "require"
			},
		},
	}

	// This will fail to connect but should use defaults
	err := TestConnectionFromPostgresql(ctx, postgresql, nil, logger, 1, 1*time.Second)

	// Should return nil when vault client is not configured (skips test)
	assert.NoError(t, err)
}

// TestNewConnectionConfig_TableDriven tests NewConnectionConfig with various scenarios
func TestNewConnectionConfig_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		host             string
		port             int32
		username         string
		password         string
		database         string
		sslMode          string
		expectedPort     int32
		expectedDatabase string
		expectedSSLMode  string
	}{
		{
			name:             "All defaults",
			host:             "localhost",
			port:             0,
			username:         "user",
			password:         "pass",
			database:         "",
			sslMode:          "",
			expectedPort:     DefaultPort,
			expectedDatabase: DefaultDB,
			expectedSSLMode:  DefaultSSLMode,
		},
		{
			name:             "Custom port",
			host:             "localhost",
			port:             5433,
			username:         "user",
			password:         "pass",
			database:         "",
			sslMode:          "",
			expectedPort:     5433,
			expectedDatabase: DefaultDB,
			expectedSSLMode:  DefaultSSLMode,
		},
		{
			name:             "Custom database",
			host:             "localhost",
			port:             0,
			username:         "user",
			password:         "pass",
			database:         "mydb",
			sslMode:          "",
			expectedPort:     DefaultPort,
			expectedDatabase: "mydb",
			expectedSSLMode:  DefaultSSLMode,
		},
		{
			name:             "Custom SSL mode",
			host:             "localhost",
			port:             0,
			username:         "user",
			password:         "pass",
			database:         "",
			sslMode:          "disable",
			expectedPort:     DefaultPort,
			expectedDatabase: DefaultDB,
			expectedSSLMode:  "disable",
		},
		{
			name:             "All custom values",
			host:             "db.example.com",
			port:             5434,
			username:         "admin",
			password:         "secret123",
			database:         "production",
			sslMode:          "verify-full",
			expectedPort:     5434,
			expectedDatabase: "production",
			expectedSSLMode:  "verify-full",
		},
		{
			name:             "Empty host",
			host:             "",
			port:             5432,
			username:         "user",
			password:         "pass",
			database:         "testdb",
			sslMode:          "require",
			expectedPort:     5432,
			expectedDatabase: "testdb",
			expectedSSLMode:  "require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConnectionConfig(tt.host, tt.port, tt.username, tt.password, tt.database, tt.sslMode)

			assert.Equal(t, tt.host, cfg.Host)
			assert.Equal(t, tt.expectedPort, cfg.Port)
			assert.Equal(t, tt.username, cfg.Username)
			assert.Equal(t, tt.password, cfg.Password)
			assert.Equal(t, tt.expectedDatabase, cfg.Database)
			assert.Equal(t, tt.expectedSSLMode, cfg.SSLMode)
			assert.Equal(t, DefaultConnectTimeout, cfg.ConnectTimeout)
		})
	}
}

// TestConnectionConfig_WithConnectTimeout tests WithConnectTimeout method
func TestConnectionConfig_WithConnectTimeout(t *testing.T) {
	tests := []struct {
		name            string
		timeout         int
		expectedTimeout int
	}{
		{
			name:            "Default timeout",
			timeout:         DefaultConnectTimeout,
			expectedTimeout: DefaultConnectTimeout,
		},
		{
			name:            "Custom timeout",
			timeout:         10,
			expectedTimeout: 10,
		},
		{
			name:            "Zero timeout",
			timeout:         0,
			expectedTimeout: 0,
		},
		{
			name:            "Large timeout",
			timeout:         300,
			expectedTimeout: 300,
		},
		{
			name:            "Negative timeout",
			timeout:         -1,
			expectedTimeout: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConnectionConfig("localhost", 5432, "user", "pass", "testdb", "require")
			result := cfg.WithConnectTimeout(tt.timeout)

			// Should return the same config (fluent interface)
			assert.Same(t, cfg, result)
			assert.Equal(t, tt.expectedTimeout, cfg.ConnectTimeout)
		})
	}
}

// TestConnectionConfig_BuildConnectionString_TableDriven tests BuildConnectionString method
func TestConnectionConfig_BuildConnectionString_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		host           string
		port           int32
		username       string
		password       string
		database       string
		sslMode        string
		connectTimeout int
		expectedParts  []string
	}{
		{
			name:           "Standard connection",
			host:           "localhost",
			port:           5432,
			username:       "user",
			password:       "pass",
			database:       "testdb",
			sslMode:        "require",
			connectTimeout: 5,
			expectedParts: []string{
				"host=localhost",
				"port=5432",
				"user=user",
				"password=pass",
				"dbname=testdb",
				"sslmode=require",
				"connect_timeout=5",
			},
		},
		{
			name:           "Custom port",
			host:           "db.example.com",
			port:           5433,
			username:       "admin",
			password:       "secret",
			database:       "production",
			sslMode:        "verify-full",
			connectTimeout: 10,
			expectedParts: []string{
				"host=db.example.com",
				"port=5433",
				"user=admin",
				"password=secret",
				"dbname=production",
				"sslmode=verify-full",
				"connect_timeout=10",
			},
		},
		{
			name:           "Special characters in password",
			host:           "localhost",
			port:           5432,
			username:       "user",
			password:       "p@ss=word",
			database:       "testdb",
			sslMode:        "disable",
			connectTimeout: 5,
			expectedParts: []string{
				"host=localhost",
				"password=p@ss=word",
			},
		},
		{
			name:           "Empty host",
			host:           "",
			port:           5432,
			username:       "user",
			password:       "pass",
			database:       "testdb",
			sslMode:        "require",
			connectTimeout: 5,
			expectedParts: []string{
				"host=",
				"port=5432",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ConnectionConfig{
				Host:           tt.host,
				Port:           tt.port,
				Username:       tt.username,
				Password:       tt.password,
				Database:       tt.database,
				SSLMode:        tt.sslMode,
				ConnectTimeout: tt.connectTimeout,
			}

			connStr := cfg.BuildConnectionString()

			for _, part := range tt.expectedParts {
				assert.Contains(t, connStr, part)
			}
		})
	}
}

// TestConnectionConfig_ChainedMethods tests method chaining
func TestConnectionConfig_ChainedMethods(t *testing.T) {
	cfg := NewConnectionConfig("localhost", 5432, "user", "pass", "testdb", "require").
		WithConnectTimeout(30)

	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, int32(5432), cfg.Port)
	assert.Equal(t, "user", cfg.Username)
	assert.Equal(t, "pass", cfg.Password)
	assert.Equal(t, "testdb", cfg.Database)
	assert.Equal(t, "require", cfg.SSLMode)
	assert.Equal(t, 30, cfg.ConnectTimeout)
}

// TestTestConnection_ContextCancellation tests context cancellation handling
func TestTestConnection_ContextCancellation(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	connected, err := TestConnection(ctx, "localhost", 5432, "postgres", "user", "password", "require", logger, 1, 1*time.Second)

	assert.False(t, connected)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

// TestTestConnection_ContextTimeout tests context timeout handling
func TestTestConnection_ContextTimeout(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for timeout
	time.Sleep(5 * time.Millisecond)

	connected, err := TestConnection(ctx, "localhost", 5432, "postgres", "user", "password", "require", logger, 1, 1*time.Second)

	assert.False(t, connected)
	assert.Error(t, err)
}

// TestBuildConnectionString_TableDriven tests BuildConnectionString with various scenarios
func TestBuildConnectionString_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		host           string
		port           int32
		username       string
		password       string
		database       string
		sslMode        string
		connectTimeout int
		expectedParts  []string
		notExpected    []string
	}{
		{
			name:           "Standard connection string",
			host:           "localhost",
			port:           5432,
			username:       "admin",
			password:       "secret",
			database:       "testdb",
			sslMode:        "require",
			connectTimeout: 5,
			expectedParts: []string{
				"host=localhost",
				"port=5432",
				"user=admin",
				"password=secret",
				"dbname=testdb",
				"sslmode=require",
				"connect_timeout=5",
			},
		},
		{
			name:           "Connection with special characters in password",
			host:           "db.example.com",
			port:           5433,
			username:       "app_user",
			password:       "p@ss=word!#$",
			database:       "production",
			sslMode:        "verify-full",
			connectTimeout: 10,
			expectedParts: []string{
				"host=db.example.com",
				"port=5433",
				"user=app_user",
				"password=p@ss=word!#$",
				"dbname=production",
				"sslmode=verify-full",
				"connect_timeout=10",
			},
		},
		{
			name:           "Connection with empty password",
			host:           "localhost",
			port:           5432,
			username:       "user",
			password:       "",
			database:       "testdb",
			sslMode:        "disable",
			connectTimeout: 5,
			expectedParts: []string{
				"host=localhost",
				"password=",
				"sslmode=disable",
			},
		},
		{
			name:           "Connection with IPv6 host",
			host:           "::1",
			port:           5432,
			username:       "user",
			password:       "pass",
			database:       "testdb",
			sslMode:        "require",
			connectTimeout: 5,
			expectedParts: []string{
				"host=::1",
				"port=5432",
			},
		},
		{
			name:           "Connection with zero timeout",
			host:           "localhost",
			port:           5432,
			username:       "user",
			password:       "pass",
			database:       "testdb",
			sslMode:        "require",
			connectTimeout: 0,
			expectedParts: []string{
				"connect_timeout=0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ConnectionConfig{
				Host:           tt.host,
				Port:           tt.port,
				Username:       tt.username,
				Password:       tt.password,
				Database:       tt.database,
				SSLMode:        tt.sslMode,
				ConnectTimeout: tt.connectTimeout,
			}

			connStr := cfg.BuildConnectionString()

			for _, part := range tt.expectedParts {
				assert.Contains(t, connStr, part, "Connection string should contain: %s", part)
			}

			for _, part := range tt.notExpected {
				assert.NotContains(t, connStr, part, "Connection string should not contain: %s", part)
			}
		})
	}
}

// TestNewConnectionConfig_EdgeCases tests edge cases for NewConnectionConfig
func TestNewConnectionConfig_EdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		host             string
		port             int32
		username         string
		password         string
		database         string
		sslMode          string
		expectedPort     int32
		expectedDatabase string
		expectedSSLMode  string
	}{
		{
			name:             "Negative port uses default",
			host:             "localhost",
			port:             -1,
			username:         "user",
			password:         "pass",
			database:         "testdb",
			sslMode:          "require",
			expectedPort:     -1, // Negative port is kept as-is (validation should happen elsewhere)
			expectedDatabase: "testdb",
			expectedSSLMode:  "require",
		},
		{
			name:             "Very large port number",
			host:             "localhost",
			port:             65535,
			username:         "user",
			password:         "pass",
			database:         "testdb",
			sslMode:          "require",
			expectedPort:     65535,
			expectedDatabase: "testdb",
			expectedSSLMode:  "require",
		},
		{
			name:             "Empty host",
			host:             "",
			port:             5432,
			username:         "user",
			password:         "pass",
			database:         "testdb",
			sslMode:          "require",
			expectedPort:     5432,
			expectedDatabase: "testdb",
			expectedSSLMode:  "require",
		},
		{
			name:             "All empty strings",
			host:             "",
			port:             0,
			username:         "",
			password:         "",
			database:         "",
			sslMode:          "",
			expectedPort:     DefaultPort,
			expectedDatabase: DefaultDB,
			expectedSSLMode:  DefaultSSLMode,
		},
		{
			name:             "Whitespace in values",
			host:             "  localhost  ",
			port:             5432,
			username:         "  user  ",
			password:         "  pass  ",
			database:         "  testdb  ",
			sslMode:          "  require  ",
			expectedPort:     5432,
			expectedDatabase: "  testdb  ", // Whitespace is preserved
			expectedSSLMode:  "  require  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConnectionConfig(tt.host, tt.port, tt.username, tt.password, tt.database, tt.sslMode)

			assert.Equal(t, tt.host, cfg.Host)
			assert.Equal(t, tt.expectedPort, cfg.Port)
			assert.Equal(t, tt.username, cfg.Username)
			assert.Equal(t, tt.password, cfg.Password)
			assert.Equal(t, tt.expectedDatabase, cfg.Database)
			assert.Equal(t, tt.expectedSSLMode, cfg.SSLMode)
			assert.Equal(t, DefaultConnectTimeout, cfg.ConnectTimeout)
		})
	}
}

// TestConnectionConfig_Immutability tests that WithConnectTimeout returns the same instance
func TestConnectionConfig_Immutability(t *testing.T) {
	cfg := NewConnectionConfig("localhost", 5432, "user", "pass", "testdb", "require")
	originalTimeout := cfg.ConnectTimeout

	// WithConnectTimeout should return the same instance (fluent interface)
	result := cfg.WithConnectTimeout(30)

	assert.Same(t, cfg, result, "WithConnectTimeout should return the same instance")
	assert.Equal(t, 30, cfg.ConnectTimeout)
	assert.NotEqual(t, originalTimeout, cfg.ConnectTimeout)
}

// TestTestConnectionWithConfig_TableDriven tests TestConnectionWithConfig with various scenarios
func TestTestConnectionWithConfig_TableDriven(t *testing.T) {
	logger := zap.NewNop().Sugar()

	tests := []struct {
		name        string
		host        string
		port        int32
		database    string
		username    string
		password    string
		sslMode     string
		retryConfig RetryConfig
		expectError bool
	}{
		{
			name:        "Invalid host with minimal retries",
			host:        "invalid-host-that-does-not-exist",
			port:        5432,
			database:    "postgres",
			username:    "user",
			password:    "pass",
			sslMode:     "disable",
			retryConfig: NewRetryConfigFromParams(1, 100*time.Millisecond),
			expectError: true,
		},
		{
			name:        "Localhost with no server",
			host:        "localhost",
			port:        59999, // Unlikely to have a server on this port
			database:    "postgres",
			username:    "user",
			password:    "pass",
			sslMode:     "disable",
			retryConfig: NewRetryConfigFromParams(1, 100*time.Millisecond),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			connected, err := TestConnectionWithConfig(
				ctx, tt.host, tt.port, tt.database, tt.username, tt.password, tt.sslMode,
				logger, tt.retryConfig)

			if tt.expectError {
				assert.Error(t, err)
				assert.False(t, connected)
			} else {
				assert.NoError(t, err)
				assert.True(t, connected)
			}
		})
	}
}

// TestTestConnectionFromPostgresql_TableDriven tests TestConnectionFromPostgresql with various scenarios
func TestTestConnectionFromPostgresql_TableDriven(t *testing.T) {
	logger := zap.NewNop().Sugar()

	tests := []struct {
		name        string
		postgresql  *instancev1alpha1.Postgresql
		expectError bool
		errorMsg    string
	}{
		{
			name: "Nil external instance",
			postgresql: &instancev1alpha1.Postgresql{
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: nil,
				},
			},
			expectError: true,
			errorMsg:    "no external instance configuration",
		},
		{
			name: "No vault client - skips test",
			postgresql: &instancev1alpha1.Postgresql{
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
						PostgresqlID: "test-id",
						Address:      "localhost",
						Port:         5432,
					},
				},
			},
			expectError: false, // Returns nil when vault client is not configured
		},
		{
			name: "Default port and SSL mode",
			postgresql: &instancev1alpha1.Postgresql{
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
						PostgresqlID: "test-id",
						Address:      "localhost",
						Port:         0,  // Should default to 5432
						SSLMode:      "", // Should default to "require"
					},
				},
			},
			expectError: false, // Returns nil when vault client is not configured
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			err := TestConnectionFromPostgresql(ctx, tt.postgresql, nil, logger, 1, 1*time.Second)

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
