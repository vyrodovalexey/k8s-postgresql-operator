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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/ratelimit"
)

// TestGetVaultCredentialsWithRetryAndRateLimit_TableDriven tests the function
func TestGetVaultCredentialsWithRetryAndRateLimit_TableDriven(t *testing.T) {
	tests := []struct {
		name                string
		vaultClient         interface{} // nil or *vault.Client
		postgresqlID        string
		retries             int
		retryDelay          time.Duration
		expectedUsername    string
		expectedPassword    string
		expectedError       bool
		expectedErrorSubstr string
	}{
		{
			name:                "Nil vault client returns error",
			vaultClient:         nil,
			postgresqlID:        "test-id",
			retries:             3,
			retryDelay:          10 * time.Millisecond,
			expectedUsername:    "",
			expectedPassword:    "",
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
		{
			name:                "Nil vault client with zero retries",
			vaultClient:         nil,
			postgresqlID:        "test-id",
			retries:             0,
			retryDelay:          0,
			expectedUsername:    "",
			expectedPassword:    "",
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
		{
			name:                "Nil vault client with single retry",
			vaultClient:         nil,
			postgresqlID:        "test-id",
			retries:             1,
			retryDelay:          1 * time.Millisecond,
			expectedUsername:    "",
			expectedPassword:    "",
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
		{
			name:                "Nil vault client with empty postgresqlID",
			vaultClient:         nil,
			postgresqlID:        "",
			retries:             3,
			retryDelay:          10 * time.Millisecond,
			expectedUsername:    "",
			expectedPassword:    "",
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			logger := zap.NewNop().Sugar()
			ctx := context.Background()

			// Act
			username, password, err := getVaultCredentialsWithRetryAndRateLimit(
				ctx, nil, tt.postgresqlID, logger, tt.retries, tt.retryDelay, nil)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
				if tt.expectedErrorSubstr != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorSubstr)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedUsername, username)
			assert.Equal(t, tt.expectedPassword, password)
		})
	}
}

// TestGetVaultUserCredentialsWithRetryAndRateLimit_TableDriven tests the function
func TestGetVaultUserCredentialsWithRetryAndRateLimit_TableDriven(t *testing.T) {
	tests := []struct {
		name                string
		vaultClient         interface{} // nil or *vault.Client
		postgresqlID        string
		username            string
		retries             int
		retryDelay          time.Duration
		expectedPassword    string
		expectedError       bool
		expectedErrorSubstr string
	}{
		{
			name:                "Nil vault client returns error",
			vaultClient:         nil,
			postgresqlID:        "test-id",
			username:            "testuser",
			retries:             3,
			retryDelay:          10 * time.Millisecond,
			expectedPassword:    "",
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
		{
			name:                "Nil vault client with zero retries",
			vaultClient:         nil,
			postgresqlID:        "test-id",
			username:            "testuser",
			retries:             0,
			retryDelay:          0,
			expectedPassword:    "",
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
		{
			name:                "Nil vault client with single retry",
			vaultClient:         nil,
			postgresqlID:        "test-id",
			username:            "testuser",
			retries:             1,
			retryDelay:          1 * time.Millisecond,
			expectedPassword:    "",
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
		{
			name:                "Nil vault client with empty username",
			vaultClient:         nil,
			postgresqlID:        "test-id",
			username:            "",
			retries:             3,
			retryDelay:          10 * time.Millisecond,
			expectedPassword:    "",
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
		{
			name:                "Nil vault client with empty postgresqlID",
			vaultClient:         nil,
			postgresqlID:        "",
			username:            "testuser",
			retries:             3,
			retryDelay:          10 * time.Millisecond,
			expectedPassword:    "",
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			logger := zap.NewNop().Sugar()
			ctx := context.Background()

			// Act
			password, err := getVaultUserCredentialsWithRetryAndRateLimit(
				ctx, nil, tt.postgresqlID, tt.username, logger, tt.retries, tt.retryDelay, nil)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
				if tt.expectedErrorSubstr != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorSubstr)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedPassword, password)
		})
	}
}

// TestStoreVaultUserCredentialsWithRetryAndRateLimit_TableDriven tests the function
func TestStoreVaultUserCredentialsWithRetryAndRateLimit_TableDriven(t *testing.T) {
	tests := []struct {
		name                string
		vaultClient         interface{} // nil or *vault.Client
		postgresqlID        string
		username            string
		password            string
		retries             int
		retryDelay          time.Duration
		expectedError       bool
		expectedErrorSubstr string
	}{
		{
			name:                "Nil vault client returns error",
			vaultClient:         nil,
			postgresqlID:        "test-id",
			username:            "testuser",
			password:            "testpass",
			retries:             3,
			retryDelay:          10 * time.Millisecond,
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
		{
			name:                "Nil vault client with zero retries",
			vaultClient:         nil,
			postgresqlID:        "test-id",
			username:            "testuser",
			password:            "testpass",
			retries:             0,
			retryDelay:          0,
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
		{
			name:                "Nil vault client with single retry",
			vaultClient:         nil,
			postgresqlID:        "test-id",
			username:            "testuser",
			password:            "testpass",
			retries:             1,
			retryDelay:          1 * time.Millisecond,
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
		{
			name:                "Nil vault client with empty password",
			vaultClient:         nil,
			postgresqlID:        "test-id",
			username:            "testuser",
			password:            "",
			retries:             3,
			retryDelay:          10 * time.Millisecond,
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
		{
			name:                "Nil vault client with empty username",
			vaultClient:         nil,
			postgresqlID:        "test-id",
			username:            "",
			password:            "testpass",
			retries:             3,
			retryDelay:          10 * time.Millisecond,
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
		{
			name:                "Nil vault client with empty postgresqlID",
			vaultClient:         nil,
			postgresqlID:        "",
			username:            "testuser",
			password:            "testpass",
			retries:             3,
			retryDelay:          10 * time.Millisecond,
			expectedError:       true,
			expectedErrorSubstr: "vault client is not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			logger := zap.NewNop().Sugar()
			ctx := context.Background()

			// Act
			err := storeVaultUserCredentialsWithRetryAndRateLimit(
				ctx, nil, tt.postgresqlID, tt.username, tt.password, logger, tt.retries, tt.retryDelay, nil)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
				if tt.expectedErrorSubstr != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorSubstr)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestGetVaultCredentialsWithRetryAndRateLimit_ContextCancellation tests context cancellation
func TestGetVaultCredentialsWithRetryAndRateLimit_ContextCancellation(t *testing.T) {
	// Arrange
	logger := zap.NewNop().Sugar()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Act - with nil client, should return error before checking context
	username, password, err := getVaultCredentialsWithRetryAndRateLimit(
		ctx, nil, "test-id", logger, 3, 10*time.Millisecond, nil)

	// Assert - nil client error takes precedence
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
	assert.Empty(t, username)
	assert.Empty(t, password)
}

// TestGetVaultUserCredentialsWithRetryAndRateLimit_ContextCancellation tests context cancellation
func TestGetVaultUserCredentialsWithRetryAndRateLimit_ContextCancellation(t *testing.T) {
	// Arrange
	logger := zap.NewNop().Sugar()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Act - with nil client, should return error before checking context
	password, err := getVaultUserCredentialsWithRetryAndRateLimit(
		ctx, nil, "test-id", "testuser", logger, 3, 10*time.Millisecond, nil)

	// Assert - nil client error takes precedence
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
	assert.Empty(t, password)
}

// TestStoreVaultUserCredentialsWithRetryAndRateLimit_ContextCancellation tests context cancellation
func TestStoreVaultUserCredentialsWithRetryAndRateLimit_ContextCancellation(t *testing.T) {
	// Arrange
	logger := zap.NewNop().Sugar()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Act - with nil client, should return error before checking context
	err := storeVaultUserCredentialsWithRetryAndRateLimit(
		ctx, nil, "test-id", "testuser", "testpass", logger, 3, 10*time.Millisecond, nil)

	// Assert - nil client error takes precedence
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// TestRetryFunctions_EdgeCases tests edge cases for retry functions
func TestRetryFunctions_EdgeCases(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	t.Run("GetVaultCredentialsWithRetryAndRateLimit with negative retries", func(t *testing.T) {
		username, password, err := getVaultCredentialsWithRetryAndRateLimit(
			ctx, nil, "test-id", logger, -1, 10*time.Millisecond, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault client is not configured")
		assert.Empty(t, username)
		assert.Empty(t, password)
	})

	t.Run("GetVaultUserCredentialsWithRetryAndRateLimit with negative retries", func(t *testing.T) {
		password, err := getVaultUserCredentialsWithRetryAndRateLimit(
			ctx, nil, "test-id", "testuser", logger, -1, 10*time.Millisecond, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault client is not configured")
		assert.Empty(t, password)
	})

	t.Run("StoreVaultUserCredentialsWithRetryAndRateLimit with negative retries", func(t *testing.T) {
		err := storeVaultUserCredentialsWithRetryAndRateLimit(
			ctx, nil, "test-id", "testuser", "testpass", logger, -1, 10*time.Millisecond, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault client is not configured")
	})

	t.Run("GetVaultCredentialsWithRetryAndRateLimit with negative delay", func(t *testing.T) {
		username, password, err := getVaultCredentialsWithRetryAndRateLimit(
			ctx, nil, "test-id", logger, 3, -10*time.Millisecond, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault client is not configured")
		assert.Empty(t, username)
		assert.Empty(t, password)
	})
}

// TestRetryFunctions_WithRateLimiter tests retry functions with rate limiter
func TestRetryFunctions_WithRateLimiter(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	// Create a rate limiter with high rate to not block tests
	rateLimiter := ratelimit.NewRateLimiter(ratelimit.ServiceTypeVault, 1000, 100)

	t.Run("GetVaultCredentialsWithRetryAndRateLimit with rate limiter", func(t *testing.T) {
		username, password, err := getVaultCredentialsWithRetryAndRateLimit(
			ctx, nil, "test-id", logger, 3, 10*time.Millisecond, rateLimiter)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault client is not configured")
		assert.Empty(t, username)
		assert.Empty(t, password)
	})

	t.Run("GetVaultUserCredentialsWithRetryAndRateLimit with rate limiter", func(t *testing.T) {
		password, err := getVaultUserCredentialsWithRetryAndRateLimit(
			ctx, nil, "test-id", "testuser", logger, 3, 10*time.Millisecond, rateLimiter)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault client is not configured")
		assert.Empty(t, password)
	})

	t.Run("StoreVaultUserCredentialsWithRetryAndRateLimit with rate limiter", func(t *testing.T) {
		err := storeVaultUserCredentialsWithRetryAndRateLimit(
			ctx, nil, "test-id", "testuser", "testpass", logger, 3, 10*time.Millisecond, rateLimiter)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault client is not configured")
	})
}

// TestRetryFunctions_LargeRetryCount tests with large retry counts
func TestRetryFunctions_LargeRetryCount(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	t.Run("GetVaultCredentialsWithRetryAndRateLimit with large retry count", func(t *testing.T) {
		// Should return immediately due to nil client check
		start := time.Now()
		username, password, err := getVaultCredentialsWithRetryAndRateLimit(
			ctx, nil, "test-id", logger, 100, 10*time.Millisecond, nil)
		elapsed := time.Since(start)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault client is not configured")
		assert.Empty(t, username)
		assert.Empty(t, password)
		// Should return immediately, not wait for retries
		assert.Less(t, elapsed, 100*time.Millisecond)
	})

	t.Run("GetVaultUserCredentialsWithRetryAndRateLimit with large retry count", func(t *testing.T) {
		start := time.Now()
		password, err := getVaultUserCredentialsWithRetryAndRateLimit(
			ctx, nil, "test-id", "testuser", logger, 100, 10*time.Millisecond, nil)
		elapsed := time.Since(start)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault client is not configured")
		assert.Empty(t, password)
		assert.Less(t, elapsed, 100*time.Millisecond)
	})

	t.Run("StoreVaultUserCredentialsWithRetryAndRateLimit with large retry count", func(t *testing.T) {
		start := time.Now()
		err := storeVaultUserCredentialsWithRetryAndRateLimit(
			ctx, nil, "test-id", "testuser", "testpass", logger, 100, 10*time.Millisecond, nil)
		elapsed := time.Since(start)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault client is not configured")
		assert.Less(t, elapsed, 100*time.Millisecond)
	})
}

// TestRetryFunctions_SpecialCharacters tests with special characters in parameters
func TestRetryFunctions_SpecialCharacters(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	specialIDs := []string{
		"test-id-with-dashes",
		"test_id_with_underscores",
		"test.id.with.dots",
		"test/id/with/slashes",
		"test id with spaces",
		"test-id-123",
		"UPPERCASE-ID",
		"MixedCase-ID",
	}

	for _, id := range specialIDs {
		t.Run("GetVaultCredentialsWithRetryAndRateLimit with ID: "+id, func(t *testing.T) {
			username, password, err := getVaultCredentialsWithRetryAndRateLimit(
				ctx, nil, id, logger, 3, 10*time.Millisecond, nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "vault client is not configured")
			assert.Empty(t, username)
			assert.Empty(t, password)
		})
	}

	specialUsernames := []string{
		"user-with-dashes",
		"user_with_underscores",
		"user.with.dots",
		"user@domain.com",
		"user+tag",
		"UPPERCASE_USER",
	}

	for _, username := range specialUsernames {
		t.Run("GetVaultUserCredentialsWithRetryAndRateLimit with username: "+username, func(t *testing.T) {
			password, err := getVaultUserCredentialsWithRetryAndRateLimit(
				ctx, nil, "test-id", username, logger, 3, 10*time.Millisecond, nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "vault client is not configured")
			assert.Empty(t, password)
		})
	}
}

// TestRetryFunctions_ContextTimeout tests context timeout behavior
func TestRetryFunctions_ContextTimeout(t *testing.T) {
	logger := zap.NewNop().Sugar()

	t.Run("GetVaultCredentialsWithRetryAndRateLimit with timeout context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Wait for context to timeout
		time.Sleep(2 * time.Millisecond)

		username, password, err := getVaultCredentialsWithRetryAndRateLimit(
			ctx, nil, "test-id", logger, 3, 10*time.Millisecond, nil)
		assert.Error(t, err)
		// Nil client error takes precedence
		assert.Contains(t, err.Error(), "vault client is not configured")
		assert.Empty(t, username)
		assert.Empty(t, password)
	})

	t.Run("GetVaultUserCredentialsWithRetryAndRateLimit with timeout context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		time.Sleep(2 * time.Millisecond)

		password, err := getVaultUserCredentialsWithRetryAndRateLimit(
			ctx, nil, "test-id", "testuser", logger, 3, 10*time.Millisecond, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault client is not configured")
		assert.Empty(t, password)
	})

	t.Run("StoreVaultUserCredentialsWithRetryAndRateLimit with timeout context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		time.Sleep(2 * time.Millisecond)

		err := storeVaultUserCredentialsWithRetryAndRateLimit(
			ctx, nil, "test-id", "testuser", "testpass", logger, 3, 10*time.Millisecond, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault client is not configured")
	})
}
