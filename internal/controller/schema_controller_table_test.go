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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestSchemaReconciler_Reconcile_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		request        ctrl.Request
		setupMocks     func(*MockControllerClient, *MockStatusWriter)
		expectedResult ctrl.Result
		expectedError  bool
	}{
		{
			name: "Schema not found - should return empty result",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "s1", Namespace: "default"},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				mc.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "schema"))
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Get error - should return error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "s1", Namespace: "default"},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				mc.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("get error"))
			},
			expectedResult: ctrl.Result{},
			expectedError:  true,
		},
		{
			name: "PostgreSQL not found - should requeue",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "s1", Namespace: "default"},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				s := &instancev1alpha1.Schema{
					ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "default"},
					Spec: instancev1alpha1.SchemaSpec{
						PostgresqlID: "test-id",
						Schema:       "myschema",
						Owner:        "owner",
					},
				}
				mc.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(s, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Schema)
					*obj = *s
				})
				mc.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{}
				})
				mc.On("Status").Return(ms)
				ms.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "PostgreSQL not connected - should requeue",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "s1", Namespace: "default"},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				s := &instancev1alpha1.Schema{
					ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "default"},
					Spec: instancev1alpha1.SchemaSpec{
						PostgresqlID: "test-id",
						Schema:       "myschema",
						Owner:        "owner",
					},
				}
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, false)
				mc.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(s, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Schema)
					*obj = *s
				})
				mc.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{*postgresql}
				})
				mc.On("Status").Return(ms)
				ms.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "PostgreSQL connected, no vault - syncSchema called",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "s1", Namespace: "default"},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				s := &instancev1alpha1.Schema{
					ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "default"},
					Spec: instancev1alpha1.SchemaSpec{
						PostgresqlID: "test-id",
						Schema:       "myschema",
						Owner:        "owner",
					},
				}
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
				mc.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(s, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Schema)
					*obj = *s
				})
				mc.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{*postgresql}
				})
				mc.On("Status").Return(ms)
				ms.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Status update fails on PG not found",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "s1", Namespace: "default"},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				s := &instancev1alpha1.Schema{
					ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "default"},
					Spec: instancev1alpha1.SchemaSpec{
						PostgresqlID: "test-id",
						Schema:       "myschema",
						Owner:        "owner",
					},
				}
				mc.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(s, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Schema)
					*obj = *s
				})
				mc.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{}
				})
				mc.On("Status").Return(ms)
				ms.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("status update error"))
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockControllerClient)
			mockStatus := new(MockStatusWriter)
			tt.setupMocks(mockClient, mockStatus)

			reconciler := &SchemaReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:                      mockClient,
					VaultClient:                 nil,
					Log:                         zap.NewNop().Sugar(),
					Recorder:                    record.NewFakeRecorder(100),
					PostgresqlConnectionRetries: 1,
					PostgresqlConnectionTimeout: 1 * time.Millisecond,
					VaultAvailabilityRetries:    1,
					VaultAvailabilityRetryDelay: 1 * time.Millisecond,
				},
			}

			result, err := reconciler.Reconcile(context.Background(), tt.request)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
			mockClient.AssertExpectations(t)
			mockStatus.AssertExpectations(t)
		})
	}
}

func TestSchemaReconciler_SyncSchema_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		schema          *instancev1alpha1.Schema
		info            *connectionInfo
		statusUpdateErr error
		expectedError   bool
	}{
		{
			name: "Sync schema - pg operation fails (no real DB), status update succeeds",
			schema: &instancev1alpha1.Schema{
				ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "default"},
				Spec: instancev1alpha1.SchemaSpec{
					PostgresqlID: "test-id",
					Schema:       "myschema",
					Owner:        "owner",
				},
			},
			info: &connectionInfo{
				ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
					PostgresqlID: "test-id",
					Address:      "localhost",
				},
				Port:     5432,
				SSLMode:  "require",
				Username: "admin",
				Password: "secret",
			},
			statusUpdateErr: nil,
			expectedError:   false,
		},
		{
			name: "Sync schema - status update fails",
			schema: &instancev1alpha1.Schema{
				ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "default"},
				Spec: instancev1alpha1.SchemaSpec{
					PostgresqlID: "test-id",
					Schema:       "myschema",
					Owner:        "owner",
				},
			},
			info: &connectionInfo{
				ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
					PostgresqlID: "test-id",
					Address:      "localhost",
				},
				Port:     5432,
				SSLMode:  "require",
				Username: "admin",
				Password: "secret",
			},
			statusUpdateErr: fmt.Errorf("status update error"),
			expectedError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockControllerClient)
			mockStatus := new(MockStatusWriter)

			mockClient.On("Status").Return(mockStatus)
			mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(tt.statusUpdateErr)

			reconciler := &SchemaReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:                      mockClient,
					Log:                         zap.NewNop().Sugar(),
					Recorder:                    record.NewFakeRecorder(100),
					PostgresqlConnectionRetries: 1,
					PostgresqlConnectionTimeout: 1 * time.Millisecond,
				},
			}

			_, err := reconciler.syncSchema(context.Background(), tt.schema, tt.info)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NotNil(t, tt.schema.Status.LastSyncAttempt)
			mockClient.AssertExpectations(t)
			mockStatus.AssertExpectations(t)
		})
	}
}
