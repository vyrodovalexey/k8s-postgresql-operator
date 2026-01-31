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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestRoleGroupReconciler_Reconcile_Comprehensive(t *testing.T) {
	tests := []struct {
		name                string
		request             ctrl.Request
		setupMocks          func(*MockControllerClient, *MockStatusWriter)
		expectedResult      ctrl.Result
		expectedError       bool
		expectedErrorSubstr string
	}{
		{
			name: "RoleGroup not found - should return empty result",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rolegroup",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.NewNotFound(schema.GroupResource{}, "rolegroup"))
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Get RoleGroup error - should return error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rolegroup",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("internal error"))
			},
			expectedResult:      ctrl.Result{},
			expectedError:       true,
			expectedErrorSubstr: "internal error",
		},
		{
			name: "PostgreSQL not found - should requeue",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rolegroup",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				roleGroup := createTestRoleGroup("test-rolegroup", "default", "non-existent-id", "testgroup", []string{"member1"})
				postgresqlList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(roleGroup, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.RoleGroup)
					*obj = *roleGroup
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Status").Return(mockStatus)
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "PostgreSQL not connected - should requeue",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rolegroup",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				roleGroup := createTestRoleGroup("test-rolegroup", "default", "test-id", "testgroup", []string{"member1"})
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, false)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(roleGroup, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.RoleGroup)
					*obj = *roleGroup
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Status").Return(mockStatus)
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "No external instance - should requeue (PostgreSQL not found)",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rolegroup",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				roleGroup := createTestRoleGroup("test-rolegroup", "default", "test-id", "testgroup", []string{"member1"})
				postgresql := &instancev1alpha1.Postgresql{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: nil,
					},
					Status: instancev1alpha1.PostgresqlStatus{
						Connected: true,
					},
				}
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(roleGroup, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.RoleGroup)
					*obj = *roleGroup
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Status").Return(mockStatus)
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "Vault client not available - should return empty result",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rolegroup",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				roleGroup := createTestRoleGroup("test-rolegroup", "default", "test-id", "testgroup", []string{"member1"})
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(roleGroup, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.RoleGroup)
					*obj = *roleGroup
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Status").Return(mockStatus)
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Status update error on PostgreSQL not found - should still requeue",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rolegroup",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				roleGroup := createTestRoleGroup("test-rolegroup", "default", "non-existent-id", "testgroup", []string{"member1"})
				postgresqlList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(roleGroup, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.RoleGroup)
					*obj = *roleGroup
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Status").Return(mockStatus)
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(fmt.Errorf("status update error"))
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "List PostgreSQL error - should requeue",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rolegroup",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				roleGroup := createTestRoleGroup("test-rolegroup", "default", "test-id", "testgroup", []string{"member1"})

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(roleGroup, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.RoleGroup)
					*obj = *roleGroup
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(fmt.Errorf("list error"))
				mockClient.On("Status").Return(mockStatus)
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "Empty member roles - should succeed",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rolegroup",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				roleGroup := createTestRoleGroup("test-rolegroup", "default", "test-id", "testgroup", []string{})
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(roleGroup, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.RoleGroup)
					*obj = *roleGroup
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Status").Return(mockStatus)
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Multiple member roles - should succeed",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rolegroup",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				roleGroup := createTestRoleGroup("test-rolegroup", "default", "test-id", "testgroup", []string{"member1", "member2", "member3"})
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(roleGroup, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.RoleGroup)
					*obj = *roleGroup
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Status").Return(mockStatus)
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockClient := new(MockControllerClient)
			mockStatusWriter := new(MockStatusWriter)
			logger := zap.NewNop().Sugar()

			tt.setupMocks(mockClient, mockStatusWriter)

			reconciler := &RoleGroupReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:                      mockClient,
					VaultClient:                 nil, // No vault client for these tests
					Log:                         logger,
					PostgresqlConnectionRetries: 3,
					PostgresqlConnectionTimeout: 10 * time.Second,
					VaultAvailabilityRetries:    3,
					VaultAvailabilityRetryDelay: 10 * time.Second,
				},
			}

			// Act
			result, err := reconciler.Reconcile(context.Background(), tt.request)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
				if tt.expectedErrorSubstr != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorSubstr)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestRoleGroupReconciler_Reconcile_DefaultPortAndSSLMode(t *testing.T) {
	// Arrange
	mockClient := new(MockControllerClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	roleGroup := createTestRoleGroup("test-rolegroup", "default", "test-id", "testgroup", []string{"member1"})
	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pg1",
			Namespace: "default",
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "test-id",
				Address:      "localhost",
				Port:         0,  // Default port
				SSLMode:      "", // Default SSL mode
			},
		},
		Status: instancev1alpha1.PostgresqlStatus{
			Connected: true,
		},
	}
	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{*postgresql},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(roleGroup, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.RoleGroup)
		*obj = *roleGroup
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})
	mockClient.On("Status").Return(mockStatusWriter)
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &RoleGroupReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			VaultClient:                 nil,
			Log:                         logger,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-rolegroup",
			Namespace: "default",
		},
	}

	// Act
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
}
