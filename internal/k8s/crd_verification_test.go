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

package k8s

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MockK8sClient is a mock implementation of client.Client for testing
type MockK8sClient struct {
	mock.Mock
	client.Client
}

func (m *MockK8sClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := m.Called(ctx, list, opts)
	if args.Get(0) != nil {
		if pgList, ok := list.(*instancev1alpha1.PostgresqlList); ok {
			if pgListArg, ok := args.Get(0).(*instancev1alpha1.PostgresqlList); ok {
				*pgList = *pgListArg
			}
		}
		if dbList, ok := list.(*instancev1alpha1.DatabaseList); ok {
			if dbListArg, ok := args.Get(0).(*instancev1alpha1.DatabaseList); ok {
				*dbList = *dbListArg
			}
		}
	}
	return args.Error(1)
}

func TestFindPostgresqlByID_Success(t *testing.T) {
	mockClient := new(MockK8sClient)
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
			},
		},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(postgresqlList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})

	result, err := FindPostgresqlByID(context.Background(), mockClient, postgresqlID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, postgresqlID, result.Spec.ExternalInstance.PostgresqlID)
	mockClient.AssertExpectations(t)
}

func TestFindPostgresqlByID_NotFound(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "non-existent-id"

	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(postgresqlList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})

	result, err := FindPostgresqlByID(context.Background(), mockClient, postgresqlID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
	mockClient.AssertExpectations(t)
}

func TestFindPostgresqlByID_ListError(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id"

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("list error"))

	result, err := FindPostgresqlByID(context.Background(), mockClient, postgresqlID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to list")
	mockClient.AssertExpectations(t)
}

func TestFindPostgresqlByID_NoExternalInstance(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id"

	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pg1",
					Namespace: "default",
				},
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: nil, // No external instance
				},
			},
		},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(postgresqlList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})

	result, err := FindPostgresqlByID(context.Background(), mockClient, postgresqlID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
	mockClient.AssertExpectations(t)
}

func TestFindDatabaseByPostgresqlIDAndName_Success(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id-123"
	databaseName := "testdb"

	databaseList := &instancev1alpha1.DatabaseList{
		Items: []instancev1alpha1.Database{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db1",
					Namespace: "default",
				},
				Spec: instancev1alpha1.DatabaseSpec{
					PostgresqlID: postgresqlID,
					Database:     databaseName,
				},
			},
		},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(databaseList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.DatabaseList)
		*list = *databaseList
	})

	result, err := FindDatabaseByPostgresqlIDAndName(context.Background(), mockClient, postgresqlID, databaseName)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, postgresqlID, result.Spec.PostgresqlID)
	assert.Equal(t, databaseName, result.Spec.Database)
	mockClient.AssertExpectations(t)
}

func TestFindDatabaseByPostgresqlIDAndName_NotFound(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id-123"
	databaseName := "nonexistent"

	databaseList := &instancev1alpha1.DatabaseList{
		Items: []instancev1alpha1.Database{},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(databaseList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.DatabaseList)
		*list = *databaseList
	})

	result, err := FindDatabaseByPostgresqlIDAndName(context.Background(), mockClient, postgresqlID, databaseName)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
	mockClient.AssertExpectations(t)
}

func TestDatabaseExistsByPostgresqlIDAndName_Exists(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id-123"
	databaseName := "testdb"

	databaseList := &instancev1alpha1.DatabaseList{
		Items: []instancev1alpha1.Database{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db1",
					Namespace: "default",
				},
				Spec: instancev1alpha1.DatabaseSpec{
					PostgresqlID: postgresqlID,
					Database:     databaseName,
				},
			},
		},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(databaseList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.DatabaseList)
		*list = *databaseList
	})

	exists, err := DatabaseExistsByPostgresqlIDAndName(context.Background(), mockClient, postgresqlID, databaseName)

	assert.NoError(t, err)
	assert.True(t, exists)
	mockClient.AssertExpectations(t)
}

func TestDatabaseExistsByPostgresqlIDAndName_NotExists(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id-123"
	databaseName := "nonexistent"

	databaseList := &instancev1alpha1.DatabaseList{
		Items: []instancev1alpha1.Database{},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(databaseList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.DatabaseList)
		*list = *databaseList
	})

	exists, err := DatabaseExistsByPostgresqlIDAndName(context.Background(), mockClient, postgresqlID, databaseName)

	assert.NoError(t, err)
	assert.False(t, exists)
	mockClient.AssertExpectations(t)
}

func TestDatabaseExistsByPostgresqlIDAndName_ListError(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id-123"
	databaseName := "testdb"

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("list error"))

	exists, err := DatabaseExistsByPostgresqlIDAndName(context.Background(), mockClient, postgresqlID, databaseName)

	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "failed to list")
	mockClient.AssertExpectations(t)
}
