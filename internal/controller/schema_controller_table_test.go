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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// createTestSchema creates a test Schema instance
func createTestSchema(name, namespace, postgresqlID, schemaName, owner string) *instancev1alpha1.Schema {
	return createTestSchemaWithDatabase(name, namespace, postgresqlID, schemaName, owner, "testdb")
}

// createTestSchemaWithDatabase creates a test Schema instance with a specific database
func createTestSchemaWithDatabase(name, namespace, postgresqlID, schemaName, owner, database string) *instancev1alpha1.Schema {
	return &instancev1alpha1.Schema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.SchemaSpec{
			PostgresqlID: postgresqlID,
			Schema:       schemaName,
			Owner:        owner,
			Database:     database,
		},
	}
}

func TestSchemaReconciler_Reconcile_TableDriven(t *testing.T) {
	tests := []struct {
		name                string
		request             ctrl.Request
		setupMocks          func(*MockControllerClient, *MockStatusWriter, *MockVaultClient)
		expectedResult      ctrl.Result
		expectedError       bool
		expectedErrorSubstr string
	}{
		{
			name: "Schema not found - should return no error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent-schema",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.NewNotFound(schema.GroupResource{}, "schema"))
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Get error - should return error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-schema",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("connection error"))
			},
			expectedResult:      ctrl.Result{},
			expectedError:       true,
			expectedErrorSubstr: "connection error",
		},
		{
			name: "PostgreSQL not found - should requeue",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-schema",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				schemaObj := createTestSchema("test-schema", "default", "non-existent-id", "myschema", "owner")
				postgresqlList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(schemaObj, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Schema)
					*obj = *schemaObj
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
					Name:      "test-schema",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				schemaObj := createTestSchema("test-schema", "default", "test-id", "myschema", "owner")
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, false)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(schemaObj, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Schema)
					*obj = *schemaObj
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
			name: "No external instance configuration - should requeue (PostgreSQL not found)",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-schema",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				schemaObj := createTestSchema("test-schema", "default", "test-id", "myschema", "owner")
				// PostgreSQL with nil ExternalInstance won't be found by FindPostgresqlByID
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
					Return(schemaObj, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Schema)
					*obj = *schemaObj
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
			name: "Valid schema with connected PostgreSQL - should succeed",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-schema",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				schemaObj := createTestSchema("test-schema", "default", "test-id", "myschema", "owner")
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(schemaObj, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Schema)
					*obj = *schemaObj
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
			name: "Status update error - should return error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-schema",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				schemaObj := createTestSchema("test-schema", "default", "test-id", "myschema", "owner")
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(schemaObj, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Schema)
					*obj = *schemaObj
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Status").Return(mockStatus)
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("status update error"))
			},
			expectedResult:      ctrl.Result{},
			expectedError:       true,
			expectedErrorSubstr: "status update error",
		},
		{
			name: "PostgreSQL with default port - should use default",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-schema",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter, mockVault *MockVaultClient) {
				schemaObj := createTestSchema("test-schema", "default", "test-id", "myschema", "owner")
				postgresql := &instancev1alpha1.Postgresql{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
							Address:      "localhost",
							Port:         0, // Default port
							SSLMode:      "",
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
					Return(schemaObj, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Schema)
					*obj = *schemaObj
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
			mockClient := new(MockControllerClient)
			mockStatusWriter := new(MockStatusWriter)
			mockVaultClient := new(MockVaultClient)

			tt.setupMocks(mockClient, mockStatusWriter, mockVaultClient)

			// Use a short timeout for tests to avoid long waits on retry logic
			baseCfg := getBaseReconcilerConfig(mockClient, mockVaultClient)
			// Override retry settings for faster tests
			baseCfg.PostgresqlConnectionRetries = 1
			baseCfg.PostgresqlConnectionTimeout = 10 * time.Millisecond

			reconciler := &SchemaReconciler{
				BaseReconcilerConfig: baseCfg,
			}

			// Use a context with timeout to prevent tests from hanging
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result, err := reconciler.Reconcile(ctx, tt.request)

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
