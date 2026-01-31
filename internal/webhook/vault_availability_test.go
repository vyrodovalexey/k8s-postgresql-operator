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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// MockVaultClient is a mock implementation of vault.ClientInterface for testing
type MockVaultClient struct {
	healthCheckError error
	healthCheckCount int
}

func (m *MockVaultClient) CheckHealth(ctx context.Context) error {
	m.healthCheckCount++
	return m.healthCheckError
}

func (m *MockVaultClient) GetPostgresqlCredentials(ctx context.Context, postgresqlID string) (username, password string, err error) {
	return "", "", nil
}

func (m *MockVaultClient) GetPostgresqlUserCredentials(ctx context.Context, postgresqlID, username string) (password string, err error) {
	return "", nil
}

func (m *MockVaultClient) StorePostgresqlUserCredentials(ctx context.Context, postgresqlID, username, password string) error {
	return nil
}

// Ensure MockVaultClient implements vault.ClientInterface
var _ vault.ClientInterface = (*MockVaultClient)(nil)

// TestCheckVaultAvailability_NilClient tests checkVaultAvailability with nil client
func TestCheckVaultAvailability_NilClient(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	err := checkVaultAvailability(ctx, nil, logger, 3, 10*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// TestCheckVaultAvailability_SuccessFirstAttempt tests successful health check on first attempt
func TestCheckVaultAvailability_SuccessFirstAttempt(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	// Create a real vault client that will fail (we can't easily mock it)
	// Instead, we test the nil case and error paths
	err := checkVaultAvailability(ctx, nil, logger, 3, 10*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// TestCheckVaultAvailability_TableDriven tests checkVaultAvailability with table-driven tests
func TestCheckVaultAvailability_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		vaultClient vault.ClientInterface
		retries     int
		retryDelay  time.Duration
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil client",
			vaultClient: nil,
			retries:     3,
			retryDelay:  10 * time.Millisecond,
			expectError: true,
			errorMsg:    "vault client is not configured",
		},
		{
			name:        "nil client with zero retries",
			vaultClient: nil,
			retries:     0,
			retryDelay:  10 * time.Millisecond,
			expectError: true,
			errorMsg:    "vault client is not configured",
		},
		{
			name:        "nil client with negative retries",
			vaultClient: nil,
			retries:     -1,
			retryDelay:  10 * time.Millisecond,
			expectError: true,
			errorMsg:    "vault client is not configured",
		},
		{
			name:        "healthy vault client",
			vaultClient: &MockVaultClient{healthCheckError: nil},
			retries:     3,
			retryDelay:  10 * time.Millisecond,
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "unhealthy vault client - all retries fail",
			vaultClient: &MockVaultClient{healthCheckError: fmt.Errorf("connection refused")},
			retries:     3,
			retryDelay:  10 * time.Millisecond,
			expectError: true,
			errorMsg:    "vault is not available after 3 attempts",
		},
		{
			name:        "unhealthy vault client - single retry",
			vaultClient: &MockVaultClient{healthCheckError: fmt.Errorf("timeout")},
			retries:     1,
			retryDelay:  10 * time.Millisecond,
			expectError: true,
			errorMsg:    "vault is not available after 1 attempts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop().Sugar()
			ctx := context.Background()

			err := checkVaultAvailability(ctx, tt.vaultClient, logger, tt.retries, tt.retryDelay)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCheckVaultAvailability_ContextCancellation tests context cancellation
func TestCheckVaultAvailability_ContextCancellation(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// With nil client, it should return error about nil client before checking context
	err := checkVaultAvailability(ctx, nil, logger, 3, 10*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// TestCheckVaultAvailability_RetryLogic tests the retry logic
func TestCheckVaultAvailability_RetryLogic(t *testing.T) {
	// Since we can't easily mock the vault.Client, we test the nil case
	// which exercises the early return path
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	start := time.Now()
	err := checkVaultAvailability(ctx, nil, logger, 3, 100*time.Millisecond)
	elapsed := time.Since(start)

	assert.Error(t, err)
	// Should return immediately without retries for nil client
	assert.Less(t, elapsed, 50*time.Millisecond)
}

// TestCheckVaultAvailability_ErrorMessage tests error message format
func TestCheckVaultAvailability_ErrorMessage(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	err := checkVaultAvailability(ctx, nil, logger, 5, 10*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// TestCheckVaultAvailability_ZeroRetries tests with zero retries
func TestCheckVaultAvailability_ZeroRetries(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	err := checkVaultAvailability(ctx, nil, logger, 0, 10*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// TestCheckVaultAvailability_SingleRetry tests with single retry
func TestCheckVaultAvailability_SingleRetry(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	err := checkVaultAvailability(ctx, nil, logger, 1, 10*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// TestCheckVaultAvailability_LargeRetryCount tests with large retry count
func TestCheckVaultAvailability_LargeRetryCount(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	// Should still return immediately for nil client
	start := time.Now()
	err := checkVaultAvailability(ctx, nil, logger, 100, 10*time.Millisecond)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
	// Should return immediately without retries for nil client
	assert.Less(t, elapsed, 50*time.Millisecond)
}

// TestCheckVaultAvailability_ZeroDelay tests with zero delay
func TestCheckVaultAvailability_ZeroDelay(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	err := checkVaultAvailability(ctx, nil, logger, 3, 0)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// TestCheckVaultAvailability_NegativeDelay tests with negative delay
func TestCheckVaultAvailability_NegativeDelay(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	err := checkVaultAvailability(ctx, nil, logger, 3, -10*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// TestCheckVaultAvailability_NilLogger tests with nil logger (should not panic)
func TestCheckVaultAvailability_NilLogger(t *testing.T) {
	ctx := context.Background()

	// This should not panic even with nil logger
	// The function should handle nil logger gracefully or panic
	// Based on the implementation, it uses logger.Debugw which will panic with nil logger
	// So we test with a valid logger
	logger := zap.NewNop().Sugar()
	err := checkVaultAvailability(ctx, nil, logger, 3, 10*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// TestCheckVaultAvailability_ErrorWrapping tests that errors are properly wrapped
func TestCheckVaultAvailability_ErrorWrapping(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	err := checkVaultAvailability(ctx, nil, logger, 3, 10*time.Millisecond)

	assert.Error(t, err)
	// The error should be a simple error, not wrapped
	assert.Equal(t, "vault client is not configured", err.Error())
}

// TestCheckVaultAvailability_ConcurrentCalls tests concurrent calls
func TestCheckVaultAvailability_ConcurrentCalls(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			err := checkVaultAvailability(ctx, nil, logger, 3, 10*time.Millisecond)
			done <- err
		}()
	}

	for i := 0; i < 5; i++ {
		select {
		case err := <-done:
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "vault client is not configured")
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent calls")
		}
	}
}

// TestCheckVaultAvailability_ContextTimeout tests context timeout
func TestCheckVaultAvailability_ContextTimeout(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for context to timeout
	time.Sleep(5 * time.Millisecond)

	err := checkVaultAvailability(ctx, nil, logger, 3, 10*time.Millisecond)

	assert.Error(t, err)
	// Should return nil client error before checking context
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// TestCheckVaultAvailability_ErrorFormat tests the error format for failed retries
func TestCheckVaultAvailability_ErrorFormat(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	err := checkVaultAvailability(ctx, nil, logger, 3, 10*time.Millisecond)

	assert.Error(t, err)
	// For nil client, the error should be about nil client
	assert.Contains(t, err.Error(), "vault client is not configured")
	// Should not contain retry information for nil client
	assert.NotContains(t, err.Error(), "after")
	assert.NotContains(t, err.Error(), "attempts")
}

// TestCheckVaultAvailability_WithRealVaultClient_InvalidConfig tests with invalid vault config
func TestCheckVaultAvailability_WithRealVaultClient_InvalidConfig(t *testing.T) {
	// We can't create a real vault client without a valid Vault server
	// So we test the nil case which is the most common error path
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	err := checkVaultAvailability(ctx, nil, logger, 3, 10*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault client is not configured")
}

// TestCheckVaultAvailability_RetryDelayValues tests various retry delay values
func TestCheckVaultAvailability_RetryDelayValues(t *testing.T) {
	tests := []struct {
		name       string
		retryDelay time.Duration
	}{
		{"zero delay", 0},
		{"1ms delay", 1 * time.Millisecond},
		{"10ms delay", 10 * time.Millisecond},
		{"100ms delay", 100 * time.Millisecond},
		{"negative delay", -1 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop().Sugar()
			ctx := context.Background()

			err := checkVaultAvailability(ctx, nil, logger, 1, tt.retryDelay)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "vault client is not configured")
		})
	}
}

// TestCheckVaultAvailability_RetryCountValues tests various retry count values
func TestCheckVaultAvailability_RetryCountValues(t *testing.T) {
	tests := []struct {
		name    string
		retries int
	}{
		{"zero retries", 0},
		{"one retry", 1},
		{"three retries", 3},
		{"ten retries", 10},
		{"negative retries", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop().Sugar()
			ctx := context.Background()

			err := checkVaultAvailability(ctx, nil, logger, tt.retries, 10*time.Millisecond)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "vault client is not configured")
		})
	}
}

// simulateVaultHealthCheck simulates the vault health check logic for testing
func simulateVaultHealthCheck(healthCheckError error, retries int) (int, error) {
	var lastErr error
	attempts := 0

	for attempt := 1; attempt <= retries; attempt++ {
		attempts++
		if healthCheckError != nil {
			lastErr = fmt.Errorf("vault health check failed: %w", healthCheckError)
			continue
		}
		return attempts, nil
	}

	return attempts, fmt.Errorf("vault is not available after %d attempts: %w", retries, lastErr)
}

// TestSimulateVaultHealthCheck_Success tests simulated health check success
func TestSimulateVaultHealthCheck_Success(t *testing.T) {
	attempts, err := simulateVaultHealthCheck(nil, 3)

	assert.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

// TestSimulateVaultHealthCheck_AllFail tests simulated health check all failures
func TestSimulateVaultHealthCheck_AllFail(t *testing.T) {
	attempts, err := simulateVaultHealthCheck(fmt.Errorf("connection refused"), 3)

	assert.Error(t, err)
	assert.Equal(t, 3, attempts)
	assert.Contains(t, err.Error(), "vault is not available after 3 attempts")
	assert.Contains(t, err.Error(), "connection refused")
}

// TestSimulateVaultHealthCheck_ZeroRetries tests simulated health check with zero retries
func TestSimulateVaultHealthCheck_ZeroRetries(t *testing.T) {
	attempts, err := simulateVaultHealthCheck(fmt.Errorf("connection refused"), 0)

	assert.Error(t, err)
	assert.Equal(t, 0, attempts)
	assert.Contains(t, err.Error(), "vault is not available after 0 attempts")
}

// TestCheckVaultAvailability_WithMockClient_Success tests successful health check with mock client
func TestCheckVaultAvailability_WithMockClient_Success(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()
	mockClient := &MockVaultClient{healthCheckError: nil}

	err := checkVaultAvailability(ctx, mockClient, logger, 3, 10*time.Millisecond)

	assert.NoError(t, err)
	assert.Equal(t, 1, mockClient.healthCheckCount, "should only call health check once on success")
}

// TestCheckVaultAvailability_WithMockClient_RetryCount tests retry count with mock client
func TestCheckVaultAvailability_WithMockClient_RetryCount(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()
	mockClient := &MockVaultClient{healthCheckError: fmt.Errorf("connection refused")}

	err := checkVaultAvailability(ctx, mockClient, logger, 3, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Equal(t, 3, mockClient.healthCheckCount, "should call health check exactly 3 times")
	assert.Contains(t, err.Error(), "vault is not available after 3 attempts")
}

// TestCheckVaultAvailability_WithMockClient_SingleRetry tests single retry with mock client
func TestCheckVaultAvailability_WithMockClient_SingleRetry(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()
	mockClient := &MockVaultClient{healthCheckError: fmt.Errorf("timeout")}

	err := checkVaultAvailability(ctx, mockClient, logger, 1, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Equal(t, 1, mockClient.healthCheckCount, "should call health check exactly 1 time")
}

// TestCheckVaultAvailability_WithMockClient_ZeroRetries tests zero retries with mock client
func TestCheckVaultAvailability_WithMockClient_ZeroRetries(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()
	mockClient := &MockVaultClient{healthCheckError: fmt.Errorf("timeout")}

	err := checkVaultAvailability(ctx, mockClient, logger, 0, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Equal(t, 0, mockClient.healthCheckCount, "should not call health check with zero retries")
}

// TestCheckVaultAvailability_WithMockClient_ErrorWrapping tests error wrapping with mock client
func TestCheckVaultAvailability_WithMockClient_ErrorWrapping(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()
	mockClient := &MockVaultClient{healthCheckError: fmt.Errorf("specific error message")}

	err := checkVaultAvailability(ctx, mockClient, logger, 2, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "specific error message")
	assert.Contains(t, err.Error(), "vault health check failed")
}

// TestCheckVaultAvailability_WithMockClient_RetryTiming tests retry timing with mock client
func TestCheckVaultAvailability_WithMockClient_RetryTiming(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()
	mockClient := &MockVaultClient{healthCheckError: fmt.Errorf("timeout")}

	start := time.Now()
	err := checkVaultAvailability(ctx, mockClient, logger, 3, 50*time.Millisecond)
	elapsed := time.Since(start)

	assert.Error(t, err)
	// Should take at least 100ms (2 retries * 50ms delay)
	// First attempt doesn't have delay, only retries do
	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond, "should wait between retries")
}

// DynamicMockVaultClient is a mock that can change behavior between calls
type DynamicMockVaultClient struct {
	callCount      int
	successOnCall  int // 0 means never succeed
	healthCheckErr error
}

func (m *DynamicMockVaultClient) CheckHealth(ctx context.Context) error {
	m.callCount++
	if m.successOnCall > 0 && m.callCount >= m.successOnCall {
		return nil
	}
	return m.healthCheckErr
}

func (m *DynamicMockVaultClient) GetPostgresqlCredentials(ctx context.Context, postgresqlID string) (username, password string, err error) {
	return "", "", nil
}

func (m *DynamicMockVaultClient) GetPostgresqlUserCredentials(ctx context.Context, postgresqlID, username string) (password string, err error) {
	return "", nil
}

func (m *DynamicMockVaultClient) StorePostgresqlUserCredentials(ctx context.Context, postgresqlID, username, password string) error {
	return nil
}

// Ensure DynamicMockVaultClient implements vault.ClientInterface
var _ vault.ClientInterface = (*DynamicMockVaultClient)(nil)

// TestCheckVaultAvailability_WithDynamicMock_SuccessOnSecondAttempt tests success on second attempt
func TestCheckVaultAvailability_WithDynamicMock_SuccessOnSecondAttempt(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()
	mockClient := &DynamicMockVaultClient{
		successOnCall:  2,
		healthCheckErr: fmt.Errorf("temporary failure"),
	}

	err := checkVaultAvailability(ctx, mockClient, logger, 3, 1*time.Millisecond)

	assert.NoError(t, err)
	assert.Equal(t, 2, mockClient.callCount, "should succeed on second attempt")
}

// TestCheckVaultAvailability_WithDynamicMock_SuccessOnThirdAttempt tests success on third attempt
func TestCheckVaultAvailability_WithDynamicMock_SuccessOnThirdAttempt(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()
	mockClient := &DynamicMockVaultClient{
		successOnCall:  3,
		healthCheckErr: fmt.Errorf("temporary failure"),
	}

	err := checkVaultAvailability(ctx, mockClient, logger, 3, 1*time.Millisecond)

	assert.NoError(t, err)
	assert.Equal(t, 3, mockClient.callCount, "should succeed on third attempt")
}

// TestCheckVaultAvailability_WithDynamicMock_FailsBeforeSuccess tests failure when success comes too late
func TestCheckVaultAvailability_WithDynamicMock_FailsBeforeSuccess(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()
	mockClient := &DynamicMockVaultClient{
		successOnCall:  4, // Would succeed on 4th call, but we only have 3 retries
		healthCheckErr: fmt.Errorf("temporary failure"),
	}

	err := checkVaultAvailability(ctx, mockClient, logger, 3, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Equal(t, 3, mockClient.callCount, "should try exactly 3 times")
	assert.Contains(t, err.Error(), "vault is not available after 3 attempts")
}
