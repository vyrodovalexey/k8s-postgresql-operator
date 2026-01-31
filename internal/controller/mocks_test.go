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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMockVaultClient_GetPostgresqlCredentials tests the mock implementation
func TestMockVaultClient_GetPostgresqlCredentials(t *testing.T) {
	tests := []struct {
		name             string
		postgresqlID     string
		expectedUsername string
		expectedPassword string
		expectedError    error
	}{
		{
			name:             "Success case",
			postgresqlID:     "test-id",
			expectedUsername: "admin",
			expectedPassword: "secret",
			expectedError:    nil,
		},
		{
			name:             "Error case",
			postgresqlID:     "error-id",
			expectedUsername: "",
			expectedPassword: "",
			expectedError:    fmt.Errorf("vault error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVault := new(MockVaultClient)
			mockVault.On("GetPostgresqlCredentials", context.Background(), tt.postgresqlID).
				Return(tt.expectedUsername, tt.expectedPassword, tt.expectedError)

			username, password, err := mockVault.GetPostgresqlCredentials(context.Background(), tt.postgresqlID)

			assert.Equal(t, tt.expectedUsername, username)
			assert.Equal(t, tt.expectedPassword, password)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
			mockVault.AssertExpectations(t)
		})
	}
}

// TestMockVaultClient_GetPostgresqlUserCredentials tests the mock implementation
func TestMockVaultClient_GetPostgresqlUserCredentials(t *testing.T) {
	tests := []struct {
		name             string
		postgresqlID     string
		username         string
		expectedPassword string
		expectedError    error
	}{
		{
			name:             "Success case",
			postgresqlID:     "test-id",
			username:         "testuser",
			expectedPassword: "userpass",
			expectedError:    nil,
		},
		{
			name:             "Error case",
			postgresqlID:     "error-id",
			username:         "erroruser",
			expectedPassword: "",
			expectedError:    fmt.Errorf("user not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVault := new(MockVaultClient)
			mockVault.On("GetPostgresqlUserCredentials", context.Background(), tt.postgresqlID, tt.username).
				Return(tt.expectedPassword, tt.expectedError)

			password, err := mockVault.GetPostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username)

			assert.Equal(t, tt.expectedPassword, password)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
			mockVault.AssertExpectations(t)
		})
	}
}

// TestMockVaultClient_StorePostgresqlUserCredentials tests the mock implementation
func TestMockVaultClient_StorePostgresqlUserCredentials(t *testing.T) {
	tests := []struct {
		name          string
		postgresqlID  string
		username      string
		password      string
		expectedError error
	}{
		{
			name:          "Success case",
			postgresqlID:  "test-id",
			username:      "testuser",
			password:      "newpass",
			expectedError: nil,
		},
		{
			name:          "Error case",
			postgresqlID:  "error-id",
			username:      "erroruser",
			password:      "pass",
			expectedError: fmt.Errorf("storage error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVault := new(MockVaultClient)
			mockVault.On("StorePostgresqlUserCredentials", context.Background(), tt.postgresqlID, tt.username, tt.password).
				Return(tt.expectedError)

			err := mockVault.StorePostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username, tt.password)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
			mockVault.AssertExpectations(t)
		})
	}
}

// TestMockVaultClient_StorePostgresqlCredentials tests the mock implementation
func TestMockVaultClient_StorePostgresqlCredentials(t *testing.T) {
	tests := []struct {
		name          string
		postgresqlID  string
		username      string
		password      string
		expectedError error
	}{
		{
			name:          "Success case",
			postgresqlID:  "test-id",
			username:      "admin",
			password:      "adminpass",
			expectedError: nil,
		},
		{
			name:          "Error case",
			postgresqlID:  "error-id",
			username:      "admin",
			password:      "pass",
			expectedError: fmt.Errorf("storage error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVault := new(MockVaultClient)
			mockVault.On("StorePostgresqlCredentials", context.Background(), tt.postgresqlID, tt.username, tt.password).
				Return(tt.expectedError)

			err := mockVault.StorePostgresqlCredentials(context.Background(), tt.postgresqlID, tt.username, tt.password)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
			mockVault.AssertExpectations(t)
		})
	}
}

// TestMockVaultClient_CheckHealth tests the mock implementation
func TestMockVaultClient_CheckHealth(t *testing.T) {
	tests := []struct {
		name          string
		expectedError error
	}{
		{
			name:          "Healthy",
			expectedError: nil,
		},
		{
			name:          "Unhealthy",
			expectedError: fmt.Errorf("vault unhealthy"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVault := new(MockVaultClient)
			mockVault.On("CheckHealth", context.Background()).Return(tt.expectedError)

			err := mockVault.CheckHealth(context.Background())

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
			mockVault.AssertExpectations(t)
		})
	}
}

// TestMockVaultClient_ImplementsInterface verifies the mock implements the interface
func TestMockVaultClient_ImplementsInterface(t *testing.T) {
	var _ VaultClientInterface = (*MockVaultClient)(nil)
}
