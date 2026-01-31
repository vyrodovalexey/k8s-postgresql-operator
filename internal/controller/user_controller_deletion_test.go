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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestUserReconciler_handleDeletion_TableDriven(t *testing.T) {
	tests := []struct {
		name                string
		user                *instancev1alpha1.User
		setupMocks          func(*MockControllerClient, *MockStatusWriter)
		expectedResult      ctrl.Result
		expectedError       bool
		expectedErrorSubstr string
	}{
		{
			name: "DeleteFromCRD false - should remove finalizer and return",
			user: func() *instancev1alpha1.User {
				now := metav1.Now()
				user := createTestUser("test-user", "default", "test-id", "testuser", false)
				user.DeletionTimestamp = &now
				user.Finalizers = []string{userFinalizerName}
				user.Spec.DeleteFromCRD = false
				return user
			}(),
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "DeleteFromCRD false without finalizer - should return immediately",
			user: func() *instancev1alpha1.User {
				now := metav1.Now()
				user := createTestUser("test-user", "default", "test-id", "testuser", false)
				user.DeletionTimestamp = &now
				user.Finalizers = []string{}
				user.Spec.DeleteFromCRD = false
				return user
			}(),
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				// No mocks needed - should return immediately
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "DeleteFromCRD true without finalizer - should return immediately",
			user: func() *instancev1alpha1.User {
				now := metav1.Now()
				user := createTestUser("test-user", "default", "test-id", "testuser", false)
				user.DeletionTimestamp = &now
				user.Finalizers = []string{}
				user.Spec.DeleteFromCRD = true
				return user
			}(),
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				// No mocks needed - should return immediately
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "DeleteFromCRD true - PostgreSQL not found - should remove finalizer",
			user: func() *instancev1alpha1.User {
				now := metav1.Now()
				user := createTestUser("test-user", "default", "non-existent-id", "testuser", false)
				user.DeletionTimestamp = &now
				user.Finalizers = []string{userFinalizerName}
				user.Spec.DeleteFromCRD = true
				return user
			}(),
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				postgresqlList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}
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
			name: "DeleteFromCRD true - PostgreSQL not connected - should remove finalizer",
			user: func() *instancev1alpha1.User {
				now := metav1.Now()
				user := createTestUser("test-user", "default", "test-id", "testuser", false)
				user.DeletionTimestamp = &now
				user.Finalizers = []string{userFinalizerName}
				user.Spec.DeleteFromCRD = true
				return user
			}(),
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, false)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}
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
			name: "DeleteFromCRD true - No external instance - should remove finalizer",
			user: func() *instancev1alpha1.User {
				now := metav1.Now()
				user := createTestUser("test-user", "default", "test-id", "testuser", false)
				user.DeletionTimestamp = &now
				user.Finalizers = []string{userFinalizerName}
				user.Spec.DeleteFromCRD = true
				return user
			}(),
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
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
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				// PostgreSQL with nil ExternalInstance won't be found by FindPostgresqlByID
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "DeleteFromCRD true - Vault client nil - should remove finalizer",
			user: func() *instancev1alpha1.User {
				now := metav1.Now()
				user := createTestUser("test-user", "default", "test-id", "testuser", false)
				user.DeletionTimestamp = &now
				user.Finalizers = []string{userFinalizerName}
				user.Spec.DeleteFromCRD = true
				return user
			}(),
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}
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
			name: "Update finalizer error - should return error",
			user: func() *instancev1alpha1.User {
				now := metav1.Now()
				user := createTestUser("test-user", "default", "test-id", "testuser", false)
				user.DeletionTimestamp = &now
				user.Finalizers = []string{userFinalizerName}
				user.Spec.DeleteFromCRD = false
				return user
			}(),
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(fmt.Errorf("update error"))
			},
			expectedResult:      ctrl.Result{},
			expectedError:       true,
			expectedErrorSubstr: "update error",
		},
		{
			name: "List PostgreSQL error - should remove finalizer anyway",
			user: func() *instancev1alpha1.User {
				now := metav1.Now()
				user := createTestUser("test-user", "default", "test-id", "testuser", false)
				user.DeletionTimestamp = &now
				user.Finalizers = []string{userFinalizerName}
				user.Spec.DeleteFromCRD = true
				return user
			}(),
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(fmt.Errorf("list error"))
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
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

			reconciler := &UserReconciler{
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
			result, err := reconciler.handleDeletion(context.Background(), tt.user)

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

func TestUserReconciler_Reconcile_DeletionTimestamp(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*MockControllerClient, *MockStatusWriter)
		expectedResult ctrl.Result
		expectedError  bool
	}{
		{
			name: "User with deletion timestamp - DeleteFromCRD false",
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				user := createTestUser("test-user", "default", "test-id", "testuser", false)
				user.DeletionTimestamp = &now
				user.Finalizers = []string{userFinalizerName}
				user.Spec.DeleteFromCRD = false

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
				})
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "User with deletion timestamp - DeleteFromCRD true, PostgreSQL not found",
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				user := createTestUser("test-user", "default", "non-existent-id", "testuser", false)
				user.DeletionTimestamp = &now
				user.Finalizers = []string{userFinalizerName}
				user.Spec.DeleteFromCRD = true

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
				})
				postgresqlList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockClient := new(MockControllerClient)
			mockStatusWriter := new(MockStatusWriter)
			logger := zap.NewNop().Sugar()

			tt.setupMocks(mockClient, mockStatusWriter)

			reconciler := &UserReconciler{
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
					Name:      "test-user",
					Namespace: "default",
				},
			}

			// Act
			result, err := reconciler.Reconcile(context.Background(), req)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestUserReconciler_Reconcile_AddFinalizer(t *testing.T) {
	// Arrange
	mockClient := new(MockControllerClient)
	logger := zap.NewNop().Sugar()

	user := createTestUser("test-user", "default", "test-id", "testuser", false)
	user.Finalizers = []string{} // No finalizer yet
	user.Spec.DeleteFromCRD = true

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(user, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.User)
		*obj = *user
	})
	mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &UserReconciler{
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
			Name:      "test-user",
			Namespace: "default",
		},
	}

	// Act
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{Requeue: true}, result)
	mockClient.AssertExpectations(t)
}

func TestUserReconciler_Reconcile_AddFinalizerError(t *testing.T) {
	// Arrange
	mockClient := new(MockControllerClient)
	logger := zap.NewNop().Sugar()

	user := createTestUser("test-user", "default", "test-id", "testuser", false)
	user.Finalizers = []string{} // No finalizer yet
	user.Spec.DeleteFromCRD = true

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(user, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.User)
		*obj = *user
	})
	mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("update error"))

	reconciler := &UserReconciler{
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
			Name:      "test-user",
			Namespace: "default",
		},
	}

	// Act
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update error")
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
}

func TestUserReconciler_handleDeletion_MultipleFinalizerScenarios(t *testing.T) {
	tests := []struct {
		name           string
		finalizers     []string
		deleteFromCRD  bool
		expectedUpdate bool
	}{
		{
			name:           "Only our finalizer present",
			finalizers:     []string{userFinalizerName},
			deleteFromCRD:  false,
			expectedUpdate: true,
		},
		{
			name:           "Multiple finalizers including ours",
			finalizers:     []string{"other-finalizer", userFinalizerName, "another-finalizer"},
			deleteFromCRD:  false,
			expectedUpdate: true,
		},
		{
			name:           "No finalizers",
			finalizers:     []string{},
			deleteFromCRD:  false,
			expectedUpdate: false,
		},
		{
			name:           "Only other finalizers",
			finalizers:     []string{"other-finalizer"},
			deleteFromCRD:  false,
			expectedUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockClient := new(MockControllerClient)
			logger := zap.NewNop().Sugar()

			now := metav1.Now()
			user := createTestUser("test-user", "default", "test-id", "testuser", false)
			user.DeletionTimestamp = &now
			user.Finalizers = tt.finalizers
			user.Spec.DeleteFromCRD = tt.deleteFromCRD

			if tt.expectedUpdate {
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			}

			reconciler := &UserReconciler{
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

			// Act
			result, err := reconciler.handleDeletion(context.Background(), user)

			// Assert
			assert.NoError(t, err)
			assert.Equal(t, ctrl.Result{}, result)
			mockClient.AssertExpectations(t)
		})
	}
}
