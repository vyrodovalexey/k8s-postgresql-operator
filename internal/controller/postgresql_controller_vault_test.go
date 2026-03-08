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
	"net/http"
	"sync/atomic"
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

func TestPostgresqlReconciler_Reconcile_WithVault_Success(t *testing.T) {
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

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "test-id",
				Address:      "localhost",
				Port:         5432,
				SSLMode:      "require",
			},
		},
		Status: instancev1alpha1.PostgresqlStatus{Connected: false},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(postgresql, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.Postgresql)
		*obj = *postgresql
	})
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &PostgresqlReconciler{
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
		NamespacedName: types.NamespacedName{Name: "pg1", Namespace: "default"},
	})

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}

func TestPostgresqlReconciler_Reconcile_WithVault_VaultFails(t *testing.T) {
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

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "test-id",
				Address:      "localhost",
				Port:         5432,
				SSLMode:      "require",
			},
		},
		Status: instancev1alpha1.PostgresqlStatus{Connected: false},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(postgresql, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.Postgresql)
		*obj = *postgresql
	})
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &PostgresqlReconciler{
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
		NamespacedName: types.NamespacedName{Name: "pg1", Namespace: "default"},
	})

	// Even if vault fails, the reconciler continues with empty credentials
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}

// ---------- resolveInstanceCredentials tests ----------

func TestResolveInstanceCredentials_InstanceAdminExists(t *testing.T) {
	// instance_admin found in Vault, returns creds directly
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"login":    "instance-admin",
						"password": "instance-pass",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	reconciler := &PostgresqlReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	login, password, usedDefault := reconciler.resolveInstanceCredentials(
		context.Background(), "test-id")

	assert.Equal(t, "instance-admin", login)
	assert.Equal(t, "instance-pass", password)
	assert.False(t, usedDefault)
}

func TestResolveInstanceCredentials_FallbackToDefault(t *testing.T) {
	// instance_admin returns 404, default credentials found
	var callCount int32
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&callCount, 1)
			if count <= 1 {
				// First call: instance_admin not found
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{"secret not found"},
				})
				return
			}
			// Second call: default credentials found
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"login":    "default-admin",
						"password": "default-pass",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	reconciler := &PostgresqlReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	login, password, usedDefault := reconciler.resolveInstanceCredentials(
		context.Background(), "test-id")

	assert.Equal(t, "default-admin", login)
	assert.Equal(t, "default-pass", password)
	assert.True(t, usedDefault)
}

func TestResolveInstanceCredentials_BothNotFound(t *testing.T) {
	// Both instance_admin and default return 404
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"secret not found"},
			})
		},
	})
	defer srv.Close()

	reconciler := &PostgresqlReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	login, password, usedDefault := reconciler.resolveInstanceCredentials(
		context.Background(), "test-id")

	assert.Empty(t, login)
	assert.Empty(t, password)
	assert.False(t, usedDefault)
}

func TestResolveInstanceCredentials_VaultError(t *testing.T) {
	// Non-404 error from Vault - should not fallback to default
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"permission denied"},
			})
		},
	})
	defer srv.Close()

	reconciler := &PostgresqlReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	login, password, usedDefault := reconciler.resolveInstanceCredentials(
		context.Background(), "test-id")

	assert.Empty(t, login)
	assert.Empty(t, password)
	assert.False(t, usedDefault)
}

// ---------- storeInstanceAdminCredentials tests ----------

func TestStoreInstanceAdminCredentials_Success(t *testing.T) {
	storeHandler := func(w http.ResponseWriter, r *http.Request) {
		vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"version": 1,
			},
		})
	}

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"PUT /v1/secret/data/":  storeHandler,
		"POST /v1/secret/data/": storeHandler,
	})
	defer srv.Close()

	recorder := record.NewFakeRecorder(100)
	reconciler := &PostgresqlReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    recorder,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
	}

	reconciler.storeInstanceAdminCredentials(
		context.Background(), postgresql, "test-id", "admin", "pass")

	// Should emit a Normal event
	select {
	case event := <-recorder.Events:
		assert.Contains(t, event, "InstanceRegistered")
	default:
		t.Fatal("expected an event to be recorded")
	}
}

func TestStoreInstanceAdminCredentials_Failure(t *testing.T) {
	failHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"permission denied"},
		})
	}

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"PUT /v1/secret/data/":  failHandler,
		"POST /v1/secret/data/": failHandler,
	})
	defer srv.Close()

	recorder := record.NewFakeRecorder(100)
	reconciler := &PostgresqlReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    recorder,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
	}

	reconciler.storeInstanceAdminCredentials(
		context.Background(), postgresql, "test-id", "admin", "pass")

	// Should emit a Warning event
	select {
	case event := <-recorder.Events:
		assert.Contains(t, event, "CredentialStoreFailed")
	default:
		t.Fatal("expected a warning event to be recorded")
	}
}

// ---------- handleInstanceAdminPasswordRotation tests ----------

func TestHandleInstanceAdminPasswordRotation_NoNewPassword(t *testing.T) {
	// GetInstanceAdminNewPassword returns empty string - no rotation needed
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"login":    "admin",
						"password": "currentpass",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	recorder := record.NewFakeRecorder(100)
	reconciler := &PostgresqlReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    recorder,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 1 * time.Millisecond,
		},
	}

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
	}
	externalInstance := &instancev1alpha1.ExternalPostgresqlInstance{
		PostgresqlID: "test-id",
		Address:      "localhost",
	}

	reconciler.handleInstanceAdminPasswordRotation(
		context.Background(), postgresql, externalInstance, 5432, "require")

	// No events should be emitted since no rotation was needed
	select {
	case event := <-recorder.Events:
		t.Fatalf("unexpected event: %s", event)
	default:
		// Expected - no events
	}
}

func TestHandleInstanceAdminPasswordRotation_GetNewPasswordFails(t *testing.T) {
	// GetInstanceAdminNewPassword fails with error
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"permission denied"},
			})
		},
	})
	defer srv.Close()

	recorder := record.NewFakeRecorder(100)
	reconciler := &PostgresqlReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    recorder,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 1 * time.Millisecond,
		},
	}

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
	}
	externalInstance := &instancev1alpha1.ExternalPostgresqlInstance{
		PostgresqlID: "test-id",
		Address:      "localhost",
	}

	reconciler.handleInstanceAdminPasswordRotation(
		context.Background(), postgresql, externalInstance, 5432, "require")

	// No events should be emitted since the check failed
	select {
	case event := <-recorder.Events:
		t.Fatalf("unexpected event: %s", event)
	default:
		// Expected - no events
	}
}

func TestHandleInstanceAdminPasswordRotation_Success(t *testing.T) {
	// Full rotation flow: new_password found, PG change succeeds, Vault rotation succeeds
	pgServer := newMockPGServer(t, true)
	defer pgServer.close()

	var getCallCount int32
	storeHandler := func(w http.ResponseWriter, r *http.Request) {
		vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"version": 2,
			},
		})
	}

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&getCallCount, 1)
			if count <= 1 {
				// First call: GetInstanceAdminNewPassword - returns new_password
				vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"login":        "admin",
							"password":     "oldpass",
							"new_password": "newpass123",
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
				return
			}
			// Subsequent calls: GetPostgresqlCredentials (current creds) and RotateInstanceAdminPassword (read)
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"login":    "admin",
						"password": "oldpass",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
		"PUT /v1/secret/data/":  storeHandler,
		"POST /v1/secret/data/": storeHandler,
	})
	defer srv.Close()

	recorder := record.NewFakeRecorder(100)
	reconciler := &PostgresqlReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    recorder,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 5 * time.Second,
		},
	}

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
	}
	externalInstance := &instancev1alpha1.ExternalPostgresqlInstance{
		PostgresqlID: "test-id",
		Address:      "127.0.0.1",
	}

	reconciler.handleInstanceAdminPasswordRotation(
		context.Background(), postgresql, externalInstance, pgServer.port, "disable")

	// Should emit PasswordRotated event
	select {
	case event := <-recorder.Events:
		assert.Contains(t, event, "PasswordRotated")
	default:
		t.Fatal("expected PasswordRotated event")
	}
}

func TestHandleInstanceAdminPasswordRotation_PGFails(t *testing.T) {
	// PG change fails - Vault should not be updated
	var getCallCount int32
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&getCallCount, 1)
			if count <= 1 {
				// First call: GetInstanceAdminNewPassword - returns new_password
				vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"login":        "admin",
							"password":     "oldpass",
							"new_password": "newpass123",
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
				return
			}
			// Second call: GetPostgresqlCredentials (current creds)
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"login":    "admin",
						"password": "oldpass",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	recorder := record.NewFakeRecorder(100)
	reconciler := &PostgresqlReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    recorder,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 1 * time.Millisecond,
		},
	}

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
	}
	externalInstance := &instancev1alpha1.ExternalPostgresqlInstance{
		PostgresqlID: "test-id",
		Address:      "127.0.0.1",
	}

	// Use a port that nothing is listening on - PG connection will fail
	reconciler.handleInstanceAdminPasswordRotation(
		context.Background(), postgresql, externalInstance, 1, "disable")

	// Should emit PasswordRotationFailed event
	select {
	case event := <-recorder.Events:
		assert.Contains(t, event, "PasswordRotationFailed")
	default:
		t.Fatal("expected PasswordRotationFailed event")
	}
}

func TestHandleInstanceAdminPasswordRotation_VaultRotateFails(t *testing.T) {
	// PG change succeeds but Vault rotation fails
	pgServer := newMockPGServer(t, true)
	defer pgServer.close()

	var getCallCount int32
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&getCallCount, 1)
			if count <= 1 {
				// First call: GetInstanceAdminNewPassword - returns new_password
				vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"login":        "admin",
							"password":     "oldpass",
							"new_password": "newpass123",
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
				return
			}
			// Second call: GetPostgresqlCredentials (current creds)
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"login":    "admin",
						"password": "oldpass",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
		// RotateInstanceAdminPassword does a GET then PUT - the PUT will fail
		"PUT /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"permission denied"},
			})
		},
		"POST /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"permission denied"},
			})
		},
	})
	defer srv.Close()

	recorder := record.NewFakeRecorder(100)
	reconciler := &PostgresqlReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    recorder,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 5 * time.Second,
		},
	}

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
	}
	externalInstance := &instancev1alpha1.ExternalPostgresqlInstance{
		PostgresqlID: "test-id",
		Address:      "127.0.0.1",
	}

	reconciler.handleInstanceAdminPasswordRotation(
		context.Background(), postgresql, externalInstance, pgServer.port, "disable")

	// Should emit VaultRotationFailed event
	select {
	case event := <-recorder.Events:
		assert.Contains(t, event, "VaultRotationFailed")
	default:
		t.Fatal("expected VaultRotationFailed event")
	}
}

func TestHandleInstanceAdminPasswordRotation_GetCurrentCredsFails(t *testing.T) {
	// new_password found but getting current credentials fails
	var getCallCount int32
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&getCallCount, 1)
			if count <= 1 {
				// First call: GetInstanceAdminNewPassword - returns new_password
				vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"login":        "admin",
							"password":     "oldpass",
							"new_password": "newpass123",
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
				return
			}
			// Second call: GetPostgresqlCredentials fails
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"permission denied"},
			})
		},
	})
	defer srv.Close()

	recorder := record.NewFakeRecorder(100)
	reconciler := &PostgresqlReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    recorder,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 1 * time.Millisecond,
		},
	}

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
	}
	externalInstance := &instancev1alpha1.ExternalPostgresqlInstance{
		PostgresqlID: "test-id",
		Address:      "localhost",
	}

	reconciler.handleInstanceAdminPasswordRotation(
		context.Background(), postgresql, externalInstance, 5432, "require")

	// No events should be emitted since getting current creds failed (returns early)
	select {
	case event := <-recorder.Events:
		t.Fatalf("unexpected event: %s", event)
	default:
		// Expected - no events
	}
}

func TestPostgresqlReconciler_ReconcileExternalInstance_WithVault_DefaultPort(t *testing.T) {
	// Test with default port (0) and empty SSL mode
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"login":    "admin",
						"password": "pass",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{Name: "pg1", Namespace: "default"},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "test-id",
				Address:      "localhost",
				Port:         0,  // default port
				SSLMode:      "", // default SSL
			},
		},
		Status: instancev1alpha1.PostgresqlStatus{Connected: false},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(postgresql, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.Postgresql)
		*obj = *postgresql
	})
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &PostgresqlReconciler{
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
		NamespacedName: types.NamespacedName{Name: "pg1", Namespace: "default"},
	})

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}
