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

package postgresql

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// ---------- helpers for creating real vault.Client in postgresql tests ----------

// newPGMockVaultServer creates an httptest server that simulates Vault HTTP API.
func newPGMockVaultServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
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

// pgVaultJSONResponse writes a JSON response.
func pgVaultJSONResponse(w http.ResponseWriter, statusCode int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}

// newPGTestVaultClient creates a real *vault.Client backed by an httptest server.
func newPGTestVaultClient(t *testing.T, handlers map[string]http.HandlerFunc) (*vault.Client, *httptest.Server) {
	t.Helper()

	// Add K8s auth login handler
	authHandler := func(w http.ResponseWriter, r *http.Request) {
		pgVaultJSONResponse(w, http.StatusOK, map[string]interface{}{
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

	srv := newPGMockVaultServer(t, allHandlers)

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

// ---------- resolveVaultCredentials tests ----------

func TestResolveVaultCredentials_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		postgresqlID string
		// handler receives the request path so it can differentiate between
		// instance_admin and default credential requests
		handler    func(w http.ResponseWriter, r *http.Request)
		wantLogin  string
		wantPass   string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:         "success with instance_admin credentials",
			postgresqlID: "my-pg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// instance_admin path: /v1/secret/data/pdb/my-pg/instance_admin
				pgVaultJSONResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"login":    "pgadmin",
							"password": "supersecret",
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
			},
			wantLogin: "pgadmin",
			wantPass:  "supersecret",
			wantErr:   false,
		},
		{
			name:         "fallback to default credentials on 404",
			postgresqlID: "missing-pg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// instance_admin returns 404, default returns success
				if strings.Contains(r.URL.Path, "instance_admin") {
					w.WriteHeader(http.StatusNotFound)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"errors": []string{"secret not found"},
					})
					return
				}
				// default credentials path: /v1/secret/data/pdb/default
				if strings.Contains(r.URL.Path, "/default") {
					pgVaultJSONResponse(w, http.StatusOK, map[string]interface{}{
						"data": map[string]interface{}{
							"data": map[string]interface{}{
								"login":    "default-admin",
								"password": "default-pass",
							},
							"metadata": map[string]interface{}{"version": 1},
						},
					})
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			wantLogin: "default-admin",
			wantPass:  "default-pass",
			wantErr:   false,
		},
		{
			name:         "both instance_admin and default fail",
			postgresqlID: "no-creds-pg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// Both paths return 404
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{"secret not found"},
				})
			},
			wantErr:    true,
			wantErrMsg: "no credentials available",
		},
		{
			name:         "non-404 vault error - no fallback",
			postgresqlID: "forbidden-pg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// Return 403 forbidden for instance_admin - should NOT fallback
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{"permission denied"},
				})
			},
			wantErr:    true,
			wantErrMsg: "failed to get credentials from Vault",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			vaultClient, srv := newPGTestVaultClient(t, map[string]http.HandlerFunc{
				"GET /v1/secret/data/": tt.handler,
			})
			defer srv.Close()

			logger := zap.NewNop().Sugar()
			ctx := context.Background()

			// Act
			login, password, err := resolveVaultCredentials(ctx, vaultClient, tt.postgresqlID, logger)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
				assert.Empty(t, login)
				assert.Empty(t, password)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantLogin, login)
				assert.Equal(t, tt.wantPass, password)
			}
		})
	}
}
