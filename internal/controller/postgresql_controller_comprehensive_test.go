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

// TestPostgresqlReconciler_Reconcile_Comprehensive tests various reconciliation scenarios
func TestPostgresqlReconciler_Reconcile_Comprehensive(t *testing.T) {
	tests := []struct {
		name                string
		request             ctrl.Request
		setupMocks          func(*MockControllerClient, *MockStatusWriter)
		expectedResult      ctrl.Result
		expectedError       bool
		expectedErrorSubstr string
	}{
		{
			name: "PostgreSQL not found - should return no error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent-pg",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.NewNotFound(schema.GroupResource{}, "postgresql"))
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Get error - should return error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-pg",
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
			name: "PostgreSQL with no external instance - should return empty result",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-pg",
					Namespace: "default",
				},
			},
			setupMocks: func(mockClient *MockControllerClient, mockStatus *MockStatusWriter) {
				postgresql := &instancev1alpha1.Postgresql{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pg",
						Namespace: "default",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: nil,
					},
				}
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(postgresql, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Postgresql)
					*obj = *postgresql
				})
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

			reconciler := &PostgresqlReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:                      mockClient,
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
			mockStatusWriter.AssertExpectations(t)
		})
	}
}

// TestPostgresqlReconciler_Reconcile_ExternalInstance tests external instance scenarios
func TestPostgresqlReconciler_Reconcile_ExternalInstance(t *testing.T) {
	t.Run("External instance with default port and SSL mode", func(t *testing.T) {
		// This test verifies that the reconciler handles external instances
		// but requires actual PostgreSQL connection for full testing
		t.Skip("Skipping test that requires real PostgreSQL connection - use integration tests instead")
	})
}

// TestPostgresqlReconciler_ReconcileExternalInstance_NilVaultClient tests reconciliation without Vault
func TestPostgresqlReconciler_ReconcileExternalInstance_NilVaultClient(t *testing.T) {
	t.Run("External instance without Vault client", func(t *testing.T) {
		// This test verifies that the reconciler handles external instances without Vault
		// but requires actual PostgreSQL connection for full testing
		t.Skip("Skipping test that requires real PostgreSQL connection - use integration tests instead")
	})
}

// TestPostgresqlReconciler_LogConnectionResult tests the logConnectionResult method
func TestPostgresqlReconciler_LogConnectionResult(t *testing.T) {
	// Note: logConnectionResult is a private method that logs and records events
	// It's tested indirectly through the Reconcile method
	// Direct testing would require exposing the method or using reflection
	t.Skip("logConnectionResult is tested indirectly through Reconcile")
}

// TestPostgresqlReconciler_Reconcile_MultipleScenarios tests multiple scenarios in sequence
func TestPostgresqlReconciler_Reconcile_MultipleScenarios(t *testing.T) {
	tests := []struct {
		name           string
		postgresql     *instancev1alpha1.Postgresql
		expectedResult ctrl.Result
		expectedError  bool
	}{
		{
			name: "PostgreSQL with empty spec",
			postgresql: &instancev1alpha1.Postgresql{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-spec-pg",
					Namespace: "default",
				},
				Spec: instancev1alpha1.PostgresqlSpec{},
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "PostgreSQL with nil external instance",
			postgresql: &instancev1alpha1.Postgresql{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nil-external-pg",
					Namespace: "default",
				},
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: nil,
				},
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockClient := new(MockControllerClient)
			logger := zap.NewNop().Sugar()

			mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(tt.postgresql, nil).Run(func(args mock.Arguments) {
				obj := args.Get(2).(*instancev1alpha1.Postgresql)
				*obj = *tt.postgresql
			})

			reconciler := &PostgresqlReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:                      mockClient,
					Log:                         logger,
					PostgresqlConnectionRetries: 3,
					PostgresqlConnectionTimeout: 10 * time.Second,
					VaultAvailabilityRetries:    3,
					VaultAvailabilityRetryDelay: 10 * time.Second,
				},
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.postgresql.Name,
					Namespace: tt.postgresql.Namespace,
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

// TestPostgresqlReconciler_Reconcile_ErrorCases tests various error scenarios
func TestPostgresqlReconciler_Reconcile_ErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*MockControllerClient)
		expectedError bool
	}{
		{
			name: "Internal server error",
			setupMocks: func(mockClient *MockControllerClient) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.NewInternalError(fmt.Errorf("internal error")))
			},
			expectedError: true,
		},
		{
			name: "Forbidden error",
			setupMocks: func(mockClient *MockControllerClient) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.NewForbidden(schema.GroupResource{}, "test", fmt.Errorf("forbidden")))
			},
			expectedError: true,
		},
		{
			name: "Timeout error",
			setupMocks: func(mockClient *MockControllerClient) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.NewServerTimeout(schema.GroupResource{}, "test", 1))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockClient := new(MockControllerClient)
			logger := zap.NewNop().Sugar()

			tt.setupMocks(mockClient)

			reconciler := &PostgresqlReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:                      mockClient,
					Log:                         logger,
					PostgresqlConnectionRetries: 3,
					PostgresqlConnectionTimeout: 10 * time.Second,
					VaultAvailabilityRetries:    3,
					VaultAvailabilityRetryDelay: 10 * time.Second,
				},
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-pg",
					Namespace: "default",
				},
			}

			// Act
			_, err := reconciler.Reconcile(context.Background(), req)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockClient.AssertExpectations(t)
		})
	}
}
