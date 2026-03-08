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

func TestPostgresqlReconciler_Reconcile_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		request        ctrl.Request
		setupMocks     func(*MockControllerClient, *MockStatusWriter)
		expectedResult ctrl.Result
		expectedError  bool
	}{
		{
			name: "Not found - should return empty result",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "pg1", Namespace: "default"},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				mc.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "postgresql"))
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Get error - should return error",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "pg1", Namespace: "default"},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				mc.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("get error"))
			},
			expectedResult: ctrl.Result{},
			expectedError:  true,
		},
		{
			name: "No external instance - should return empty result",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "pg1", Namespace: "default"},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				pg := &instancev1alpha1.Postgresql{
					ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
					Spec:       instancev1alpha1.PostgresqlSpec{ExternalInstance: nil},
				}
				mc.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(pg, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Postgresql)
					*obj = *pg
				})
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "External instance with default port - should requeue",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "pg1", Namespace: "default"},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				pg := &instancev1alpha1.Postgresql{
					ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
							Address:      "localhost",
							Port:         0,  // default port
							SSLMode:      "", // default SSL
						},
					},
					Status: instancev1alpha1.PostgresqlStatus{Connected: false},
				}
				mc.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(pg, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Postgresql)
					*obj = *pg
				})
				mc.On("Status").Return(ms)
				ms.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "External instance - status update fails",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "pg1", Namespace: "default"},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				pg := &instancev1alpha1.Postgresql{
					ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
							Address:      "localhost",
							Port:         5432,
							SSLMode:      "require",
						},
					},
					Status: instancev1alpha1.PostgresqlStatus{Connected: false},
				}
				mc.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(pg, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Postgresql)
					*obj = *pg
				})
				mc.On("Status").Return(ms)
				ms.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("status update error"))
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:  false,
		},
		{
			name: "External instance - connection fails with error, status needs update",
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "pg1", Namespace: "default"},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				pg := &instancev1alpha1.Postgresql{
					ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
							Address:      "localhost",
							Port:         5432,
							SSLMode:      "require",
						},
					},
					Status: instancev1alpha1.PostgresqlStatus{
						Connected:         false,
						ConnectionAddress: "localhost:5432",
						Conditions: []metav1.Condition{
							{
								Type:    "Ready",
								Status:  metav1.ConditionFalse,
								Reason:  "ConnectionFailed",
								Message: "old message that will change",
							},
						},
					},
				}
				mc.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(pg, nil).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*instancev1alpha1.Postgresql)
					*obj = *pg
				})
				mc.On("Status").Return(ms)
				ms.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
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

			reconciler := &PostgresqlReconciler{
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

func TestBuildConnectionCondition_TableDriven(t *testing.T) {
	tests := []struct {
		name              string
		connected         bool
		connErr           error
		postgresqlID      string
		connectionAddress string
		expectedStatus    metav1.ConditionStatus
		expectedReason    string
		expectedMsgPart   string
	}{
		{
			name:              "Connected successfully",
			connected:         true,
			connErr:           nil,
			postgresqlID:      "test-id",
			connectionAddress: "localhost:5432",
			expectedStatus:    metav1.ConditionTrue,
			expectedReason:    "Connected",
			expectedMsgPart:   "Successfully connected",
		},
		{
			name:              "Not connected with error",
			connected:         false,
			connErr:           fmt.Errorf("connection refused"),
			postgresqlID:      "test-id",
			connectionAddress: "localhost:5432",
			expectedStatus:    metav1.ConditionFalse,
			expectedReason:    "ConnectionFailed",
			expectedMsgPart:   "connection refused",
		},
		{
			name:              "Not connected without error",
			connected:         false,
			connErr:           nil,
			postgresqlID:      "test-id",
			connectionAddress: "localhost:5432",
			expectedStatus:    metav1.ConditionFalse,
			expectedReason:    "ConnectionFailed",
			expectedMsgPart:   "Failed to connect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, reason, message := buildConnectionCondition(
				tt.connected, tt.connErr, tt.postgresqlID, tt.connectionAddress,
			)
			assert.Equal(t, tt.expectedStatus, status)
			assert.Equal(t, tt.expectedReason, reason)
			assert.Contains(t, message, tt.expectedMsgPart)
		})
	}
}

func TestUpdateConnectionStatus_TableDriven(t *testing.T) {
	tests := []struct {
		name              string
		postgresql        *instancev1alpha1.Postgresql
		connectionAddress string
		connected         bool
		connErr           error
		expectUpdate      bool
	}{
		{
			name: "Address changed - needs update",
			postgresql: &instancev1alpha1.Postgresql{
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
						PostgresqlID: "test-id",
					},
				},
				Status: instancev1alpha1.PostgresqlStatus{
					ConnectionAddress: "old-address:5432",
					Connected:         false,
				},
			},
			connectionAddress: "new-address:5432",
			connected:         false,
			connErr:           nil,
			expectUpdate:      true,
		},
		{
			name: "Connected status changed - needs update",
			postgresql: &instancev1alpha1.Postgresql{
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
						PostgresqlID: "test-id",
					},
				},
				Status: instancev1alpha1.PostgresqlStatus{
					ConnectionAddress: "localhost:5432",
					Connected:         false,
				},
			},
			connectionAddress: "localhost:5432",
			connected:         true,
			connErr:           nil,
			expectUpdate:      true,
		},
		{
			name: "Condition changed - needs update",
			postgresql: &instancev1alpha1.Postgresql{
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
						PostgresqlID: "test-id",
					},
				},
				Status: instancev1alpha1.PostgresqlStatus{
					ConnectionAddress: "localhost:5432",
					Connected:         false,
					Conditions: []metav1.Condition{
						{
							Type:    "Ready",
							Status:  metav1.ConditionFalse,
							Reason:  "ConnectionFailed",
							Message: "old message",
						},
					},
				},
			},
			connectionAddress: "localhost:5432",
			connected:         false,
			connErr:           fmt.Errorf("new error"),
			expectUpdate:      true,
		},
		{
			name: "No condition exists - needs update",
			postgresql: &instancev1alpha1.Postgresql{
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
						PostgresqlID: "test-id",
					},
				},
				Status: instancev1alpha1.PostgresqlStatus{
					ConnectionAddress: "localhost:5432",
					Connected:         false,
					Conditions:        []metav1.Condition{},
				},
			},
			connectionAddress: "localhost:5432",
			connected:         false,
			connErr:           nil,
			expectUpdate:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &PostgresqlReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Log: zap.NewNop().Sugar(),
				},
			}

			result := reconciler.updateConnectionStatus(
				tt.postgresql, tt.connectionAddress, tt.connected, tt.connErr,
			)
			assert.Equal(t, tt.expectUpdate, result)
		})
	}
}

func TestLogAndPersistStatus_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		connected       bool
		connErr         error
		statusUpdateErr error
	}{
		{
			name:            "Connected - status update succeeds",
			connected:       true,
			connErr:         nil,
			statusUpdateErr: nil,
		},
		{
			name:            "Not connected - status update succeeds",
			connected:       false,
			connErr:         fmt.Errorf("connection refused"),
			statusUpdateErr: nil,
		},
		{
			name:            "Status update fails",
			connected:       true,
			connErr:         nil,
			statusUpdateErr: fmt.Errorf("update error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockControllerClient)
			mockStatus := new(MockStatusWriter)

			mockClient.On("Status").Return(mockStatus)
			mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(tt.statusUpdateErr)

			reconciler := &PostgresqlReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:   mockClient,
					Log:      zap.NewNop().Sugar(),
					Recorder: record.NewFakeRecorder(100),
				},
			}

			pg := &instancev1alpha1.Postgresql{
				ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
						PostgresqlID: "test-id",
						Address:      "localhost",
						Port:         5432,
					},
				},
			}

			// Should not panic
			reconciler.logAndPersistStatus(
				context.Background(), pg, pg.Spec.ExternalInstance,
				5432, "postgres", "admin", tt.connected, "localhost:5432", tt.connErr,
			)

			assert.NotNil(t, pg.Status.LastConnectionAttempt)
			mockClient.AssertExpectations(t)
			mockStatus.AssertExpectations(t)
		})
	}
}
