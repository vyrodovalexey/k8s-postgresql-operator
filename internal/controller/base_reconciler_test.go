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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestResolvePostgresqlConnection_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		postgresqlID   string
		setupMocks     func(*MockControllerClient)
		expectNil      bool
		expectedReason string
		expectedPort   int32
		expectedSSL    string
	}{
		{
			name:         "PostgreSQL not found - list returns empty",
			postgresqlID: "non-existent-id",
			setupMocks: func(mockClient *MockControllerClient) {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{}
				})
			},
			expectNil:      true,
			expectedReason: "PostgresqlNotFound",
		},
		{
			name:         "PostgreSQL not connected",
			postgresqlID: "test-id",
			setupMocks: func(mockClient *MockControllerClient) {
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, false)
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{*postgresql}
				})
			},
			expectNil:      true,
			expectedReason: "PostgresqlNotConnected",
		},
		{
			name:         "PostgreSQL has nil external instance - not found by FindPostgresqlByID",
			postgresqlID: "test-id",
			setupMocks: func(mockClient *MockControllerClient) {
				// FindPostgresqlByID checks ExternalInstance != nil && PostgresqlID == postgresqlID
				// So a PostgreSQL with nil ExternalInstance won't match and returns "not found"
				postgresql := &instancev1alpha1.Postgresql{
					ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
					Spec:       instancev1alpha1.PostgresqlSpec{ExternalInstance: nil},
					Status:     instancev1alpha1.PostgresqlStatus{Connected: true},
				}
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{*postgresql}
				})
			},
			expectNil:      true,
			expectedReason: "PostgresqlNotFound",
		},
		{
			name:         "Success with explicit port and SSL",
			postgresqlID: "test-id",
			setupMocks: func(mockClient *MockControllerClient) {
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{*postgresql}
				})
			},
			expectNil:    false,
			expectedPort: 5432,
			expectedSSL:  "require",
		},
		{
			name:         "Success with default port (0 -> 5432)",
			postgresqlID: "test-id",
			setupMocks: func(mockClient *MockControllerClient) {
				postgresql := &instancev1alpha1.Postgresql{
					ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
							Address:      "localhost",
							Port:         0,
							SSLMode:      "",
						},
					},
					Status: instancev1alpha1.PostgresqlStatus{Connected: true},
				}
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{*postgresql}
				})
			},
			expectNil:    false,
			expectedPort: 5432,
			expectedSSL:  "require",
		},
		{
			name:         "List error",
			postgresqlID: "test-id",
			setupMocks: func(mockClient *MockControllerClient) {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(assert.AnError)
			},
			expectNil:      true,
			expectedReason: "PostgresqlNotFound",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockControllerClient)
			tt.setupMocks(mockClient)

			cfg := &BaseReconcilerConfig{
				Client: mockClient,
				Log:    zap.NewNop().Sugar(),
			}

			info, reason, _ := cfg.resolvePostgresqlConnection(context.Background(), tt.postgresqlID)

			if tt.expectNil {
				assert.Nil(t, info)
				assert.Equal(t, tt.expectedReason, reason)
			} else {
				assert.NotNil(t, info)
				assert.Equal(t, tt.expectedPort, info.Port)
				assert.Equal(t, tt.expectedSSL, info.SSLMode)
				assert.Empty(t, reason)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestResolveAdminCredentials_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		vaultClient interface{} // nil or non-nil marker
		info        *connectionInfo
		expectError bool
	}{
		{
			name:        "Vault client is nil - should return nil error",
			vaultClient: nil,
			info: &connectionInfo{
				ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
					PostgresqlID: "test-id",
					Address:      "localhost",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &BaseReconcilerConfig{
				Log:                         zap.NewNop().Sugar(),
				VaultClient:                 nil,
				VaultAvailabilityRetries:    1,
				VaultAvailabilityRetryDelay: 1 * time.Millisecond,
			}

			err := cfg.resolveAdminCredentials(context.Background(), tt.info)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConnectionInfo_Structure(t *testing.T) {
	info := &connectionInfo{
		ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
			PostgresqlID: "test-id",
			Address:      "localhost",
			Port:         5432,
			SSLMode:      "require",
		},
		Port:     5432,
		SSLMode:  "require",
		Username: "admin",
		Password: "secret",
	}

	assert.Equal(t, "test-id", info.ExternalInstance.PostgresqlID)
	assert.Equal(t, "localhost", info.ExternalInstance.Address)
	assert.Equal(t, int32(5432), info.Port)
	assert.Equal(t, "require", info.SSLMode)
	assert.Equal(t, "admin", info.Username)
	assert.Equal(t, "secret", info.Password)
}
