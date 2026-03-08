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

func TestRoleGroupReconciler_Reconcile_WithVault_VaultFails(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"vault error"},
			})
		},
	})
	defer srv.Close()

	rg := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "rg1", Namespace: "default"},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: "test-id",
			GroupRole:    "testrole",
			MemberRoles:  []string{"member1"},
		},
	}
	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(rg, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.RoleGroup)
		*obj = *rg
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{*postgresql}
	})
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &RoleGroupReconciler{
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
		NamespacedName: types.NamespacedName{Name: "rg1", Namespace: "default"},
	})

	// When vault fails for rolegroup, it returns VaultError condition and requeues
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}

func TestRoleGroupReconciler_Reconcile_WithVault_VaultFails_StatusUpdateFails(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"vault error"},
			})
		},
	})
	defer srv.Close()

	rg := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "rg1", Namespace: "default"},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: "test-id",
			GroupRole:    "testrole",
			MemberRoles:  []string{"member1"},
		},
	}
	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(rg, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.RoleGroup)
		*obj = *rg
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{*postgresql}
	})
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("status update error"))

	reconciler := &RoleGroupReconciler{
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
		NamespacedName: types.NamespacedName{Name: "rg1", Namespace: "default"},
	})

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}
