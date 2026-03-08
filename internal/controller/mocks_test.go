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

func TestMockVaultClient_GetPostgresqlCredentials(t *testing.T) {
	tests := []struct {
		name             string
		postgresqlID     string
		returnUsername   string
		returnPassword   string
		returnErr        error
		expectedUsername string
		expectedPassword string
		expectedError    bool
	}{
		{
			name:             "Success",
			postgresqlID:     "test-id",
			returnUsername:   "admin",
			returnPassword:   "secret",
			returnErr:        nil,
			expectedUsername: "admin",
			expectedPassword: "secret",
			expectedError:    false,
		},
		{
			name:             "Error",
			postgresqlID:     "test-id",
			returnUsername:   "",
			returnPassword:   "",
			returnErr:        fmt.Errorf("vault error"),
			expectedUsername: "",
			expectedPassword: "",
			expectedError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVault := new(MockVaultClient)
			mockVault.On("GetPostgresqlCredentials", context.Background(), tt.postgresqlID).
				Return(tt.returnUsername, tt.returnPassword, tt.returnErr)

			username, password, err := mockVault.GetPostgresqlCredentials(context.Background(), tt.postgresqlID)

			assert.Equal(t, tt.expectedUsername, username)
			assert.Equal(t, tt.expectedPassword, password)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockVault.AssertExpectations(t)
		})
	}
}

func TestMockVaultClient_GetPostgresqlUserCredentials(t *testing.T) {
	tests := []struct {
		name             string
		postgresqlID     string
		username         string
		returnPassword   string
		returnErr        error
		expectedPassword string
		expectedError    bool
	}{
		{
			name:             "Success",
			postgresqlID:     "test-id",
			username:         "testuser",
			returnPassword:   "userpass",
			returnErr:        nil,
			expectedPassword: "userpass",
			expectedError:    false,
		},
		{
			name:             "Error",
			postgresqlID:     "test-id",
			username:         "testuser",
			returnPassword:   "",
			returnErr:        fmt.Errorf("vault error"),
			expectedPassword: "",
			expectedError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVault := new(MockVaultClient)
			mockVault.On("GetPostgresqlUserCredentials", context.Background(), tt.postgresqlID, tt.username).
				Return(tt.returnPassword, tt.returnErr)

			password, err := mockVault.GetPostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username)

			assert.Equal(t, tt.expectedPassword, password)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockVault.AssertExpectations(t)
		})
	}
}

func TestMockVaultClient_StorePostgresqlUserCredentials(t *testing.T) {
	tests := []struct {
		name          string
		postgresqlID  string
		username      string
		password      string
		returnErr     error
		expectedError bool
	}{
		{
			name:          "Success",
			postgresqlID:  "test-id",
			username:      "testuser",
			password:      "userpass",
			returnErr:     nil,
			expectedError: false,
		},
		{
			name:          "Error",
			postgresqlID:  "test-id",
			username:      "testuser",
			password:      "userpass",
			returnErr:     fmt.Errorf("vault error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVault := new(MockVaultClient)
			mockVault.On("StorePostgresqlUserCredentials", context.Background(), tt.postgresqlID, tt.username, tt.password).
				Return(tt.returnErr)

			err := mockVault.StorePostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username, tt.password)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockVault.AssertExpectations(t)
		})
	}
}

func TestMockVaultClient_StorePostgresqlCredentials(t *testing.T) {
	mockVault := new(MockVaultClient)
	mockVault.On("StorePostgresqlCredentials", context.Background(), "test-id", "admin", "secret").
		Return(nil)

	err := mockVault.StorePostgresqlCredentials(context.Background(), "test-id", "admin", "secret")
	assert.NoError(t, err)
	mockVault.AssertExpectations(t)
}

func TestMockVaultClient_CheckHealth(t *testing.T) {
	tests := []struct {
		name          string
		returnErr     error
		expectedError bool
	}{
		{
			name:          "Healthy",
			returnErr:     nil,
			expectedError: false,
		},
		{
			name:          "Unhealthy",
			returnErr:     fmt.Errorf("vault unhealthy"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVault := new(MockVaultClient)
			mockVault.On("CheckHealth", context.Background()).Return(tt.returnErr)

			err := mockVault.CheckHealth(context.Background())

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockVault.AssertExpectations(t)
		})
	}
}

// TestVaultClientInterface_Compliance verifies MockVaultClient implements VaultClientInterface
func TestVaultClientInterface_Compliance(t *testing.T) {
	var _ VaultClientInterface = (*MockVaultClient)(nil)
}
