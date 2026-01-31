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

// TestCheckDuplicatePostgresqlID_TableDriven tests CheckDuplicatePostgresqlID with table-driven tests
func TestCheckDuplicatePostgresqlID_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		currentName      string
		currentNamespace string
		postgresqlID     string
		isUpdate         bool
		existingItems    []instancev1alpha1.Postgresql
		listError        error
		expectFound      bool
		expectError      bool
		errorMsg         string
	}{
		{
			name:             "no duplicate - empty list",
			currentName:      "pg1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			isUpdate:         false,
			existingItems:    []instancev1alpha1.Postgresql{},
			expectFound:      false,
			expectError:      false,
		},
		{
			name:             "duplicate found - different resource",
			currentName:      "pg1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			isUpdate:         false,
			existingItems: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg2",
						Namespace: "other-namespace",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
						},
					},
				},
			},
			expectFound: true,
			expectError: false,
		},
		{
			name:             "no duplicate - same resource on update",
			currentName:      "pg1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			isUpdate:         true,
			existingItems: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
						},
					},
				},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name:             "duplicate found on create - same name different namespace",
			currentName:      "pg1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			isUpdate:         false,
			existingItems: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg1",
						Namespace: "other-namespace",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
						},
					},
				},
			},
			expectFound: true,
			expectError: false,
		},
		{
			name:             "no duplicate - nil external instance",
			currentName:      "pg1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			isUpdate:         false,
			existingItems: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg2",
						Namespace: "default",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: nil,
					},
				},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name:             "no duplicate - different postgresqlID",
			currentName:      "pg1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			isUpdate:         false,
			existingItems: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg2",
						Namespace: "default",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "different-id",
						},
					},
				},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name:             "list error",
			currentName:      "pg1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			isUpdate:         false,
			listError:        fmt.Errorf("list error"),
			expectFound:      false,
			expectError:      true,
			errorMsg:         "failed to list",
		},
		{
			name:             "multiple items - one duplicate",
			currentName:      "pg1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			isUpdate:         false,
			existingItems: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg2",
						Namespace: "default",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "other-id",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg3",
						Namespace: "other-namespace",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
						},
					},
				},
			},
			expectFound: true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)

			postgresqlList := &instancev1alpha1.PostgresqlList{
				Items: tt.existingItems,
			}

			if tt.listError != nil {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil, tt.listError)
			} else {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(postgresqlList, nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
			}

			result, err := CheckDuplicatePostgresqlID(
				context.Background(), mockClient, tt.currentName, tt.currentNamespace, tt.postgresqlID, tt.isUpdate)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectFound, result.Found)
				if tt.expectFound {
					assert.Equal(t, "PostgreSQL", result.Resource)
					assert.NotEmpty(t, result.Message)
					assert.NotNil(t, result.Existing)
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// TestCheckDuplicateUser_TableDriven tests CheckDuplicateUser with table-driven tests
func TestCheckDuplicateUser_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		currentName      string
		currentNamespace string
		postgresqlID     string
		username         string
		isUpdate         bool
		existingItems    []instancev1alpha1.User
		listError        error
		expectFound      bool
		expectError      bool
	}{
		{
			name:             "no duplicate - empty list",
			currentName:      "user1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			username:         "testuser",
			isUpdate:         false,
			existingItems:    []instancev1alpha1.User{},
			expectFound:      false,
			expectError:      false,
		},
		{
			name:             "duplicate found",
			currentName:      "user1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			username:         "testuser",
			isUpdate:         false,
			existingItems: []instancev1alpha1.User{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user2",
						Namespace: "other-namespace",
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID: "pg-1",
						Username:     "testuser",
					},
				},
			},
			expectFound: true,
			expectError: false,
		},
		{
			name:             "no duplicate - same resource on update",
			currentName:      "user1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			username:         "testuser",
			isUpdate:         true,
			existingItems: []instancev1alpha1.User{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID: "pg-1",
						Username:     "testuser",
					},
				},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name:             "no duplicate - different username",
			currentName:      "user1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			username:         "testuser",
			isUpdate:         false,
			existingItems: []instancev1alpha1.User{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user2",
						Namespace: "default",
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID: "pg-1",
						Username:     "differentuser",
					},
				},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name:             "no duplicate - different postgresqlID",
			currentName:      "user1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			username:         "testuser",
			isUpdate:         false,
			existingItems: []instancev1alpha1.User{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user2",
						Namespace: "default",
					},
					Spec: instancev1alpha1.UserSpec{
						PostgresqlID: "pg-2",
						Username:     "testuser",
					},
				},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name:             "list error",
			currentName:      "user1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			username:         "testuser",
			isUpdate:         false,
			listError:        fmt.Errorf("list error"),
			expectFound:      false,
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)

			userList := &instancev1alpha1.UserList{
				Items: tt.existingItems,
			}

			if tt.listError != nil {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil, tt.listError)
			} else {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(userList, nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.UserList)
					*list = *userList
				})
			}

			result, err := CheckDuplicateUser(
				context.Background(), mockClient, tt.currentName, tt.currentNamespace, tt.postgresqlID, tt.username, tt.isUpdate)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectFound, result.Found)
				if tt.expectFound {
					assert.Equal(t, "User", result.Resource)
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// TestCheckDuplicateDatabase_TableDriven tests CheckDuplicateDatabase with table-driven tests
func TestCheckDuplicateDatabase_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		currentName      string
		currentNamespace string
		postgresqlID     string
		databaseName     string
		isUpdate         bool
		existingItems    []instancev1alpha1.Database
		listError        error
		expectFound      bool
		expectError      bool
	}{
		{
			name:             "no duplicate - empty list",
			currentName:      "db1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			databaseName:     "testdb",
			isUpdate:         false,
			existingItems:    []instancev1alpha1.Database{},
			expectFound:      false,
			expectError:      false,
		},
		{
			name:             "duplicate found",
			currentName:      "db1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			databaseName:     "testdb",
			isUpdate:         false,
			existingItems: []instancev1alpha1.Database{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "db2",
						Namespace: "other-namespace",
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID: "pg-1",
						Database:     "testdb",
					},
				},
			},
			expectFound: true,
			expectError: false,
		},
		{
			name:             "no duplicate - same resource on update",
			currentName:      "db1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			databaseName:     "testdb",
			isUpdate:         true,
			existingItems: []instancev1alpha1.Database{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "db1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID: "pg-1",
						Database:     "testdb",
					},
				},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name:             "list error",
			currentName:      "db1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			databaseName:     "testdb",
			isUpdate:         false,
			listError:        fmt.Errorf("list error"),
			expectFound:      false,
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)

			databaseList := &instancev1alpha1.DatabaseList{
				Items: tt.existingItems,
			}

			if tt.listError != nil {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil, tt.listError)
			} else {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(databaseList, nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.DatabaseList)
					*list = *databaseList
				})
			}

			result, err := CheckDuplicateDatabase(
				context.Background(), mockClient, tt.currentName, tt.currentNamespace, tt.postgresqlID, tt.databaseName, tt.isUpdate)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectFound, result.Found)
				if tt.expectFound {
					assert.Equal(t, "Database", result.Resource)
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// TestCheckDuplicateGrant_TableDriven tests CheckDuplicateGrant with table-driven tests
func TestCheckDuplicateGrant_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		currentName      string
		currentNamespace string
		postgresqlID     string
		role             string
		databaseName     string
		isUpdate         bool
		existingItems    []instancev1alpha1.Grant
		listError        error
		expectFound      bool
		expectError      bool
	}{
		{
			name:             "no duplicate - empty list",
			currentName:      "grant1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			role:             "testrole",
			databaseName:     "testdb",
			isUpdate:         false,
			existingItems:    []instancev1alpha1.Grant{},
			expectFound:      false,
			expectError:      false,
		},
		{
			name:             "duplicate found",
			currentName:      "grant1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			role:             "testrole",
			databaseName:     "testdb",
			isUpdate:         false,
			existingItems: []instancev1alpha1.Grant{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "grant2",
						Namespace: "other-namespace",
					},
					Spec: instancev1alpha1.GrantSpec{
						PostgresqlID: "pg-1",
						Role:         "testrole",
						Database:     "testdb",
					},
				},
			},
			expectFound: true,
			expectError: false,
		},
		{
			name:             "no duplicate - same resource on update",
			currentName:      "grant1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			role:             "testrole",
			databaseName:     "testdb",
			isUpdate:         true,
			existingItems: []instancev1alpha1.Grant{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "grant1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.GrantSpec{
						PostgresqlID: "pg-1",
						Role:         "testrole",
						Database:     "testdb",
					},
				},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name:             "no duplicate - different role",
			currentName:      "grant1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			role:             "testrole",
			databaseName:     "testdb",
			isUpdate:         false,
			existingItems: []instancev1alpha1.Grant{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "grant2",
						Namespace: "default",
					},
					Spec: instancev1alpha1.GrantSpec{
						PostgresqlID: "pg-1",
						Role:         "differentrole",
						Database:     "testdb",
					},
				},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name:             "no duplicate - different database",
			currentName:      "grant1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			role:             "testrole",
			databaseName:     "testdb",
			isUpdate:         false,
			existingItems: []instancev1alpha1.Grant{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "grant2",
						Namespace: "default",
					},
					Spec: instancev1alpha1.GrantSpec{
						PostgresqlID: "pg-1",
						Role:         "testrole",
						Database:     "differentdb",
					},
				},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name:             "list error",
			currentName:      "grant1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			role:             "testrole",
			databaseName:     "testdb",
			isUpdate:         false,
			listError:        fmt.Errorf("list error"),
			expectFound:      false,
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)

			grantList := &instancev1alpha1.GrantList{
				Items: tt.existingItems,
			}

			if tt.listError != nil {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil, tt.listError)
			} else {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(grantList, nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.GrantList)
					*list = *grantList
				})
			}

			result, err := CheckDuplicateGrant(
				context.Background(), mockClient, tt.currentName, tt.currentNamespace, tt.postgresqlID, tt.role, tt.databaseName, tt.isUpdate)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectFound, result.Found)
				if tt.expectFound {
					assert.Equal(t, "Grant", result.Resource)
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// TestCheckDuplicateRoleGroup_TableDriven tests CheckDuplicateRoleGroup with table-driven tests
func TestCheckDuplicateRoleGroup_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		currentName      string
		currentNamespace string
		postgresqlID     string
		groupRole        string
		isUpdate         bool
		existingItems    []instancev1alpha1.RoleGroup
		listError        error
		expectFound      bool
		expectError      bool
	}{
		{
			name:             "no duplicate - empty list",
			currentName:      "rg1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			groupRole:        "testgroup",
			isUpdate:         false,
			existingItems:    []instancev1alpha1.RoleGroup{},
			expectFound:      false,
			expectError:      false,
		},
		{
			name:             "duplicate found",
			currentName:      "rg1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			groupRole:        "testgroup",
			isUpdate:         false,
			existingItems: []instancev1alpha1.RoleGroup{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rg2",
						Namespace: "other-namespace",
					},
					Spec: instancev1alpha1.RoleGroupSpec{
						PostgresqlID: "pg-1",
						GroupRole:    "testgroup",
					},
				},
			},
			expectFound: true,
			expectError: false,
		},
		{
			name:             "no duplicate - same resource on update",
			currentName:      "rg1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			groupRole:        "testgroup",
			isUpdate:         true,
			existingItems: []instancev1alpha1.RoleGroup{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rg1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.RoleGroupSpec{
						PostgresqlID: "pg-1",
						GroupRole:    "testgroup",
					},
				},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name:             "list error",
			currentName:      "rg1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			groupRole:        "testgroup",
			isUpdate:         false,
			listError:        fmt.Errorf("list error"),
			expectFound:      false,
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)

			roleGroupList := &instancev1alpha1.RoleGroupList{
				Items: tt.existingItems,
			}

			if tt.listError != nil {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil, tt.listError)
			} else {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(roleGroupList, nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.RoleGroupList)
					*list = *roleGroupList
				})
			}

			result, err := CheckDuplicateRoleGroup(
				context.Background(), mockClient, tt.currentName, tt.currentNamespace, tt.postgresqlID, tt.groupRole, tt.isUpdate)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectFound, result.Found)
				if tt.expectFound {
					assert.Equal(t, "RoleGroup", result.Resource)
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// TestCheckDuplicateSchema_TableDriven tests CheckDuplicateSchema with table-driven tests
func TestCheckDuplicateSchema_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		currentName      string
		currentNamespace string
		postgresqlID     string
		schemaName       string
		isUpdate         bool
		existingItems    []instancev1alpha1.Schema
		listError        error
		expectFound      bool
		expectError      bool
	}{
		{
			name:             "no duplicate - empty list",
			currentName:      "schema1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			schemaName:       "testschema",
			isUpdate:         false,
			existingItems:    []instancev1alpha1.Schema{},
			expectFound:      false,
			expectError:      false,
		},
		{
			name:             "duplicate found",
			currentName:      "schema1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			schemaName:       "testschema",
			isUpdate:         false,
			existingItems: []instancev1alpha1.Schema{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "schema2",
						Namespace: "other-namespace",
					},
					Spec: instancev1alpha1.SchemaSpec{
						PostgresqlID: "pg-1",
						Schema:       "testschema",
					},
				},
			},
			expectFound: true,
			expectError: false,
		},
		{
			name:             "no duplicate - same resource on update",
			currentName:      "schema1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			schemaName:       "testschema",
			isUpdate:         true,
			existingItems: []instancev1alpha1.Schema{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "schema1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.SchemaSpec{
						PostgresqlID: "pg-1",
						Schema:       "testschema",
					},
				},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name:             "list error",
			currentName:      "schema1",
			currentNamespace: "default",
			postgresqlID:     "pg-1",
			schemaName:       "testschema",
			isUpdate:         false,
			listError:        fmt.Errorf("list error"),
			expectFound:      false,
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)

			schemaList := &instancev1alpha1.SchemaList{
				Items: tt.existingItems,
			}

			if tt.listError != nil {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil, tt.listError)
			} else {
				mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(schemaList, nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.SchemaList)
					*list = *schemaList
				})
			}

			result, err := CheckDuplicateSchema(
				context.Background(), mockClient, tt.currentName, tt.currentNamespace, tt.postgresqlID, tt.schemaName, tt.isUpdate)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectFound, result.Found)
				if tt.expectFound {
					assert.Equal(t, "Schema", result.Resource)
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// TestDuplicateCheckResult_Fields tests DuplicateCheckResult field values
func TestDuplicateCheckResult_Fields(t *testing.T) {
	mockClient := new(MockK8sClient)

	existingPg := instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-pg",
			Namespace: "existing-namespace",
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "test-id",
			},
		},
	}

	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{existingPg},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(postgresqlList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})

	result, err := CheckDuplicatePostgresqlID(
		context.Background(), mockClient, "new-pg", "default", "test-id", false)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Found)
	assert.Equal(t, "PostgreSQL", result.Resource)
	assert.Equal(t, "test-id", result.Fields["postgresqlID"])
	assert.Contains(t, result.Message, "test-id")
	assert.Contains(t, result.Message, "existing-namespace")
	assert.Contains(t, result.Message, "existing-pg")
	assert.Equal(t, "existing-pg", result.Existing.GetName())
	assert.Equal(t, "existing-namespace", result.Existing.GetNamespace())
}
