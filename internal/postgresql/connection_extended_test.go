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

// ============================================================================
// TestConnectionWithConfig Extended Tests
// ============================================================================

func TestTestConnectionWithConfig_ExtendedCases(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()

	tests := []struct {
		name        string
		host        string
		port        int32
		database    string
		username    string
		password    string
		sslMode     string
		cfg         RetryConfig
		expectError bool
	}{
		{
			name:     "Invalid host - connection fails quickly",
			host:     "invalid-host-that-does-not-exist",
			port:     5432,
			database: "postgres",
			username: "admin",
			password: "password",
			sslMode:  "disable",
			cfg: RetryConfig{
				InitialInterval:     10 * time.Millisecond,
				MaxInterval:         50 * time.Millisecond,
				MaxElapsedTime:      100 * time.Millisecond,
				Multiplier:          2.0,
				RandomizationFactor: 0.0,
			},
			expectError: true,
		},
		{
			name:     "Empty host - connection fails",
			host:     "",
			port:     5432,
			database: "postgres",
			username: "admin",
			password: "password",
			sslMode:  "disable",
			cfg: RetryConfig{
				InitialInterval:     10 * time.Millisecond,
				MaxInterval:         50 * time.Millisecond,
				MaxElapsedTime:      100 * time.Millisecond,
				Multiplier:          2.0,
				RandomizationFactor: 0.0,
			},
			expectError: true,
		},
		{
			name:     "Default port and database",
			host:     "invalid-host",
			port:     0,
			database: "",
			username: "admin",
			password: "password",
			sslMode:  "",
			cfg: RetryConfig{
				InitialInterval:     10 * time.Millisecond,
				MaxInterval:         50 * time.Millisecond,
				MaxElapsedTime:      100 * time.Millisecond,
				Multiplier:          2.0,
				RandomizationFactor: 0.0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			success, err := TestConnectionWithConfig(ctx, tt.host, tt.port, tt.database, tt.username, tt.password, tt.sslMode, sugar, tt.cfg)

			if tt.expectError {
				assert.Error(t, err)
				assert.False(t, success)
			} else {
				assert.NoError(t, err)
				assert.True(t, success)
			}
		})
	}
}

func TestTestConnectionWithConfig_ContextCancellationExtended(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cfg := RetryConfig{
		InitialInterval:     10 * time.Millisecond,
		MaxInterval:         50 * time.Millisecond,
		MaxElapsedTime:      1 * time.Second,
		Multiplier:          2.0,
		RandomizationFactor: 0.0,
	}

	success, err := TestConnectionWithConfig(ctx, "localhost", 5432, "postgres", "admin", "password", "disable", sugar, cfg)

	assert.Error(t, err)
	assert.False(t, success)
	assert.Contains(t, err.Error(), "cancelled")
}

// ============================================================================
// TestConnectionFromPostgresql Extended Tests
// ============================================================================

func TestTestConnectionFromPostgresql_ExtendedCases(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()

	tests := []struct {
		name        string
		postgresql  *instancev1alpha1.Postgresql
		expectError bool
		errorMsg    string
	}{
		{
			name: "No external instance configuration",
			postgresql: &instancev1alpha1.Postgresql{
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: nil,
				},
			},
			expectError: true,
			errorMsg:    "no external instance configuration",
		},
		{
			name: "No vault client - skip connection test",
			postgresql: &instancev1alpha1.Postgresql{
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
						Address:      "localhost",
						Port:         5432,
						SSLMode:      "disable",
						PostgresqlID: "test-id",
					},
				},
			},
			expectError: false, // Returns nil when vault client is nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			err := TestConnectionFromPostgresql(ctx, tt.postgresql, nil, sugar, 1, 10*time.Millisecond)

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

func TestTestConnectionFromPostgresql_DefaultValuesExtended(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()

	// Test with default port and SSL mode
	postgresql := &instancev1alpha1.Postgresql{
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				Address:      "localhost",
				Port:         0,  // Should default to 5432
				SSLMode:      "", // Should default to "require"
				PostgresqlID: "test-id",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Without vault client, should return nil
	err := TestConnectionFromPostgresql(ctx, postgresql, nil, sugar, 1, 10*time.Millisecond)
	assert.NoError(t, err)
}

// ============================================================================
// TestConnection Extended Tests
// ============================================================================

func TestTestConnection_ExtendedCases(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()

	tests := []struct {
		name         string
		host         string
		port         int32
		database     string
		username     string
		password     string
		sslMode      string
		retries      int
		retryTimeout time.Duration
		expectError  bool
	}{
		{
			name:         "Invalid host with minimal retries",
			host:         "invalid-host",
			port:         5432,
			database:     "postgres",
			username:     "admin",
			password:     "password",
			sslMode:      "disable",
			retries:      1,
			retryTimeout: 10 * time.Millisecond,
			expectError:  true,
		},
		{
			name:         "Zero retries",
			host:         "invalid-host",
			port:         5432,
			database:     "postgres",
			username:     "admin",
			password:     "password",
			sslMode:      "disable",
			retries:      0,
			retryTimeout: 10 * time.Millisecond,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			success, err := TestConnection(ctx, tt.host, tt.port, tt.database, tt.username, tt.password, tt.sslMode, sugar, tt.retries, tt.retryTimeout)

			if tt.expectError {
				assert.Error(t, err)
				assert.False(t, success)
			} else {
				assert.NoError(t, err)
				assert.True(t, success)
			}
		})
	}
}

// ============================================================================
// ConnectionConfig Extended Tests
// ============================================================================

func TestConnectionConfig_ExtendedCases(t *testing.T) {
	tests := []struct {
		name           string
		host           string
		port           int32
		username       string
		password       string
		database       string
		sslMode        string
		connectTimeout int
		expectedPort   int32
		expectedDB     string
		expectedSSL    string
	}{
		{
			name:         "All defaults",
			host:         "localhost",
			port:         0,
			username:     "user",
			password:     "pass",
			database:     "",
			sslMode:      "",
			expectedPort: DefaultPort,
			expectedDB:   DefaultDB,
			expectedSSL:  DefaultSSLMode,
		},
		{
			name:         "Custom values",
			host:         "myhost",
			port:         5433,
			username:     "myuser",
			password:     "mypass",
			database:     "mydb",
			sslMode:      "verify-full",
			expectedPort: 5433,
			expectedDB:   "mydb",
			expectedSSL:  "verify-full",
		},
		{
			name:         "Special characters in password",
			host:         "localhost",
			port:         5432,
			username:     "user",
			password:     "p@ss'word\"with$pecial",
			database:     "testdb",
			sslMode:      "disable",
			expectedPort: 5432,
			expectedDB:   "testdb",
			expectedSSL:  "disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConnectionConfig(tt.host, tt.port, tt.username, tt.password, tt.database, tt.sslMode)

			assert.Equal(t, tt.host, cfg.Host)
			assert.Equal(t, tt.expectedPort, cfg.Port)
			assert.Equal(t, tt.username, cfg.Username)
			assert.Equal(t, tt.password, cfg.Password)
			assert.Equal(t, tt.expectedDB, cfg.Database)
			assert.Equal(t, tt.expectedSSL, cfg.SSLMode)
			assert.Equal(t, DefaultConnectTimeout, cfg.ConnectTimeout)

			// Test WithConnectTimeout
			cfg = cfg.WithConnectTimeout(30)
			assert.Equal(t, 30, cfg.ConnectTimeout)

			// Test BuildConnectionString
			connStr := cfg.BuildConnectionString()
			assert.Contains(t, connStr, "host="+tt.host)
			assert.Contains(t, connStr, "user="+tt.username)
			assert.Contains(t, connStr, "password="+tt.password)
			assert.Contains(t, connStr, "dbname="+tt.expectedDB)
			assert.Contains(t, connStr, "sslmode="+tt.expectedSSL)
			assert.Contains(t, connStr, "connect_timeout=30")
		})
	}
}
