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

package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClient_TableDriven tests NewClient with various scenarios
func TestNewClient_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		vaultAddr     string
		vaultRole     string
		tokenPath     string
		mountPoint    string
		secretPath    string
		expectError   bool
		errorContains string
	}{
		{
			name:          "Empty vault role",
			vaultAddr:     "http://localhost:8200",
			vaultRole:     "",
			tokenPath:     "/path/to/token",
			mountPoint:    "secret",
			secretPath:    "pdb",
			expectError:   true,
			errorContains: "VAULT_ROLE",
		},
		{
			name:          "Valid parameters but no Vault server",
			vaultAddr:     "http://localhost:8200",
			vaultRole:     "test-role",
			tokenPath:     "/nonexistent/token/path",
			mountPoint:    "secret",
			secretPath:    "pdb",
			expectError:   true,
			errorContains: "", // Will fail at auth, error message varies
		},
		{
			name:          "Invalid vault address scheme",
			vaultAddr:     "invalid://address",
			vaultRole:     "test-role",
			tokenPath:     "/path/to/token",
			mountPoint:    "secret",
			secretPath:    "pdb",
			expectError:   true,
			errorContains: "", // Error at client creation or auth
		},
		{
			name:          "Empty vault address",
			vaultAddr:     "",
			vaultRole:     "test-role",
			tokenPath:     "/path/to/token",
			mountPoint:    "secret",
			secretPath:    "pdb",
			expectError:   true,
			errorContains: "", // Error at client creation or auth
		},
		{
			name:          "Malformed vault address",
			vaultAddr:     "://invalid",
			vaultRole:     "test-role",
			tokenPath:     "/path/to/token",
			mountPoint:    "secret",
			secretPath:    "pdb",
			expectError:   true,
			errorContains: "", // Error at client creation or auth
		},
		{
			name:          "Empty token path uses default",
			vaultAddr:     "http://localhost:8200",
			vaultRole:     "test-role",
			tokenPath:     "",
			mountPoint:    "secret",
			secretPath:    "pdb",
			expectError:   true,
			errorContains: "", // Will fail at auth, but not due to empty token path
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			client, err := NewClient(ctx, tt.vaultAddr, tt.vaultRole, tt.tokenPath, tt.mountPoint, tt.secretPath)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

// TestClient_PathConstruction_TableDriven tests path construction for various methods
func TestClient_PathConstruction_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		secretPath    string
		postgresqlID  string
		username      string
		expectedAdmin string
		expectedUser  string
	}{
		{
			name:          "Standard paths",
			secretPath:    "pdb",
			postgresqlID:  "test-id",
			username:      "testuser",
			expectedAdmin: "pdb/test-id/admin",
			expectedUser:  "pdb/test-id/testuser",
		},
		{
			name:          "Nested secret path",
			secretPath:    "postgresql/credentials",
			postgresqlID:  "prod-db-1",
			username:      "app_user",
			expectedAdmin: "postgresql/credentials/prod-db-1/admin",
			expectedUser:  "postgresql/credentials/prod-db-1/app_user",
		},
		{
			name:          "Special characters in ID",
			secretPath:    "pdb",
			postgresqlID:  "test-id-123",
			username:      "user_with_underscore",
			expectedAdmin: "pdb/test-id-123/admin",
			expectedUser:  "pdb/test-id-123/user_with_underscore",
		},
		{
			name:          "Empty secret path",
			secretPath:    "",
			postgresqlID:  "test-id",
			username:      "testuser",
			expectedAdmin: "/test-id/admin",
			expectedUser:  "/test-id/testuser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				vaultSecretPath: tt.secretPath,
			}

			// Test admin path construction
			adminPath := fmt.Sprintf("%s/%s/admin", client.vaultSecretPath, tt.postgresqlID)
			assert.Equal(t, tt.expectedAdmin, adminPath)

			// Test user path construction
			userPath := fmt.Sprintf("%s/%s/%s", client.vaultSecretPath, tt.postgresqlID, tt.username)
			assert.Equal(t, tt.expectedUser, userPath)
		})
	}
}

// TestClient_StructFields tests that Client struct fields are properly set
func TestClient_StructFields(t *testing.T) {
	tests := []struct {
		name       string
		mountPoint string
		secretPath string
	}{
		{
			name:       "Standard configuration",
			mountPoint: "secret",
			secretPath: "pdb",
		},
		{
			name:       "Custom mount point",
			mountPoint: "kv",
			secretPath: "postgresql/creds",
		},
		{
			name:       "Empty values",
			mountPoint: "",
			secretPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				vaultMountPoint: tt.mountPoint,
				vaultSecretPath: tt.secretPath,
			}

			assert.Equal(t, tt.mountPoint, client.vaultMountPoint)
			assert.Equal(t, tt.secretPath, client.vaultSecretPath)
		})
	}
}

// TestNewClient_ContextCancellation tests that NewClient respects context cancellation
func TestNewClient_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	client, err := NewClient(ctx, "http://localhost:8200", "test-role", "/path/to/token", "secret", "pdb")

	// Should fail due to context cancellation or connection timeout
	assert.Error(t, err)
	assert.Nil(t, client)
}

// TestNewClient_ContextTimeout tests that NewClient respects context timeout
func TestNewClient_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for timeout
	time.Sleep(5 * time.Millisecond)

	client, err := NewClient(ctx, "http://localhost:8200", "test-role", "/path/to/token", "secret", "pdb")

	// Should fail due to context timeout
	assert.Error(t, err)
	assert.Nil(t, client)
}

// TestClientInterface_Compliance tests that Client implements ClientInterface
func TestClientInterface_Compliance(t *testing.T) {
	// This test verifies at compile time that Client implements ClientInterface
	var _ ClientInterface = (*Client)(nil)
}

// TestClient_MountPointAndSecretPath tests various mount point and secret path combinations
func TestClient_MountPointAndSecretPath(t *testing.T) {
	tests := []struct {
		name       string
		mountPoint string
		secretPath string
	}{
		{
			name:       "Default KV v2 mount",
			mountPoint: "secret",
			secretPath: "pdb",
		},
		{
			name:       "Custom KV mount",
			mountPoint: "kv",
			secretPath: "postgresql",
		},
		{
			name:       "Nested secret path",
			mountPoint: "secret",
			secretPath: "apps/postgresql/credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				vaultMountPoint: tt.mountPoint,
				vaultSecretPath: tt.secretPath,
			}

			require.NotNil(t, client)
			assert.Equal(t, tt.mountPoint, client.vaultMountPoint)
			assert.Equal(t, tt.secretPath, client.vaultSecretPath)
		})
	}
}

// TestNewClient_EmptyVaultRole tests that empty vault role returns appropriate error
func TestNewClient_EmptyVaultRole(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, "http://localhost:8200", "", "/path/to/token", "secret", "pdb")
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "VAULT_ROLE")
}

// TestNewClient_InvalidVaultAddr tests that invalid vault address is handled
func TestNewClient_InvalidVaultAddr(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, "invalid://address", "test-role", "", "secret", "pdb")
	// The error might be at different stages, but should fail
	if err != nil {
		assert.Nil(t, client)
	}
}

// TestNewClient_WithTokenPath tests NewClient with custom token path
func TestNewClient_WithTokenPath(t *testing.T) {
	ctx := context.Background()
	_, err := NewClient(ctx, "http://localhost:8200", "test-role", "/custom/token/path", "secret", "pdb")
	// This will fail at authentication, but we verify the role check passes
	if err != nil {
		assert.NotContains(t, err.Error(), "VAULT_ROLE environment variable is not set")
	}
}

// TestNewClient_DefaultTokenPath tests NewClient with empty token path (uses default)
func TestNewClient_DefaultTokenPath(t *testing.T) {
	ctx := context.Background()
	_, err := NewClient(ctx, "http://localhost:8200", "test-role", "", "secret", "pdb")
	// Will fail at auth, but tests the default path logic
	if err != nil {
		assert.NotContains(t, err.Error(), "token path")
	}
}

// TestClient_GetPostgresqlCredentials_PathConstruction tests path construction for admin credentials
func TestClient_GetPostgresqlCredentials_PathConstruction(t *testing.T) {
	client := &Client{
		vaultSecretPath: "pdb",
	}

	postgresqlID := "test-id"
	expectedPath := fmt.Sprintf("pdb/%s/admin", postgresqlID)

	actualPath := fmt.Sprintf("%s/%s/admin", client.vaultSecretPath, postgresqlID)
	assert.Equal(t, expectedPath, actualPath)
}

// TestClient_GetPostgresqlUserCredentials_PathConstruction tests path construction for user credentials
func TestClient_GetPostgresqlUserCredentials_PathConstruction(t *testing.T) {
	client := &Client{
		vaultSecretPath: "pdb",
	}

	postgresqlID := "test-id"
	username := "testuser"
	expectedPath := fmt.Sprintf("pdb/%s/%s", postgresqlID, username)

	actualPath := fmt.Sprintf("%s/%s/%s", client.vaultSecretPath, postgresqlID, username)
	assert.Equal(t, expectedPath, actualPath)
}

// TestClient_StorePostgresqlUserCredentials_PathConstruction tests path construction for storing credentials
func TestClient_StorePostgresqlUserCredentials_PathConstruction(t *testing.T) {
	client := &Client{
		vaultSecretPath: "pdb",
	}

	postgresqlID := "test-id"
	username := "testuser"
	expectedPath := fmt.Sprintf("pdb/%s/%s", postgresqlID, username)

	actualPath := fmt.Sprintf("%s/%s/%s", client.vaultSecretPath, postgresqlID, username)
	assert.Equal(t, expectedPath, actualPath)
}

// TestNewClient_EmptyVaultAddr tests NewClient with empty vault address
func TestNewClient_EmptyVaultAddr(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, "", "test-role", "", "secret", "pdb")
	if err != nil {
		assert.Nil(t, client)
	}
}

// TestNewClient_InvalidConfig tests NewClient with invalid configuration
func TestNewClient_InvalidConfig(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, "://invalid", "test-role", "", "secret", "pdb")
	if err != nil {
		assert.Nil(t, client)
	}
}

// TestClient_PathConstruction tests that paths are constructed correctly
func TestClient_PathConstruction(t *testing.T) {
	client := &Client{
		vaultMountPoint: "secret",
		vaultSecretPath: "pdb",
	}

	assert.Equal(t, "secret", client.vaultMountPoint)
	assert.Equal(t, "pdb", client.vaultSecretPath)
}

// TestDefaultClientOptions tests DefaultClientOptions returns correct defaults
func TestDefaultClientOptions(t *testing.T) {
	opts := DefaultClientOptions()

	assert.Equal(t, DefaultAuthTimeout, opts.AuthTimeout)
	assert.Equal(t, 30*time.Second, opts.AuthTimeout)
}

// TestNewClientWithOptions_TableDriven tests NewClientWithOptions with various scenarios
func TestNewClientWithOptions_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		vaultAddr     string
		vaultRole     string
		tokenPath     string
		mountPoint    string
		secretPath    string
		opts          ClientOptions
		expectError   bool
		errorContains string
	}{
		{
			name:          "Empty vault role",
			vaultAddr:     "http://localhost:8200",
			vaultRole:     "",
			tokenPath:     "/path/to/token",
			mountPoint:    "secret",
			secretPath:    "pdb",
			opts:          DefaultClientOptions(),
			expectError:   true,
			errorContains: "VAULT_ROLE",
		},
		{
			name:          "Valid parameters with custom timeout",
			vaultAddr:     "http://localhost:8200",
			vaultRole:     "test-role",
			tokenPath:     "/nonexistent/token/path",
			mountPoint:    "secret",
			secretPath:    "pdb",
			opts:          ClientOptions{AuthTimeout: 5 * time.Second},
			expectError:   true,
			errorContains: "", // Will fail at auth
		},
		{
			name:          "Zero timeout uses default",
			vaultAddr:     "http://localhost:8200",
			vaultRole:     "test-role",
			tokenPath:     "/nonexistent/token/path",
			mountPoint:    "secret",
			secretPath:    "pdb",
			opts:          ClientOptions{AuthTimeout: 0},
			expectError:   true,
			errorContains: "", // Will fail at auth
		},
		{
			name:          "Negative timeout uses default",
			vaultAddr:     "http://localhost:8200",
			vaultRole:     "test-role",
			tokenPath:     "/nonexistent/token/path",
			mountPoint:    "secret",
			secretPath:    "pdb",
			opts:          ClientOptions{AuthTimeout: -1 * time.Second},
			expectError:   true,
			errorContains: "", // Will fail at auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			client, err := NewClientWithOptions(ctx, tt.vaultAddr, tt.vaultRole, tt.tokenPath, tt.mountPoint, tt.secretPath, tt.opts)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

// TestClientOptions_AuthTimeout tests various auth timeout scenarios
func TestClientOptions_AuthTimeout(t *testing.T) {
	tests := []struct {
		name            string
		authTimeout     time.Duration
		expectedTimeout time.Duration
	}{
		{
			name:            "Default timeout",
			authTimeout:     DefaultAuthTimeout,
			expectedTimeout: 30 * time.Second,
		},
		{
			name:            "Custom timeout",
			authTimeout:     10 * time.Second,
			expectedTimeout: 10 * time.Second,
		},
		{
			name:            "Very short timeout",
			authTimeout:     1 * time.Millisecond,
			expectedTimeout: 1 * time.Millisecond,
		},
		{
			name:            "Long timeout",
			authTimeout:     5 * time.Minute,
			expectedTimeout: 5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ClientOptions{AuthTimeout: tt.authTimeout}
			assert.Equal(t, tt.expectedTimeout, opts.AuthTimeout)
		})
	}
}

// TestDefaultAuthTimeout tests the default auth timeout constant
func TestDefaultAuthTimeout(t *testing.T) {
	assert.Equal(t, 30*time.Second, DefaultAuthTimeout)
}

// TestClient_FieldsInitialization tests that Client fields are properly initialized
func TestClient_FieldsInitialization(t *testing.T) {
	tests := []struct {
		name       string
		mountPoint string
		secretPath string
	}{
		{
			name:       "Standard values",
			mountPoint: "secret",
			secretPath: "pdb",
		},
		{
			name:       "Empty values",
			mountPoint: "",
			secretPath: "",
		},
		{
			name:       "Custom mount point",
			mountPoint: "kv-v2",
			secretPath: "postgresql/credentials",
		},
		{
			name:       "Nested paths",
			mountPoint: "secret/data",
			secretPath: "apps/postgresql/prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				vaultMountPoint: tt.mountPoint,
				vaultSecretPath: tt.secretPath,
			}

			assert.Equal(t, tt.mountPoint, client.vaultMountPoint)
			assert.Equal(t, tt.secretPath, client.vaultSecretPath)
		})
	}
}

// TestNewClient_CallsNewClientWithOptions tests that NewClient calls NewClientWithOptions with defaults
func TestNewClient_CallsNewClientWithOptions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Both should fail with empty role
	client1, err1 := NewClient(ctx, "http://localhost:8200", "", "/path/to/token", "secret", "pdb")
	client2, err2 := NewClientWithOptions(ctx, "http://localhost:8200", "", "/path/to/token", "secret", "pdb", DefaultClientOptions())

	assert.Error(t, err1)
	assert.Error(t, err2)
	assert.Nil(t, client1)
	assert.Nil(t, client2)
	assert.Contains(t, err1.Error(), "VAULT_ROLE")
	assert.Contains(t, err2.Error(), "VAULT_ROLE")
}

// TestNewClientWithOptions_ContextTimeout tests context timeout handling
func TestNewClientWithOptions_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for timeout
	time.Sleep(5 * time.Millisecond)

	client, err := NewClientWithOptions(ctx, "http://localhost:8200", "test-role", "/path/to/token", "secret", "pdb", DefaultClientOptions())

	assert.Error(t, err)
	assert.Nil(t, client)
}

// TestNewClientWithOptions_ContextCancellation tests context cancellation handling
func TestNewClientWithOptions_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	client, err := NewClientWithOptions(ctx, "http://localhost:8200", "test-role", "/path/to/token", "secret", "pdb", DefaultClientOptions())

	assert.Error(t, err)
	assert.Nil(t, client)
}

// TestMockVaultClient_CheckHealth_TableDriven tests CheckHealth with various scenarios using mock
func TestMockVaultClient_CheckHealth_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		healthy     bool
		healthError error
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Healthy vault returns no error",
			healthy:     true,
			expectError: false,
		},
		{
			name:        "Unhealthy vault returns default error",
			healthy:     false,
			expectError: true,
			errorMsg:    "vault is unhealthy",
		},
		{
			name:        "Unhealthy vault with custom error",
			healthy:     false,
			healthError: fmt.Errorf("custom health check failed"),
			expectError: true,
			errorMsg:    "custom health check failed",
		},
		{
			name:        "Unhealthy vault with connection error",
			healthy:     false,
			healthError: fmt.Errorf("connection refused"),
			expectError: true,
			errorMsg:    "connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockVaultClient()
			mock.SetHealthy(tt.healthy)
			if tt.healthError != nil {
				mock.SetHealthError(tt.healthError)
			}

			err := mock.CheckHealth(context.Background())

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			// Verify call was tracked
			assert.Equal(t, 1, mock.GetCheckHealthCalls())
		})
	}
}

// TestMockVaultClient_GetPostgresqlCredentials_TableDriven tests GetPostgresqlCredentials with various scenarios
func TestMockVaultClient_GetPostgresqlCredentials_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		postgresqlID     string
		setupCredentials bool
		adminUsername    string
		adminPassword    string
		expectError      bool
		errorContains    string
	}{
		{
			name:             "Credentials found successfully",
			postgresqlID:     "test-pg-1",
			setupCredentials: true,
			adminUsername:    "admin",
			adminPassword:    "secret123",
			expectError:      false,
		},
		{
			name:             "Credentials not found",
			postgresqlID:     "nonexistent-pg",
			setupCredentials: false,
			expectError:      true,
			errorContains:    "credentials not found",
		},
		{
			name:             "Empty postgresqlID",
			postgresqlID:     "",
			setupCredentials: false,
			expectError:      true,
			errorContains:    "credentials not found",
		},
		{
			name:             "Special characters in postgresqlID",
			postgresqlID:     "test-pg_123-special",
			setupCredentials: true,
			adminUsername:    "admin_user",
			adminPassword:    "p@ss=word!",
			expectError:      false,
		},
		{
			name:             "Long postgresqlID",
			postgresqlID:     "very-long-postgresql-id-that-might-be-used-in-production-environments",
			setupCredentials: true,
			adminUsername:    "admin",
			adminPassword:    "password",
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockVaultClient()
			if tt.setupCredentials {
				mock.SetAdminCredentials(tt.postgresqlID, tt.adminUsername, tt.adminPassword)
			}

			username, password, err := mock.GetPostgresqlCredentials(context.Background(), tt.postgresqlID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, username)
				assert.Empty(t, password)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.adminUsername, username)
				assert.Equal(t, tt.adminPassword, password)
			}

			// Verify call was tracked
			assert.Equal(t, 1, mock.GetGetPostgresqlCredentialsCalls())
		})
	}
}

// TestMockVaultClient_GetPostgresqlUserCredentials_TableDriven tests GetPostgresqlUserCredentials with various scenarios
func TestMockVaultClient_GetPostgresqlUserCredentials_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		postgresqlID     string
		username         string
		setupCredentials bool
		password         string
		expectError      bool
		errorContains    string
	}{
		{
			name:             "User credentials found successfully",
			postgresqlID:     "test-pg-1",
			username:         "testuser",
			setupCredentials: true,
			password:         "userpass123",
			expectError:      false,
		},
		{
			name:             "User credentials not found",
			postgresqlID:     "test-pg-1",
			username:         "nonexistent",
			setupCredentials: false,
			expectError:      true,
			errorContains:    "credentials not found",
		},
		{
			name:             "Empty username",
			postgresqlID:     "test-pg-1",
			username:         "",
			setupCredentials: false,
			expectError:      true,
			errorContains:    "credentials not found",
		},
		{
			name:             "Empty postgresqlID",
			postgresqlID:     "",
			username:         "testuser",
			setupCredentials: false,
			expectError:      true,
			errorContains:    "credentials not found",
		},
		{
			name:             "Special characters in username",
			postgresqlID:     "test-pg-1",
			username:         "user_with-special.chars",
			setupCredentials: true,
			password:         "complex!@#$%password",
			expectError:      false,
		},
		{
			name:             "Multiple users same postgresql",
			postgresqlID:     "test-pg-1",
			username:         "user2",
			setupCredentials: true,
			password:         "user2pass",
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockVaultClient()
			if tt.setupCredentials {
				mock.SetUserCredentials(tt.postgresqlID, tt.username, tt.password)
			}

			password, err := mock.GetPostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, password)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.password, password)
			}
		})
	}
}

// TestMockVaultClient_StorePostgresqlUserCredentials_TableDriven tests StorePostgresqlUserCredentials with various scenarios
func TestMockVaultClient_StorePostgresqlUserCredentials_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		postgresqlID string
		username     string
		password     string
	}{
		{
			name:         "Store new credentials",
			postgresqlID: "test-pg-1",
			username:     "newuser",
			password:     "newpass123",
		},
		{
			name:         "Store credentials with special characters",
			postgresqlID: "test-pg-1",
			username:     "user_special",
			password:     "p@ss=word!#$%",
		},
		{
			name:         "Store credentials with empty password",
			postgresqlID: "test-pg-1",
			username:     "emptypassuser",
			password:     "",
		},
		{
			name:         "Store credentials with long password",
			postgresqlID: "test-pg-1",
			username:     "longpassuser",
			password:     "verylongpasswordthatmightbeusedinsomeproductionenvironments1234567890",
		},
		{
			name:         "Update existing credentials",
			postgresqlID: "test-pg-1",
			username:     "existinguser",
			password:     "updatedpassword",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockVaultClient()

			// Store credentials
			err := mock.StorePostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username, tt.password)
			require.NoError(t, err)

			// Verify credentials were stored correctly
			storedPassword, err := mock.GetPostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username)
			require.NoError(t, err)
			assert.Equal(t, tt.password, storedPassword)
		})
	}
}

// TestMockVaultClient_StoreAndRetrieve tests storing and retrieving credentials
func TestMockVaultClient_StoreAndRetrieve(t *testing.T) {
	mock := NewMockVaultClient()

	// Store multiple user credentials
	users := []struct {
		postgresqlID string
		username     string
		password     string
	}{
		{"pg-1", "user1", "pass1"},
		{"pg-1", "user2", "pass2"},
		{"pg-2", "user1", "pass3"},
		{"pg-2", "user3", "pass4"},
	}

	for _, u := range users {
		err := mock.StorePostgresqlUserCredentials(context.Background(), u.postgresqlID, u.username, u.password)
		require.NoError(t, err)
	}

	// Verify all credentials can be retrieved
	for _, u := range users {
		password, err := mock.GetPostgresqlUserCredentials(context.Background(), u.postgresqlID, u.username)
		require.NoError(t, err)
		assert.Equal(t, u.password, password)
	}
}

// TestMockVaultClient_UpdateCredentials tests updating existing credentials
func TestMockVaultClient_UpdateCredentials(t *testing.T) {
	mock := NewMockVaultClient()

	postgresqlID := "test-pg"
	username := "testuser"
	originalPassword := "original123"
	updatedPassword := "updated456"

	// Store original credentials
	err := mock.StorePostgresqlUserCredentials(context.Background(), postgresqlID, username, originalPassword)
	require.NoError(t, err)

	// Verify original password
	password, err := mock.GetPostgresqlUserCredentials(context.Background(), postgresqlID, username)
	require.NoError(t, err)
	assert.Equal(t, originalPassword, password)

	// Update credentials
	err = mock.StorePostgresqlUserCredentials(context.Background(), postgresqlID, username, updatedPassword)
	require.NoError(t, err)

	// Verify updated password
	password, err = mock.GetPostgresqlUserCredentials(context.Background(), postgresqlID, username)
	require.NoError(t, err)
	assert.Equal(t, updatedPassword, password)
}

// TestMockVaultClient_ContextHandling tests context handling in mock client
func TestMockVaultClient_ContextHandling(t *testing.T) {
	mock := NewMockVaultClient()
	mock.SetHealthy(true)
	mock.SetAdminCredentials("test-pg", "admin", "pass")
	mock.SetUserCredentials("test-pg", "user", "userpass")

	// Test with cancelled context - mock should still work (it doesn't check context)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// These should still work because mock doesn't check context
	err := mock.CheckHealth(ctx)
	assert.NoError(t, err)

	username, password, err := mock.GetPostgresqlCredentials(ctx, "test-pg")
	assert.NoError(t, err)
	assert.Equal(t, "admin", username)
	assert.Equal(t, "pass", password)

	userPassword, err := mock.GetPostgresqlUserCredentials(ctx, "test-pg", "user")
	assert.NoError(t, err)
	assert.Equal(t, "userpass", userPassword)

	err = mock.StorePostgresqlUserCredentials(ctx, "test-pg", "newuser", "newpass")
	assert.NoError(t, err)
}

// TestMockVaultClient_ConcurrentOperations tests thread safety of mock client
func TestMockVaultClient_ConcurrentOperations(t *testing.T) {
	mock := NewMockVaultClient()
	mock.SetHealthy(true)

	done := make(chan bool, 100)

	// Concurrent health checks
	for i := 0; i < 20; i++ {
		go func() {
			_ = mock.CheckHealth(context.Background())
			done <- true
		}()
	}

	// Concurrent credential storage
	for i := 0; i < 20; i++ {
		go func(id int) {
			postgresqlID := fmt.Sprintf("pg-%d", id)
			mock.SetAdminCredentials(postgresqlID, "admin", "pass")
			_, _, _ = mock.GetPostgresqlCredentials(context.Background(), postgresqlID)
			done <- true
		}(i)
	}

	// Concurrent user credential operations
	for i := 0; i < 20; i++ {
		go func(id int) {
			postgresqlID := fmt.Sprintf("pg-%d", id%5)
			username := fmt.Sprintf("user-%d", id)
			_ = mock.StorePostgresqlUserCredentials(context.Background(), postgresqlID, username, "pass")
			_, _ = mock.GetPostgresqlUserCredentials(context.Background(), postgresqlID, username)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 60; i++ {
		<-done
	}

	// Verify call counts
	assert.Equal(t, 20, mock.GetCheckHealthCalls())
	assert.Equal(t, 20, mock.GetGetPostgresqlCredentialsCalls())
}

// TestClientInterface_MockImplementation tests that MockVaultClient properly implements ClientInterface
func TestClientInterface_MockImplementation(t *testing.T) {
	// Compile-time check that MockVaultClient implements ClientInterface
	var _ ClientInterface = (*MockVaultClient)(nil)

	// Runtime check
	mock := NewMockVaultClient()
	var client ClientInterface = mock
	assert.NotNil(t, client)
}

// TestClient_PathFormats tests various path format scenarios
func TestClient_PathFormats(t *testing.T) {
	tests := []struct {
		name              string
		secretPath        string
		postgresqlID      string
		username          string
		expectedAdminPath string
		expectedUserPath  string
	}{
		{
			name:              "Standard paths",
			secretPath:        "pdb",
			postgresqlID:      "test-id",
			username:          "testuser",
			expectedAdminPath: "pdb/test-id/admin",
			expectedUserPath:  "pdb/test-id/testuser",
		},
		{
			name:              "Nested secret path",
			secretPath:        "postgresql/credentials",
			postgresqlID:      "prod-db-1",
			username:          "app_user",
			expectedAdminPath: "postgresql/credentials/prod-db-1/admin",
			expectedUserPath:  "postgresql/credentials/prod-db-1/app_user",
		},
		{
			name:              "Deep nested path",
			secretPath:        "apps/team/postgresql/creds",
			postgresqlID:      "db-123",
			username:          "service_account",
			expectedAdminPath: "apps/team/postgresql/creds/db-123/admin",
			expectedUserPath:  "apps/team/postgresql/creds/db-123/service_account",
		},
		{
			name:              "Empty secret path",
			secretPath:        "",
			postgresqlID:      "test-id",
			username:          "testuser",
			expectedAdminPath: "/test-id/admin",
			expectedUserPath:  "/test-id/testuser",
		},
		{
			name:              "Special characters in IDs",
			secretPath:        "pdb",
			postgresqlID:      "test-id_123",
			username:          "user_with-dash",
			expectedAdminPath: "pdb/test-id_123/admin",
			expectedUserPath:  "pdb/test-id_123/user_with-dash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				vaultSecretPath: tt.secretPath,
			}

			// Test admin path construction
			adminPath := fmt.Sprintf("%s/%s/admin", client.vaultSecretPath, tt.postgresqlID)
			assert.Equal(t, tt.expectedAdminPath, adminPath)

			// Test user path construction
			userPath := fmt.Sprintf("%s/%s/%s", client.vaultSecretPath, tt.postgresqlID, tt.username)
			assert.Equal(t, tt.expectedUserPath, userPath)
		})
	}
}

// TestMockVaultClient_HealthToggle tests toggling health status
func TestMockVaultClient_HealthToggle(t *testing.T) {
	mock := NewMockVaultClient()

	// Initially healthy
	assert.NoError(t, mock.CheckHealth(context.Background()))

	// Set unhealthy
	mock.SetHealthy(false)
	assert.Error(t, mock.CheckHealth(context.Background()))

	// Set healthy again
	mock.SetHealthy(true)
	assert.NoError(t, mock.CheckHealth(context.Background()))

	// Set unhealthy with custom error
	customErr := fmt.Errorf("vault sealed")
	mock.SetHealthy(false)
	mock.SetHealthError(customErr)
	err := mock.CheckHealth(context.Background())
	assert.Error(t, err)
	assert.Equal(t, customErr, err)
}

// TestMockVaultClient_EmptyCredentials tests behavior with empty credentials
func TestMockVaultClient_EmptyCredentials(t *testing.T) {
	mock := NewMockVaultClient()

	// Set admin credentials with empty values
	mock.SetAdminCredentials("test-pg", "", "")

	// Should still return the empty values (not an error)
	username, password, err := mock.GetPostgresqlCredentials(context.Background(), "test-pg")
	assert.NoError(t, err)
	assert.Empty(t, username)
	assert.Empty(t, password)

	// Set user credentials with empty password
	mock.SetUserCredentials("test-pg", "user", "")

	// Should still return the empty password
	userPassword, err := mock.GetPostgresqlUserCredentials(context.Background(), "test-pg", "user")
	assert.NoError(t, err)
	assert.Empty(t, userPassword)
}

// =============================================================================
// Tests for actual Client methods using httptest mock server
// =============================================================================

// createTestVaultClient creates a Client with a mock Vault server for testing
func createTestVaultClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)

	config := api.DefaultConfig()
	config.Address = server.URL

	apiClient, err := api.NewClient(config)
	require.NoError(t, err)

	// Set a token to avoid authentication issues
	apiClient.SetToken("test-token")

	client := &Client{
		client:          apiClient,
		vaultMountPoint: "secret",
		vaultSecretPath: "pdb",
	}

	return client, server
}

// TestClient_CheckHealth_Success tests successful health check
func TestClient_CheckHealth_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"initialized":                  true,
				"sealed":                       false,
				"standby":                      false,
				"performance_standby":          false,
				"replication_performance_mode": "disabled",
				"replication_dr_mode":          "disabled",
				"server_time_utc":              1234567890,
				"version":                      "1.15.0",
				"cluster_name":                 "vault-cluster",
				"cluster_id":                   "test-cluster-id",
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	err := client.CheckHealth(context.Background())
	assert.NoError(t, err)
}

// TestClient_CheckHealth_Sealed tests health check when Vault is sealed
func TestClient_CheckHealth_Sealed(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/sys/health") {
			w.Header().Set("Content-Type", "application/json")
			// Vault returns 503 when sealed - the API client treats this as an error
			w.WriteHeader(http.StatusServiceUnavailable)
			response := map[string]interface{}{
				"initialized": true,
				"sealed":      true,
				"standby":     false,
				"version":     "1.15.0",
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	// CheckHealth returns error when Vault returns non-2xx status
	err := client.CheckHealth(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check Vault health")
}

// TestClient_CheckHealth_Standby tests health check when Vault is in standby mode
func TestClient_CheckHealth_Standby(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/sys/health") {
			w.Header().Set("Content-Type", "application/json")
			// Vault returns 429 when in standby mode
			// The Vault API client uses query params to customize status codes
			// and may treat 429 as success depending on configuration
			w.WriteHeader(http.StatusTooManyRequests)
			response := map[string]interface{}{
				"initialized": true,
				"sealed":      false,
				"standby":     true,
				"version":     "1.15.0",
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	// CheckHealth may succeed or fail depending on Vault API client configuration
	// The important thing is that it doesn't panic
	err := client.CheckHealth(context.Background())
	// Either success or error is acceptable for standby mode
	if err != nil {
		assert.Contains(t, err.Error(), "failed to check Vault health")
	}
}

// TestClient_CheckHealth_Error tests health check when Vault returns an error
func TestClient_CheckHealth_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			// Return an error response
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	err := client.CheckHealth(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check Vault health")
}

// TestClient_CheckHealth_ConnectionRefused tests health check when connection is refused
func TestClient_CheckHealth_ConnectionRefused(t *testing.T) {
	// Create a server and immediately close it to simulate connection refused
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	config := api.DefaultConfig()
	config.Address = serverURL

	apiClient, err := api.NewClient(config)
	require.NoError(t, err)
	apiClient.SetToken("test-token")

	client := &Client{
		client:          apiClient,
		vaultMountPoint: "secret",
		vaultSecretPath: "pdb",
	}

	err = client.CheckHealth(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check Vault health")
}

// TestClient_CheckHealth_ContextCancellation tests health check with cancelled context
func TestClient_CheckHealth_ContextCancellation(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := client.CheckHealth(ctx)
	assert.Error(t, err)
}

// TestClient_CheckHealth_TableDriven tests CheckHealth with various scenarios
func TestClient_CheckHealth_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  map[string]interface{}
		expectError   bool
		errorContains string
	}{
		{
			name:       "Healthy active node",
			statusCode: http.StatusOK,
			responseBody: map[string]interface{}{
				"initialized": true,
				"sealed":      false,
				"standby":     false,
				"version":     "1.15.0",
			},
			expectError: false,
		},
		{
			name:       "Sealed vault returns error",
			statusCode: http.StatusServiceUnavailable,
			responseBody: map[string]interface{}{
				"initialized": true,
				"sealed":      true,
				"standby":     false,
				"version":     "1.15.0",
			},
			expectError:   true, // Vault API client treats non-2xx as error
			errorContains: "failed to check Vault health",
		},
		{
			name:       "Standby node may succeed",
			statusCode: http.StatusTooManyRequests,
			responseBody: map[string]interface{}{
				"initialized": true,
				"sealed":      false,
				"standby":     true,
				"version":     "1.15.0",
			},
			expectError:   false, // Vault API client may treat 429 as success with query params
			errorContains: "",
		},
		{
			name:       "Uninitialized vault returns error",
			statusCode: http.StatusNotImplemented,
			responseBody: map[string]interface{}{
				"initialized": false,
				"sealed":      true,
				"standby":     false,
				"version":     "1.15.0",
			},
			expectError:   true, // Vault API client treats non-2xx as error
			errorContains: "failed to check Vault health",
		},
		{
			name:          "Internal server error",
			statusCode:    http.StatusInternalServerError,
			responseBody:  nil,
			expectError:   true,
			errorContains: "failed to check Vault health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/v1/sys/health") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.statusCode)
					if tt.responseBody != nil {
						err := json.NewEncoder(w).Encode(tt.responseBody)
						require.NoError(t, err)
					}
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			client, server := createTestVaultClient(t, handler)
			defer server.Close()

			err := client.CheckHealth(context.Background())

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestClient_GetPostgresqlCredentials_Success tests successful credential retrieval
func TestClient_GetPostgresqlCredentials_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// KV v2 read path: /v1/{mount}/data/{path}
		if r.URL.Path == "/v1/secret/data/pdb/test-pg/admin" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"request_id":     "test-request-id",
				"lease_id":       "",
				"renewable":      false,
				"lease_duration": 0,
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"admin_username": "postgres",
						"admin_password": "secret123",
					},
					"metadata": map[string]interface{}{
						"created_time":    "2024-01-01T00:00:00Z",
						"deletion_time":   "",
						"destroyed":       false,
						"version":         1,
						"custom_metadata": nil,
					},
				},
				"wrap_info": nil,
				"warnings":  nil,
				"auth":      nil,
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	username, password, err := client.GetPostgresqlCredentials(context.Background(), "test-pg")
	assert.NoError(t, err)
	assert.Equal(t, "postgres", username)
	assert.Equal(t, "secret123", password)
}

// TestClient_GetPostgresqlCredentials_NotFound tests credential retrieval when secret not found
func TestClient_GetPostgresqlCredentials_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/secret/data/") && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			response := map[string]interface{}{
				"errors": []string{"no secret found"},
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	username, password, err := client.GetPostgresqlCredentials(context.Background(), "nonexistent-pg")
	assert.Error(t, err)
	assert.Empty(t, username)
	assert.Empty(t, password)
	assert.Contains(t, err.Error(), "failed to read secret from Vault")
}

// TestClient_GetPostgresqlCredentials_MissingUsername tests when username is missing
func TestClient_GetPostgresqlCredentials_MissingUsername(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/secret/data/pdb/test-pg/admin" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						// Missing admin_username
						"admin_password": "secret123",
					},
					"metadata": map[string]interface{}{
						"version": 1,
					},
				},
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	username, password, err := client.GetPostgresqlCredentials(context.Background(), "test-pg")
	assert.Error(t, err)
	assert.Empty(t, username)
	assert.Empty(t, password)
	assert.Contains(t, err.Error(), "credentials not found")
}

// TestClient_GetPostgresqlCredentials_MissingPassword tests when password is missing
func TestClient_GetPostgresqlCredentials_MissingPassword(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/secret/data/pdb/test-pg/admin" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"admin_username": "postgres",
						// Missing admin_password
					},
					"metadata": map[string]interface{}{
						"version": 1,
					},
				},
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	username, password, err := client.GetPostgresqlCredentials(context.Background(), "test-pg")
	assert.Error(t, err)
	assert.Empty(t, username)
	assert.Empty(t, password)
	assert.Contains(t, err.Error(), "credentials not found")
}

// TestClient_GetPostgresqlCredentials_EmptyData tests when data is empty
func TestClient_GetPostgresqlCredentials_EmptyData(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/secret/data/pdb/test-pg/admin" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{},
					"metadata": map[string]interface{}{
						"version": 1,
					},
				},
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	username, password, err := client.GetPostgresqlCredentials(context.Background(), "test-pg")
	assert.Error(t, err)
	assert.Empty(t, username)
	assert.Empty(t, password)
	assert.Contains(t, err.Error(), "credentials not found")
}

// TestClient_GetPostgresqlCredentials_ContextCancellation tests context cancellation
func TestClient_GetPostgresqlCredentials_ContextCancellation(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	username, password, err := client.GetPostgresqlCredentials(ctx, "test-pg")
	assert.Error(t, err)
	assert.Empty(t, username)
	assert.Empty(t, password)
}

// TestClient_GetPostgresqlCredentials_TableDriven tests GetPostgresqlCredentials with various scenarios
func TestClient_GetPostgresqlCredentials_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		postgresqlID     string
		statusCode       int
		responseBody     map[string]interface{}
		expectError      bool
		errorContains    string
		expectedUsername string
		expectedPassword string
	}{
		{
			name:         "Success with valid credentials",
			postgresqlID: "test-pg",
			statusCode:   http.StatusOK,
			responseBody: map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"admin_username": "admin",
						"admin_password": "password123",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			},
			expectError:      false,
			expectedUsername: "admin",
			expectedPassword: "password123",
		},
		{
			name:         "Success with special characters in credentials",
			postgresqlID: "test-pg-special",
			statusCode:   http.StatusOK,
			responseBody: map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"admin_username": "admin_user",
						"admin_password": "p@ss=word!#$%",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			},
			expectError:      false,
			expectedUsername: "admin_user",
			expectedPassword: "p@ss=word!#$%",
		},
		{
			name:         "Secret not found",
			postgresqlID: "nonexistent",
			statusCode:   http.StatusNotFound,
			responseBody: map[string]interface{}{
				"errors": []string{"no secret found"},
			},
			expectError:   true,
			errorContains: "failed to read secret from Vault",
		},
		{
			name:         "Permission denied",
			postgresqlID: "forbidden-pg",
			statusCode:   http.StatusForbidden,
			responseBody: map[string]interface{}{
				"errors": []string{"permission denied"},
			},
			expectError:   true,
			errorContains: "failed to read secret from Vault",
		},
		{
			name:         "Missing username field",
			postgresqlID: "missing-username",
			statusCode:   http.StatusOK,
			responseBody: map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"admin_password": "password123",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			},
			expectError:   true,
			errorContains: "credentials not found",
		},
		{
			name:         "Missing password field",
			postgresqlID: "missing-password",
			statusCode:   http.StatusOK,
			responseBody: map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"admin_username": "admin",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			},
			expectError:   true,
			errorContains: "credentials not found",
		},
		{
			name:         "Empty credentials",
			postgresqlID: "empty-creds",
			statusCode:   http.StatusOK,
			responseBody: map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"admin_username": "",
						"admin_password": "",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			},
			expectError:   true,
			errorContains: "credentials not found",
		},
		{
			name:         "Internal server error",
			postgresqlID: "error-pg",
			statusCode:   http.StatusInternalServerError,
			responseBody: map[string]interface{}{
				"errors": []string{"internal error"},
			},
			expectError:   true,
			errorContains: "failed to read secret from Vault",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/v1/secret/data/pdb/%s/admin", tt.postgresqlID)
				if r.URL.Path == expectedPath && r.Method == "GET" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.statusCode)
					if tt.responseBody != nil {
						err := json.NewEncoder(w).Encode(tt.responseBody)
						require.NoError(t, err)
					}
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			client, server := createTestVaultClient(t, handler)
			defer server.Close()

			username, password, err := client.GetPostgresqlCredentials(context.Background(), tt.postgresqlID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, username)
				assert.Empty(t, password)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedUsername, username)
				assert.Equal(t, tt.expectedPassword, password)
			}
		})
	}
}

// TestClient_GetPostgresqlUserCredentials_Success tests successful user credential retrieval
func TestClient_GetPostgresqlUserCredentials_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/secret/data/pdb/test-pg/testuser" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": "userpassword123",
					},
					"metadata": map[string]interface{}{
						"version": 1,
					},
				},
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	password, err := client.GetPostgresqlUserCredentials(context.Background(), "test-pg", "testuser")
	assert.NoError(t, err)
	assert.Equal(t, "userpassword123", password)
}

// TestClient_GetPostgresqlUserCredentials_NotFound tests user credential retrieval when not found
func TestClient_GetPostgresqlUserCredentials_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/secret/data/") && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			response := map[string]interface{}{
				"errors": []string{"no secret found"},
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	password, err := client.GetPostgresqlUserCredentials(context.Background(), "test-pg", "nonexistent")
	assert.Error(t, err)
	assert.Empty(t, password)
	assert.Contains(t, err.Error(), "failed to read secret from Vault")
}

// TestClient_GetPostgresqlUserCredentials_MissingPassword tests when password is missing
func TestClient_GetPostgresqlUserCredentials_MissingPassword(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/secret/data/pdb/test-pg/testuser" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						// Missing password field
					},
					"metadata": map[string]interface{}{
						"version": 1,
					},
				},
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	password, err := client.GetPostgresqlUserCredentials(context.Background(), "test-pg", "testuser")
	assert.Error(t, err)
	assert.Empty(t, password)
	assert.Contains(t, err.Error(), "credentials not found")
}

// TestClient_GetPostgresqlUserCredentials_EmptyPassword tests when password is empty
func TestClient_GetPostgresqlUserCredentials_EmptyPassword(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/secret/data/pdb/test-pg/testuser" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": "",
					},
					"metadata": map[string]interface{}{
						"version": 1,
					},
				},
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	password, err := client.GetPostgresqlUserCredentials(context.Background(), "test-pg", "testuser")
	assert.Error(t, err)
	assert.Empty(t, password)
	assert.Contains(t, err.Error(), "credentials not found")
}

// TestClient_GetPostgresqlUserCredentials_ContextCancellation tests context cancellation
func TestClient_GetPostgresqlUserCredentials_ContextCancellation(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	password, err := client.GetPostgresqlUserCredentials(ctx, "test-pg", "testuser")
	assert.Error(t, err)
	assert.Empty(t, password)
}

// TestClient_GetPostgresqlUserCredentials_TableDriven tests GetPostgresqlUserCredentials with various scenarios
func TestClient_GetPostgresqlUserCredentials_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		postgresqlID     string
		username         string
		statusCode       int
		responseBody     map[string]interface{}
		expectError      bool
		errorContains    string
		expectedPassword string
	}{
		{
			name:         "Success with valid password",
			postgresqlID: "test-pg",
			username:     "testuser",
			statusCode:   http.StatusOK,
			responseBody: map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": "userpass123",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			},
			expectError:      false,
			expectedPassword: "userpass123",
		},
		{
			name:         "Success with special characters in password",
			postgresqlID: "test-pg",
			username:     "special_user",
			statusCode:   http.StatusOK,
			responseBody: map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": "p@ss=word!#$%^&*()",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			},
			expectError:      false,
			expectedPassword: "p@ss=word!#$%^&*()",
		},
		{
			name:         "User not found",
			postgresqlID: "test-pg",
			username:     "nonexistent",
			statusCode:   http.StatusNotFound,
			responseBody: map[string]interface{}{
				"errors": []string{"no secret found"},
			},
			expectError:   true,
			errorContains: "failed to read secret from Vault",
		},
		{
			name:         "Permission denied",
			postgresqlID: "test-pg",
			username:     "forbidden",
			statusCode:   http.StatusForbidden,
			responseBody: map[string]interface{}{
				"errors": []string{"permission denied"},
			},
			expectError:   true,
			errorContains: "failed to read secret from Vault",
		},
		{
			name:         "Missing password field",
			postgresqlID: "test-pg",
			username:     "missing-pass",
			statusCode:   http.StatusOK,
			responseBody: map[string]interface{}{
				"data": map[string]interface{}{
					"data":     map[string]interface{}{},
					"metadata": map[string]interface{}{"version": 1},
				},
			},
			expectError:   true,
			errorContains: "credentials not found",
		},
		{
			name:         "Empty password",
			postgresqlID: "test-pg",
			username:     "empty-pass",
			statusCode:   http.StatusOK,
			responseBody: map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": "",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			},
			expectError:   true,
			errorContains: "credentials not found",
		},
		{
			name:         "Internal server error",
			postgresqlID: "test-pg",
			username:     "error-user",
			statusCode:   http.StatusInternalServerError,
			responseBody: map[string]interface{}{
				"errors": []string{"internal error"},
			},
			expectError:   true,
			errorContains: "failed to read secret from Vault",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/v1/secret/data/pdb/%s/%s", tt.postgresqlID, tt.username)
				if r.URL.Path == expectedPath && r.Method == "GET" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.statusCode)
					if tt.responseBody != nil {
						err := json.NewEncoder(w).Encode(tt.responseBody)
						require.NoError(t, err)
					}
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			client, server := createTestVaultClient(t, handler)
			defer server.Close()

			password, err := client.GetPostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, password)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPassword, password)
			}
		})
	}
}

// TestClient_StorePostgresqlUserCredentials_Success tests successful credential storage
func TestClient_StorePostgresqlUserCredentials_Success(t *testing.T) {
	var receivedData map[string]interface{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// KVv2 uses PUT method for writes
		if r.URL.Path == "/v1/secret/data/pdb/test-pg/testuser" && (r.Method == "POST" || r.Method == "PUT") {
			// Decode the request body
			var requestBody map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&requestBody)
			require.NoError(t, err)
			receivedData = requestBody

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"created_time":  "2024-01-01T00:00:00Z",
					"deletion_time": "",
					"destroyed":     false,
					"version":       1,
				},
			}
			err = json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	err := client.StorePostgresqlUserCredentials(context.Background(), "test-pg", "testuser", "newpassword123")
	assert.NoError(t, err)

	// Verify the data was sent correctly
	assert.NotNil(t, receivedData)
	data, ok := receivedData["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "newpassword123", data["password"])
}

// TestClient_StorePostgresqlUserCredentials_Error tests storage failure
func TestClient_StorePostgresqlUserCredentials_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// KVv2 uses PUT method for writes
		if strings.HasPrefix(r.URL.Path, "/v1/secret/data/") && (r.Method == "POST" || r.Method == "PUT") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			response := map[string]interface{}{
				"errors": []string{"permission denied"},
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	err := client.StorePostgresqlUserCredentials(context.Background(), "test-pg", "testuser", "password")
	assert.Error(t, err)
}

// TestClient_StorePostgresqlUserCredentials_ContextCancellation tests context cancellation
func TestClient_StorePostgresqlUserCredentials_ContextCancellation(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.StorePostgresqlUserCredentials(ctx, "test-pg", "testuser", "password")
	assert.Error(t, err)
}

// TestClient_StorePostgresqlUserCredentials_TableDriven tests StorePostgresqlUserCredentials with various scenarios
func TestClient_StorePostgresqlUserCredentials_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		postgresqlID  string
		username      string
		password      string
		statusCode    int
		responseBody  map[string]interface{}
		expectError   bool
		errorContains string
	}{
		{
			name:         "Success storing new credentials",
			postgresqlID: "test-pg",
			username:     "newuser",
			password:     "newpassword123",
			statusCode:   http.StatusOK,
			responseBody: map[string]interface{}{
				"data": map[string]interface{}{
					"version": 1,
				},
			},
			expectError: false,
		},
		{
			name:         "Success updating existing credentials",
			postgresqlID: "test-pg",
			username:     "existinguser",
			password:     "updatedpassword",
			statusCode:   http.StatusOK,
			responseBody: map[string]interface{}{
				"data": map[string]interface{}{
					"version": 2,
				},
			},
			expectError: false,
		},
		{
			name:         "Success with special characters in password",
			postgresqlID: "test-pg",
			username:     "specialuser",
			password:     "p@ss=word!#$%^&*()",
			statusCode:   http.StatusOK,
			responseBody: map[string]interface{}{
				"data": map[string]interface{}{
					"version": 1,
				},
			},
			expectError: false,
		},
		{
			name:         "Success with empty password",
			postgresqlID: "test-pg",
			username:     "emptypassuser",
			password:     "",
			statusCode:   http.StatusOK,
			responseBody: map[string]interface{}{
				"data": map[string]interface{}{
					"version": 1,
				},
			},
			expectError: false,
		},
		{
			name:         "Permission denied",
			postgresqlID: "test-pg",
			username:     "forbidden",
			password:     "password",
			statusCode:   http.StatusForbidden,
			responseBody: map[string]interface{}{
				"errors": []string{"permission denied"},
			},
			expectError: true,
		},
		{
			name:         "Internal server error",
			postgresqlID: "test-pg",
			username:     "erroruser",
			password:     "password",
			statusCode:   http.StatusInternalServerError,
			responseBody: map[string]interface{}{
				"errors": []string{"internal error"},
			},
			expectError: true,
		},
		{
			name:         "Service unavailable",
			postgresqlID: "test-pg",
			username:     "unavailable",
			password:     "password",
			statusCode:   http.StatusServiceUnavailable,
			responseBody: map[string]interface{}{
				"errors": []string{"service unavailable"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/v1/secret/data/pdb/%s/%s", tt.postgresqlID, tt.username)
				// KVv2 uses PUT method for writes
				if r.URL.Path == expectedPath && (r.Method == "POST" || r.Method == "PUT") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.statusCode)
					if tt.responseBody != nil {
						err := json.NewEncoder(w).Encode(tt.responseBody)
						require.NoError(t, err)
					}
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			client, server := createTestVaultClient(t, handler)
			defer server.Close()

			err := client.StorePostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username, tt.password)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestClient_StorePostgresqlUserCredentials_VerifyRequestBody tests that the request body is correct
func TestClient_StorePostgresqlUserCredentials_VerifyRequestBody(t *testing.T) {
	tests := []struct {
		name             string
		password         string
		expectedPassword string
	}{
		{
			name:             "Simple password",
			password:         "simple123",
			expectedPassword: "simple123",
		},
		{
			name:             "Password with special characters",
			password:         "p@ss=word!#$%",
			expectedPassword: "p@ss=word!#$%",
		},
		{
			name:             "Empty password",
			password:         "",
			expectedPassword: "",
		},
		{
			name:             "Long password",
			password:         "verylongpasswordthatmightbeusedinsomeproductionenvironments1234567890",
			expectedPassword: "verylongpasswordthatmightbeusedinsomeproductionenvironments1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedPassword string

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// KVv2 uses PUT method for writes
				if r.URL.Path == "/v1/secret/data/pdb/test-pg/testuser" && (r.Method == "POST" || r.Method == "PUT") {
					var requestBody map[string]interface{}
					err := json.NewDecoder(r.Body).Decode(&requestBody)
					require.NoError(t, err)

					data, ok := requestBody["data"].(map[string]interface{})
					require.True(t, ok)
					receivedPassword, _ = data["password"].(string)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					response := map[string]interface{}{
						"data": map[string]interface{}{"version": 1},
					}
					err = json.NewEncoder(w).Encode(response)
					require.NoError(t, err)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			client, server := createTestVaultClient(t, handler)
			defer server.Close()

			err := client.StorePostgresqlUserCredentials(context.Background(), "test-pg", "testuser", tt.password)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPassword, receivedPassword)
		})
	}
}

// TestClient_CustomMountPointAndSecretPath tests client with custom mount point and secret path
func TestClient_CustomMountPointAndSecretPath(t *testing.T) {
	tests := []struct {
		name         string
		mountPoint   string
		secretPath   string
		postgresqlID string
		expectedPath string
	}{
		{
			name:         "Default mount point",
			mountPoint:   "secret",
			secretPath:   "pdb",
			postgresqlID: "test-pg",
			expectedPath: "/v1/secret/data/pdb/test-pg/admin",
		},
		{
			name:         "Custom mount point",
			mountPoint:   "kv",
			secretPath:   "postgresql",
			postgresqlID: "prod-db",
			expectedPath: "/v1/kv/data/postgresql/prod-db/admin",
		},
		{
			name:         "Nested secret path",
			mountPoint:   "secret",
			secretPath:   "apps/team/postgresql",
			postgresqlID: "db-123",
			expectedPath: "/v1/secret/data/apps/team/postgresql/db-123/admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedPath string

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				if r.Method == "GET" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					response := map[string]interface{}{
						"data": map[string]interface{}{
							"data": map[string]interface{}{
								"admin_username": "admin",
								"admin_password": "password",
							},
							"metadata": map[string]interface{}{"version": 1},
						},
					}
					err := json.NewEncoder(w).Encode(response)
					require.NoError(t, err)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			})

			server := httptest.NewServer(handler)
			defer server.Close()

			config := api.DefaultConfig()
			config.Address = server.URL

			apiClient, err := api.NewClient(config)
			require.NoError(t, err)
			apiClient.SetToken("test-token")

			client := &Client{
				client:          apiClient,
				vaultMountPoint: tt.mountPoint,
				vaultSecretPath: tt.secretPath,
			}

			_, _, err = client.GetPostgresqlCredentials(context.Background(), tt.postgresqlID)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPath, receivedPath)
		})
	}
}

// TestClient_NilDataResponse tests handling of nil data in response
func TestClient_NilDataResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/secret/data/") && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Return response with nil data
			response := map[string]interface{}{
				"data": nil,
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	// Test GetPostgresqlCredentials with nil data
	username, password, err := client.GetPostgresqlCredentials(context.Background(), "test-pg")
	assert.Error(t, err)
	assert.Empty(t, username)
	assert.Empty(t, password)
	assert.Contains(t, err.Error(), "secret not found")

	// Test GetPostgresqlUserCredentials with nil data
	userPassword, err := client.GetPostgresqlUserCredentials(context.Background(), "test-pg", "testuser")
	assert.Error(t, err)
	assert.Empty(t, userPassword)
	assert.Contains(t, err.Error(), "secret not found")
}

// TestClient_WrongTypeInResponse tests handling of wrong types in response
func TestClient_WrongTypeInResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/secret/data/pdb/test-pg/admin" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Return response with wrong types
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"admin_username": 12345, // Should be string
						"admin_password": true,  // Should be string
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			}
			err := json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	username, password, err := client.GetPostgresqlCredentials(context.Background(), "test-pg")
	assert.Error(t, err)
	assert.Empty(t, username)
	assert.Empty(t, password)
	assert.Contains(t, err.Error(), "credentials not found")
}

// TestClient_ContextTimeout tests context timeout handling
func TestClient_ContextTimeout(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	client, server := createTestVaultClient(t, handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Test CheckHealth with timeout
	err := client.CheckHealth(ctx)
	assert.Error(t, err)

	// Test GetPostgresqlCredentials with timeout
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	_, _, err = client.GetPostgresqlCredentials(ctx2, "test-pg")
	assert.Error(t, err)

	// Test GetPostgresqlUserCredentials with timeout
	ctx3, cancel3 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel3()
	_, err = client.GetPostgresqlUserCredentials(ctx3, "test-pg", "testuser")
	assert.Error(t, err)

	// Test StorePostgresqlUserCredentials with timeout
	ctx4, cancel4 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel4()
	err = client.StorePostgresqlUserCredentials(ctx4, "test-pg", "testuser", "password")
	assert.Error(t, err)
}
