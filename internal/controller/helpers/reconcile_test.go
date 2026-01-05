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

package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// MockClient is a mock implementation of client.Client
type MockClient struct {
	mock.Mock
	client.Client
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := m.Called(ctx, list, opts)
	if args.Get(0) != nil {
		return args.Error(0)
	}
	return nil
}

func TestRequeueAfter(t *testing.T) {
	duration := 30 * time.Second
	result, err := RequeueAfter(duration)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestFindAndValidatePostgresql(t *testing.T) {
	tests := []struct {
		name          string
		postgresqlID  string
		setupMock     func(*MockClient, string)
		expectedError bool
		checkResult   func(*testing.T, *PostgresqlInstanceInfo, error)
	}{
		{
			name:         "Success - PostgreSQL found and connected",
			postgresqlID: "test-id",
			setupMock: func(m *MockClient, id string) {
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pg1",
								Namespace: "default",
							},
							Spec: instancev1alpha1.PostgresqlSpec{
								ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
									PostgresqlID: id,
									Address:      "localhost",
									Port:         5432,
									SSLMode:      "require",
								},
							},
							Status: instancev1alpha1.PostgresqlStatus{
								Connected: true,
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
			},
			expectedError: false,
			checkResult: func(t *testing.T, info *PostgresqlInstanceInfo, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, info)
				assert.NotNil(t, info.Postgresql)
				assert.NotNil(t, info.ExternalInstance)
				assert.Equal(t, int32(5432), info.Port)
				assert.Equal(t, "require", info.SSLMode)
			},
		},
		{
			name:         "Error - PostgreSQL not found",
			postgresqlID: "non-existent",
			setupMock: func(m *MockClient, id string) {
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
			},
			expectedError: true,
		},
		{
			name:         "Error - PostgreSQL not connected",
			postgresqlID: "test-id",
			setupMock: func(m *MockClient, id string) {
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pg1",
								Namespace: "default",
							},
							Spec: instancev1alpha1.PostgresqlSpec{
								ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
									PostgresqlID: id,
									Address:      "localhost",
									Port:         5432,
								},
							},
							Status: instancev1alpha1.PostgresqlStatus{
								Connected: false,
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
			},
			expectedError: true,
		},
		{
			name:         "Error - No external instance configuration",
			postgresqlID: "test-id",
			setupMock: func(m *MockClient, id string) {
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pg1",
								Namespace: "default",
							},
							Spec: instancev1alpha1.PostgresqlSpec{
								ExternalInstance: nil,
							},
							Status: instancev1alpha1.PostgresqlStatus{
								Connected: true,
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
			},
			expectedError: true,
		},
		{
			name:         "Success - Default port and SSL mode",
			postgresqlID: "test-id",
			setupMock: func(m *MockClient, id string) {
				postgresqlList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pg1",
								Namespace: "default",
							},
							Spec: instancev1alpha1.PostgresqlSpec{
								ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
									PostgresqlID: id,
									Address:      "localhost",
									Port:         0,  // Should use default
									SSLMode:      "", // Should use default
								},
							},
							Status: instancev1alpha1.PostgresqlStatus{
								Connected: true,
							},
						},
					},
				}
				m.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					*list = *postgresqlList
				})
			},
			expectedError: false,
			checkResult: func(t *testing.T, info *PostgresqlInstanceInfo, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, info)
				// Port should default to 5432
				assert.Equal(t, int32(5432), info.Port)
				// SSLMode should default to "require"
				assert.Equal(t, "require", info.SSLMode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockClient)
			tt.setupMock(mockClient, tt.postgresqlID)

			info, err := FindAndValidatePostgresql(context.Background(), mockClient, tt.postgresqlID)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, info)
			} else {
				if tt.checkResult != nil {
					tt.checkResult(t, info, err)
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}

// Add a test for List error
func TestFindAndValidatePostgresql_ListError(t *testing.T) {
	mockClient := new(MockClient)
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(assert.AnError)

	info, err := FindAndValidatePostgresql(context.Background(), mockClient, "test-id")
	assert.Error(t, err)
	assert.Nil(t, info)
	mockClient.AssertExpectations(t)
}
