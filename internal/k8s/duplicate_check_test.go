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
)

func TestCheckDuplicatePostgresqlID_Success(t *testing.T) {
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
					},
				},
			},
		},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(postgresqlList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})

	result, err := CheckDuplicatePostgresqlID(context.Background(), mockClient, "pg2", "default", postgresqlID, false)

	assert.NoError(t, err)
	assert.True(t, result.Found)
	assert.NotNil(t, result.Existing)
	assert.Equal(t, "PostgreSQL", result.Resource)
	assert.Contains(t, result.Message, postgresqlID)
	mockClient.AssertExpectations(t)
}

func TestCheckDuplicatePostgresqlID_NoDuplicate(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id-123"

	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(postgresqlList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})

	result, err := CheckDuplicatePostgresqlID(context.Background(), mockClient, "pg1", "default", postgresqlID, false)

	assert.NoError(t, err)
	assert.False(t, result.Found)
	mockClient.AssertExpectations(t)
}

func TestCheckDuplicatePostgresqlID_SkipCurrentOnUpdate(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id-123"
	currentName := "pg1"
	currentNamespace := "default"

	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      currentName,
					Namespace: currentNamespace,
				},
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
						PostgresqlID: postgresqlID,
					},
				},
			},
		},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(postgresqlList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})

	// Should not find duplicate when updating the same resource
	result, err := CheckDuplicatePostgresqlID(context.Background(), mockClient, currentName, currentNamespace, postgresqlID, true)

	assert.NoError(t, err)
	assert.False(t, result.Found)
	mockClient.AssertExpectations(t)
}

func TestCheckDuplicateUser_Success(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id-123"
	username := "testuser"

	userList := &instancev1alpha1.UserList{
		Items: []instancev1alpha1.User{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user1",
					Namespace: "default",
				},
				Spec: instancev1alpha1.UserSpec{
					PostgresqlID: postgresqlID,
					Username:     username,
				},
			},
		},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(userList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.UserList)
		*list = *userList
	})

	result, err := CheckDuplicateUser(context.Background(), mockClient, "user2", "default", postgresqlID, username, false)

	assert.NoError(t, err)
	assert.True(t, result.Found)
	assert.Equal(t, "User", result.Resource)
	assert.Contains(t, result.Message, postgresqlID)
	assert.Contains(t, result.Message, username)
	mockClient.AssertExpectations(t)
}

func TestCheckDuplicateDatabase_Success(t *testing.T) {
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

	result, err := CheckDuplicateDatabase(context.Background(), mockClient, "db2", "default", postgresqlID, databaseName, false)

	assert.NoError(t, err)
	assert.True(t, result.Found)
	assert.Equal(t, "Database", result.Resource)
	mockClient.AssertExpectations(t)
}

func TestCheckDuplicateGrant_Success(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id-123"
	role := "testrole"
	databaseName := "testdb"

	grantList := &instancev1alpha1.GrantList{
		Items: []instancev1alpha1.Grant{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "grant1",
					Namespace: "default",
				},
				Spec: instancev1alpha1.GrantSpec{
					PostgresqlID: postgresqlID,
					Role:         role,
					Database:     databaseName,
				},
			},
		},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(grantList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.GrantList)
		*list = *grantList
	})

	result, err := CheckDuplicateGrant(context.Background(), mockClient, "grant2", "default", postgresqlID, role, databaseName, false)

	assert.NoError(t, err)
	assert.True(t, result.Found)
	assert.Equal(t, "Grant", result.Resource)
	mockClient.AssertExpectations(t)
}

func TestCheckDuplicateRoleGroup_Success(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id-123"
	groupRole := "testgroup"

	roleGroupList := &instancev1alpha1.RoleGroupList{
		Items: []instancev1alpha1.RoleGroup{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rg1",
					Namespace: "default",
				},
				Spec: instancev1alpha1.RoleGroupSpec{
					PostgresqlID: postgresqlID,
					GroupRole:    groupRole,
				},
			},
		},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(roleGroupList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.RoleGroupList)
		*list = *roleGroupList
	})

	result, err := CheckDuplicateRoleGroup(context.Background(), mockClient, "rg2", "default", postgresqlID, groupRole, false)

	assert.NoError(t, err)
	assert.True(t, result.Found)
	assert.Equal(t, "RoleGroup", result.Resource)
	mockClient.AssertExpectations(t)
}

func TestCheckDuplicateSchema_Success(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id-123"
	schemaName := "testschema"

	schemaList := &instancev1alpha1.SchemaList{
		Items: []instancev1alpha1.Schema{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "schema1",
					Namespace: "default",
				},
				Spec: instancev1alpha1.SchemaSpec{
					PostgresqlID: postgresqlID,
					Schema:       schemaName,
				},
			},
		},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(schemaList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.SchemaList)
		*list = *schemaList
	})

	result, err := CheckDuplicateSchema(context.Background(), mockClient, "schema2", "default", postgresqlID, schemaName, false)

	assert.NoError(t, err)
	assert.True(t, result.Found)
	assert.Equal(t, "Schema", result.Resource)
	mockClient.AssertExpectations(t)
}

func TestCheckDuplicatePostgresqlID_ListError(t *testing.T) {
	mockClient := new(MockK8sClient)
	postgresqlID := "test-id"

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("list error"))

	result, err := CheckDuplicatePostgresqlID(context.Background(), mockClient, "pg1", "default", postgresqlID, false)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to list")
	mockClient.AssertExpectations(t)
}
