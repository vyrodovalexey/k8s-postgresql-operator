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

func TestUserReconciler_SyncUser_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		user            *instancev1alpha1.User
		info            *connectionInfo
		dbPassword      string
		statusUpdateErr error
		expectedError   bool
	}{
		{
			name: "Sync user - pg operation fails (no real DB), status update succeeds",
			user: &instancev1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
				Spec: instancev1alpha1.UserSpec{
					PostgresqlID: "test-id",
					Username:     "testuser",
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
			dbPassword:      "userpass",
			statusUpdateErr: nil,
			expectedError:   false,
		},
		{
			name: "Sync user - status update fails",
			user: &instancev1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
				Spec: instancev1alpha1.UserSpec{
					PostgresqlID: "test-id",
					Username:     "testuser",
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
			dbPassword:      "userpass",
			statusUpdateErr: fmt.Errorf("status update error"),
			expectedError:   true,
		},
		{
			name: "Sync user with empty password",
			user: &instancev1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
				Spec: instancev1alpha1.UserSpec{
					PostgresqlID: "test-id",
					Username:     "testuser",
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
			dbPassword:      "",
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

			reconciler := &UserReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:                      mockClient,
					Log:                         zap.NewNop().Sugar(),
					Recorder:                    record.NewFakeRecorder(100),
					PostgresqlConnectionRetries: 1,
					PostgresqlConnectionTimeout: 1 * time.Millisecond,
				},
			}

			_, err := reconciler.syncUser(context.Background(), tt.user, tt.info, tt.dbPassword)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NotNil(t, tt.user.Status.LastSyncAttempt)
			mockClient.AssertExpectations(t)
			mockStatus.AssertExpectations(t)
		})
	}
}

func TestUserReconciler_ResolveUserPassword_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		vaultNil      bool
		user          *instancev1alpha1.User
		expectedEmpty bool
	}{
		{
			name:     "Vault client is nil - returns empty",
			vaultNil: true,
			user: &instancev1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
				Spec: instancev1alpha1.UserSpec{
					PostgresqlID: "test-id",
					Username:     "testuser",
				},
			},
			expectedEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &UserReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					VaultClient:                 nil,
					Log:                         zap.NewNop().Sugar(),
					VaultAvailabilityRetries:    1,
					VaultAvailabilityRetryDelay: 1 * time.Millisecond,
				},
			}

			password := reconciler.resolveUserPassword(context.Background(), tt.user)

			if tt.expectedEmpty {
				assert.Empty(t, password)
			} else {
				assert.NotEmpty(t, password)
			}
		})
	}
}

func TestUserReconciler_Reconcile_ConnectedPG_NoVault(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	user := createTestUser("test-user", "default", "test-id", "testuser", false)
	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(user, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.User)
		*obj = *user
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{*postgresql}
	})
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &UserReconciler{
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
		NamespacedName: types.NamespacedName{Name: "test-user", Namespace: "default"},
	})

	// VaultClient is nil, so resolveUserPassword returns "", and since VaultClient is nil,
	// the condition `dbPassword == "" && r.VaultClient != nil` is false, so syncUser is called
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}

func TestUserReconciler_Reconcile_StatusUpdateFails_OnPGNotFound(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	user := createTestUser("test-user", "default", "non-existent-id", "testuser", false)

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(user, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.User)
		*obj = *user
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{}
	})
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("status update error"))

	reconciler := &UserReconciler{
		BaseReconcilerConfig: getBaseReconcilerConfig(mockClient, nil),
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-user", Namespace: "default"},
	})

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}

func TestGenerateRandomPassword_Uniqueness(t *testing.T) {
	passwords := make(map[string]bool)
	for i := 0; i < 10; i++ {
		p, err := generateRandomPassword(32)
		assert.NoError(t, err)
		assert.Len(t, p, 32)
		passwords[p] = true
	}
	// All 10 passwords should be unique
	assert.Equal(t, 10, len(passwords))
}

func TestGenerateRandomPassword_Length1(t *testing.T) {
	p, err := generateRandomPassword(1)
	assert.NoError(t, err)
	assert.Len(t, p, 1)
}
