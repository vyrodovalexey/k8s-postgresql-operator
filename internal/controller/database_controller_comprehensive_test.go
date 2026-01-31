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

// TestDatabaseReconciler_Reconcile_Comprehensive tests various reconciliation scenarios
func TestDatabaseReconciler_Reconcile_Comprehensive(t *testing.T) {
	tests := []struct {
		name                string
		request             ctrl.Request
		setupMocks          func(*MockControllerClient, *MockStatusWriter)
		expectedResult      ctrl.Result
		expectedError       bool
		expectedErrorSubstr string
	}{
		{
			name: "Database not found - should return no error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.NewNotFound(schema.GroupResource{}, "database"))
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Get error - should return error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("connection error"))
			},
			expectedResult:      ctrl.Result{},
			expectedError:       true,
			expectedErrorSubstr: "connection error",
		},
		{
			name: "Database with deletion timestamp and no finalizer - should return immediately",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				database := &instancev1alpha1.Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-db",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{},
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID:  "test-id",
						Database:      "testdb",
						DeleteFromCRD: false,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
				})
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Database with DeleteFromCRD true but no finalizer - should add finalizer",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				database := &instancev1alpha1.Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "test-db",
						Namespace:  "default",
						Finalizers: []string{},
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID:  "test-id",
						Database:      "testdb",
						DeleteFromCRD: true,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
				})
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{Requeue: true},
			expectedError:  false,
		},
		{
			name: "Database with DeleteFromCRD true - finalizer add fails",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				database := &instancev1alpha1.Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "test-db",
						Namespace:  "default",
						Finalizers: []string{},
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID:  "test-id",
						Database:      "testdb",
						DeleteFromCRD: true,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
				})
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(fmt.Errorf("update failed"))
			},
			expectedResult:      ctrl.Result{},
			expectedError:       true,
			expectedErrorSubstr: "update failed",
		},
		{
			name: "PostgreSQL not found - should requeue",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				database := createTestDatabase("test-db", "default", "non-existent-id", "testdb", "owner", false)
				postgresqlList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
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
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				database := createTestDatabase("test-db", "default", "test-id", "testdb", "owner", false)
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, false)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
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
			name: "PostgreSQL has no external instance - should requeue",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				database := createTestDatabase("test-db", "default", "test-id", "testdb", "owner", false)
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
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
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
			name: "Status update fails after PostgreSQL not found",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				database := createTestDatabase("test-db", "default", "non-existent-id", "testdb", "owner", false)
				postgresqlList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Status").Return(mockStatus)
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(fmt.Errorf("status update failed"))
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "List PostgreSQL error - should requeue",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				database := createTestDatabase("test-db", "default", "test-id", "testdb", "owner", false)

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(fmt.Errorf("list error"))
				mockClient.On("Status").Return(mockStatus)
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockClient := new(MockControllerClient)
			mockStatusWriter := new(MockStatusWriter)

			tt.setupMocks(mockClient, mockStatusWriter)

			reconciler := &DatabaseReconciler{
				BaseReconcilerConfig: getBaseReconcilerConfig(mockClient, nil),
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
			mockStatusWriter.AssertExpectations(t)
		})
	}
}

// TestDatabaseReconciler_HandleDeletion_Comprehensive tests deletion handling scenarios
func TestDatabaseReconciler_HandleDeletion_Comprehensive(t *testing.T) {
	tests := []struct {
		name                string
		request             ctrl.Request
		setupMocks          func(*MockControllerClient, *MockStatusWriter)
		expectedResult      ctrl.Result
		expectedError       bool
		expectedErrorSubstr string
	}{
		{
			name: "Deletion with DeleteFromCRD false and finalizer - should remove finalizer",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				database := &instancev1alpha1.Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-db",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{databaseFinalizerName},
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID:  "test-id",
						Database:      "testdb",
						DeleteFromCRD: false,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
				})
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Deletion with DeleteFromCRD false and no finalizer - should return immediately",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				database := &instancev1alpha1.Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-db",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{},
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID:  "test-id",
						Database:      "testdb",
						DeleteFromCRD: false,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
				})
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Deletion with DeleteFromCRD true but no finalizer - should return immediately",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				database := &instancev1alpha1.Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-db",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{},
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID:  "test-id",
						Database:      "testdb",
						DeleteFromCRD: true,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
				})
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Deletion with PostgreSQL not found - should remove finalizer",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				database := &instancev1alpha1.Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-db",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{databaseFinalizerName},
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID:  "non-existent-id",
						Database:      "testdb",
						DeleteFromCRD: true,
					},
				}
				postgresqlList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Deletion with PostgreSQL not connected - should remove finalizer",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				database := &instancev1alpha1.Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-db",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{databaseFinalizerName},
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID:  "test-id",
						Database:      "testdb",
						DeleteFromCRD: true,
					},
				}
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, false)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Deletion with no external instance - should remove finalizer",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				database := &instancev1alpha1.Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-db",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{databaseFinalizerName},
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID:  "test-id",
						Database:      "testdb",
						DeleteFromCRD: true,
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
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Deletion with nil Vault client - should remove finalizer",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				database := &instancev1alpha1.Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-db",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{databaseFinalizerName},
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID:  "test-id",
						Database:      "testdb",
						DeleteFromCRD: true,
					},
				}
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Deletion - finalizer removal fails",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-db",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				database := &instancev1alpha1.Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-db",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{databaseFinalizerName},
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID:  "test-id",
						Database:      "testdb",
						DeleteFromCRD: false,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(database, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Database)
					*obj = *database
				})
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(fmt.Errorf("update failed"))
			},
			expectedResult:      ctrl.Result{},
			expectedError:       true,
			expectedErrorSubstr: "update failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockClient := new(MockControllerClient)
			mockStatusWriter := new(MockStatusWriter)

			tt.setupMocks(mockClient, mockStatusWriter)

			reconciler := &DatabaseReconciler{
				BaseReconcilerConfig: getBaseReconcilerConfig(mockClient, nil),
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
			mockStatusWriter.AssertExpectations(t)
		})
	}
}
