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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestDatabaseReconciler_EnsureFinalizer_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		database       *instancev1alpha1.Database
		setupMocks     func(*MockControllerClient)
		expectedResult ctrl.Result
		expectedDone   bool
	}{
		{
			name: "DeleteFromCRD is false - no finalizer needed",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{Name: "db1", Namespace: "default"},
				Spec:       instancev1alpha1.DatabaseSpec{DeleteFromCRD: false},
			},
			setupMocks:     func(mc *MockControllerClient) {},
			expectedResult: ctrl.Result{},
			expectedDone:   false,
		},
		{
			name: "Finalizer already exists",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "db1",
					Namespace:  "default",
					Finalizers: []string{databaseFinalizerName},
				},
				Spec: instancev1alpha1.DatabaseSpec{DeleteFromCRD: true},
			},
			setupMocks:     func(mc *MockControllerClient) {},
			expectedResult: ctrl.Result{},
			expectedDone:   false,
		},
		{
			name: "Finalizer added successfully",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "db1",
					Namespace:  "default",
					Finalizers: []string{},
				},
				Spec: instancev1alpha1.DatabaseSpec{DeleteFromCRD: true},
			},
			setupMocks: func(mc *MockControllerClient) {
				mc.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{Requeue: true},
			expectedDone:   true,
		},
		{
			name: "Finalizer add fails",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "db1",
					Namespace:  "default",
					Finalizers: []string{},
				},
				Spec: instancev1alpha1.DatabaseSpec{DeleteFromCRD: true},
			},
			setupMocks: func(mc *MockControllerClient) {
				mc.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("update error"))
			},
			expectedResult: ctrl.Result{},
			expectedDone:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockControllerClient)
			tt.setupMocks(mockClient)

			reconciler := &DatabaseReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:   mockClient,
					Log:      zap.NewNop().Sugar(),
					Recorder: record.NewFakeRecorder(100),
				},
			}

			result, done := reconciler.ensureFinalizer(context.Background(), tt.database)
			assert.Equal(t, tt.expectedResult, result)
			assert.Equal(t, tt.expectedDone, done)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestDatabaseReconciler_RemoveDBFinalizer_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		database       *instancev1alpha1.Database
		updateErr      error
		expectedResult ctrl.Result
		expectedError  bool
	}{
		{
			name: "Remove finalizer successfully",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "db1",
					Namespace:  "default",
					Finalizers: []string{databaseFinalizerName},
				},
			},
			updateErr:      nil,
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "Remove finalizer fails",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "db1",
					Namespace:  "default",
					Finalizers: []string{databaseFinalizerName},
				},
			},
			updateErr:      fmt.Errorf("update error"),
			expectedResult: ctrl.Result{},
			expectedError:  true,
		},
		{
			name: "Finalizer not present - still succeeds",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "db1",
					Namespace:  "default",
					Finalizers: []string{},
				},
			},
			updateErr:      nil,
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockControllerClient)
			mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(tt.updateErr)

			reconciler := &DatabaseReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:   mockClient,
					Log:      zap.NewNop().Sugar(),
					Recorder: record.NewFakeRecorder(100),
				},
			}

			result, err := reconciler.removeDBFinalizer(context.Background(), tt.database)
			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestDatabaseReconciler_SyncDatabase_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		database        *instancev1alpha1.Database
		info            *connectionInfo
		statusUpdateErr error
		expectCreated   bool
	}{
		{
			name: "Sync database with default schema - pg operation will fail (no real DB)",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{Name: "db1", Namespace: "default"},
				Spec: instancev1alpha1.DatabaseSpec{
					PostgresqlID: "test-id",
					Database:     "testdb",
					Owner:        "testowner",
					Schema:       "",
				},
			},
			info: &connectionInfo{
				ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
					PostgresqlID: "test-id",
					Address:      "localhost",
				},
				Port:     5432,
				SSLMode:  "require",
				Username: "admin",
				Password: "secret",
			},
			statusUpdateErr: nil,
			expectCreated:   false, // pg.CreateOrUpdateDatabase will fail since no real DB
		},
		{
			name: "Sync database with explicit schema",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{Name: "db1", Namespace: "default"},
				Spec: instancev1alpha1.DatabaseSpec{
					PostgresqlID: "test-id",
					Database:     "testdb",
					Owner:        "testowner",
					Schema:       "myschema",
				},
			},
			info: &connectionInfo{
				ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
					PostgresqlID: "test-id",
					Address:      "localhost",
				},
				Port:     5432,
				SSLMode:  "require",
				Username: "admin",
				Password: "secret",
			},
			statusUpdateErr: nil,
			expectCreated:   false,
		},
		{
			name: "Sync database - status update fails",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{Name: "db1", Namespace: "default"},
				Spec: instancev1alpha1.DatabaseSpec{
					PostgresqlID: "test-id",
					Database:     "testdb",
					Owner:        "testowner",
				},
			},
			info: &connectionInfo{
				ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
					PostgresqlID: "test-id",
					Address:      "localhost",
				},
				Port:     5432,
				SSLMode:  "require",
				Username: "admin",
				Password: "secret",
			},
			statusUpdateErr: fmt.Errorf("status update error"),
			expectCreated:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockControllerClient)
			mockStatus := new(MockStatusWriter)

			mockClient.On("Status").Return(mockStatus)
			mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(tt.statusUpdateErr)

			reconciler := &DatabaseReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:                      mockClient,
					Log:                         zap.NewNop().Sugar(),
					Recorder:                    record.NewFakeRecorder(100),
					PostgresqlConnectionRetries: 1,
					PostgresqlConnectionTimeout: 1 * time.Millisecond,
				},
			}

			err := reconciler.syncDatabase(context.Background(), tt.database, tt.info)

			if tt.statusUpdateErr != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NotNil(t, tt.database.Status.LastSyncAttempt)
			mockClient.AssertExpectations(t)
			mockStatus.AssertExpectations(t)
		})
	}
}

func TestDatabaseReconciler_HandleDeletion_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		database       *instancev1alpha1.Database
		setupMocks     func(*MockControllerClient, *MockStatusWriter)
		expectedResult ctrl.Result
		expectedError  bool
	}{
		{
			name: "DeleteFromCRD false, no finalizer - return empty",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "db1",
					Namespace:  "default",
					Finalizers: []string{},
				},
				Spec: instancev1alpha1.DatabaseSpec{DeleteFromCRD: false},
			},
			setupMocks:     func(mc *MockControllerClient, ms *MockStatusWriter) {},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "DeleteFromCRD false, has finalizer - remove finalizer",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "db1",
					Namespace:  "default",
					Finalizers: []string{databaseFinalizerName},
				},
				Spec: instancev1alpha1.DatabaseSpec{DeleteFromCRD: false},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				mc.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "DeleteFromCRD true, no finalizer - return empty",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "db1",
					Namespace:  "default",
					Finalizers: []string{},
				},
				Spec: instancev1alpha1.DatabaseSpec{DeleteFromCRD: true},
			},
			setupMocks:     func(mc *MockControllerClient, ms *MockStatusWriter) {},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "DeleteFromCRD true, has finalizer, PG not found - remove finalizer",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "db1",
					Namespace:  "default",
					Finalizers: []string{databaseFinalizerName},
				},
				Spec: instancev1alpha1.DatabaseSpec{
					PostgresqlID:  "test-id",
					Database:      "testdb",
					DeleteFromCRD: true,
				},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				// List returns empty - PG not found
				mc.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{}
				})
				mc.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "DeleteFromCRD true, has finalizer, PG found, vault nil - remove finalizer",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "db1",
					Namespace:  "default",
					Finalizers: []string{databaseFinalizerName},
				},
				Spec: instancev1alpha1.DatabaseSpec{
					PostgresqlID:  "test-id",
					Database:      "testdb",
					DeleteFromCRD: true,
				},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
				mc.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{*postgresql}
				})
				mc.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "DeleteFromCRD false, has finalizer - remove finalizer fails",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "db1",
					Namespace:  "default",
					Finalizers: []string{databaseFinalizerName},
				},
				Spec: instancev1alpha1.DatabaseSpec{DeleteFromCRD: false},
			},
			setupMocks: func(mc *MockControllerClient, ms *MockStatusWriter) {
				mc.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("update error"))
			},
			expectedResult: ctrl.Result{},
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockControllerClient)
			mockStatus := new(MockStatusWriter)
			tt.setupMocks(mockClient, mockStatus)

			reconciler := &DatabaseReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:                      mockClient,
					VaultClient:                 nil,
					Log:                         zap.NewNop().Sugar(),
					Recorder:                    record.NewFakeRecorder(100),
					PostgresqlConnectionRetries: 1,
					PostgresqlConnectionTimeout: 1 * time.Millisecond,
					VaultAvailabilityRetries:    1,
					VaultAvailabilityRetryDelay: 1 * time.Millisecond,
				},
			}

			result, err := reconciler.handleDeletion(context.Background(), tt.database)
			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockClient.AssertExpectations(t)
			mockStatus.AssertExpectations(t)
		})
	}
}

func TestDatabaseReconciler_ResolveDeletionConnection_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		database      *instancev1alpha1.Database
		setupMocks    func(*MockControllerClient)
		expectNilConn bool
		expectResult  bool
	}{
		{
			name: "PG not found - returns early result",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{Name: "db1", Namespace: "default"},
				Spec: instancev1alpha1.DatabaseSpec{
					PostgresqlID: "non-existent",
				},
			},
			setupMocks: func(mc *MockControllerClient) {
				mc.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{}
				})
				mc.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectNilConn: true,
			expectResult:  true,
		},
		{
			name: "PG found but vault nil - returns early result",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{Name: "db1", Namespace: "default"},
				Spec: instancev1alpha1.DatabaseSpec{
					PostgresqlID: "test-id",
				},
			},
			setupMocks: func(mc *MockControllerClient) {
				postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
				mc.On("List", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					list := args.Get(1).(*instancev1alpha1.PostgresqlList)
					list.Items = []instancev1alpha1.Postgresql{*postgresql}
				})
				mc.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectNilConn: true,
			expectResult:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockControllerClient)
			tt.setupMocks(mockClient)

			reconciler := &DatabaseReconciler{
				BaseReconcilerConfig: BaseReconcilerConfig{
					Client:                      mockClient,
					VaultClient:                 nil,
					Log:                         zap.NewNop().Sugar(),
					Recorder:                    record.NewFakeRecorder(100),
					VaultAvailabilityRetries:    1,
					VaultAvailabilityRetryDelay: 1 * time.Millisecond,
				},
			}

			connInfo, earlyResult, _ := reconciler.resolveDeletionConnection(context.Background(), tt.database)

			if tt.expectNilConn {
				assert.Nil(t, connInfo)
			}
			if tt.expectResult {
				assert.NotNil(t, earlyResult)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestDatabaseReconciler_Reconcile_StatusUpdateFails(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	database := createTestDatabase("test-database", "default", "non-existent-id", "testdb", "testowner", false)

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(database, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.Database)
		*obj = *database
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{}
	})
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("status update error"))

	reconciler := &DatabaseReconciler{
		BaseReconcilerConfig: getBaseReconcilerConfig(mockClient, nil),
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-database", Namespace: "default"},
	})

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}

func TestDatabaseReconciler_HandleDeletion_DeleteFromCRD_PGNotConnected(t *testing.T) {
	mockClient := new(MockControllerClient)

	now := metav1.Now()
	database := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "db1",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{databaseFinalizerName},
		},
		Spec: instancev1alpha1.DatabaseSpec{
			PostgresqlID:  "test-id",
			Database:      "testdb",
			DeleteFromCRD: true,
		},
	}

	// PG not connected
	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, false)
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{*postgresql}
	})
	mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &DatabaseReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			VaultClient:                 nil,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 1 * time.Millisecond,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	result, err := reconciler.handleDeletion(context.Background(), database)
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
}

func TestDatabaseReconciler_HandleDeletion_RemoveFinalizerFails(t *testing.T) {
	mockClient := new(MockControllerClient)

	now := metav1.Now()
	database := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "db1",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{databaseFinalizerName},
		},
		Spec: instancev1alpha1.DatabaseSpec{
			PostgresqlID:  "test-id",
			Database:      "testdb",
			DeleteFromCRD: true,
		},
	}

	// PG not found
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{}
	})
	mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("update error"))

	reconciler := &DatabaseReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			VaultClient:                 nil,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	result, err := reconciler.handleDeletion(context.Background(), database)
	assert.Error(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
}

func TestDatabaseReconciler_Reconcile_DeletionTimestamp_DeleteFromCRDFalse_NoFinalizer(t *testing.T) {
	mockClient := new(MockControllerClient)

	now := metav1.Now()
	database := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "db1",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{},
		},
		Spec: instancev1alpha1.DatabaseSpec{
			PostgresqlID:  "test-id",
			Database:      "testdb",
			DeleteFromCRD: false,
		},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(database, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.Database)
		*obj = *database
	})

	reconciler := &DatabaseReconciler{
		BaseReconcilerConfig: getBaseReconcilerConfig(mockClient, nil),
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "db1", Namespace: "default"},
	})

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
}

func TestDatabaseReconciler_Reconcile_DeletionTimestamp_DeleteFromCRDTrue_NoFinalizer(t *testing.T) {
	mockClient := new(MockControllerClient)

	now := metav1.Now()
	database := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "db1",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{},
		},
		Spec: instancev1alpha1.DatabaseSpec{
			PostgresqlID:  "test-id",
			Database:      "testdb",
			DeleteFromCRD: true,
		},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(database, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.Database)
		*obj = *database
	})

	reconciler := &DatabaseReconciler{
		BaseReconcilerConfig: getBaseReconcilerConfig(mockClient, nil),
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "db1", Namespace: "default"},
	})

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
}

func TestDatabaseReconciler_Reconcile_WithConnectedPG_NoVault(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	database := createTestDatabase("test-database", "default", "test-id", "testdb", "testowner", false)
	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(database, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.Database)
		*obj = *database
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{*postgresql}
	})
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &DatabaseReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			VaultClient:                 nil,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 1 * time.Millisecond,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-database", Namespace: "default"},
	})

	// syncDatabase will be called, pg operation will fail (no real DB), but status update succeeds
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}
