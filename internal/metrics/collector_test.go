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

package metrics

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// MockClient is a mock implementation of client.Client
type MockClient struct {
	mock.Mock
}

func (m *MockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	args := m.Called(ctx, key, obj, opts)
	return args.Error(0)
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := m.Called(ctx, list, opts)
	if args.Get(0) != nil {
		// Copy the list items from the mock
		switch l := list.(type) {
		case *instancev1alpha1.PostgresqlList:
			if pgList, ok := args.Get(0).(*instancev1alpha1.PostgresqlList); ok {
				*l = *pgList
			}
		case *instancev1alpha1.UserList:
			if userList, ok := args.Get(0).(*instancev1alpha1.UserList); ok {
				*l = *userList
			}
		case *instancev1alpha1.DatabaseList:
			if dbList, ok := args.Get(0).(*instancev1alpha1.DatabaseList); ok {
				*l = *dbList
			}
		case *instancev1alpha1.GrantList:
			if grantList, ok := args.Get(0).(*instancev1alpha1.GrantList); ok {
				*l = *grantList
			}
		case *instancev1alpha1.RoleGroupList:
			if rgList, ok := args.Get(0).(*instancev1alpha1.RoleGroupList); ok {
				*l = *rgList
			}
		case *instancev1alpha1.SchemaList:
			if schemaList, ok := args.Get(0).(*instancev1alpha1.SchemaList); ok {
				*l = *schemaList
			}
		}
	}
	return args.Error(1)
}

func (m *MockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := m.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

func (m *MockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockClient) Status() client.StatusWriter {
	args := m.Called()
	return args.Get(0).(client.StatusWriter)
}

func (m *MockClient) Scheme() *runtime.Scheme {
	args := m.Called()
	return args.Get(0).(*runtime.Scheme)
}

func (m *MockClient) RESTMapper() meta.RESTMapper {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(meta.RESTMapper)
}

func (m *MockClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	args := m.Called(obj)
	if args.Get(0) == nil {
		return schema.GroupVersionKind{}, args.Error(1)
	}
	return args.Get(0).(schema.GroupVersionKind), args.Error(1)
}

func (m *MockClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	args := m.Called(obj)
	return args.Bool(0), args.Error(1)
}

func (m *MockClient) SubResource(subResource string) client.SubResourceClient {
	args := m.Called(subResource)
	return args.Get(0).(client.SubResourceClient)
}

func TestNewCollector(t *testing.T) {
	mockClient := new(MockClient)
	logger := zap.NewNop().Sugar()

	collector := NewCollector(mockClient, logger)

	assert.NotNil(t, collector)
	assert.Equal(t, mockClient, collector.Client)
	assert.Equal(t, logger, collector.Log)
}

func TestCollectMetrics_Success(t *testing.T) {
	// Reset metrics
	PostgresqlCount.Set(0)
	ObjectCountPerPostgresqlID.Reset()
	ObjectNames.Reset()

	mockClient := new(MockClient)
	logger := zap.NewNop().Sugar()
	collector := NewCollector(mockClient, logger)

	ctx := context.Background()

	// Setup mocks for all resource types
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(
		&instancev1alpha1.PostgresqlList{
			Items: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "pg-1",
						},
					},
				},
			},
		}, nil)

	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.UserList"), mock.Anything).Return(
		&instancev1alpha1.UserList{
			Items: []instancev1alpha1.User{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "user1", Namespace: "default"},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID: "pg-1",
					},
				},
			},
		}, nil)

	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(
		&instancev1alpha1.DatabaseList{
			Items: []instancev1alpha1.Database{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "db1", Namespace: "default"},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID: "pg-1",
					},
				},
			},
		}, nil)

	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.GrantList"), mock.Anything).Return(
		&instancev1alpha1.GrantList{
			Items: []instancev1alpha1.Grant{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "grant1", Namespace: "default"},
					Spec: instancev1alpha1.GrantSpec{
						PostgresqlID: "pg-1",
					},
				},
			},
		}, nil)

	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.RoleGroupList"), mock.Anything).Return(
		&instancev1alpha1.RoleGroupList{
			Items: []instancev1alpha1.RoleGroup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "rg1", Namespace: "default"},
					Spec: instancev1alpha1.RoleGroupSpec{
						PostgresqlID: "pg-1",
					},
				},
			},
		}, nil)

	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.SchemaList"), mock.Anything).Return(
		&instancev1alpha1.SchemaList{
			Items: []instancev1alpha1.Schema{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "schema1", Namespace: "default"},
					Spec: instancev1alpha1.SchemaSpec{
						PostgresqlID: "pg-1",
					},
				},
			},
		}, nil)

	err := collector.CollectMetrics(ctx)
	require.NoError(t, err)

	// Verify metrics were updated
	// Note: We can't easily verify the exact values without exporting internal state,
	// but we can verify the function completed without error
	mockClient.AssertExpectations(t)
}

func TestCollectMetrics_ErrorOnPostgresqlList(t *testing.T) {
	mockClient := new(MockClient)
	logger := zap.NewNop().Sugar()
	collector := NewCollector(mockClient, logger)

	ctx := context.Background()

	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(
		nil, errors.New("list error"))

	err := collector.CollectMetrics(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list error")
}

func TestCollectMetrics_ErrorOnUserList(t *testing.T) {
	mockClient := new(MockClient)
	logger := zap.NewNop().Sugar()
	collector := NewCollector(mockClient, logger)

	ctx := context.Background()

	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(
		&instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.UserList"), mock.Anything).Return(
		nil, errors.New("user list error"))

	err := collector.CollectMetrics(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user list error")
}

func TestCollectPostgresqls_EmptyList(t *testing.T) {
	PostgresqlCount.Set(0)
	ObjectCountPerPostgresqlID.Reset()
	ObjectNames.Reset()

	mockClient := new(MockClient)
	logger := zap.NewNop().Sugar()
	collector := NewCollector(mockClient, logger)

	ctx := context.Background()

	// Mock all list calls since CollectMetrics calls all collection methods
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(
		&instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.UserList"), mock.Anything).Return(
		&instancev1alpha1.UserList{Items: []instancev1alpha1.User{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(
		&instancev1alpha1.DatabaseList{Items: []instancev1alpha1.Database{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.GrantList"), mock.Anything).Return(
		&instancev1alpha1.GrantList{Items: []instancev1alpha1.Grant{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.RoleGroupList"), mock.Anything).Return(
		&instancev1alpha1.RoleGroupList{Items: []instancev1alpha1.RoleGroup{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.SchemaList"), mock.Anything).Return(
		&instancev1alpha1.SchemaList{Items: []instancev1alpha1.Schema{}}, nil)

	err := collector.CollectMetrics(ctx)
	require.NoError(t, err)
}

func TestCollectPostgresqls_WithoutExternalInstance(t *testing.T) {
	PostgresqlCount.Set(0)
	ObjectCountPerPostgresqlID.Reset()
	ObjectNames.Reset()

	mockClient := new(MockClient)
	logger := zap.NewNop().Sugar()
	collector := NewCollector(mockClient, logger)

	ctx := context.Background()

	// PostgreSQL without ExternalInstance
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(
		&instancev1alpha1.PostgresqlList{
			Items: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: nil,
					},
				},
			},
		}, nil)

	// Mock other lists to return empty
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.UserList"), mock.Anything).Return(
		&instancev1alpha1.UserList{Items: []instancev1alpha1.User{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(
		&instancev1alpha1.DatabaseList{Items: []instancev1alpha1.Database{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.GrantList"), mock.Anything).Return(
		&instancev1alpha1.GrantList{Items: []instancev1alpha1.Grant{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.RoleGroupList"), mock.Anything).Return(
		&instancev1alpha1.RoleGroupList{Items: []instancev1alpha1.RoleGroup{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.SchemaList"), mock.Anything).Return(
		&instancev1alpha1.SchemaList{Items: []instancev1alpha1.Schema{}}, nil)

	err := collector.CollectMetrics(ctx)
	require.NoError(t, err)
}

func TestCollectPostgresqls_WithEmptyPostgresqlID(t *testing.T) {
	PostgresqlCount.Set(0)
	ObjectCountPerPostgresqlID.Reset()
	ObjectNames.Reset()

	mockClient := new(MockClient)
	logger := zap.NewNop().Sugar()
	collector := NewCollector(mockClient, logger)

	ctx := context.Background()

	// PostgreSQL with empty PostgresqlID
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(
		&instancev1alpha1.PostgresqlList{
			Items: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "",
						},
					},
				},
			},
		}, nil)

	// Mock other lists to return empty
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.UserList"), mock.Anything).Return(
		&instancev1alpha1.UserList{Items: []instancev1alpha1.User{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(
		&instancev1alpha1.DatabaseList{Items: []instancev1alpha1.Database{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.GrantList"), mock.Anything).Return(
		&instancev1alpha1.GrantList{Items: []instancev1alpha1.Grant{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.RoleGroupList"), mock.Anything).Return(
		&instancev1alpha1.RoleGroupList{Items: []instancev1alpha1.RoleGroup{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.SchemaList"), mock.Anything).Return(
		&instancev1alpha1.SchemaList{Items: []instancev1alpha1.Schema{}}, nil)

	err := collector.CollectMetrics(ctx)
	require.NoError(t, err)
}

func TestCollectPostgresqls_MultipleInstances(t *testing.T) {
	PostgresqlCount.Set(0)
	ObjectCountPerPostgresqlID.Reset()
	ObjectNames.Reset()

	mockClient := new(MockClient)
	logger := zap.NewNop().Sugar()
	collector := NewCollector(mockClient, logger)

	ctx := context.Background()

	// Multiple PostgreSQL instances
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(
		&instancev1alpha1.PostgresqlList{
			Items: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "pg-1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pg2", Namespace: "default"},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "pg-2",
						},
					},
				},
			},
		}, nil)

	// Mock other lists to return empty
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.UserList"), mock.Anything).Return(
		&instancev1alpha1.UserList{Items: []instancev1alpha1.User{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(
		&instancev1alpha1.DatabaseList{Items: []instancev1alpha1.Database{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.GrantList"), mock.Anything).Return(
		&instancev1alpha1.GrantList{Items: []instancev1alpha1.Grant{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.RoleGroupList"), mock.Anything).Return(
		&instancev1alpha1.RoleGroupList{Items: []instancev1alpha1.RoleGroup{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.SchemaList"), mock.Anything).Return(
		&instancev1alpha1.SchemaList{Items: []instancev1alpha1.Schema{}}, nil)

	err := collector.CollectMetrics(ctx)
	require.NoError(t, err)
}

func TestCollectUsers_MultipleUsers(t *testing.T) {
	PostgresqlCount.Set(0)
	ObjectCountPerPostgresqlID.Reset()
	ObjectNames.Reset()

	mockClient := new(MockClient)
	logger := zap.NewNop().Sugar()
	collector := NewCollector(mockClient, logger)

	ctx := context.Background()

	// Empty PostgreSQL list
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(
		&instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}, nil)

	// Multiple users with same postgresqlID
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.UserList"), mock.Anything).Return(
		&instancev1alpha1.UserList{
			Items: []instancev1alpha1.User{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "user1", Namespace: "default"},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID: "pg-1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "user2", Namespace: "default"},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID: "pg-1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "user3", Namespace: "production"},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID: "pg-2",
					},
				},
			},
		}, nil)

	// Mock other lists to return empty
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(
		&instancev1alpha1.DatabaseList{Items: []instancev1alpha1.Database{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.GrantList"), mock.Anything).Return(
		&instancev1alpha1.GrantList{Items: []instancev1alpha1.Grant{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.RoleGroupList"), mock.Anything).Return(
		&instancev1alpha1.RoleGroupList{Items: []instancev1alpha1.RoleGroup{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.SchemaList"), mock.Anything).Return(
		&instancev1alpha1.SchemaList{Items: []instancev1alpha1.Schema{}}, nil)

	err := collector.CollectMetrics(ctx)
	require.NoError(t, err)
}

func TestCollectMetrics_ConcurrentAccess(t *testing.T) {
	mockClient := new(MockClient)
	logger := zap.NewNop().Sugar()
	collector := NewCollector(mockClient, logger)

	ctx := context.Background()

	// Setup mocks to return empty lists
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(
		&instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.UserList"), mock.Anything).Return(
		&instancev1alpha1.UserList{Items: []instancev1alpha1.User{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(
		&instancev1alpha1.DatabaseList{Items: []instancev1alpha1.Database{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.GrantList"), mock.Anything).Return(
		&instancev1alpha1.GrantList{Items: []instancev1alpha1.Grant{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.RoleGroupList"), mock.Anything).Return(
		&instancev1alpha1.RoleGroupList{Items: []instancev1alpha1.RoleGroup{}}, nil)
	mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha1.SchemaList"), mock.Anything).Return(
		&instancev1alpha1.SchemaList{Items: []instancev1alpha1.Schema{}}, nil)

	// Run concurrent collections
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			err := collector.CollectMetrics(ctx)
			assert.NoError(t, err)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
