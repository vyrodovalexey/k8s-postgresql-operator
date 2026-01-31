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

// TestFindPostgresqlByID_TableDriven tests FindPostgresqlByID with table-driven tests
func TestFindPostgresqlByID_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		postgresqlID  string
		existingItems []instancev1alpha1.Postgresql
		listError     error
		expectFound   bool
		expectError   bool
		errorMsg      string
	}{
		{
			name:         "found - single item",
			postgresqlID: "test-id",
			existingItems: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
							Address:      "localhost",
							Port:         5432,
						},
					},
				},
			},
			expectFound: true,
			expectError: false,
		},
		{
			name:         "found - multiple items",
			postgresqlID: "test-id-2",
			existingItems: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id-1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg2",
						Namespace: "other-namespace",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id-2",
						},
					},
				},
			},
			expectFound: true,
			expectError: false,
		},
		{
			name:          "not found - empty list",
			postgresqlID:  "test-id",
			existingItems: []instancev1alpha1.Postgresql{},
			expectFound:   false,
			expectError:   true,
			errorMsg:      "not found",
		},
		{
			name:         "not found - no matching ID",
			postgresqlID: "test-id",
			existingItems: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg1",
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
			expectError: true,
			errorMsg:    "not found",
		},
		{
			name:         "not found - nil external instance",
			postgresqlID: "test-id",
			existingItems: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: nil,
					},
				},
			},
			expectFound: false,
			expectError: true,
			errorMsg:    "not found",
		},
		{
			name:         "list error",
			postgresqlID: "test-id",
			listError:    fmt.Errorf("list error"),
			expectFound:  false,
			expectError:  true,
			errorMsg:     "failed to list",
		},
		{
			name:         "found - first match returned",
			postgresqlID: "test-id",
			existingItems: []instancev1alpha1.Postgresql{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg1",
						Namespace: "namespace1",
					},
					Spec: instancev1alpha1.PostgresqlSpec{
						ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
							PostgresqlID: "test-id",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pg2",
						Namespace: "namespace2",
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

			result, err := FindPostgresqlByID(context.Background(), mockClient, tt.postgresqlID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.postgresqlID, result.Spec.ExternalInstance.PostgresqlID)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// TestFindDatabaseByPostgresqlIDAndName_TableDriven tests FindDatabaseByPostgresqlIDAndName with table-driven tests
func TestFindDatabaseByPostgresqlIDAndName_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		postgresqlID  string
		databaseName  string
		existingItems []instancev1alpha1.Database
		listError     error
		expectFound   bool
		expectError   bool
		errorMsg      string
	}{
		{
			name:         "found - single item",
			postgresqlID: "pg-1",
			databaseName: "testdb",
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
			expectFound: true,
			expectError: false,
		},
		{
			name:         "found - multiple items",
			postgresqlID: "pg-1",
			databaseName: "testdb2",
			existingItems: []instancev1alpha1.Database{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "db1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID: "pg-1",
						Database:     "testdb1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "db2",
						Namespace: "other-namespace",
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID: "pg-1",
						Database:     "testdb2",
					},
				},
			},
			expectFound: true,
			expectError: false,
		},
		{
			name:          "not found - empty list",
			postgresqlID:  "pg-1",
			databaseName:  "testdb",
			existingItems: []instancev1alpha1.Database{},
			expectFound:   false,
			expectError:   true,
			errorMsg:      "not found",
		},
		{
			name:         "not found - different postgresqlID",
			postgresqlID: "pg-1",
			databaseName: "testdb",
			existingItems: []instancev1alpha1.Database{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "db1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID: "pg-2",
						Database:     "testdb",
					},
				},
			},
			expectFound: false,
			expectError: true,
			errorMsg:    "not found",
		},
		{
			name:         "not found - different database name",
			postgresqlID: "pg-1",
			databaseName: "testdb",
			existingItems: []instancev1alpha1.Database{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "db1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID: "pg-1",
						Database:     "differentdb",
					},
				},
			},
			expectFound: false,
			expectError: true,
			errorMsg:    "not found",
		},
		{
			name:         "list error",
			postgresqlID: "pg-1",
			databaseName: "testdb",
			listError:    fmt.Errorf("list error"),
			expectFound:  false,
			expectError:  true,
			errorMsg:     "failed to list",
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

			result, err := FindDatabaseByPostgresqlIDAndName(context.Background(), mockClient, tt.postgresqlID, tt.databaseName)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.postgresqlID, result.Spec.PostgresqlID)
				assert.Equal(t, tt.databaseName, result.Spec.Database)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// TestDatabaseExistsByPostgresqlIDAndName_TableDriven tests DatabaseExistsByPostgresqlIDAndName with table-driven tests
func TestDatabaseExistsByPostgresqlIDAndName_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		postgresqlID  string
		databaseName  string
		existingItems []instancev1alpha1.Database
		listError     error
		expectExists  bool
		expectError   bool
		errorMsg      string
	}{
		{
			name:         "exists - single item",
			postgresqlID: "pg-1",
			databaseName: "testdb",
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
			expectExists: true,
			expectError:  false,
		},
		{
			name:         "exists - multiple items",
			postgresqlID: "pg-1",
			databaseName: "testdb2",
			existingItems: []instancev1alpha1.Database{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "db1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID: "pg-1",
						Database:     "testdb1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "db2",
						Namespace: "other-namespace",
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID: "pg-1",
						Database:     "testdb2",
					},
				},
			},
			expectExists: true,
			expectError:  false,
		},
		{
			name:          "not exists - empty list",
			postgresqlID:  "pg-1",
			databaseName:  "testdb",
			existingItems: []instancev1alpha1.Database{},
			expectExists:  false,
			expectError:   false,
		},
		{
			name:         "not exists - different postgresqlID",
			postgresqlID: "pg-1",
			databaseName: "testdb",
			existingItems: []instancev1alpha1.Database{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "db1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID: "pg-2",
						Database:     "testdb",
					},
				},
			},
			expectExists: false,
			expectError:  false,
		},
		{
			name:         "not exists - different database name",
			postgresqlID: "pg-1",
			databaseName: "testdb",
			existingItems: []instancev1alpha1.Database{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "db1",
						Namespace: "default",
					},
					Spec: instancev1alpha1.DatabaseSpec{
						PostgresqlID: "pg-1",
						Database:     "differentdb",
					},
				},
			},
			expectExists: false,
			expectError:  false,
		},
		{
			name:         "list error",
			postgresqlID: "pg-1",
			databaseName: "testdb",
			listError:    fmt.Errorf("list error"),
			expectExists: false,
			expectError:  true,
			errorMsg:     "failed to list",
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

			exists, err := DatabaseExistsByPostgresqlIDAndName(context.Background(), mockClient, tt.postgresqlID, tt.databaseName)

			if tt.expectError {
				assert.Error(t, err)
				assert.False(t, exists)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectExists, exists)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// TestFindPostgresqlByID_ReturnedValues tests that FindPostgresqlByID returns correct values
func TestFindPostgresqlByID_ReturnedValues(t *testing.T) {
	mockClient := new(MockK8sClient)

	expectedPg := instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pg",
			Namespace: "test-namespace",
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "test-id",
				Address:      "postgres.example.com",
				Port:         5432,
				SSLMode:      "require",
			},
		},
	}

	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{expectedPg},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(postgresqlList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})

	result, err := FindPostgresqlByID(context.Background(), mockClient, "test-id")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-pg", result.Name)
	assert.Equal(t, "test-namespace", result.Namespace)
	assert.Equal(t, "test-id", result.Spec.ExternalInstance.PostgresqlID)
	assert.Equal(t, "postgres.example.com", result.Spec.ExternalInstance.Address)
	assert.Equal(t, int32(5432), result.Spec.ExternalInstance.Port)
	assert.Equal(t, "require", result.Spec.ExternalInstance.SSLMode)
}

// TestFindDatabaseByPostgresqlIDAndName_ReturnedValues tests that FindDatabaseByPostgresqlIDAndName returns correct values
func TestFindDatabaseByPostgresqlIDAndName_ReturnedValues(t *testing.T) {
	mockClient := new(MockK8sClient)

	expectedDb := instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "test-namespace",
		},
		Spec: instancev1alpha1.DatabaseSpec{
			PostgresqlID: "pg-1",
			Database:     "testdb",
		},
	}

	databaseList := &instancev1alpha1.DatabaseList{
		Items: []instancev1alpha1.Database{expectedDb},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(databaseList, nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.DatabaseList)
		*list = *databaseList
	})

	result, err := FindDatabaseByPostgresqlIDAndName(context.Background(), mockClient, "pg-1", "testdb")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-db", result.Name)
	assert.Equal(t, "test-namespace", result.Namespace)
	assert.Equal(t, "pg-1", result.Spec.PostgresqlID)
	assert.Equal(t, "testdb", result.Spec.Database)
}
