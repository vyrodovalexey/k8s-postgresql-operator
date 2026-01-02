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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// MockPostgresqlVaultClient is a mock implementation of vault.Client for testing
type MockPostgresqlVaultClient struct {
	mock.Mock
}

func (m *MockPostgresqlVaultClient) GetPostgresqlCredentials(ctx context.Context, postgresqlID string) (string, string, error) {
	args := m.Called(ctx, postgresqlID)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockPostgresqlVaultClient) GetPostgresqlUserCredentials(ctx context.Context, postgresqlID, username string) (string, error) {
	args := m.Called(ctx, postgresqlID, username)
	return args.String(0), args.Error(1)
}

func (m *MockPostgresqlVaultClient) StorePostgresqlUserCredentials(ctx context.Context, postgresqlID, username, password string) error {
	args := m.Called(ctx, postgresqlID, username, password)
	return args.Error(0)
}

func (m *MockPostgresqlVaultClient) StorePostgresqlCredentials(ctx context.Context, postgresqlID, username, password string) error {
	args := m.Called(ctx, postgresqlID, username, password)
	return args.Error(0)
}

// MockClient is a mock implementation of client.Client for testing
type MockControllerClient struct {
	mock.Mock
	client.Client
}

func (m *MockControllerClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	args := m.Called(ctx, key, obj, opts)
	if args.Get(0) != nil {
		if pg, ok := obj.(*instancev1alpha1.Postgresql); ok {
			if pgObj, ok := args.Get(0).(*instancev1alpha1.Postgresql); ok {
				*pg = *pgObj
			}
		}
	}
	return args.Error(1)
}

func (m *MockControllerClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := m.Called(ctx, list, opts)
	return args.Error(0)
}

func (m *MockControllerClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockControllerClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockControllerClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockControllerClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := m.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

func (m *MockControllerClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockControllerClient) Status() client.StatusWriter {
	args := m.Called()
	return args.Get(0).(client.StatusWriter)
}

func (m *MockControllerClient) Scheme() *runtime.Scheme {
	args := m.Called()
	return args.Get(0).(*runtime.Scheme)
}

func (m *MockControllerClient) RESTMapper() meta.RESTMapper {
	args := m.Called()
	return args.Get(0).(meta.RESTMapper)
}

func (m *MockControllerClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	args := m.Called(obj)
	return args.Get(0).(schema.GroupVersionKind), args.Error(1)
}

func (m *MockControllerClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	args := m.Called(obj)
	return args.Bool(0), args.Error(1)
}

func (m *MockControllerClient) SubResource(subResource string) client.SubResourceClient {
	args := m.Called(subResource)
	return args.Get(0).(client.SubResourceClient)
}

func (m *MockControllerClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

// MockStatusWriter is a mock implementation of client.StatusWriter
type MockStatusWriter struct {
	mock.Mock
}

func (m *MockStatusWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	args := m.Called(ctx, obj, subResource, opts)
	return args.Error(0)
}

func (m *MockStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	args := m.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

func TestPostgresqlReconciler_Reconcile_NotFound(t *testing.T) {
	mockClient := new(MockControllerClient)
	logger := zap.NewNop().Sugar()

	reconciler := &PostgresqlReconciler{
		Client: mockClient,
		Log:    logger,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-postgresql",
			Namespace: "default",
		},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.NewNotFound(schema.GroupResource{}, "postgresql"))

	result, err := reconciler.Reconcile(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
}

func TestPostgresqlReconciler_Reconcile_NoExternalInstance(t *testing.T) {
	mockClient := new(MockControllerClient)
	logger := zap.NewNop().Sugar()

	reconciler := &PostgresqlReconciler{
		Client: mockClient,
		Log:    logger,
	}

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-postgresql",
			Namespace: "default",
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: nil,
		},
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-postgresql",
			Namespace: "default",
		},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(postgresql, nil)

	result, err := reconciler.Reconcile(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
}

func TestFindCondition(t *testing.T) {
	conditions := []metav1.Condition{
		{
			Type:   "Ready",
			Status: metav1.ConditionTrue,
		},
		{
			Type:   "Other",
			Status: metav1.ConditionFalse,
		},
	}

	// Test finding existing condition
	found := findCondition(conditions, "Ready")
	assert.NotNil(t, found)
	assert.Equal(t, "Ready", found.Type)
	assert.Equal(t, metav1.ConditionTrue, found.Status)

	// Test finding non-existent condition
	notFound := findCondition(conditions, "NonExistent")
	assert.Nil(t, notFound)

	// Test with empty conditions
	empty := findCondition([]metav1.Condition{}, "Ready")
	assert.Nil(t, empty)
}

func TestUpdateCondition(t *testing.T) {
	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Generation: 1,
		},
		Status: instancev1alpha1.PostgresqlStatus{
			Conditions: []metav1.Condition{},
		},
	}

	// Test adding new condition
	updateCondition(postgresql, "Ready", metav1.ConditionTrue, "TestReason", "Test message")
	assert.Len(t, postgresql.Status.Conditions, 1)
	assert.Equal(t, "Ready", postgresql.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, postgresql.Status.Conditions[0].Status)
	assert.Equal(t, "TestReason", postgresql.Status.Conditions[0].Reason)
	assert.Equal(t, "Test message", postgresql.Status.Conditions[0].Message)

	// Test updating existing condition
	updateCondition(postgresql, "Ready", metav1.ConditionFalse, "NewReason", "New message")
	assert.Len(t, postgresql.Status.Conditions, 1)
	assert.Equal(t, "Ready", postgresql.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, postgresql.Status.Conditions[0].Status)
	assert.Equal(t, "NewReason", postgresql.Status.Conditions[0].Reason)
	assert.Equal(t, "New message", postgresql.Status.Conditions[0].Message)
}

func TestPostgresqlReconciler_Reconcile_GetError(t *testing.T) {
	mockClient := new(MockControllerClient)
	logger := zap.NewNop().Sugar()

	reconciler := &PostgresqlReconciler{
		Client: mockClient,
		Log:    logger,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-postgresql",
			Namespace: "default",
		},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("some error"))

	result, err := reconciler.Reconcile(context.Background(), req)

	assert.Error(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
}

func TestPostgresqlReconciler_Reconcile_WithExternalInstance(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	reconciler := &PostgresqlReconciler{
		Client:      mockClient,
		VaultClient: nil, // No vault client for this test
		Log:         logger,
	}

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-postgresql",
			Namespace: "default",
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "test-id",
				Address:      "localhost",
				Port:         5432,
				SSLMode:      "require",
			},
		},
		Status: instancev1alpha1.PostgresqlStatus{
			Connected: false,
		},
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-postgresql",
			Namespace: "default",
		},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(postgresql, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.Postgresql)
		*obj = *postgresql
	})
	// Vault client is nil, so it won't be called
	mockClient.On("Status").Return(mockStatusWriter)
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	result, err := reconciler.Reconcile(context.Background(), req)

	// Should requeue even if connection fails
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
	mockStatusWriter.AssertExpectations(t)
}
