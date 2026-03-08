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

package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// healthyVaultResponse returns a JSON body representing a healthy Vault.
func healthyVaultResponse() map[string]interface{} {
	return map[string]interface{}{
		"initialized":                  true,
		"sealed":                       false,
		"standby":                      false,
		"performance_standby":          false,
		"replication_performance_mode": "disabled",
		"replication_dr_mode":          "disabled",
		"server_time_utc":              1234567890,
		"version":                      "1.0.0",
		"cluster_name":                 "test",
		"cluster_id":                   "test-id",
	}
}

// healthyHandler returns a handler that responds with a healthy Vault status.
func healthyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthyVaultResponse())
	}
}

// unhealthyHandler returns a handler that responds with an unhealthy Vault status.
func unhealthyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// newTestVaultClient creates a vault.Client using a test HTTP server
// with the given mux handler. It handles the k8s auth login endpoint.
func newTestVaultClient(t *testing.T, mux *http.ServeMux) *vault.Client {
	t.Helper()

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	tokenFile := filepath.Join(t.TempDir(), "token")
	err := os.WriteFile(tokenFile, []byte("fake-sa-token"), 0o600)
	require.NoError(t, err)

	client, err := vault.NewClient(context.Background(), ts.URL, "test-role", tokenFile, "secret", "pdb")
	require.NoError(t, err)
	return client
}

// newVaultMuxWithAuth creates a mux with the k8s auth login handler pre-registered.
func newVaultMuxWithAuth() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/kubernetes/login", func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"auth": map[string]interface{}{
				"client_token":   "test-token",
				"accessor":       "test-accessor",
				"policies":       []string{"default"},
				"token_policies": []string{"default"},
				"lease_duration": 3600,
				"renewable":      true,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	return mux
}

// newTestVaultClientWithMux creates a vault.Client using a test HTTP server
// with the given health handler. It handles the k8s auth login endpoint too.
func newTestVaultClientWithMux(t *testing.T, healthHandler http.HandlerFunc) *vault.Client {
	t.Helper()
	mux := newVaultMuxWithAuth()
	mux.HandleFunc("/v1/sys/health", healthHandler)
	return newTestVaultClient(t, mux)
}

// newHealthyVaultClient creates a vault.Client that passes health checks
// but will fail on credential retrieval (no KV endpoint configured).
func newHealthyVaultClient(t *testing.T) *vault.Client {
	t.Helper()
	return newTestVaultClientWithMux(t, healthyHandler())
}

// newUnhealthyVaultClient creates a vault.Client that fails health checks.
func newUnhealthyVaultClient(t *testing.T) *vault.Client {
	t.Helper()
	return newTestVaultClientWithMux(t, unhealthyHandler())
}

func TestCheckVaultAvailability_NilClient(t *testing.T) {
	logger := zap.NewNop().Sugar()
	err := checkVaultAvailability(context.Background(), nil, logger, 3, 1*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

func TestCheckVaultAvailability_HealthCheckSuccess(t *testing.T) {
	logger := zap.NewNop().Sugar()

	client := newHealthyVaultClient(t)
	err := checkVaultAvailability(context.Background(), client, logger, 1, 1*time.Millisecond)
	assert.NoError(t, err)
}

func TestCheckVaultAvailability_HealthCheckSuccessAfterRetry(t *testing.T) {
	logger := zap.NewNop().Sugar()

	var callCount int32
	handler := func(w http.ResponseWriter, _ *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthyVaultResponse())
	}

	client := newTestVaultClientWithMux(t, handler)
	err := checkVaultAvailability(context.Background(), client, logger, 3, 1*time.Millisecond)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&callCount), int32(2))
}

func TestCheckVaultAvailability_AllRetriesFail(t *testing.T) {
	logger := zap.NewNop().Sugar()

	client := newUnhealthyVaultClient(t)
	err := checkVaultAvailability(context.Background(), client, logger, 2, 1*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault is not available after 2 attempts")
}

func TestCheckVaultAvailability_SingleRetryFails(t *testing.T) {
	logger := zap.NewNop().Sugar()

	client := newUnhealthyVaultClient(t)
	err := checkVaultAvailability(context.Background(), client, logger, 1, 1*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault is not available after 1 attempts")
}

func TestCheckVaultAvailability_SuccessOnLastRetry(t *testing.T) {
	logger := zap.NewNop().Sugar()

	var callCount int32
	handler := func(w http.ResponseWriter, _ *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthyVaultResponse())
	}

	client := newTestVaultClientWithMux(t, handler)
	err := checkVaultAvailability(context.Background(), client, logger, 3, 1*time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
}
