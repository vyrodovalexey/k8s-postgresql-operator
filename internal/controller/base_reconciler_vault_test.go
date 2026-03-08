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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestResolveAdminCredentials_WithVaultClient_Success(t *testing.T) {
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

	cfg := &BaseReconcilerConfig{
		Log:                         zap.NewNop().Sugar(),
		VaultClient:                 vaultClient,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 1 * time.Millisecond,
	}

	info := &connectionInfo{
		ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
			PostgresqlID: "test-id",
			Address:      "localhost",
		},
	}

	err := cfg.resolveAdminCredentials(context.Background(), info)

	assert.NoError(t, err)
	assert.Equal(t, "pgadmin", info.Username)
	assert.Equal(t, "supersecret", info.Password)
}

func TestResolveAdminCredentials_WithVaultClient_Failure(t *testing.T) {
	vaultClient, srv := newTestVaultClient(t, map[string]http.HandlerFunc{
		"GET /v1/secret/data/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []string{"permission denied"},
			})
		},
	})
	defer srv.Close()

	cfg := &BaseReconcilerConfig{
		Log:                         zap.NewNop().Sugar(),
		VaultClient:                 vaultClient,
		VaultAvailabilityRetries:    1,
		VaultAvailabilityRetryDelay: 1 * time.Millisecond,
	}

	info := &connectionInfo{
		ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
			PostgresqlID: "test-id",
			Address:      "localhost",
		},
	}

	err := cfg.resolveAdminCredentials(context.Background(), info)

	assert.Error(t, err)
	assert.Empty(t, info.Username)
	assert.Empty(t, info.Password)
}

func TestResolveAdminCredentials_NilVaultClient(t *testing.T) {
	cfg := &BaseReconcilerConfig{
		Log:                         zap.NewNop().Sugar(),
		VaultClient:                 nil,
		VaultAvailabilityRetries:    1,
		VaultAvailabilityRetryDelay: 1 * time.Millisecond,
	}

	info := &connectionInfo{
		ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
			PostgresqlID: "test-id",
			Address:      "localhost",
		},
	}

	err := cfg.resolveAdminCredentials(context.Background(), info)

	assert.NoError(t, err)
	assert.Empty(t, info.Username)
	assert.Empty(t, info.Password)
}
