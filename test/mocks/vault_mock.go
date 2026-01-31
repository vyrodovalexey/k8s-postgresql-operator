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

// Package mocks provides mock implementations for testing
package mocks

import (
	"context"
	"fmt"
	"sync"

	"github.com/stretchr/testify/mock"
)

// VaultClientInterface defines the interface for Vault operations
type VaultClientInterface interface {
	GetPostgresqlCredentials(ctx context.Context, postgresqlID string) (username, password string, err error)
	GetPostgresqlUserCredentials(ctx context.Context, postgresqlID, username string) (password string, err error)
	StorePostgresqlUserCredentials(ctx context.Context, postgresqlID, username, password string) error
	CheckHealth(ctx context.Context) error
}

// MockVaultClient is a mock implementation of VaultClientInterface for testing
type MockVaultClient struct {
	mock.Mock
	mu sync.RWMutex

	// In-memory storage for credentials
	adminCredentials map[string]AdminCredentials
	userCredentials  map[string]string // key: postgresqlID/username
	healthy          bool
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

// GetPostgresqlCredentials mocks GetPostgresqlCredentials
func (m *MockVaultClient) GetPostgresqlCredentials(
	ctx context.Context, postgresqlID string) (username, password string, err error) {
	args := m.Called(ctx, postgresqlID)
	return args.String(0), args.String(1), args.Error(2)
}

// GetPostgresqlUserCredentials mocks GetPostgresqlUserCredentials
func (m *MockVaultClient) GetPostgresqlUserCredentials(
	ctx context.Context, postgresqlID, username string) (password string, err error) {
	args := m.Called(ctx, postgresqlID, username)
	return args.String(0), args.Error(1)
}

// StorePostgresqlUserCredentials mocks StorePostgresqlUserCredentials
func (m *MockVaultClient) StorePostgresqlUserCredentials(
	ctx context.Context, postgresqlID, username, password string) error {
	args := m.Called(ctx, postgresqlID, username, password)
	return args.Error(0)
}

// CheckHealth mocks CheckHealth
func (m *MockVaultClient) CheckHealth(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Ensure MockVaultClient implements VaultClientInterface
var _ VaultClientInterface = (*MockVaultClient)(nil)

// InMemoryVaultClient is an in-memory implementation of VaultClientInterface for testing
// This is useful for functional tests that need actual credential storage
type InMemoryVaultClient struct {
	mu sync.RWMutex

	// In-memory storage for credentials
	adminCredentials map[string]AdminCredentials
	userCredentials  map[string]string // key: postgresqlID/username
	healthy          bool
	healthError      error
}

// NewInMemoryVaultClient creates a new InMemoryVaultClient
func NewInMemoryVaultClient() *InMemoryVaultClient {
	return &InMemoryVaultClient{
		adminCredentials: make(map[string]AdminCredentials),
		userCredentials:  make(map[string]string),
		healthy:          true,
	}
}

// GetPostgresqlCredentials retrieves admin credentials from in-memory storage
func (c *InMemoryVaultClient) GetPostgresqlCredentials(
	ctx context.Context, postgresqlID string) (username, password string, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	creds, ok := c.adminCredentials[postgresqlID]
	if !ok {
		return "", "", fmt.Errorf("credentials not found for postgresqlID: %s", postgresqlID)
	}
	return creds.Username, creds.Password, nil
}

// GetPostgresqlUserCredentials retrieves user credentials from in-memory storage
func (c *InMemoryVaultClient) GetPostgresqlUserCredentials(
	ctx context.Context, postgresqlID, username string) (password string, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", postgresqlID, username)
	pwd, ok := c.userCredentials[key]
	if !ok {
		return "", fmt.Errorf("credentials not found for user %s in postgresqlID: %s", username, postgresqlID)
	}
	return pwd, nil
}

// StorePostgresqlUserCredentials stores user credentials in in-memory storage
func (c *InMemoryVaultClient) StorePostgresqlUserCredentials(
	ctx context.Context, postgresqlID, username, password string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s/%s", postgresqlID, username)
	c.userCredentials[key] = password
	return nil
}

// CheckHealth checks if the mock vault is healthy
func (c *InMemoryVaultClient) CheckHealth(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.healthy {
		if c.healthError != nil {
			return c.healthError
		}
		return fmt.Errorf("vault is unhealthy")
	}
	return nil
}

// SetAdminCredentials sets admin credentials for a PostgreSQL instance
func (c *InMemoryVaultClient) SetAdminCredentials(postgresqlID, username, password string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.adminCredentials[postgresqlID] = AdminCredentials{
		Username: username,
		Password: password,
	}
}

// SetUserCredentials sets user credentials for a PostgreSQL instance
func (c *InMemoryVaultClient) SetUserCredentials(postgresqlID, username, password string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s/%s", postgresqlID, username)
	c.userCredentials[key] = password
}

// SetHealthy sets the health status of the mock vault
func (c *InMemoryVaultClient) SetHealthy(healthy bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.healthy = healthy
}

// SetHealthError sets a custom health error
func (c *InMemoryVaultClient) SetHealthError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.healthError = err
}

// ClearCredentials clears all stored credentials
func (c *InMemoryVaultClient) ClearCredentials() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.adminCredentials = make(map[string]AdminCredentials)
	c.userCredentials = make(map[string]string)
}

// HasUserCredentials checks if user credentials exist
func (c *InMemoryVaultClient) HasUserCredentials(postgresqlID, username string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", postgresqlID, username)
	_, ok := c.userCredentials[key]
	return ok
}

// Ensure InMemoryVaultClient implements VaultClientInterface
var _ VaultClientInterface = (*InMemoryVaultClient)(nil)

// FailingVaultClient is a VaultClientInterface that always fails
type FailingVaultClient struct {
	Error error
}

// NewFailingVaultClient creates a new FailingVaultClient
func NewFailingVaultClient(err error) *FailingVaultClient {
	if err == nil {
		err = fmt.Errorf("vault operation failed")
	}
	return &FailingVaultClient{Error: err}
}

// GetPostgresqlCredentials always returns an error
func (c *FailingVaultClient) GetPostgresqlCredentials(
	ctx context.Context, postgresqlID string) (username, password string, err error) {
	return "", "", c.Error
}

// GetPostgresqlUserCredentials always returns an error
func (c *FailingVaultClient) GetPostgresqlUserCredentials(
	ctx context.Context, postgresqlID, username string) (password string, err error) {
	return "", c.Error
}

// StorePostgresqlUserCredentials always returns an error
func (c *FailingVaultClient) StorePostgresqlUserCredentials(
	ctx context.Context, postgresqlID, username, password string) error {
	return c.Error
}

// CheckHealth always returns an error
func (c *FailingVaultClient) CheckHealth(ctx context.Context) error {
	return c.Error
}

// Ensure FailingVaultClient implements VaultClientInterface
var _ VaultClientInterface = (*FailingVaultClient)(nil)
