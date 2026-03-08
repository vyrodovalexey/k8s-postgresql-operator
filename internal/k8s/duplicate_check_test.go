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

// --- CheckDuplicatePostgresqlID tests ---

func TestCheckDuplicatePostgresqlID(t *testing.T) {
	tests := []struct {
		name             string
		currentName      string
		currentNamespace string
		postgresqlID     string
		isUpdate         bool
		setupMock        func(*MockK8sClient)
		wantFound        bool
		wantErr          bool
		errContains      string
	}{
		{
			name:             "duplicate found - different resource",
			currentName:      "pg2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				pgList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
							Spec: instancev1alpha1.PostgresqlSpec{
								ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
									PostgresqlID: "test-id-123",
								},
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(pgList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.PostgresqlList)
						*list = *pgList
					})
			},
			wantFound: true,
			wantErr:   false,
		},
		{
			name:             "no duplicate - empty list",
			currentName:      "pg1",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				pgList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(pgList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.PostgresqlList)
						*list = *pgList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "skip current on update",
			currentName:      "pg1",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			isUpdate:         true,
			setupMock: func(m *MockK8sClient) {
				pgList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
							Spec: instancev1alpha1.PostgresqlSpec{
								ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
									PostgresqlID: "test-id-123",
								},
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(pgList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.PostgresqlList)
						*list = *pgList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "no duplicate - nil external instance",
			currentName:      "pg2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				pgList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
							Spec:       instancev1alpha1.PostgresqlSpec{ExternalInstance: nil},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(pgList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.PostgresqlList)
						*list = *pgList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "no duplicate - different ID",
			currentName:      "pg2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				pgList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
							Spec: instancev1alpha1.PostgresqlSpec{
								ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
									PostgresqlID: "other-id",
								},
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(pgList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.PostgresqlList)
						*list = *pgList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "list error",
			currentName:      "pg1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("list error"))
			},
			wantFound:   false,
			wantErr:     true,
			errContains: "failed to list",
		},
		{
			name:             "duplicate found on create - not skipped",
			currentName:      "pg1",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				pgList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
							Spec: instancev1alpha1.PostgresqlSpec{
								ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
									PostgresqlID: "test-id-123",
								},
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(pgList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.PostgresqlList)
						*list = *pgList
					})
			},
			wantFound: true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)
			tt.setupMock(mockClient)

			result, err := CheckDuplicatePostgresqlID(
				context.Background(), mockClient,
				tt.currentName, tt.currentNamespace, tt.postgresqlID, tt.isUpdate)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantFound, result.Found)
				if tt.wantFound {
					assert.NotNil(t, result.Existing)
					assert.Equal(t, "PostgreSQL", result.Resource)
					assert.Contains(t, result.Message, tt.postgresqlID)
					assert.Equal(t, tt.postgresqlID, result.Fields["postgresqlID"])
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// --- CheckDuplicateUser tests ---

func TestCheckDuplicateUser(t *testing.T) {
	tests := []struct {
		name             string
		currentName      string
		currentNamespace string
		postgresqlID     string
		username         string
		isUpdate         bool
		setupMock        func(*MockK8sClient)
		wantFound        bool
		wantErr          bool
		errContains      string
	}{
		{
			name:             "duplicate found",
			currentName:      "user2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			username:         "testuser",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				userList := &instancev1alpha1.UserList{
					Items: []instancev1alpha1.User{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "user1", Namespace: "default"},
							Spec: instancev1alpha1.UserSpec{
								PostgresqlID: "test-id-123",
								Username:     "testuser",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(userList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.UserList)
						*list = *userList
					})
			},
			wantFound: true,
			wantErr:   false,
		},
		{
			name:             "no duplicate - empty list",
			currentName:      "user1",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			username:         "testuser",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				userList := &instancev1alpha1.UserList{Items: []instancev1alpha1.User{}}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(userList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.UserList)
						*list = *userList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "skip current on update",
			currentName:      "user1",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			username:         "testuser",
			isUpdate:         true,
			setupMock: func(m *MockK8sClient) {
				userList := &instancev1alpha1.UserList{
					Items: []instancev1alpha1.User{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "user1", Namespace: "default"},
							Spec: instancev1alpha1.UserSpec{
								PostgresqlID: "test-id-123",
								Username:     "testuser",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(userList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.UserList)
						*list = *userList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "no duplicate - different postgresqlID",
			currentName:      "user2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			username:         "testuser",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				userList := &instancev1alpha1.UserList{
					Items: []instancev1alpha1.User{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "user1", Namespace: "default"},
							Spec: instancev1alpha1.UserSpec{
								PostgresqlID: "other-id",
								Username:     "testuser",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(userList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.UserList)
						*list = *userList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "no duplicate - different username",
			currentName:      "user2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			username:         "testuser",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				userList := &instancev1alpha1.UserList{
					Items: []instancev1alpha1.User{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "user1", Namespace: "default"},
							Spec: instancev1alpha1.UserSpec{
								PostgresqlID: "test-id-123",
								Username:     "otheruser",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(userList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.UserList)
						*list = *userList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "list error",
			currentName:      "user1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			username:         "testuser",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("list error"))
			},
			wantFound:   false,
			wantErr:     true,
			errContains: "failed to list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)
			tt.setupMock(mockClient)

			result, err := CheckDuplicateUser(
				context.Background(), mockClient,
				tt.currentName, tt.currentNamespace, tt.postgresqlID, tt.username, tt.isUpdate)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantFound, result.Found)
				if tt.wantFound {
					assert.NotNil(t, result.Existing)
					assert.Equal(t, "User", result.Resource)
					assert.Contains(t, result.Message, tt.postgresqlID)
					assert.Contains(t, result.Message, tt.username)
					assert.Equal(t, tt.postgresqlID, result.Fields["postgresqlID"])
					assert.Equal(t, tt.username, result.Fields["username"])
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// --- CheckDuplicateDatabase tests ---

func TestCheckDuplicateDatabase(t *testing.T) {
	tests := []struct {
		name             string
		currentName      string
		currentNamespace string
		postgresqlID     string
		databaseName     string
		isUpdate         bool
		setupMock        func(*MockK8sClient)
		wantFound        bool
		wantErr          bool
		errContains      string
	}{
		{
			name:             "duplicate found",
			currentName:      "db2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			databaseName:     "testdb",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				dbList := &instancev1alpha1.DatabaseList{
					Items: []instancev1alpha1.Database{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "db1", Namespace: "default"},
							Spec: instancev1alpha1.DatabaseSpec{
								PostgresqlID: "test-id-123",
								Database:     "testdb",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(dbList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.DatabaseList)
						*list = *dbList
					})
			},
			wantFound: true,
			wantErr:   false,
		},
		{
			name:             "no duplicate - empty list",
			currentName:      "db1",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			databaseName:     "testdb",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				dbList := &instancev1alpha1.DatabaseList{Items: []instancev1alpha1.Database{}}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(dbList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.DatabaseList)
						*list = *dbList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "skip current on update",
			currentName:      "db1",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			databaseName:     "testdb",
			isUpdate:         true,
			setupMock: func(m *MockK8sClient) {
				dbList := &instancev1alpha1.DatabaseList{
					Items: []instancev1alpha1.Database{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "db1", Namespace: "default"},
							Spec: instancev1alpha1.DatabaseSpec{
								PostgresqlID: "test-id-123",
								Database:     "testdb",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(dbList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.DatabaseList)
						*list = *dbList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "no duplicate - different postgresqlID",
			currentName:      "db2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			databaseName:     "testdb",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				dbList := &instancev1alpha1.DatabaseList{
					Items: []instancev1alpha1.Database{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "db1", Namespace: "default"},
							Spec: instancev1alpha1.DatabaseSpec{
								PostgresqlID: "other-id",
								Database:     "testdb",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(dbList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.DatabaseList)
						*list = *dbList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "no duplicate - different database name",
			currentName:      "db2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			databaseName:     "testdb",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				dbList := &instancev1alpha1.DatabaseList{
					Items: []instancev1alpha1.Database{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "db1", Namespace: "default"},
							Spec: instancev1alpha1.DatabaseSpec{
								PostgresqlID: "test-id-123",
								Database:     "otherdb",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(dbList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.DatabaseList)
						*list = *dbList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "list error",
			currentName:      "db1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			databaseName:     "testdb",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("list error"))
			},
			wantFound:   false,
			wantErr:     true,
			errContains: "failed to list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)
			tt.setupMock(mockClient)

			result, err := CheckDuplicateDatabase(
				context.Background(), mockClient,
				tt.currentName, tt.currentNamespace, tt.postgresqlID, tt.databaseName, tt.isUpdate)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantFound, result.Found)
				if tt.wantFound {
					assert.NotNil(t, result.Existing)
					assert.Equal(t, "Database", result.Resource)
					assert.Contains(t, result.Message, tt.postgresqlID)
					assert.Contains(t, result.Message, tt.databaseName)
					assert.Equal(t, tt.postgresqlID, result.Fields["postgresqlID"])
					assert.Equal(t, tt.databaseName, result.Fields["database"])
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// --- CheckDuplicateGrant tests ---

func TestCheckDuplicateGrant(t *testing.T) {
	tests := []struct {
		name             string
		currentName      string
		currentNamespace string
		postgresqlID     string
		role             string
		databaseName     string
		isUpdate         bool
		setupMock        func(*MockK8sClient)
		wantFound        bool
		wantErr          bool
		errContains      string
	}{
		{
			name:             "duplicate found",
			currentName:      "grant2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			role:             "testrole",
			databaseName:     "testdb",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				grantList := &instancev1alpha1.GrantList{
					Items: []instancev1alpha1.Grant{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "grant1", Namespace: "default"},
							Spec: instancev1alpha1.GrantSpec{
								PostgresqlID: "test-id-123",
								Role:         "testrole",
								Database:     "testdb",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(grantList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.GrantList)
						*list = *grantList
					})
			},
			wantFound: true,
			wantErr:   false,
		},
		{
			name:             "no duplicate - empty list",
			currentName:      "grant1",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			role:             "testrole",
			databaseName:     "testdb",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				grantList := &instancev1alpha1.GrantList{Items: []instancev1alpha1.Grant{}}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(grantList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.GrantList)
						*list = *grantList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "skip current on update",
			currentName:      "grant1",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			role:             "testrole",
			databaseName:     "testdb",
			isUpdate:         true,
			setupMock: func(m *MockK8sClient) {
				grantList := &instancev1alpha1.GrantList{
					Items: []instancev1alpha1.Grant{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "grant1", Namespace: "default"},
							Spec: instancev1alpha1.GrantSpec{
								PostgresqlID: "test-id-123",
								Role:         "testrole",
								Database:     "testdb",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(grantList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.GrantList)
						*list = *grantList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "no duplicate - different postgresqlID",
			currentName:      "grant2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			role:             "testrole",
			databaseName:     "testdb",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				grantList := &instancev1alpha1.GrantList{
					Items: []instancev1alpha1.Grant{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "grant1", Namespace: "default"},
							Spec: instancev1alpha1.GrantSpec{
								PostgresqlID: "other-id",
								Role:         "testrole",
								Database:     "testdb",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(grantList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.GrantList)
						*list = *grantList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "no duplicate - different role",
			currentName:      "grant2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			role:             "testrole",
			databaseName:     "testdb",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				grantList := &instancev1alpha1.GrantList{
					Items: []instancev1alpha1.Grant{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "grant1", Namespace: "default"},
							Spec: instancev1alpha1.GrantSpec{
								PostgresqlID: "test-id-123",
								Role:         "otherrole",
								Database:     "testdb",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(grantList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.GrantList)
						*list = *grantList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "no duplicate - different database",
			currentName:      "grant2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			role:             "testrole",
			databaseName:     "testdb",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				grantList := &instancev1alpha1.GrantList{
					Items: []instancev1alpha1.Grant{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "grant1", Namespace: "default"},
							Spec: instancev1alpha1.GrantSpec{
								PostgresqlID: "test-id-123",
								Role:         "testrole",
								Database:     "otherdb",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(grantList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.GrantList)
						*list = *grantList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "list error",
			currentName:      "grant1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			role:             "testrole",
			databaseName:     "testdb",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("list error"))
			},
			wantFound:   false,
			wantErr:     true,
			errContains: "failed to list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)
			tt.setupMock(mockClient)

			result, err := CheckDuplicateGrant(
				context.Background(), mockClient,
				tt.currentName, tt.currentNamespace, tt.postgresqlID, tt.role, tt.databaseName, tt.isUpdate)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantFound, result.Found)
				if tt.wantFound {
					assert.NotNil(t, result.Existing)
					assert.Equal(t, "Grant", result.Resource)
					assert.Contains(t, result.Message, tt.postgresqlID)
					assert.Contains(t, result.Message, tt.role)
					assert.Contains(t, result.Message, tt.databaseName)
					assert.Equal(t, tt.postgresqlID, result.Fields["postgresqlID"])
					assert.Equal(t, tt.role, result.Fields["role"])
					assert.Equal(t, tt.databaseName, result.Fields["database"])
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// --- CheckDuplicateRoleGroup tests ---

func TestCheckDuplicateRoleGroup(t *testing.T) {
	tests := []struct {
		name             string
		currentName      string
		currentNamespace string
		postgresqlID     string
		groupRole        string
		isUpdate         bool
		setupMock        func(*MockK8sClient)
		wantFound        bool
		wantErr          bool
		errContains      string
	}{
		{
			name:             "duplicate found",
			currentName:      "rg2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			groupRole:        "testgroup",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				rgList := &instancev1alpha1.RoleGroupList{
					Items: []instancev1alpha1.RoleGroup{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "rg1", Namespace: "default"},
							Spec: instancev1alpha1.RoleGroupSpec{
								PostgresqlID: "test-id-123",
								GroupRole:    "testgroup",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(rgList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.RoleGroupList)
						*list = *rgList
					})
			},
			wantFound: true,
			wantErr:   false,
		},
		{
			name:             "no duplicate - empty list",
			currentName:      "rg1",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			groupRole:        "testgroup",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				rgList := &instancev1alpha1.RoleGroupList{Items: []instancev1alpha1.RoleGroup{}}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(rgList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.RoleGroupList)
						*list = *rgList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "skip current on update",
			currentName:      "rg1",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			groupRole:        "testgroup",
			isUpdate:         true,
			setupMock: func(m *MockK8sClient) {
				rgList := &instancev1alpha1.RoleGroupList{
					Items: []instancev1alpha1.RoleGroup{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "rg1", Namespace: "default"},
							Spec: instancev1alpha1.RoleGroupSpec{
								PostgresqlID: "test-id-123",
								GroupRole:    "testgroup",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(rgList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.RoleGroupList)
						*list = *rgList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "no duplicate - different postgresqlID",
			currentName:      "rg2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			groupRole:        "testgroup",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				rgList := &instancev1alpha1.RoleGroupList{
					Items: []instancev1alpha1.RoleGroup{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "rg1", Namespace: "default"},
							Spec: instancev1alpha1.RoleGroupSpec{
								PostgresqlID: "other-id",
								GroupRole:    "testgroup",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(rgList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.RoleGroupList)
						*list = *rgList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "no duplicate - different groupRole",
			currentName:      "rg2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			groupRole:        "testgroup",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				rgList := &instancev1alpha1.RoleGroupList{
					Items: []instancev1alpha1.RoleGroup{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "rg1", Namespace: "default"},
							Spec: instancev1alpha1.RoleGroupSpec{
								PostgresqlID: "test-id-123",
								GroupRole:    "othergroup",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(rgList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.RoleGroupList)
						*list = *rgList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "list error",
			currentName:      "rg1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			groupRole:        "testgroup",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("list error"))
			},
			wantFound:   false,
			wantErr:     true,
			errContains: "failed to list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)
			tt.setupMock(mockClient)

			result, err := CheckDuplicateRoleGroup(
				context.Background(), mockClient,
				tt.currentName, tt.currentNamespace, tt.postgresqlID, tt.groupRole, tt.isUpdate)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantFound, result.Found)
				if tt.wantFound {
					assert.NotNil(t, result.Existing)
					assert.Equal(t, "RoleGroup", result.Resource)
					assert.Contains(t, result.Message, tt.postgresqlID)
					assert.Contains(t, result.Message, tt.groupRole)
					assert.Equal(t, tt.postgresqlID, result.Fields["postgresqlID"])
					assert.Equal(t, tt.groupRole, result.Fields["groupRole"])
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// --- CheckDuplicateSchema tests ---

func TestCheckDuplicateSchema(t *testing.T) {
	tests := []struct {
		name             string
		currentName      string
		currentNamespace string
		postgresqlID     string
		schemaName       string
		isUpdate         bool
		setupMock        func(*MockK8sClient)
		wantFound        bool
		wantErr          bool
		errContains      string
	}{
		{
			name:             "duplicate found",
			currentName:      "schema2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			schemaName:       "testschema",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				schemaList := &instancev1alpha1.SchemaList{
					Items: []instancev1alpha1.Schema{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "schema1", Namespace: "default"},
							Spec: instancev1alpha1.SchemaSpec{
								PostgresqlID: "test-id-123",
								Schema:       "testschema",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(schemaList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.SchemaList)
						*list = *schemaList
					})
			},
			wantFound: true,
			wantErr:   false,
		},
		{
			name:             "no duplicate - empty list",
			currentName:      "schema1",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			schemaName:       "testschema",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				schemaList := &instancev1alpha1.SchemaList{Items: []instancev1alpha1.Schema{}}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(schemaList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.SchemaList)
						*list = *schemaList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "skip current on update",
			currentName:      "schema1",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			schemaName:       "testschema",
			isUpdate:         true,
			setupMock: func(m *MockK8sClient) {
				schemaList := &instancev1alpha1.SchemaList{
					Items: []instancev1alpha1.Schema{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "schema1", Namespace: "default"},
							Spec: instancev1alpha1.SchemaSpec{
								PostgresqlID: "test-id-123",
								Schema:       "testschema",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(schemaList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.SchemaList)
						*list = *schemaList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "no duplicate - different postgresqlID",
			currentName:      "schema2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			schemaName:       "testschema",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				schemaList := &instancev1alpha1.SchemaList{
					Items: []instancev1alpha1.Schema{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "schema1", Namespace: "default"},
							Spec: instancev1alpha1.SchemaSpec{
								PostgresqlID: "other-id",
								Schema:       "testschema",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(schemaList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.SchemaList)
						*list = *schemaList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "no duplicate - different schema name",
			currentName:      "schema2",
			currentNamespace: "default",
			postgresqlID:     "test-id-123",
			schemaName:       "testschema",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				schemaList := &instancev1alpha1.SchemaList{
					Items: []instancev1alpha1.Schema{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "schema1", Namespace: "default"},
							Spec: instancev1alpha1.SchemaSpec{
								PostgresqlID: "test-id-123",
								Schema:       "otherschema",
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(schemaList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.SchemaList)
						*list = *schemaList
					})
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:             "list error",
			currentName:      "schema1",
			currentNamespace: "default",
			postgresqlID:     "test-id",
			schemaName:       "testschema",
			isUpdate:         false,
			setupMock: func(m *MockK8sClient) {
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("list error"))
			},
			wantFound:   false,
			wantErr:     true,
			errContains: "failed to list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)
			tt.setupMock(mockClient)

			result, err := CheckDuplicateSchema(
				context.Background(), mockClient,
				tt.currentName, tt.currentNamespace, tt.postgresqlID, tt.schemaName, tt.isUpdate)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantFound, result.Found)
				if tt.wantFound {
					assert.NotNil(t, result.Existing)
					assert.Equal(t, "Schema", result.Resource)
					assert.Contains(t, result.Message, tt.postgresqlID)
					assert.Contains(t, result.Message, tt.schemaName)
					assert.Equal(t, tt.postgresqlID, result.Fields["postgresqlID"])
					assert.Equal(t, tt.schemaName, result.Fields["schema"])
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}
