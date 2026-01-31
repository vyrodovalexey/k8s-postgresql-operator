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
	ctrl "sigs.k8s.io/controller-runtime"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestSchemaReconciler_Reconcile_EmptyDatabase(t *testing.T) {
	// Arrange
	mockClient := new(MockControllerClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	// Create schema with empty database
	schemaObj := &instancev1alpha1.Schema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schema",
			Namespace: "default",
		},
		Spec: instancev1alpha1.SchemaSpec{
			PostgresqlID: "test-id",
			Schema:       "myschema",
			Owner:        "owner",
			Database:     "", // Empty database
		},
	}
	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{*postgresql},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(schemaObj, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.Schema)
		*obj = *schemaObj
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})
	mockClient.On("Status").Return(mockStatusWriter)
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &SchemaReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			VaultClient:                 nil,
			Log:                         logger,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-schema",
			Namespace: "default",
		},
	}

	// Act
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert - should return empty result when database is not specified
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
}

func TestSchemaReconciler_Reconcile_StatusUpdateErrorOnPostgresqlNotFound(t *testing.T) {
	// Arrange
	mockClient := new(MockControllerClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	schemaObj := createTestSchema("test-schema", "default", "non-existent-id", "myschema", "owner")
	postgresqlList := &instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(schemaObj, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.Schema)
		*obj = *schemaObj
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})
	mockClient.On("Status").Return(mockStatusWriter)
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("status update error"))

	reconciler := &SchemaReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			VaultClient:                 nil,
			Log:                         logger,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-schema",
			Namespace: "default",
		},
	}

	// Act
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert - should still requeue even if status update fails
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
}

func TestSchemaReconciler_Reconcile_StatusUpdateErrorOnPostgresqlNotConnected(t *testing.T) {
	// Arrange
	mockClient := new(MockControllerClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	schemaObj := createTestSchema("test-schema", "default", "test-id", "myschema", "owner")
	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, false)
	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{*postgresql},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(schemaObj, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.Schema)
		*obj = *schemaObj
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})
	mockClient.On("Status").Return(mockStatusWriter)
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("status update error"))

	reconciler := &SchemaReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			VaultClient:                 nil,
			Log:                         logger,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-schema",
			Namespace: "default",
		},
	}

	// Act
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert - should still requeue even if status update fails
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
}

func TestSchemaReconciler_Reconcile_StatusUpdateErrorOnEmptyDatabase(t *testing.T) {
	// Arrange
	mockClient := new(MockControllerClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	// Create schema with empty database
	schemaObj := &instancev1alpha1.Schema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schema",
			Namespace: "default",
		},
		Spec: instancev1alpha1.SchemaSpec{
			PostgresqlID: "test-id",
			Schema:       "myschema",
			Owner:        "owner",
			Database:     "", // Empty database
		},
	}
	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)
	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{*postgresql},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(schemaObj, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.Schema)
		*obj = *schemaObj
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})
	mockClient.On("Status").Return(mockStatusWriter)
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("status update error"))

	reconciler := &SchemaReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			VaultClient:                 nil,
			Log:                         logger,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-schema",
			Namespace: "default",
		},
	}

	// Act
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert - should return empty result even if status update fails
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
}

func TestSchemaReconciler_Reconcile_ListPostgresqlError(t *testing.T) {
	// Arrange
	mockClient := new(MockControllerClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	schemaObj := createTestSchema("test-schema", "default", "test-id", "myschema", "owner")

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(schemaObj, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.Schema)
		*obj = *schemaObj
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("list error"))
	mockClient.On("Status").Return(mockStatusWriter)
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &SchemaReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			VaultClient:                 nil,
			Log:                         logger,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-schema",
			Namespace: "default",
		},
	}

	// Act
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert - should requeue on list error
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
}
