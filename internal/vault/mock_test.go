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
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockVaultClient is a mock implementation of ClientInterface for testing
type MockVaultClient struct {
	mu sync.RWMutex

	// In-memory storage for credentials
	adminCredentials map[string]AdminCredentials
	userCredentials  map[string]string // key: postgresqlID/username
	healthy          bool
	healthError      error

	// Call tracking
	checkHealthCalls                    int
	getPostgresqlCredentialsCalls       int
	getPostgresqlUserCredentialsCalls   int
	storePostgresqlUserCredentialsCalls int
}

// AdminCredentials holds admin username and password
type AdminCredentials struct {
	Username string
	Password string
}

// NewMockVaultClient creates a new MockVaultClient
func NewMockVaultClient() *MockVaultClient {
	return &MockVaultClient{
		adminCredentials: make(map[string]AdminCredentials),
		userCredentials:  make(map[string]string),
		healthy:          true,
	}
}

// CheckHealth mocks CheckHealth
func (m *MockVaultClient) CheckHealth(ctx context.Context) error {
	m.mu.Lock()
	m.checkHealthCalls++
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.healthy {
		if m.healthError != nil {
			return m.healthError
		}
		return fmt.Errorf("vault is unhealthy")
	}
	return nil
}

// GetPostgresqlCredentials mocks GetPostgresqlCredentials
func (m *MockVaultClient) GetPostgresqlCredentials(
	ctx context.Context, postgresqlID string) (username, password string, err error) {
	m.mu.Lock()
	m.getPostgresqlCredentialsCalls++
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()

	creds, ok := m.adminCredentials[postgresqlID]
	if !ok {
		return "", "", fmt.Errorf("credentials not found for postgresqlID: %s", postgresqlID)
	}
	return creds.Username, creds.Password, nil
}

// GetPostgresqlUserCredentials mocks GetPostgresqlUserCredentials
func (m *MockVaultClient) GetPostgresqlUserCredentials(
	ctx context.Context, postgresqlID, username string) (password string, err error) {
	m.mu.Lock()
	m.getPostgresqlUserCredentialsCalls++
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", postgresqlID, username)
	pwd, ok := m.userCredentials[key]
	if !ok {
		return "", fmt.Errorf("credentials not found for user %s in postgresqlID: %s", username, postgresqlID)
	}
	return pwd, nil
}

// StorePostgresqlUserCredentials mocks StorePostgresqlUserCredentials
func (m *MockVaultClient) StorePostgresqlUserCredentials(
	ctx context.Context, postgresqlID, username, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.storePostgresqlUserCredentialsCalls++

	key := fmt.Sprintf("%s/%s", postgresqlID, username)
	m.userCredentials[key] = password
	return nil
}

// SetAdminCredentials sets admin credentials for a PostgreSQL instance
func (m *MockVaultClient) SetAdminCredentials(postgresqlID, username, password string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.adminCredentials[postgresqlID] = AdminCredentials{
		Username: username,
		Password: password,
	}
}

// SetUserCredentials sets user credentials for a PostgreSQL instance
func (m *MockVaultClient) SetUserCredentials(postgresqlID, username, password string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s", postgresqlID, username)
	m.userCredentials[key] = password
}

// SetHealthy sets the health status of the mock vault
func (m *MockVaultClient) SetHealthy(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.healthy = healthy
}

// SetHealthError sets a custom health error
func (m *MockVaultClient) SetHealthError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.healthError = err
}

// GetCheckHealthCalls returns the number of CheckHealth calls
func (m *MockVaultClient) GetCheckHealthCalls() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.checkHealthCalls
}

// GetGetPostgresqlCredentialsCalls returns the number of GetPostgresqlCredentials calls
func (m *MockVaultClient) GetGetPostgresqlCredentialsCalls() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getPostgresqlCredentialsCalls
}

// Ensure MockVaultClient implements ClientInterface
var _ ClientInterface = (*MockVaultClient)(nil)

// TestMockVaultClient_CheckHealth tests CheckHealth mock
func TestMockVaultClient_CheckHealth(t *testing.T) {
	tests := []struct {
		name        string
		healthy     bool
		healthError error
		expectError bool
	}{
		{
			name:        "Healthy vault",
			healthy:     true,
			expectError: false,
		},
		{
			name:        "Unhealthy vault",
			healthy:     false,
			expectError: true,
		},
		{
			name:        "Unhealthy with custom error",
			healthy:     false,
			healthError: fmt.Errorf("custom health error"),
			expectError: true,
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
				if tt.healthError != nil {
					assert.Equal(t, tt.healthError, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestMockVaultClient_GetPostgresqlCredentials tests GetPostgresqlCredentials mock
func TestMockVaultClient_GetPostgresqlCredentials(t *testing.T) {
	tests := []struct {
		name         string
		postgresqlID string
		setupCreds   bool
		username     string
		password     string
		expectError  bool
	}{
		{
			name:         "Credentials found",
			postgresqlID: "test-id",
			setupCreds:   true,
			username:     "admin",
			password:     "secret123",
			expectError:  false,
		},
		{
			name:         "Credentials not found",
			postgresqlID: "nonexistent-id",
			setupCreds:   false,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockVaultClient()
			if tt.setupCreds {
				mock.SetAdminCredentials(tt.postgresqlID, tt.username, tt.password)
			}

			username, password, err := mock.GetPostgresqlCredentials(context.Background(), tt.postgresqlID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, username)
				assert.Empty(t, password)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.username, username)
				assert.Equal(t, tt.password, password)
			}
		})
	}
}

// TestMockVaultClient_GetPostgresqlUserCredentials tests GetPostgresqlUserCredentials mock
func TestMockVaultClient_GetPostgresqlUserCredentials(t *testing.T) {
	tests := []struct {
		name         string
		postgresqlID string
		username     string
		setupCreds   bool
		password     string
		expectError  bool
	}{
		{
			name:         "User credentials found",
			postgresqlID: "test-id",
			username:     "testuser",
			setupCreds:   true,
			password:     "userpass123",
			expectError:  false,
		},
		{
			name:         "User credentials not found",
			postgresqlID: "test-id",
			username:     "nonexistent",
			setupCreds:   false,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockVaultClient()
			if tt.setupCreds {
				mock.SetUserCredentials(tt.postgresqlID, tt.username, tt.password)
			}

			password, err := mock.GetPostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, password)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.password, password)
			}
		})
	}
}

// TestMockVaultClient_StorePostgresqlUserCredentials tests StorePostgresqlUserCredentials mock
func TestMockVaultClient_StorePostgresqlUserCredentials(t *testing.T) {
	tests := []struct {
		name         string
		postgresqlID string
		username     string
		password     string
	}{
		{
			name:         "Store new credentials",
			postgresqlID: "test-id",
			username:     "newuser",
			password:     "newpass123",
		},
		{
			name:         "Update existing credentials",
			postgresqlID: "test-id",
			username:     "existinguser",
			password:     "updatedpass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockVaultClient()

			err := mock.StorePostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username, tt.password)
			require.NoError(t, err)

			// Verify credentials were stored
			storedPassword, err := mock.GetPostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username)
			require.NoError(t, err)
			assert.Equal(t, tt.password, storedPassword)
		})
	}
}

// TestMockVaultClient_CallTracking tests that calls are tracked correctly
func TestMockVaultClient_CallTracking(t *testing.T) {
	mock := NewMockVaultClient()
	mock.SetHealthy(true)
	mock.SetAdminCredentials("test-id", "admin", "pass")
	mock.SetUserCredentials("test-id", "user", "userpass")

	// Make calls
	_ = mock.CheckHealth(context.Background())
	_ = mock.CheckHealth(context.Background())
	_, _, _ = mock.GetPostgresqlCredentials(context.Background(), "test-id")
	_, _ = mock.GetPostgresqlUserCredentials(context.Background(), "test-id", "user")
	_ = mock.StorePostgresqlUserCredentials(context.Background(), "test-id", "newuser", "newpass")

	// Verify call counts
	assert.Equal(t, 2, mock.GetCheckHealthCalls())
	assert.Equal(t, 1, mock.GetGetPostgresqlCredentialsCalls())
}

// TestMockVaultClient_ConcurrentAccess tests thread safety
func TestMockVaultClient_ConcurrentAccess(t *testing.T) {
	mock := NewMockVaultClient()
	mock.SetHealthy(true)

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			postgresqlID := fmt.Sprintf("pg-%d", id)
			mock.SetAdminCredentials(postgresqlID, "admin", "pass")
			_, _, _ = mock.GetPostgresqlCredentials(context.Background(), postgresqlID)
			_ = mock.CheckHealth(context.Background())
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no panics occurred and calls were tracked
	assert.GreaterOrEqual(t, mock.GetCheckHealthCalls(), 10)
}
