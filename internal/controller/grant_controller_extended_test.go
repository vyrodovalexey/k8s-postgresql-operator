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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestGrantReconciler_SyncGrants_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		grant           *instancev1alpha1.Grant
		info            *connectionInfo
		statusUpdateErr error
		expectedError   bool
	}{
		{
			name: "Sync grants - pg operation fails (no real DB), status update succeeds",
			grant: &instancev1alpha1.Grant{
				ObjectMeta: metav1.ObjectMeta{Name: "g1", Namespace: "default"},
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
			name: "Sync grants - status update fails",
			grant: &instancev1alpha1.Grant{
				ObjectMeta: metav1.ObjectMeta{Name: "g1", Namespace: "default"},
				Spec: instancev1alpha1.GrantSpec{
					PostgresqlID: "test-id",
					Database:     "testdb",
					Role:         "testrole",
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
		{
			name: "Sync grants with default privileges",
			grant: &instancev1alpha1.Grant{
				ObjectMeta: metav1.ObjectMeta{Name: "g1", Namespace: "default"},
				Spec: instancev1alpha1.GrantSpec{
					PostgresqlID: "test-id",
					Database:     "testdb",
					Role:         "testrole",
					Grants: []instancev1alpha1.GrantItem{
						{
							Type:       instancev1alpha1.GrantTypeDatabase,
							Privileges: []string{"CONNECT"},
						},
					},
					DefaultPrivileges: []instancev1alpha1.DefaultPrivilegeItem{
						{
							ObjectType: "tables",
							Schema:     "public",
							Privileges: []string{"SELECT"},
						},
					},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockControllerClient)
			mockStatus := new(MockStatusWriter)

			mockClient.On("Status").Return(mockStatus)
			mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(tt.statusUpdateErr)

			reconciler := &GrantReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:                      mockClient,
					Log:                         zap.NewNop().Sugar(),
					Recorder:                    record.NewFakeRecorder(100),
					PostgresqlConnectionRetries: 1,
					PostgresqlConnectionTimeout: 1 * time.Millisecond,
				},
			}

			_, err := reconciler.syncGrants(context.Background(), tt.grant, tt.info)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NotNil(t, tt.grant.Status.LastSyncAttempt)
			mockClient.AssertExpectations(t)
			mockStatus.AssertExpectations(t)
		})
	}
}

func TestGrantReconciler_Reconcile_StatusUpdateFails_OnPGNotFound(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	grant := &instancev1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{Name: "g1", Namespace: "default"},
		Spec: instancev1alpha1.GrantSpec{
			PostgresqlID: "non-existent",
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
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{}
	})
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("status update error"))

	reconciler := &GrantReconciler{
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

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "g1", Namespace: "default"},
	})

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}
