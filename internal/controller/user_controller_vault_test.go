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

// ---------- handleVaultUserPassword tests ----------

func TestUserReconciler_HandleVaultUserPassword_ExistingCredentials_NoUpdate(t *testing.T) {
	// Scenario: credentials exist in Vault, updatePassword is false -> use existing
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": "existing-password",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	reconciler := &UserReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID:   "test-id",
			Username:       "testuser",
			UpdatePassword: false,
		},
	}

	password := reconciler.handleVaultUserPassword(context.Background(), user)
	assert.Equal(t, "existing-password", password)
}

func TestUserReconciler_HandleVaultUserPassword_ExistingCredentials_WithUpdate(t *testing.T) {
	// Scenario: credentials exist in Vault, updatePassword is true -> generate new and store
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
			atomic.AddInt32(&getCallCount, 1)
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": "old-password",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
		"PUT /v1/secret/data/":  storeHandler,
		"POST /v1/secret/data/": storeHandler,
	})
	defer srv.Close()

	reconciler := &UserReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID:   "test-id",
			Username:       "testuser",
			UpdatePassword: true,
		},
	}

	password := reconciler.handleVaultUserPassword(context.Background(), user)
	assert.NotEmpty(t, password)
	assert.NotEqual(t, "old-password", password) // Should be a new generated password
	assert.Len(t, password, 32)                  // Default generated password length
}

func TestUserReconciler_HandleVaultUserPassword_CredentialsNotFound_GenerateNew(t *testing.T) {
	// Scenario: credentials don't exist in Vault -> generate new and store
	storeHandler := func(w http.ResponseWriter, r *http.Request) {
		vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"version": 1,
			},
		})
	}

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"secret not found"},
			})
		},
		"PUT /v1/secret/data/":  storeHandler,
		"POST /v1/secret/data/": storeHandler,
	})
	defer srv.Close()

	reconciler := &UserReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID:   "test-id",
			Username:       "testuser",
			UpdatePassword: false,
		},
	}

	password := reconciler.handleVaultUserPassword(context.Background(), user)
	assert.NotEmpty(t, password)
	assert.Len(t, password, 32)
}

func TestUserReconciler_HandleVaultUserPassword_StoreFailsStillReturnsPassword(t *testing.T) {
	// Scenario: credentials don't exist, generate new, store fails -> still returns generated password
	failHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"store failed"},
		})
	}

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"secret not found"},
			})
		},
		"PUT /v1/secret/data/":  failHandler,
		"POST /v1/secret/data/": failHandler,
	})
	defer srv.Close()

	reconciler := &UserReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID:   "test-id",
			Username:       "testuser",
			UpdatePassword: false,
		},
	}

	password := reconciler.handleVaultUserPassword(context.Background(), user)
	// Even if store fails, the generated password is returned
	assert.NotEmpty(t, password)
	assert.Len(t, password, 32)
}

func TestUserReconciler_HandleVaultUserPassword_EmptyPasswordInVault(t *testing.T) {
	// Scenario: Vault returns empty password -> generate new
	storeHandler := func(w http.ResponseWriter, r *http.Request) {
		vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"version": 1,
			},
		})
	}

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": "",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
		"PUT /v1/secret/data/":  storeHandler,
		"POST /v1/secret/data/": storeHandler,
	})
	defer srv.Close()

	reconciler := &UserReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID:   "test-id",
			Username:       "testuser",
			UpdatePassword: false,
		},
	}

	password := reconciler.handleVaultUserPassword(context.Background(), user)
	assert.NotEmpty(t, password)
	assert.Len(t, password, 32)
}

// ---------- resolveUserPassword with vault client ----------

func TestUserReconciler_ResolveUserPassword_WithVaultClient(t *testing.T) {
	storeHandler := func(w http.ResponseWriter, r *http.Request) {
		vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"version": 1,
			},
		})
	}

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": "existing-pass",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
		"PUT /v1/secret/data/":  storeHandler,
		"POST /v1/secret/data/": storeHandler,
	})
	defer srv.Close()

	reconciler := &UserReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID:   "test-id",
			Username:       "testuser",
			UpdatePassword: false,
		},
	}

	password := reconciler.resolveUserPassword(context.Background(), user)
	assert.Equal(t, "existing-pass", password)
}

// ---------- Full Reconcile with vault client ----------

func TestUserReconciler_Reconcile_WithVaultClient_ConnectedPG(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	storeHandler := func(w http.ResponseWriter, r *http.Request) {
		vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"version": 1,
			},
		})
	}

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"login":    "admin",
						"password": "userpass",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
		"PUT /v1/secret/data/":  storeHandler,
		"POST /v1/secret/data/": storeHandler,
	})
	defer srv.Close()

	user := createTestUser("test-user", "default", "test-id", "testuser", false)
	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(user, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.User)
		*obj = *user
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{*postgresql}
	})
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &UserReconciler{
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
		NamespacedName: types.NamespacedName{Name: "test-user", Namespace: "default"},
	})

	// syncUser will be called with the password from vault
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}

// ---------- computePasswordHash tests ----------

func TestComputePasswordHash(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{
			name:     "Simple password",
			password: "testpassword",
		},
		{
			name:     "Empty password",
			password: "",
		},
		{
			name:     "Special characters",
			password: "p@$$w0rd!#%^&*()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := computePasswordHash(tt.password)
			assert.NotEmpty(t, hash)
			assert.Len(t, hash, 64) // SHA-256 hex is 64 chars

			// Same input should produce same hash (deterministic)
			hash2 := computePasswordHash(tt.password)
			assert.Equal(t, hash, hash2)
		})
	}

	// Different passwords should produce different hashes
	hash1 := computePasswordHash("password1")
	hash2 := computePasswordHash("password2")
	assert.NotEqual(t, hash1, hash2)
}

// ---------- handleVaultUserPassword hash comparison tests ----------

func TestUserReconciler_HandleVaultUserPassword_UnchangedHash(t *testing.T) {
	// Scenario: credentials exist in Vault, updatePassword is false,
	// hash matches LastAppliedPasswordHash -> returns password (no PG update needed)
	existingPassword := "existing-password"
	existingHash := computePasswordHash(existingPassword)

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": existingPassword,
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	reconciler := &UserReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID:   "test-id",
			Username:       "testuser",
			UpdatePassword: false,
		},
		Status: instancev1alpha1.UserStatus{
			LastAppliedPasswordHash: existingHash,
		},
	}

	password := reconciler.handleVaultUserPassword(context.Background(), user)
	assert.Equal(t, existingPassword, password)
}

func TestUserReconciler_HandleVaultUserPassword_ChangedHash(t *testing.T) {
	// Scenario: credentials exist in Vault, updatePassword is false,
	// hash differs from LastAppliedPasswordHash -> returns password for PG update
	newVaultPassword := "new-vault-password"
	oldHash := computePasswordHash("old-password")

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": newVaultPassword,
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	reconciler := &UserReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			VaultClient:                 vaultClient,
			Log:                         zap.NewNop().Sugar(),
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID:   "test-id",
			Username:       "testuser",
			UpdatePassword: false,
		},
		Status: instancev1alpha1.UserStatus{
			LastAppliedPasswordHash: oldHash,
		},
	}

	password := reconciler.handleVaultUserPassword(context.Background(), user)
	assert.Equal(t, newVaultPassword, password)
}

// ---------- syncUser with password hash storage ----------

func TestUserReconciler_SyncUser_StoresPasswordHash(t *testing.T) {
	// When syncUser succeeds with a non-empty password, it should store the hash
	pgServer := newMockPGServer(t, true)
	defer pgServer.close()

	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &UserReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 5 * time.Second,
		},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID: "test-id",
			Username:     "testuser",
		},
	}

	info := &connectionInfo{
		ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
			PostgresqlID: "test-id",
			Address:      "127.0.0.1",
		},
		Port:     pgServer.port,
		SSLMode:  "disable",
		Username: "admin",
		Password: "secret",
	}

	_, err := reconciler.syncUser(context.Background(), user, info, "testpassword")
	assert.NoError(t, err)
	assert.True(t, user.Status.Created)
	assert.Equal(t, computePasswordHash("testpassword"), user.Status.LastAppliedPasswordHash)
}

func TestUserReconciler_SyncUser_EmptyPasswordNoHash(t *testing.T) {
	// When syncUser succeeds with empty password, LastAppliedPasswordHash should not be set
	pgServer := newMockPGServer(t, true)
	defer pgServer.close()

	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &UserReconciler{
		BaseReconcilerConfig: BaseReconcilerConfig{
			Client:                      mockClient,
			Log:                         zap.NewNop().Sugar(),
			Recorder:                    record.NewFakeRecorder(100),
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 5 * time.Second,
		},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "u1", Namespace: "default"},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID: "test-id",
			Username:     "testuser",
		},
	}

	info := &connectionInfo{
		ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
			PostgresqlID: "test-id",
			Address:      "127.0.0.1",
		},
		Port:     pgServer.port,
		SSLMode:  "disable",
		Username: "admin",
		Password: "secret",
	}

	_, err := reconciler.syncUser(context.Background(), user, info, "")
	assert.NoError(t, err)
	assert.True(t, user.Status.Created)
	assert.Empty(t, user.Status.LastAppliedPasswordHash)
}

func TestUserReconciler_Reconcile_WithVaultClient_VaultGetFails(t *testing.T) {
	mockClient := new(MockControllerClient)
	mockStatus := new(MockStatusWriter)

	// Use 403 Forbidden instead of 500 to avoid go-retryablehttp internal retries
	// (go-retryablehttp only retries on 5xx errors)
	failHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"vault error: permission denied"},
		})
	}

	// For this test, we need the admin credentials GET to succeed but user credentials GET to fail.
	// Since both use the same path prefix, we need to differentiate by path.
	var getCallCount int32
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&getCallCount, 1)
			if count <= 1 {
				// First call: admin credentials (resolveAdminCredentials)
				vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"login":    "admin",
							"password": "adminpass",
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
				return
			}
			// Subsequent calls: user credentials fail
			failHandler(w, r)
		},
		"PUT /v1/secret/data/":  failHandler,
		"POST /v1/secret/data/": failHandler,
	})
	defer srv.Close()

	user := createTestUser("test-user", "default", "test-id", "testuser", false)
	postgresql := createTestPostgresql("pg1", "default", "test-id", "localhost", 5432, true)

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(user, nil).Run(func(args mock.Arguments) {
		obj := args.Get(2).(*instancev1alpha1.User)
		*obj = *user
	})
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		list.Items = []instancev1alpha1.Postgresql{*postgresql}
	})
	mockClient.On("Status").Return(mockStatus)
	mockStatus.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	reconciler := &UserReconciler{
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
		NamespacedName: types.NamespacedName{Name: "test-user", Namespace: "default"},
	})

	// When vault user credentials fail, handleVaultUserPassword generates a new password
	// and tries to store it (which also fails), but still returns the generated password.
	// So syncUser is called with the generated password.
	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
	mockStatus.AssertExpectations(t)
}
