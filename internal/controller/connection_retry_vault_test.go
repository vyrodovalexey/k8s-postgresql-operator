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
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// ---------- helpers for creating real vault.Client in controller tests ----------

// newMockVaultServer creates an httptest server that simulates Vault HTTP API.
func newMockVaultServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for pattern, handler := range handlers {
			parts := strings.SplitN(pattern, " ", 2)
			method, prefix := parts[0], parts[1]
			if r.Method == method && strings.HasPrefix(r.URL.Path, prefix) {
				handler(w, r)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"no handler for " + r.Method + " " + r.URL.Path},
		})
	}))
}

// vaultJSONResponse writes a JSON response.
func vaultJSONResponse(w http.ResponseWriter, statusCode int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}

// newTestVaultClient creates a real *vault.Client backed by an httptest server.
// It uses vault.NewClient with a mock K8s auth endpoint.
func newTestVaultClient(t *testing.T, handlers map[string]http.HandlerFunc) (*vault.Client, *httptest.Server) {
	t.Helper()

	// Add K8s auth login handler
	authHandler := func(w http.ResponseWriter, r *http.Request) {
		vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
			"auth": map[string]interface{}{
				"client_token":   "s.test-token-12345",
				"accessor":       "accessor-12345",
				"policies":       []string{"default"},
				"token_policies": []string{"default"},
				"lease_duration": 3600,
				"renewable":      true,
			},
		})
	}

	allHandlers := make(map[string]http.HandlerFunc)
	allHandlers["POST /v1/auth/kubernetes/login"] = authHandler
	allHandlers["PUT /v1/auth/kubernetes/login"] = authHandler
	for k, v := range handlers {
		allHandlers[k] = v
	}

	srv := newMockVaultServer(t, allHandlers)

	// Create temp token file
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token")
	err := os.WriteFile(tokenFile, []byte("fake-jwt-token"), 0600)
	require.NoError(t, err)

	client, err := vault.NewClient(context.Background(), srv.URL, "test-role", tokenFile, "secret", "pdb")
	require.NoError(t, err)
	require.NotNil(t, client)

	return client, srv
}

// ---------- getVaultCredentialsWithRetry tests ----------

func TestGetVaultCredentialsWithRetry_Success(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"admin_username": "pgadmin",
						"admin_password": "supersecret",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	username, password, err := getVaultCredentialsWithRetry(
		ctx, vaultClient, "test-id", logger, 3, 1*time.Millisecond)

	assert.NoError(t, err)
	assert.Equal(t, "pgadmin", username)
	assert.Equal(t, "supersecret", password)
}

func TestGetVaultCredentialsWithRetry_AllRetriesFail(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"permission denied"},
			})
		},
	})
	defer srv.Close()

	username, password, err := getVaultCredentialsWithRetry(
		ctx, vaultClient, "test-id", logger, 2, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get credentials from Vault after 2 attempts")
	assert.Empty(t, username)
	assert.Empty(t, password)
}

func TestGetVaultCredentialsWithRetry_SuccessAfterRetry(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	var callCount int32
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&callCount, 1)
			if count < 2 {
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{"temporary error"},
				})
				return
			}
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"admin_username": "admin",
						"admin_password": "pass",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	username, password, err := getVaultCredentialsWithRetry(
		ctx, vaultClient, "test-id", logger, 3, 1*time.Millisecond)

	assert.NoError(t, err)
	assert.Equal(t, "admin", username)
	assert.Equal(t, "pass", password)
}

func TestGetVaultCredentialsWithRetry_SingleRetryFails(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"permission denied"},
			})
		},
	})
	defer srv.Close()

	username, password, err := getVaultCredentialsWithRetry(
		ctx, vaultClient, "test-id", logger, 1, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get credentials from Vault after 1 attempts")
	assert.Empty(t, username)
	assert.Empty(t, password)
}

// ---------- getVaultUserCredentialsWithRetry tests ----------

func TestGetVaultUserCredentialsWithRetry_Success(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": "userpass123",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	password, err := getVaultUserCredentialsWithRetry(
		ctx, vaultClient, "test-id", "testuser", logger, 3, 1*time.Millisecond)

	assert.NoError(t, err)
	assert.Equal(t, "userpass123", password)
}

func TestGetVaultUserCredentialsWithRetry_AllRetriesFail(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"permission denied"},
			})
		},
	})
	defer srv.Close()

	password, err := getVaultUserCredentialsWithRetry(
		ctx, vaultClient, "test-id", "testuser", logger, 2, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get user credentials from Vault after 2 attempts")
	assert.Empty(t, password)
}

func TestGetVaultUserCredentialsWithRetry_SuccessAfterRetry(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	var callCount int32
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&callCount, 1)
			if count < 2 {
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{"temporary error"},
				})
				return
			}
			vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": "userpass",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	password, err := getVaultUserCredentialsWithRetry(
		ctx, vaultClient, "test-id", "testuser", logger, 3, 1*time.Millisecond)

	assert.NoError(t, err)
	assert.Equal(t, "userpass", password)
}

func TestGetVaultUserCredentialsWithRetry_SingleRetryFails(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"permission denied"},
			})
		},
	})
	defer srv.Close()

	password, err := getVaultUserCredentialsWithRetry(
		ctx, vaultClient, "test-id", "testuser", logger, 1, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get user credentials from Vault after 1 attempts")
	assert.Empty(t, password)
}

// ---------- storeVaultUserCredentialsWithRetry tests ----------

func TestStoreVaultUserCredentialsWithRetry_Success(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	storeHandler := func(w http.ResponseWriter, r *http.Request) {
		vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"created_time":  "2024-01-01T00:00:00Z",
				"deletion_time": "",
				"destroyed":     false,
				"version":       1,
			},
		})
	}

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"PUT /v1/secret/data/":  storeHandler,
		"POST /v1/secret/data/": storeHandler,
	})
	defer srv.Close()

	err := storeVaultUserCredentialsWithRetry(
		ctx, vaultClient, "test-id", "testuser", "newpass", logger, 3, 1*time.Millisecond)

	assert.NoError(t, err)
}

func TestStoreVaultUserCredentialsWithRetry_AllRetriesFail(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

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

	err := storeVaultUserCredentialsWithRetry(
		ctx, vaultClient, "test-id", "testuser", "newpass", logger, 2, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store user credentials in Vault after 2 attempts")
}

func TestStoreVaultUserCredentialsWithRetry_SuccessAfterRetry(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	var callCount int32
	retryHandler := func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count < 2 {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"temporary error"},
			})
			return
		}
		vaultJSONResponse(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"version": 1,
			},
		})
	}

	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"PUT /v1/secret/data/":  retryHandler,
		"POST /v1/secret/data/": retryHandler,
	})
	defer srv.Close()

	err := storeVaultUserCredentialsWithRetry(
		ctx, vaultClient, "test-id", "testuser", "newpass", logger, 3, 1*time.Millisecond)

	assert.NoError(t, err)
}

func TestStoreVaultUserCredentialsWithRetry_SingleRetryFails(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

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

	err := storeVaultUserCredentialsWithRetry(
		ctx, vaultClient, "test-id", "testuser", "newpass", logger, 1, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store user credentials in Vault after 1 attempts")
}
