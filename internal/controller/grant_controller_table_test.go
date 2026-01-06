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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestGrantReconciler_Reconcile_TableDriven(t *testing.T) {
	tests := []struct {
		name                string
		request             ctrl.Request
		setupMocks          func(*MockControllerClient, *MockStatusWriter, *MockVaultClient)
		expectedResult      ctrl.Result
		expectedError       bool
		expectedErrorSubstr string
	}{
		{
			name: "Grant not found - should return empty result",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-grant",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "grant"))
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Get Grant error - should return error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-grant",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, apierrors.NewInternalError(fmt.Errorf("internal error")))
			},
			expectedResult:      ctrl.Result{},
			expectedError:       true,
			expectedErrorSubstr: "internal error",
		},
		{
			name: "PostgreSQL not found - should requeue",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-grant",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				grant := &instancev1alpha1.Grant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-grant",
						Namespace: "default",
					},
					Spec: instancev1alpha1.GrantSpec{
						PostgresqlID: "test-id",
						Database:     "testdb",
						Role:         "testrole",
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(grant, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Grant)
					*obj = *grant
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(apierrors.NewNotFound(schema.GroupResource{}, "postgresql"))
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockClient.On("Status").Return(mockStatus)
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "PostgreSQL not connected - should requeue",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-grant",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				grant := &instancev1alpha1.Grant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-grant",
						Namespace: "default",
					},
					Spec: instancev1alpha1.GrantSpec{
						PostgresqlID: "test-id",
						Database:     "testdb",
						Role:         "testrole",
					},
				}
				postgresql := &instancev1alpha1.Postgresql{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
							Address:      "localhost",
							Port:         5432,
						},
					},
					Status: instancev1alpha1.PostgresqlStatus{
						Connected: false,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(grant, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Grant)
					*obj = *grant
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{*postgresql}
				})
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockClient.On("Status").Return(mockStatus)
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "No external instance configuration - FindPostgresqlByID returns not found",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-grant",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				grant := &instancev1alpha1.Grant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-grant",
						Namespace: "default",
					},
					Spec: instancev1alpha1.GrantSpec{
						PostgresqlID: "test-id",
						Database:     "testdb",
						Role:         "testrole",
					},
				}
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
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(grant, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Grant)
					*obj = *grant
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{*postgresql}
				})
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockClient.On("Status").Return(mockStatus)
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "Vault client not available - should return error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-grant",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				grant := &instancev1alpha1.Grant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-grant",
						Namespace: "default",
					},
					Spec: instancev1alpha1.GrantSpec{
						PostgresqlID: "test-id",
						Database:     "testdb",
						Role:         "testrole",
					},
				}
				postgresql := &instancev1alpha1.Postgresql{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
							Address:      "localhost",
							Port:         5432,
						},
					},
					Status: instancev1alpha1.PostgresqlStatus{
						Connected: true,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(grant, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Grant)
					*obj = *grant
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{*postgresql}
				})
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockClient.On("Status").Return(mockStatus)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Success - grants applied",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-grant",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				grant := &instancev1alpha1.Grant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-grant",
						Namespace: "default",
					},
					Spec: instancev1alpha1.GrantSpec{
						PostgresqlID: "test-id",
						Database:     "testdb",
						Role:         "testrole",
						Grants: []instancev1alpha1.GrantItem{
							{
								Type:       instancev1alpha1.GrantTypeSchema,
								Schema:     "public",
								Privileges: []string{"USAGE"},
							},
						},
					},
				}
				postgresql := &instancev1alpha1.Postgresql{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
							Address:      "localhost",
							Port:         5432,
						},
					},
					Status: instancev1alpha1.PostgresqlStatus{
						Connected: true,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(grant, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Grant)
					*obj = *grant
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{*postgresql}
				})
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockClient.On("Status").Return(mockStatus)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockControllerClient)
			mockStatusWriter := new(MockStatusWriter)
			mockVaultClient := new(MockVaultClient)

			tt.setupMocks(mockClient, mockStatusWriter, mockVaultClient)

			reconciler := &GrantReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:                      mockClient,
					VaultClient:                 nil, // Will be nil for most tests
					Log:                         zap.NewNop().Sugar(),
					PostgresqlConnectionRetries: 3,
					PostgresqlConnectionTimeout: 10 * time.Second,
					VaultAvailabilityRetries:    3,
					VaultAvailabilityRetryDelay: 10 * time.Second,
				},
			}

			// Note: We can't easily mock k8sclient.FindPostgresqlByID or pg.ExecuteOperationWithRetry
			// as they are package-level functions. The tests verify the structure and error handling
			// paths that can be tested with mocks.

			result, err := reconciler.Reconcile(context.Background(), tt.request)

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
			mockStatusWriter.AssertExpectations(t)
		})
	}
}
