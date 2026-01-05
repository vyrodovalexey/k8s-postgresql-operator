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
	"context"

	"github.com/stretchr/testify/mock"
)

// VaultClientInterface defines the interface for Vault operations
// Note: This interface matches the methods available in vault.Client
type VaultClientInterface interface {
	GetPostgresqlCredentials(ctx context.Context, postgresqlID string) (username, password string, err error)
	GetPostgresqlUserCredentials(ctx context.Context, postgresqlID, username string) (password string, err error)
	StorePostgresqlUserCredentials(ctx context.Context, postgresqlID, username, password string) error
	CheckHealth(ctx context.Context) error
}

// MockVaultClient is a mock implementation of VaultClientInterface for testing
type MockVaultClient struct {
	mock.Mock
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

// StorePostgresqlCredentials mocks StorePostgresqlCredentials
// Note: This method is not in vault.Client but kept for backward compatibility with existing tests
func (m *MockVaultClient) StorePostgresqlCredentials(
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
