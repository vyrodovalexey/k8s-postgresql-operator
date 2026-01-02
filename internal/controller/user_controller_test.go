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
	"testing"

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

// MockVaultClient is a mock implementation of vault.Client for testing
type MockVaultClient struct {
	mock.Mock
}

func (m *MockVaultClient) GetPostgresqlCredentials(ctx context.Context, postgresqlID string) (string, string, error) {
	args := m.Called(ctx, postgresqlID)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockVaultClient) GetPostgresqlUserCredentials(ctx context.Context, postgresqlID, username string) (string, error) {
	args := m.Called(ctx, postgresqlID, username)
	return args.String(0), args.Error(1)
}

func (m *MockVaultClient) StorePostgresqlUserCredentials(ctx context.Context, postgresqlID, username, password string) error {
	args := m.Called(ctx, postgresqlID, username, password)
	return args.Error(0)
}

func (m *MockVaultClient) StorePostgresqlCredentials(ctx context.Context, postgresqlID, username, password string) error {
	args := m.Called(ctx, postgresqlID, username, password)
	return args.Error(0)
}

func TestUserReconciler_Reconcile_NotFound(t *testing.T) {
	mockClient := new(MockControllerClient)
	logger := zap.NewNop().Sugar()

	reconciler := &UserReconciler{
		Client: mockClient,
		Log:    logger,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-user",
			Namespace: "default",
		},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.NewNotFound(schema.GroupResource{}, "user"))

	result, err := reconciler.Reconcile(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
}

func TestUserReconciler_FindPostgresqlByID_Success(t *testing.T) {
	mockClient := new(MockControllerClient)
	logger := zap.NewNop().Sugar()

	reconciler := &UserReconciler{
		Client: mockClient,
		Log:    logger,
	}

	postgresqlID := "test-id-123"
	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pg1",
					Namespace: "default",
				},
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
						PostgresqlID: postgresqlID,
						Address:      "localhost",
						Port:         5432,
					},
				},
				Status: instancev1alpha1.PostgresqlStatus{
					Connected: true,
				},
			},
		},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})

	result, err := reconciler.findPostgresqlByID(context.Background(), postgresqlID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, postgresqlID, result.Spec.ExternalInstance.PostgresqlID)
	mockClient.AssertExpectations(t)
}

func TestUserReconciler_FindPostgresqlByID_NotFound(t *testing.T) {
	mockClient := new(MockControllerClient)
	logger := zap.NewNop().Sugar()

	reconciler := &UserReconciler{
		Client: mockClient,
		Log:    logger,
	}

	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})

	result, err := reconciler.findPostgresqlByID(context.Background(), "non-existent-id")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
	mockClient.AssertExpectations(t)
}

func TestGenerateRandomPassword(t *testing.T) {
	password, err := generateRandomPassword(32)
	assert.NoError(t, err)
	assert.Len(t, password, 32)

	// Test different lengths
	password2, err := generateRandomPassword(16)
	assert.NoError(t, err)
	assert.Len(t, password2, 16)

	// Test that passwords are different
	assert.NotEqual(t, password, password2)
}

func TestUpdateUserCondition(t *testing.T) {
	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Generation: 1,
		},
		Status: instancev1alpha1.UserStatus{
			Conditions: []metav1.Condition{},
		},
	}

	// Test adding new condition
	updateUserCondition(user, "Ready", metav1.ConditionTrue, "TestReason", "Test message")
	assert.Len(t, user.Status.Conditions, 1)
	assert.Equal(t, "Ready", user.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, user.Status.Conditions[0].Status)
	assert.Equal(t, "TestReason", user.Status.Conditions[0].Reason)
	assert.Equal(t, "Test message", user.Status.Conditions[0].Message)

	// Test updating existing condition
	updateUserCondition(user, "Ready", metav1.ConditionFalse, "NewReason", "New message")
	assert.Len(t, user.Status.Conditions, 1)
	assert.Equal(t, "Ready", user.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, user.Status.Conditions[0].Status)
	assert.Equal(t, "NewReason", user.Status.Conditions[0].Reason)
	assert.Equal(t, "New message", user.Status.Conditions[0].Message)
}
