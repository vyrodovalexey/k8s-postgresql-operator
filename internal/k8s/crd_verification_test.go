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

// --- FindPostgresqlByID tests ---

func TestFindPostgresqlByID(t *testing.T) {
	tests := []struct {
		name         string
		postgresqlID string
		setupMock    func(*MockK8sClient)
		wantErr      bool
		errContains  string
		wantName     string
	}{
		{
			name:         "success - found by ID",
			postgresqlID: "test-id-123",
			setupMock: func(m *MockK8sClient) {
				pgList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
							Spec: instancev1alpha1.PostgresqlSpec{
								ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
									PostgresqlID: "test-id-123",
									Address:      "localhost",
									Port:         5432,
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
			wantErr:  false,
			wantName: "pg1",
		},
		{
			name:         "not found - empty list",
			postgresqlID: "non-existent-id",
			setupMock: func(m *MockK8sClient) {
				pgList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(pgList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.PostgresqlList)
						*list = *pgList
					})
			},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:         "not found - no external instance",
			postgresqlID: "test-id",
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
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:         "not found - different ID",
			postgresqlID: "test-id",
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
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:         "list error",
			postgresqlID: "test-id",
			setupMock: func(m *MockK8sClient) {
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("list error"))
			},
			wantErr:     true,
			errContains: "failed to list",
		},
		{
			name:         "success - found among multiple items",
			postgresqlID: "target-id",
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
						{
							ObjectMeta: metav1.ObjectMeta{Name: "pg2", Namespace: "default"},
							Spec:       instancev1alpha1.PostgresqlSpec{ExternalInstance: nil},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "pg3", Namespace: "ns2"},
							Spec: instancev1alpha1.PostgresqlSpec{
								ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
									PostgresqlID: "target-id",
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
			wantErr:  false,
			wantName: "pg3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)
			tt.setupMock(mockClient)

			result, err := FindPostgresqlByID(context.Background(), mockClient, tt.postgresqlID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantName, result.Name)
				assert.Equal(t, tt.postgresqlID, result.Spec.ExternalInstance.PostgresqlID)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// --- FindDatabaseByPostgresqlIDAndName tests ---

func TestFindDatabaseByPostgresqlIDAndName(t *testing.T) {
	tests := []struct {
		name         string
		postgresqlID string
		databaseName string
		setupMock    func(*MockK8sClient)
		wantErr      bool
		errContains  string
		wantName     string
	}{
		{
			name:         "success - found",
			postgresqlID: "test-id-123",
			databaseName: "testdb",
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
			wantErr:  false,
			wantName: "db1",
		},
		{
			name:         "not found - empty list",
			postgresqlID: "test-id-123",
			databaseName: "nonexistent",
			setupMock: func(m *MockK8sClient) {
				dbList := &instancev1alpha1.DatabaseList{Items: []instancev1alpha1.Database{}}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(dbList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.DatabaseList)
						*list = *dbList
					})
			},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:         "not found - different postgresqlID",
			postgresqlID: "test-id-123",
			databaseName: "testdb",
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
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:         "not found - different database name",
			postgresqlID: "test-id-123",
			databaseName: "testdb",
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
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:         "list error",
			postgresqlID: "test-id",
			databaseName: "testdb",
			setupMock: func(m *MockK8sClient) {
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("list error"))
			},
			wantErr:     true,
			errContains: "failed to list",
		},
		{
			name:         "success - found among multiple items",
			postgresqlID: "target-id",
			databaseName: "targetdb",
			setupMock: func(m *MockK8sClient) {
				dbList := &instancev1alpha1.DatabaseList{
					Items: []instancev1alpha1.Database{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "db1", Namespace: "default"},
							Spec: instancev1alpha1.DatabaseSpec{
								PostgresqlID: "other-id",
								Database:     "otherdb",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{Name: "db2", Namespace: "ns2"},
							Spec: instancev1alpha1.DatabaseSpec{
								PostgresqlID: "target-id",
								Database:     "targetdb",
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
			wantErr:  false,
			wantName: "db2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)
			tt.setupMock(mockClient)

			result, err := FindDatabaseByPostgresqlIDAndName(
				context.Background(), mockClient, tt.postgresqlID, tt.databaseName)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantName, result.Name)
				assert.Equal(t, tt.postgresqlID, result.Spec.PostgresqlID)
				assert.Equal(t, tt.databaseName, result.Spec.Database)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// --- DatabaseExistsByPostgresqlIDAndName tests ---

func TestDatabaseExistsByPostgresqlIDAndName(t *testing.T) {
	tests := []struct {
		name         string
		postgresqlID string
		databaseName string
		setupMock    func(*MockK8sClient)
		wantExists   bool
		wantErr      bool
		errContains  string
	}{
		{
			name:         "exists",
			postgresqlID: "test-id-123",
			databaseName: "testdb",
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
			wantExists: true,
			wantErr:    false,
		},
		{
			name:         "not exists - empty list",
			postgresqlID: "test-id-123",
			databaseName: "nonexistent",
			setupMock: func(m *MockK8sClient) {
				dbList := &instancev1alpha1.DatabaseList{Items: []instancev1alpha1.Database{}}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(dbList, nil).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*instancev1alpha1.DatabaseList)
						*list = *dbList
					})
			},
			wantExists: false,
			wantErr:    false,
		},
		{
			name:         "not exists - different postgresqlID",
			postgresqlID: "test-id-123",
			databaseName: "testdb",
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
			wantExists: false,
			wantErr:    false,
		},
		{
			name:         "not exists - different database name",
			postgresqlID: "test-id-123",
			databaseName: "testdb",
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
			wantExists: false,
			wantErr:    false,
		},
		{
			name:         "list error",
			postgresqlID: "test-id-123",
			databaseName: "testdb",
			setupMock: func(m *MockK8sClient) {
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("list error"))
			},
			wantExists:  false,
			wantErr:     true,
			errContains: "failed to list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockK8sClient)
			tt.setupMock(mockClient)

			exists, err := DatabaseExistsByPostgresqlIDAndName(
				context.Background(), mockClient, tt.postgresqlID, tt.databaseName)

			if tt.wantErr {
				assert.Error(t, err)
				assert.False(t, exists)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantExists, exists)
			}
			mockClient.AssertExpectations(t)
		})
	}
}
