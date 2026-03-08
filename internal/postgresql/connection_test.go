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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestDefaultConstants(t *testing.T) {
	assert.Equal(t, "require", DefaultSSLMode)
	assert.Equal(t, int(5432), int(DefaultPort))
	assert.Equal(t, "postgres", DefaultDB)
}

func TestTestConnection_InvalidConnection(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	// Test with invalid connection parameters
	connected, err := TestConnection(ctx, "invalid-host", 5432, "postgres", "user", "password", "require", logger, 1, 1*time.Second)

	assert.False(t, connected)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to PostgreSQL")
}

func TestTestConnection_MultipleRetries(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	// Test with multiple retries - all should fail with invalid host
	connected, err := TestConnection(ctx, "invalid-host", 5432, "postgres", "user", "password", "disable", logger, 2, 1*time.Millisecond)

	assert.False(t, connected)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to PostgreSQL after 2 attempts")
}

func TestTestConnection_SingleRetry(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	// Test with single retry
	connected, err := TestConnection(ctx, "invalid-host", 5432, "postgres", "user", "password", "disable", logger, 1, 1*time.Millisecond)

	assert.False(t, connected)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to PostgreSQL after 1 attempts")
}

func TestTestConnection_Success(t *testing.T) {
	// Start mock PostgreSQL server that responds to SELECT 1
	srv := newMockPGServer(t, func(q string) mockPGResponse {
		return mockPGResponse{
			isSelect: true,
			value:    "1",
		}
	})
	defer srv.close()

	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	connected, err := TestConnection(ctx, "127.0.0.1", srv.port(), "postgres", "user", "password", "disable", logger, 1, 1*time.Millisecond)

	assert.True(t, connected)
	assert.NoError(t, err)
}

func TestTestConnection_SuccessAfterRetry(t *testing.T) {
	attempt := 0
	srv := newMockPGServer(t, func(q string) mockPGResponse {
		attempt++
		if attempt <= 1 {
			return mockPGResponse{isError: true, errorMsg: "temporary failure"}
		}
		return mockPGResponse{isSelect: true, value: "1"}
	})
	defer srv.close()

	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	connected, err := TestConnection(ctx, "127.0.0.1", srv.port(), "postgres", "user", "password", "disable", logger, 3, 1*time.Millisecond)

	assert.True(t, connected)
	assert.NoError(t, err)
}

func TestTestConnection_UnexpectedResult(t *testing.T) {
	srv := newMockPGServer(t, func(q string) mockPGResponse {
		return mockPGResponse{isSelect: true, value: "42"}
	})
	defer srv.close()

	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	connected, err := TestConnection(ctx, "127.0.0.1", srv.port(), "postgres", "user", "password", "disable", logger, 1, 1*time.Millisecond)

	assert.False(t, connected)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected query result")
}

func TestTestConnectionFromPostgresql_NoExternalInstance(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	postgresql := &instancev1alpha1.Postgresql{
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: nil,
		},
	}

	err := TestConnectionFromPostgresql(ctx, postgresql, nil, logger, 1, 1*time.Second)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no external instance configuration")
}

func TestTestConnectionFromPostgresql_NoVaultClient(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	postgresql := &instancev1alpha1.Postgresql{
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "test-id",
				Address:      "localhost",
				Port:         5432,
			},
		},
	}

	// Should return nil when vault client is not configured (skips test)
	err := TestConnectionFromPostgresql(ctx, postgresql, nil, logger, 1, 1*time.Second)

	assert.NoError(t, err)
}

func TestTestConnectionFromPostgresql_DefaultValues(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	postgresql := &instancev1alpha1.Postgresql{
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "test-id",
				Address:      "localhost",
				Port:         0,  // Should default to 5432
				SSLMode:      "", // Should default to "require"
			},
		},
	}

	// This will fail to connect but should use defaults
	err := TestConnectionFromPostgresql(ctx, postgresql, nil, logger, 1, 1*time.Second)

	// Should return nil when vault client is not configured (skips test)
	assert.NoError(t, err)
}

func TestTestConnectionFromPostgresql_WithCustomPort(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	postgresql := &instancev1alpha1.Postgresql{
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "test-id",
				Address:      "localhost",
				Port:         5433,
				SSLMode:      "disable",
			},
		},
	}

	// Should return nil when vault client is not configured (skips test)
	err := TestConnectionFromPostgresql(ctx, postgresql, nil, logger, 1, 1*time.Second)
	assert.NoError(t, err)
}

func TestTestConnectionFromPostgresql_WithCustomSSLMode(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	postgresql := &instancev1alpha1.Postgresql{
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "test-id",
				Address:      "localhost",
				Port:         5432,
				SSLMode:      "verify-full",
			},
		},
	}

	// Should return nil when vault client is not configured (skips test)
	err := TestConnectionFromPostgresql(ctx, postgresql, nil, logger, 1, 1*time.Second)
	assert.NoError(t, err)
}
