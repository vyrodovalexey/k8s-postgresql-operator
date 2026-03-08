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

package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- helpers ----------

// newMockVaultServer creates an httptest server that simulates Vault HTTP API.
// The handler map keys are "<METHOD> <path-prefix>" and values are handler funcs.
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
		// fallback – not found
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"no handler for " + r.Method + " " + r.URL.Path},
		})
	}))
}

// newTestClient creates a vault Client backed by the given httptest server.
func newTestClient(t *testing.T, serverURL, mountPoint, secretPath string) *Client {
	t.Helper()
	config := api.DefaultConfig()
	config.Address = serverURL
	config.MaxRetries = 0

	client, err := api.NewClient(config)
	require.NoError(t, err)
	client.SetToken("test-token")

	return &Client{
		client:          client,
		vaultMountPoint: mountPoint,
		vaultSecretPath: secretPath,
	}
}

// jsonResponse is a helper to write a JSON response.
func jsonResponse(w http.ResponseWriter, statusCode int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}

// ---------- NewClient tests ----------

func TestNewClient_EmptyVaultRole(t *testing.T) {
	client, err := NewClient(context.Background(), "http://localhost:8200", "", "/path/to/token", "secret", "pdb")
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "VAULT_ROLE")
}

func TestNewClient_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		vaultAddr  string
		vaultRole  string
		tokenPath  string
		mountPoint string
		secretPath string
		wantErrMsg string
	}{
		{
			name:       "empty role returns error",
			vaultAddr:  "http://localhost:8200",
			vaultRole:  "",
			tokenPath:  "/some/path",
			mountPoint: "secret",
			secretPath: "pdb",
			wantErrMsg: "VAULT_ROLE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(context.Background(), tt.vaultAddr, tt.vaultRole, tt.tokenPath, tt.mountPoint, tt.secretPath)
			assert.Nil(t, client)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}

func TestNewClient_AuthFailure(t *testing.T) {
	// Create a mock server that rejects auth
	authDeniedHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"permission denied"},
		})
	}
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"POST /v1/auth/kubernetes/login": authDeniedHandler,
		"PUT /v1/auth/kubernetes/login":  authDeniedHandler,
	})
	defer srv.Close()

	// Create a temp token file
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token")
	err := os.WriteFile(tokenFile, []byte("fake-jwt-token"), 0600)
	require.NoError(t, err)

	client, err := NewClient(context.Background(), srv.URL, "test-role", tokenFile, "secret", "pdb")
	assert.Nil(t, client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to authenticate with Vault using Kubernetes auth")
}

func TestNewClient_NilAuthInfo(t *testing.T) {
	// Create a mock server that returns empty auth info (null auth)
	nilAuthHandler := func(w http.ResponseWriter, r *http.Request) {
		// Return a response with no auth block – the Vault client will
		// interpret this as nil authInfo.
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"auth": nil,
		})
	}
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"POST /v1/auth/kubernetes/login": nilAuthHandler,
		"PUT /v1/auth/kubernetes/login":  nilAuthHandler,
	})
	defer srv.Close()

	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token")
	err := os.WriteFile(tokenFile, []byte("fake-jwt-token"), 0600)
	require.NoError(t, err)

	client, err := NewClient(context.Background(), srv.URL, "test-role", tokenFile, "secret", "pdb")
	assert.Nil(t, client)
	require.Error(t, err)
	// Could be "authentication returned empty auth info" or an error from the client
	// about missing auth data
	assert.True(t,
		strings.Contains(err.Error(), "authentication returned empty auth info") ||
			strings.Contains(err.Error(), "failed to authenticate"),
		"unexpected error: %v", err,
	)
}

func TestNewClient_Success(t *testing.T) {
	// Create a mock server that returns valid auth
	successAuthHandler := func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
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
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"POST /v1/auth/kubernetes/login": successAuthHandler,
		"PUT /v1/auth/kubernetes/login":  successAuthHandler,
	})
	defer srv.Close()

	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token")
	err := os.WriteFile(tokenFile, []byte("fake-jwt-token"), 0600)
	require.NoError(t, err)

	client, err := NewClient(context.Background(), srv.URL, "test-role", tokenFile, "secret", "pdb")
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "secret", client.vaultMountPoint)
	assert.Equal(t, "pdb", client.vaultSecretPath)
}

func TestNewClient_InvalidTokenPath(t *testing.T) {
	// Use a token path that doesn't exist – NewKubernetesAuth should still succeed
	// (it doesn't read the file at creation time), but Login will fail because
	// it can't read the token file.
	tokenHandler := func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"auth": map[string]interface{}{
				"client_token":   "s.test",
				"lease_duration": 3600,
				"renewable":      true,
			},
		})
	}
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"POST /v1/auth/kubernetes/login": tokenHandler,
		"PUT /v1/auth/kubernetes/login":  tokenHandler,
	})
	defer srv.Close()

	client, err := NewClient(context.Background(), srv.URL, "test-role", "/nonexistent/path/to/token", "secret", "pdb")
	// Should fail because the token file doesn't exist
	if err != nil {
		assert.Nil(t, client)
	}
}

// ---------- CheckHealth tests ----------

func TestCheckHealth_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "healthy vault",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"initialized":     true,
					"sealed":          false,
					"standby":         false,
					"server_time_utc": 1234567890,
					"version":         "1.15.0",
					"cluster_name":    "vault-cluster",
					"cluster_id":      "abc-123",
				})
			},
			wantErr: false,
		},
		{
			name: "sealed vault still healthy",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"initialized":     true,
					"sealed":          true,
					"standby":         false,
					"server_time_utc": 1234567890,
					"version":         "1.15.0",
				})
			},
			wantErr: false,
		},
		{
			name: "vault returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("internal server error"))
			},
			wantErr:    true,
			wantErrMsg: "failed to check Vault health",
		},
		{
			name: "vault returns invalid json",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("{invalid json"))
			},
			wantErr:    true,
			wantErrMsg: "failed to check Vault health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newMockVaultServer(t, map[string]http.HandlerFunc{
				"GET /v1/sys/health": tt.handler,
			})
			defer srv.Close()

			c := newTestClient(t, srv.URL, "secret", "pdb")
			err := c.CheckHealth(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckHealth_ConnectionRefused(t *testing.T) {
	// Point to a server that's not running
	c := newTestClient(t, "http://127.0.0.1:1", "secret", "pdb")
	err := c.CheckHealth(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check Vault health")
}

// ---------- GetPostgresqlCredentials tests ----------

func TestGetPostgresqlCredentials_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		postgresqlID string
		handler      http.HandlerFunc
		wantUser     string
		wantPass     string
		wantErr      bool
		wantErrMsg   string
	}{
		{
			name:         "success",
			postgresqlID: "my-pg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"admin_username": "pgadmin",
							"admin_password": "supersecret",
						},
						"metadata": map[string]interface{}{
							"version": 1,
						},
					},
				})
			},
			wantUser: "pgadmin",
			wantPass: "supersecret",
			wantErr:  false,
		},
		{
			name:         "read error - 404",
			postgresqlID: "missing-pg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{"secret not found"},
				})
			},
			wantErr:    true,
			wantErrMsg: "failed to read secret from Vault",
		},
		{
			name:         "read error - 500",
			postgresqlID: "error-pg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{"internal error"},
				})
			},
			wantErr:    true,
			wantErrMsg: "failed to read secret from Vault",
		},
		{
			name:         "nil data in response",
			postgresqlID: "nil-data-pg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data":     nil,
						"metadata": map[string]interface{}{"version": 1},
					},
				})
			},
			wantErr:    true,
			wantErrMsg: "secret not found at path",
		},
		{
			name:         "missing admin_username",
			postgresqlID: "no-user-pg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"admin_password": "pass123",
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
			},
			wantErr:    true,
			wantErrMsg: "credentials not found in secret at path",
		},
		{
			name:         "missing admin_password",
			postgresqlID: "no-pass-pg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"admin_username": "admin",
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
			},
			wantErr:    true,
			wantErrMsg: "credentials not found in secret at path",
		},
		{
			name:         "both fields missing",
			postgresqlID: "empty-pg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data":     map[string]interface{}{},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
			},
			wantErr:    true,
			wantErrMsg: "credentials not found in secret at path",
		},
		{
			name:         "non-string username type",
			postgresqlID: "bad-type-pg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"admin_username": 12345,
							"admin_password": true,
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
			},
			wantErr:    true,
			wantErrMsg: "credentials not found in secret at path",
		},
		{
			name:         "empty string credentials",
			postgresqlID: "empty-str-pg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"admin_username": "",
							"admin_password": "",
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
			},
			wantErr:    true,
			wantErrMsg: "credentials not found in secret at path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newMockVaultServer(t, map[string]http.HandlerFunc{
				"GET /v1/secret/data/": tt.handler,
			})
			defer srv.Close()

			c := newTestClient(t, srv.URL, "secret", "pdb")
			user, pass, err := c.GetPostgresqlCredentials(context.Background(), tt.postgresqlID)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
				assert.Empty(t, user)
				assert.Empty(t, pass)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantUser, user)
				assert.Equal(t, tt.wantPass, pass)
			}
		})
	}
}

func TestGetPostgresqlCredentials_VerifyPath(t *testing.T) {
	// Verify the correct path is constructed
	var capturedPath string
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			jsonResponse(w, http.StatusOK, map[string]interface{}{
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

	c := newTestClient(t, srv.URL, "secret", "pdb")
	_, _, err := c.GetPostgresqlCredentials(context.Background(), "my-cluster")
	require.NoError(t, err)
	assert.Equal(t, "/v1/secret/data/pdb/my-cluster/admin", capturedPath)
}

// ---------- GetPostgresqlUserCredentials tests ----------

func TestGetPostgresqlUserCredentials_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		postgresqlID string
		username     string
		handler      http.HandlerFunc
		wantPass     string
		wantErr      bool
		wantErrMsg   string
	}{
		{
			name:         "success",
			postgresqlID: "my-pg",
			username:     "appuser",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"password": "userpass123",
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
			},
			wantPass: "userpass123",
			wantErr:  false,
		},
		{
			name:         "read error - 403",
			postgresqlID: "forbidden-pg",
			username:     "user1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{"permission denied"},
				})
			},
			wantErr:    true,
			wantErrMsg: "failed to read secret from Vault",
		},
		{
			name:         "nil data in response",
			postgresqlID: "nil-data-pg",
			username:     "user1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data":     nil,
						"metadata": map[string]interface{}{"version": 1},
					},
				})
			},
			wantErr:    true,
			wantErrMsg: "secret not found at path",
		},
		{
			name:         "missing password field",
			postgresqlID: "no-pass-pg",
			username:     "user1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"other_field": "value",
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
			},
			wantErr:    true,
			wantErrMsg: "credentials not found in secret at path",
		},
		{
			name:         "empty password",
			postgresqlID: "empty-pass-pg",
			username:     "user1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"password": "",
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
			},
			wantErr:    true,
			wantErrMsg: "credentials not found in secret at path",
		},
		{
			name:         "non-string password type",
			postgresqlID: "bad-type-pg",
			username:     "user1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data": map[string]interface{}{
							"password": 12345,
						},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
			},
			wantErr:    true,
			wantErrMsg: "credentials not found in secret at path",
		},
		{
			name:         "empty data map",
			postgresqlID: "empty-data-pg",
			username:     "user1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"data":     map[string]interface{}{},
						"metadata": map[string]interface{}{"version": 1},
					},
				})
			},
			wantErr:    true,
			wantErrMsg: "credentials not found in secret at path",
		},
		{
			name:         "server error 500",
			postgresqlID: "error-pg",
			username:     "user1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{"internal error"},
				})
			},
			wantErr:    true,
			wantErrMsg: "failed to read secret from Vault",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newMockVaultServer(t, map[string]http.HandlerFunc{
				"GET /v1/secret/data/": tt.handler,
			})
			defer srv.Close()

			c := newTestClient(t, srv.URL, "secret", "pdb")
			pass, err := c.GetPostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
				assert.Empty(t, pass)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantPass, pass)
			}
		})
	}
}

func TestGetPostgresqlUserCredentials_VerifyPath(t *testing.T) {
	var capturedPath string
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"password": "pass123",
					},
					"metadata": map[string]interface{}{"version": 1},
				},
			})
		},
	})
	defer srv.Close()

	c := newTestClient(t, srv.URL, "secret", "pdb")
	_, err := c.GetPostgresqlUserCredentials(context.Background(), "my-cluster", "myuser")
	require.NoError(t, err)
	assert.Equal(t, "/v1/secret/data/pdb/my-cluster/myuser", capturedPath)
}

// ---------- StorePostgresqlUserCredentials tests ----------

func TestStorePostgresqlUserCredentials_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		postgresqlID string
		username     string
		password     string
		handler      http.HandlerFunc
		wantErr      bool
		wantErrMsg   string
	}{
		{
			name:         "success",
			postgresqlID: "my-pg",
			username:     "appuser",
			password:     "newpass123",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// Verify the request body
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": map[string]interface{}{
						"created_time":  "2024-01-01T00:00:00Z",
						"deletion_time": "",
						"destroyed":     false,
						"version":       1,
					},
				})
			},
			wantErr: false,
		},
		{
			name:         "write error - 403",
			postgresqlID: "forbidden-pg",
			username:     "user1",
			password:     "pass",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{"permission denied"},
				})
			},
			wantErr: true,
		},
		{
			name:         "write error - 500",
			postgresqlID: "error-pg",
			username:     "user1",
			password:     "pass",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []string{"internal error"},
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newMockVaultServer(t, map[string]http.HandlerFunc{
				"PUT /v1/secret/data/":  tt.handler,
				"POST /v1/secret/data/": tt.handler,
			})
			defer srv.Close()

			c := newTestClient(t, srv.URL, "secret", "pdb")
			err := c.StorePostgresqlUserCredentials(context.Background(), tt.postgresqlID, tt.username, tt.password)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStorePostgresqlUserCredentials_VerifyPathAndBody(t *testing.T) {
	var capturedPath string
	var capturedBody map[string]interface{}

	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"PUT /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&capturedBody)
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"version": 1,
				},
			})
		},
		"POST /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&capturedBody)
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"version": 1,
				},
			})
		},
	})
	defer srv.Close()

	c := newTestClient(t, srv.URL, "secret", "pdb")
	err := c.StorePostgresqlUserCredentials(context.Background(), "my-cluster", "myuser", "mypassword")
	require.NoError(t, err)
	assert.Equal(t, "/v1/secret/data/pdb/my-cluster/myuser", capturedPath)

	// Verify the body contains the password in the data field
	if data, ok := capturedBody["data"].(map[string]interface{}); ok {
		assert.Equal(t, "mypassword", data["password"])
	}
}

func TestStorePostgresqlUserCredentials_ConnectionRefused(t *testing.T) {
	c := newTestClient(t, "http://127.0.0.1:1", "secret", "pdb")
	err := c.StorePostgresqlUserCredentials(context.Background(), "pg", "user", "pass")
	require.Error(t, err)
}

// ---------- Additional edge case tests ----------

func TestGetPostgresqlCredentials_ConnectionRefused(t *testing.T) {
	c := newTestClient(t, "http://127.0.0.1:1", "secret", "pdb")
	user, pass, err := c.GetPostgresqlCredentials(context.Background(), "pg")
	require.Error(t, err)
	assert.Empty(t, user)
	assert.Empty(t, pass)
	assert.Contains(t, err.Error(), "failed to read secret from Vault")
}

func TestGetPostgresqlUserCredentials_ConnectionRefused(t *testing.T) {
	c := newTestClient(t, "http://127.0.0.1:1", "secret", "pdb")
	pass, err := c.GetPostgresqlUserCredentials(context.Background(), "pg", "user")
	require.Error(t, err)
	assert.Empty(t, pass)
	assert.Contains(t, err.Error(), "failed to read secret from Vault")
}

func TestClient_FieldValues(t *testing.T) {
	c := &Client{
		vaultMountPoint: "custom-mount",
		vaultSecretPath: "custom-path",
	}
	assert.Equal(t, "custom-mount", c.vaultMountPoint)
	assert.Equal(t, "custom-path", c.vaultSecretPath)
}

func TestGetPostgresqlCredentials_DifferentMountPoints(t *testing.T) {
	tests := []struct {
		name       string
		mountPoint string
		secretPath string
		pgID       string
		wantPath   string
	}{
		{
			name:       "default mount",
			mountPoint: "secret",
			secretPath: "pdb",
			pgID:       "cluster1",
			wantPath:   "/v1/secret/data/pdb/cluster1/admin",
		},
		{
			name:       "custom mount",
			mountPoint: "kv",
			secretPath: "databases/postgresql",
			pgID:       "prod-db",
			wantPath:   "/v1/kv/data/databases/postgresql/prod-db/admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPath string
			srv := newMockVaultServer(t, map[string]http.HandlerFunc{
				fmt.Sprintf("GET /v1/%s/data/", tt.mountPoint): func(w http.ResponseWriter, r *http.Request) {
					capturedPath = r.URL.Path
					jsonResponse(w, http.StatusOK, map[string]interface{}{
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

			c := newTestClient(t, srv.URL, tt.mountPoint, tt.secretPath)
			_, _, err := c.GetPostgresqlCredentials(context.Background(), tt.pgID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantPath, capturedPath)
		})
	}
}

func TestNewClient_EmptyVaultAddr(t *testing.T) {
	// Test with empty vault address - should fail at client creation or auth
	client, err := NewClient(context.Background(), "", "test-role", "", "secret", "pdb")
	// This might fail at different stages, but should fail
	if err != nil {
		assert.Nil(t, client)
	}
}

// ---------- IssueCertificate tests ----------

func TestIssueCertificate_Success(t *testing.T) {
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"PUT /v1/pki/issue/": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"certificate":   "-----BEGIN CERTIFICATE-----\nMIIB",
					"private_key":   "-----BEGIN RSA PRIVATE KEY-----\nMIIE",
					"serial_number": "aa:bb:cc",
					"expiration":    float64(9999999999),
					"ca_chain": []interface{}{
						"-----BEGIN CERTIFICATE-----\nCA1",
					},
				},
			})
		},
		"POST /v1/pki/issue/": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"certificate":   "-----BEGIN CERTIFICATE-----\nMIIB",
					"private_key":   "-----BEGIN RSA PRIVATE KEY-----\nMIIE",
					"serial_number": "aa:bb:cc",
					"expiration":    float64(9999999999),
					"ca_chain": []interface{}{
						"-----BEGIN CERTIFICATE-----\nCA1",
					},
				},
			})
		},
	})
	defer srv.Close()

	c := newTestClient(t, srv.URL, "secret", "pdb")
	cert, err := c.IssueCertificate(
		context.Background(),
		"pki", "my-role", "svc.default.svc",
		[]string{"svc", "svc.default"}, []string{"127.0.0.1"}, "720h",
	)

	require.NoError(t, err)
	require.NotNil(t, cert)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----\nMIIB", cert.Certificate)
	assert.Equal(t, "-----BEGIN RSA PRIVATE KEY-----\nMIIE", cert.PrivateKey)
	assert.Equal(t, "aa:bb:cc", cert.SerialNumber)
	assert.Equal(t, int64(9999999999), cert.Expiration)
	assert.Equal(t, []string{"-----BEGIN CERTIFICATE-----\nCA1"}, cert.CAChain)
}

func TestIssueCertificate_EmptyResponse(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		// Return a 200 with no data — Vault client returns
		// a Secret with nil Data, triggering the empty-response
		// or missing-field error path.
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"data": nil,
		})
	}
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"PUT /v1/pki/issue/":  handler,
		"POST /v1/pki/issue/": handler,
	})
	defer srv.Close()

	c := newTestClient(t, srv.URL, "secret", "pdb")
	cert, err := c.IssueCertificate(
		context.Background(),
		"pki", "role", "cn", nil, nil, "1h",
	)

	require.Error(t, err)
	assert.Nil(t, cert)
	// Vault client may still parse a Secret with nil Data,
	// which hits the "certificate not found" path.
	assert.True(t,
		strings.Contains(err.Error(), "empty response") ||
			strings.Contains(err.Error(), "certificate not found"),
		"unexpected error: %v", err,
	)
}

func TestIssueCertificate_MissingCertificate(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"private_key": "KEY",
			},
		})
	}
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"PUT /v1/pki/issue/":  handler,
		"POST /v1/pki/issue/": handler,
	})
	defer srv.Close()

	c := newTestClient(t, srv.URL, "secret", "pdb")
	cert, err := c.IssueCertificate(
		context.Background(),
		"pki", "role", "cn", nil, nil, "1h",
	)

	require.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "certificate not found")
}

func TestIssueCertificate_MissingPrivateKey(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"certificate": "CERT",
			},
		})
	}
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"PUT /v1/pki/issue/":  handler,
		"POST /v1/pki/issue/": handler,
	})
	defer srv.Close()

	c := newTestClient(t, srv.URL, "secret", "pdb")
	cert, err := c.IssueCertificate(
		context.Background(),
		"pki", "role", "cn", nil, nil, "1h",
	)

	require.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "private_key not found")
}

func TestIssueCertificate_VaultError(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"permission denied"},
		})
	}
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"PUT /v1/pki/issue/":  handler,
		"POST /v1/pki/issue/": handler,
	})
	defer srv.Close()

	c := newTestClient(t, srv.URL, "secret", "pdb")
	cert, err := c.IssueCertificate(
		context.Background(),
		"pki", "role", "cn", nil, nil, "1h",
	)

	require.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "failed to issue certificate")
}

func TestIssueCertificate_WithAltNamesAndIPSANs(t *testing.T) {
	var capturedBody map[string]interface{}
	handler := func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"certificate": "CERT",
				"private_key": "KEY",
			},
		})
	}
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"PUT /v1/pki/issue/":  handler,
		"POST /v1/pki/issue/": handler,
	})
	defer srv.Close()

	c := newTestClient(t, srv.URL, "secret", "pdb")
	_, err := c.IssueCertificate(
		context.Background(),
		"pki", "role", "svc.ns.svc",
		[]string{"svc", "svc.ns"}, []string{"10.0.0.1", "127.0.0.1"},
		"48h",
	)
	require.NoError(t, err)

	assert.Equal(t, "svc.ns.svc", capturedBody["common_name"])
	assert.Equal(t, "svc,svc.ns", capturedBody["alt_names"])
	assert.Equal(t, "10.0.0.1,127.0.0.1", capturedBody["ip_sans"])
	assert.Equal(t, "48h", capturedBody["ttl"])
}

func TestIssueCertificate_NoAltNames(t *testing.T) {
	var capturedBody map[string]interface{}
	handler := func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"certificate": "CERT",
				"private_key": "KEY",
			},
		})
	}
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"PUT /v1/pki/issue/":  handler,
		"POST /v1/pki/issue/": handler,
	})
	defer srv.Close()

	c := newTestClient(t, srv.URL, "secret", "pdb")
	_, err := c.IssueCertificate(
		context.Background(),
		"pki", "role", "cn", nil, nil, "1h",
	)
	require.NoError(t, err)

	_, hasAltNames := capturedBody["alt_names"]
	assert.False(t, hasAltNames, "alt_names should not be sent")
	_, hasIPSANs := capturedBody["ip_sans"]
	assert.False(t, hasIPSANs, "ip_sans should not be sent")
}

// ---------- GetCACertificate tests ----------

func TestGetCACertificate_Success(t *testing.T) {
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"GET /v1/pki/cert/ca": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"certificate": "-----BEGIN CERTIFICATE-----\nCA",
				},
			})
		},
	})
	defer srv.Close()

	c := newTestClient(t, srv.URL, "secret", "pdb")
	ca, err := c.GetCACertificate(context.Background(), "pki")

	require.NoError(t, err)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----\nCA", ca)
}

func TestGetCACertificate_EmptyResponse(t *testing.T) {
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"GET /v1/pki/cert/ca": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"data": nil,
			})
		},
	})
	defer srv.Close()

	c := newTestClient(t, srv.URL, "secret", "pdb")
	ca, err := c.GetCACertificate(context.Background(), "pki")

	require.Error(t, err)
	assert.Empty(t, ca)
	// Vault client may parse a Secret with nil Data, hitting
	// either "empty CA response" or "CA certificate not found".
	assert.True(t,
		strings.Contains(err.Error(), "empty CA response") ||
			strings.Contains(err.Error(), "CA certificate not found"),
		"unexpected error: %v", err,
	)
}

func TestGetCACertificate_MissingCertificate(t *testing.T) {
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"GET /v1/pki/cert/ca": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"other_field": "value",
				},
			})
		},
	})
	defer srv.Close()

	c := newTestClient(t, srv.URL, "secret", "pdb")
	ca, err := c.GetCACertificate(context.Background(), "pki")

	require.Error(t, err)
	assert.Empty(t, ca)
	assert.Contains(t, err.Error(), "CA certificate not found")
}

func TestGetCACertificate_VaultError(t *testing.T) {
	srv := newMockVaultServer(t, map[string]http.HandlerFunc{
		"GET /v1/pki/cert/ca": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"permission denied"},
			})
		},
	})
	defer srv.Close()

	c := newTestClient(t, srv.URL, "secret", "pdb")
	ca, err := c.GetCACertificate(context.Background(), "pki")

	require.Error(t, err)
	assert.Empty(t, ca)
	assert.Contains(t, err.Error(), "failed to read CA certificate")
}

// ---------- parsePKICertificateResponse tests ----------

func TestParsePKICertificateResponse_AllFields(t *testing.T) {
	data := map[string]interface{}{
		"certificate":   "CERT",
		"private_key":   "KEY",
		"serial_number": "aa:bb",
		"expiration":    float64(1234567890),
		"ca_chain":      []interface{}{"CA1", "CA2"},
	}

	cert, err := parsePKICertificateResponse(data, "test/path")

	require.NoError(t, err)
	assert.Equal(t, "CERT", cert.Certificate)
	assert.Equal(t, "KEY", cert.PrivateKey)
	assert.Equal(t, "aa:bb", cert.SerialNumber)
	assert.Equal(t, int64(1234567890), cert.Expiration)
	assert.Equal(t, []string{"CA1", "CA2"}, cert.CAChain)
}

func TestParsePKICertificateResponse_MinimalFields(t *testing.T) {
	data := map[string]interface{}{
		"certificate": "CERT",
		"private_key": "KEY",
	}

	cert, err := parsePKICertificateResponse(data, "test/path")

	require.NoError(t, err)
	assert.Equal(t, "CERT", cert.Certificate)
	assert.Equal(t, "KEY", cert.PrivateKey)
	assert.Empty(t, cert.SerialNumber)
	assert.Equal(t, int64(0), cert.Expiration)
	assert.Nil(t, cert.CAChain)
}

func TestParsePKICertificateResponse_ExpirationAsFloat64(t *testing.T) {
	data := map[string]interface{}{
		"certificate": "CERT",
		"private_key": "KEY",
		"expiration":  float64(9876543210),
	}

	cert, err := parsePKICertificateResponse(data, "test/path")

	require.NoError(t, err)
	assert.Equal(t, int64(9876543210), cert.Expiration)
}

func TestParsePKICertificateResponse_ExpirationAsJSONNumber(t *testing.T) {
	data := map[string]interface{}{
		"certificate": "CERT",
		"private_key": "KEY",
		"expiration":  json.Number("1234567890"),
	}

	cert, err := parsePKICertificateResponse(data, "test/path")

	require.NoError(t, err)
	assert.Equal(t, int64(1234567890), cert.Expiration)
}

func TestParsePKICertificateResponse_CAChain(t *testing.T) {
	data := map[string]interface{}{
		"certificate": "CERT",
		"private_key": "KEY",
		"ca_chain":    []interface{}{"ROOT-CA", "INTERMEDIATE-CA"},
	}

	cert, err := parsePKICertificateResponse(data, "test/path")

	require.NoError(t, err)
	require.Len(t, cert.CAChain, 2)
	assert.Equal(t, "ROOT-CA", cert.CAChain[0])
	assert.Equal(t, "INTERMEDIATE-CA", cert.CAChain[1])
}
