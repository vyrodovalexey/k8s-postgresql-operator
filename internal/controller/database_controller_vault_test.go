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
	"encoding/json"
	"fmt"
	"net/http"
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

// ---------- resolveDeletionConnection with vault client ----------

func TestDatabaseReconciler_ResolveDeletionConnection_WithVault_Success(t *testing.T) {
	mockClient := new(MockControllerClient)

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"login":    "pgadmin",
						"password": "supersecret",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{*postgresql}
	})

	database := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "db1", Namespace: "default"},
		Spec: instancev1alpha1.DatabaseSpec{
			PostgresqlID: "test-id",
			Database:     "testdb",
		},
	}

	reconciler := &DatabaseReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	connInfo, earlyResult, earlyErr := reconciler.resolveDeletionConnection(context.Background(), database)

	assert.NotNil(t, connInfo)
	assert.Nil(t, earlyResult)
	assert.NoError(t, earlyErr)
	assert.Equal(t, "pgadmin", connInfo.Username)
	assert.Equal(t, "supersecret", connInfo.Password)
	mockClient.AssertExpectations(t)
}

func TestDatabaseReconciler_ResolveDeletionConnection_WithVault_VaultFails(t *testing.T) {
	mockClient := new(MockControllerClient)

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"vault error"},
			})
		},
	})
	defer srv.Close()

	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{*postgresql}
	})
	mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	database := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "db1",
			Namespace:  "default",
			Finalizers: []string{databaseFinalizerName},
		},
		Spec: instancev1alpha1.DatabaseSpec{
			PostgresqlID: "test-id",
			Database:     "testdb",
		},
	}

	reconciler := &DatabaseReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	connInfo, earlyResult, earlyErr := reconciler.resolveDeletionConnection(context.Background(), database)

	// When vault fails, it removes the finalizer and returns early
	assert.Nil(t, connInfo)
	assert.NotNil(t, earlyResult)
	assert.NoError(t, earlyErr)
	mockClient.AssertExpectations(t)
}

// ---------- handleDeletion with vault client ----------

func TestDatabaseReconciler_HandleDeletion_WithVault_DeleteFromCRD_PGFound_VaultSuccess(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"login":    "pgadmin",
						"password": "supersecret",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

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

	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{*postgresql}
	})
	// pg.DeleteDatabase will fail (no real DB), so status update and requeue
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &DatabaseReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 1 * time.Millisecond,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	result, err := reconciler.handleDeletion(context.Background(), database)

	// pg.DeleteDatabase fails (no real DB), so it requeues
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}

func TestDatabaseReconciler_HandleDeletion_WithVault_DeleteFails_StatusUpdateFails(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"login":    "pgadmin",
						"password": "supersecret",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

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

	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{*postgresql}
	})
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("status update error"))

	reconciler := &DatabaseReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 1 * time.Millisecond,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	result, err := reconciler.handleDeletion(context.Background(), database)

	// pg.DeleteDatabase fails, status update also fails, but still requeues
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}

// ---------- Full Reconcile with vault client ----------

func TestDatabaseReconciler_Reconcile_WithVault_ConnectedPG(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"login":    "admin",
						"password": "adminpass",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

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
			VaultClient:                 vaultClient,
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

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}

func TestDatabaseReconciler_Reconcile_WithDeletion_VaultSuccess_PGDeleteFails(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"login":    "admin",
						"password": "adminpass",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

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
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 1 * time.Millisecond,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "db1", Namespace: "default"},
	})

	// pg.DeleteDatabase fails (no real DB), requeues
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}
