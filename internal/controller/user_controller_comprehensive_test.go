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

// TestGenerateRandomPassword_Comprehensive tests the password generation function comprehensively
func TestGenerateRandomPassword_Comprehensive(t *testing.T) {
	tests := []struct {
		name           string
		length         int
		expectError    bool
		minLength      int
		checkUniquness bool
	}{
		{
			name:           "Generate password with default length",
			length:         32,
			expectError:    false,
			minLength:      32,
			checkUniquness: true,
		},
		{
			name:           "Generate password with short length",
			length:         8,
			expectError:    false,
			minLength:      8,
			checkUniquness: true,
		},
		{
			name:           "Generate password with long length",
			length:         64,
			expectError:    false,
			minLength:      64,
			checkUniquness: true,
		},
		{
			name:           "Generate password with minimum length",
			length:         1,
			expectError:    false,
			minLength:      1,
			checkUniquness: false, // Single char passwords may repeat
		},
		{
			name:        "Generate password with zero length",
			length:      0,
			expectError: false,
			minLength:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			password, err := generateRandomPassword(tt.length)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, password, tt.length)
			}

			// Check uniqueness by generating multiple passwords
			if tt.checkUniquness && tt.length > 0 {
				passwords := make(map[string]bool)
				for i := 0; i < 10; i++ {
					p, err := generateRandomPassword(tt.length)
					assert.NoError(t, err)
					passwords[p] = true
				}
				// With sufficient length, all passwords should be unique
				if tt.length >= 8 {
					assert.Greater(t, len(passwords), 1, "Passwords should be unique")
				}
			}
		})
	}
}

// TestGenerateRandomPassword_CharacterSet tests that passwords contain expected character types
func TestGenerateRandomPassword_CharacterSet(t *testing.T) {
	// Generate a long password to ensure all character types are likely present
	password, err := generateRandomPassword(100)
	assert.NoError(t, err)

	hasLower := false
	hasUpper := false
	hasDigit := false
	hasSpecial := false

	for _, c := range password {
		switch {
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= '0' && c <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}

	// With 100 characters, we should have all types
	assert.True(t, hasLower, "Password should contain lowercase letters")
	assert.True(t, hasUpper, "Password should contain uppercase letters")
	assert.True(t, hasDigit, "Password should contain digits")
	assert.True(t, hasSpecial, "Password should contain special characters")
}

// TestUserReconciler_Reconcile_Comprehensive tests various reconciliation scenarios
func TestUserReconciler_Reconcile_Comprehensive(t *testing.T) {
	tests := []struct {
		name                string
		request             ctrl.Request
		setupMocks          func(*MockControllerClient, *MockStatusWriter)
		expectedResult      ctrl.Result
		expectedError       bool
		expectedErrorSubstr string
	}{
		{
			name: "User not found - should return no error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.NewNotFound(schema.GroupResource{}, "user"))
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Get error - should return error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-user",
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
			name: "User with deletion timestamp and no finalizer - should return immediately",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				user := &instancev1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-user",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{},
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID:  "test-id",
						Username:      "testuser",
						DeleteFromCRD: false,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
				})
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "User with DeleteFromCRD true but no finalizer - should add finalizer",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				user := &instancev1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "test-user",
						Namespace:  "default",
						Finalizers: []string{},
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID:  "test-id",
						Username:      "testuser",
						DeleteFromCRD: true,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
				})
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{Requeue: true},
			expectedError:  false,
		},
		{
			name: "User with DeleteFromCRD true - finalizer add fails",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				user := &instancev1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "test-user",
						Namespace:  "default",
						Finalizers: []string{},
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID:  "test-id",
						Username:      "testuser",
						DeleteFromCRD: true,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
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
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				user := createTestUser("test-user", "default", "non-existent-id", "testuser", false)
				postgresqlList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
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
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				user := createTestUser("test-user", "default", "test-id", "testuser", false)
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, false)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
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
			name: "PostgreSQL has no external instance - should requeue (not found by FindPostgresqlByID)",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				user := createTestUser("test-user", "default", "test-id", "testuser", false)
				// When PostgreSQL has nil ExternalInstance, FindPostgresqlByID won't find it
				// because it checks ExternalInstance != nil first
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
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Status").Return(mockStatus)
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			// FindPostgresqlByID returns "not found" when ExternalInstance is nil
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "Status update fails after PostgreSQL not found",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				user := createTestUser("test-user", "default", "non-existent-id", "testuser", false)
				postgresqlList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
				})
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
				mockClient.On("Status").Return(mockStatus)
				// Status update fails but we still requeue
				mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(fmt.Errorf("status update failed"))
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

			reconciler := &UserReconciler{
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

// TestUserReconciler_HandleDeletion_Comprehensive tests deletion handling scenarios
func TestUserReconciler_HandleDeletion_Comprehensive(t *testing.T) {
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
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				user := &instancev1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-user",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{userFinalizerName},
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID:  "test-id",
						Username:      "testuser",
						DeleteFromCRD: false,
					},
				}
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
			name: "Deletion with DeleteFromCRD false and no finalizer - should return immediately",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				user := &instancev1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-user",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{},
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID:  "test-id",
						Username:      "testuser",
						DeleteFromCRD: false,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
				})
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Deletion with DeleteFromCRD true but no finalizer - should return immediately",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				user := &instancev1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-user",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{},
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID:  "test-id",
						Username:      "testuser",
						DeleteFromCRD: true,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
				})
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Deletion with PostgreSQL not found - should remove finalizer",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				user := &instancev1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-user",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{userFinalizerName},
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID:  "non-existent-id",
						Username:      "testuser",
						DeleteFromCRD: true,
					},
				}
				postgresqlList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
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
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				user := &instancev1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-user",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{userFinalizerName},
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID:  "test-id",
						Username:      "testuser",
						DeleteFromCRD: true,
					},
				}
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, false)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
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
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				user := &instancev1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-user",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{userFinalizerName},
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID:  "test-id",
						Username:      "testuser",
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
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
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
					Name:      "test-user",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				now := metav1.Now()
				user := &instancev1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-user",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{userFinalizerName},
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID:  "test-id",
						Username:      "testuser",
						DeleteFromCRD: true,
					},
				}
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{*postgresql},
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(user, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.User)
					*obj = *user
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockClient := new(MockControllerClient)
			mockStatusWriter := new(MockStatusWriter)

			tt.setupMocks(mockClient, mockStatusWriter)

			reconciler := &UserReconciler{
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
