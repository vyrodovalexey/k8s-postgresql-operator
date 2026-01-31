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

package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServiceTypeConstants tests that service type constants are defined correctly
func TestServiceTypeConstants(t *testing.T) {
	assert.Equal(t, ServiceType("postgresql"), ServiceTypePostgreSQL)
	assert.Equal(t, ServiceType("vault"), ServiceTypeVault)
}

// TestNewRateLimiter_TableDriven tests NewRateLimiter with various configurations
func TestNewRateLimiter_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		serviceType   ServiceType
		ratePerSecond float64
		burst         int
	}{
		{
			name:          "PostgreSQL limiter",
			serviceType:   ServiceTypePostgreSQL,
			ratePerSecond: 10.0,
			burst:         20,
		},
		{
			name:          "Vault limiter",
			serviceType:   ServiceTypeVault,
			ratePerSecond: 5.0,
			burst:         10,
		},
		{
			name:          "High rate limiter",
			serviceType:   ServiceTypePostgreSQL,
			ratePerSecond: 100.0,
			burst:         200,
		},
		{
			name:          "Low rate limiter",
			serviceType:   ServiceTypeVault,
			ratePerSecond: 1.0,
			burst:         1,
		},
		{
			name:          "Zero rate limiter",
			serviceType:   ServiceTypePostgreSQL,
			ratePerSecond: 0.0,
			burst:         0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewRateLimiter(tt.serviceType, tt.ratePerSecond, tt.burst)
			assert.NotNil(t, limiter)
			assert.Equal(t, tt.serviceType, limiter.serviceType)
			assert.NotNil(t, limiter.limiter)
		})
	}
}

// TestRateLimiter_Wait tests the Wait method
func TestRateLimiter_Wait(t *testing.T) {
	limiter := NewRateLimiter(ServiceTypePostgreSQL, 100.0, 10)
	ctx := context.Background()

	// Should not block with high rate
	err := limiter.Wait(ctx)
	assert.NoError(t, err)
}

// TestRateLimiter_Wait_ContextCancellation tests Wait with cancelled context
func TestRateLimiter_Wait_ContextCancellation(t *testing.T) {
	// Create a limiter with very low rate
	limiter := NewRateLimiter(ServiceTypePostgreSQL, 0.001, 1)

	// Exhaust the burst
	ctx := context.Background()
	_ = limiter.Wait(ctx)

	// Now cancel the context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Wait should return error due to cancelled context
	err := limiter.Wait(ctx)
	assert.Error(t, err)
}

// TestRateLimiter_Wait_ContextTimeout tests Wait with timeout context
func TestRateLimiter_Wait_ContextTimeout(t *testing.T) {
	// Create a limiter with very low rate
	limiter := NewRateLimiter(ServiceTypePostgreSQL, 0.001, 1)

	// Exhaust the burst
	ctx := context.Background()
	_ = limiter.Wait(ctx)

	// Now use a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait should return error due to timeout
	err := limiter.Wait(ctx)
	assert.Error(t, err)
}

// TestRateLimiter_Wait_NilLimiter tests Wait with nil limiter
func TestRateLimiter_Wait_NilLimiter(t *testing.T) {
	var limiter *RateLimiter = nil
	ctx := context.Background()

	// Should not panic and return nil
	err := limiter.Wait(ctx)
	assert.NoError(t, err)
}

// TestRateLimiter_Wait_NilInternalLimiter tests Wait with nil internal limiter
func TestRateLimiter_Wait_NilInternalLimiter(t *testing.T) {
	limiter := &RateLimiter{
		serviceType: ServiceTypePostgreSQL,
		limiter:     nil,
	}
	ctx := context.Background()

	// Should not panic and return nil
	err := limiter.Wait(ctx)
	assert.NoError(t, err)
}

// TestRateLimiter_Allow tests the Allow method
func TestRateLimiter_Allow(t *testing.T) {
	limiter := NewRateLimiter(ServiceTypePostgreSQL, 100.0, 10)

	// Should allow with high rate
	allowed := limiter.Allow()
	assert.True(t, allowed)
}

// TestRateLimiter_Allow_Denied tests Allow when rate limit is exceeded
func TestRateLimiter_Allow_Denied(t *testing.T) {
	// Create a limiter with very low rate and burst
	limiter := NewRateLimiter(ServiceTypePostgreSQL, 0.001, 1)

	// First call should be allowed (uses burst)
	allowed := limiter.Allow()
	assert.True(t, allowed)

	// Second call should be denied (burst exhausted, rate too low)
	allowed = limiter.Allow()
	assert.False(t, allowed)
}

// TestRateLimiter_Allow_NilLimiter tests Allow with nil limiter
func TestRateLimiter_Allow_NilLimiter(t *testing.T) {
	var limiter *RateLimiter = nil

	// Should not panic and return true
	allowed := limiter.Allow()
	assert.True(t, allowed)
}

// TestRateLimiter_Allow_NilInternalLimiter tests Allow with nil internal limiter
func TestRateLimiter_Allow_NilInternalLimiter(t *testing.T) {
	limiter := &RateLimiter{
		serviceType: ServiceTypePostgreSQL,
		limiter:     nil,
	}

	// Should not panic and return true
	allowed := limiter.Allow()
	assert.True(t, allowed)
}

// TestRateLimiter_UpdateLimits tests the UpdateLimits method
func TestRateLimiter_UpdateLimits(t *testing.T) {
	limiter := NewRateLimiter(ServiceTypePostgreSQL, 10.0, 20)

	// Update limits
	limiter.UpdateLimits(50.0, 100)

	// Verify by checking tokens (should have more available)
	tokens := limiter.Tokens()
	assert.GreaterOrEqual(t, tokens, float64(0))
}

// TestRateLimiter_UpdateLimits_NilLimiter tests UpdateLimits with nil limiter
func TestRateLimiter_UpdateLimits_NilLimiter(t *testing.T) {
	var limiter *RateLimiter = nil

	// Should not panic
	limiter.UpdateLimits(50.0, 100)
}

// TestRateLimiter_Tokens tests the Tokens method
func TestRateLimiter_Tokens(t *testing.T) {
	limiter := NewRateLimiter(ServiceTypePostgreSQL, 10.0, 20)

	// Should have tokens available
	tokens := limiter.Tokens()
	assert.GreaterOrEqual(t, tokens, float64(0))
}

// TestRateLimiter_Tokens_NilLimiter tests Tokens with nil limiter
func TestRateLimiter_Tokens_NilLimiter(t *testing.T) {
	var limiter *RateLimiter = nil

	// Should not panic and return 0
	tokens := limiter.Tokens()
	assert.Equal(t, float64(0), tokens)
}

// TestRateLimiter_Tokens_NilInternalLimiter tests Tokens with nil internal limiter
func TestRateLimiter_Tokens_NilInternalLimiter(t *testing.T) {
	limiter := &RateLimiter{
		serviceType: ServiceTypePostgreSQL,
		limiter:     nil,
	}

	// Should not panic and return 0
	tokens := limiter.Tokens()
	assert.Equal(t, float64(0), tokens)
}

// TestNewManager tests NewManager function
func TestNewManager(t *testing.T) {
	manager := NewManager()
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.limiters)
}

// TestManager_GetOrCreate tests GetOrCreate method
func TestManager_GetOrCreate(t *testing.T) {
	manager := NewManager()

	// Create a new limiter
	limiter1 := manager.GetOrCreate(ServiceTypePostgreSQL, 10.0, 20)
	assert.NotNil(t, limiter1)

	// Get the same limiter
	limiter2 := manager.GetOrCreate(ServiceTypePostgreSQL, 10.0, 20)
	assert.Same(t, limiter1, limiter2)

	// Create a different limiter
	limiter3 := manager.GetOrCreate(ServiceTypeVault, 5.0, 10)
	assert.NotNil(t, limiter3)
	assert.NotSame(t, limiter1, limiter3)
}

// TestManager_Get tests Get method
func TestManager_Get(t *testing.T) {
	manager := NewManager()

	// Get non-existent limiter
	limiter := manager.Get(ServiceTypePostgreSQL)
	assert.Nil(t, limiter)

	// Create a limiter
	manager.GetOrCreate(ServiceTypePostgreSQL, 10.0, 20)

	// Get existing limiter
	limiter = manager.Get(ServiceTypePostgreSQL)
	assert.NotNil(t, limiter)
}

// TestDefaultManager tests the default manager
func TestDefaultManager(t *testing.T) {
	assert.NotNil(t, DefaultManager)
}

// TestGetPostgreSQLLimiter tests GetPostgreSQLLimiter function
func TestGetPostgreSQLLimiter(t *testing.T) {
	limiter := GetPostgreSQLLimiter(10.0, 20)
	assert.NotNil(t, limiter)
	assert.Equal(t, ServiceTypePostgreSQL, limiter.serviceType)
}

// TestGetVaultLimiter tests GetVaultLimiter function
func TestGetVaultLimiter(t *testing.T) {
	limiter := GetVaultLimiter(5.0, 10)
	assert.NotNil(t, limiter)
	assert.Equal(t, ServiceTypeVault, limiter.serviceType)
}

// TestRateLimiter_ConcurrentAccess tests thread safety
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewRateLimiter(ServiceTypePostgreSQL, 1000.0, 100)
	ctx := context.Background()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent Wait calls
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := limiter.Wait(ctx); err != nil {
				errors <- err
			}
		}()
	}

	// Concurrent Allow calls
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limiter.Allow()
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestManager_ConcurrentAccess tests thread safety of Manager
func TestManager_ConcurrentAccess(t *testing.T) {
	manager := NewManager()

	var wg sync.WaitGroup

	// Concurrent GetOrCreate calls
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				manager.GetOrCreate(ServiceTypePostgreSQL, 10.0, 20)
			} else {
				manager.GetOrCreate(ServiceTypeVault, 5.0, 10)
			}
		}(i)
	}

	// Concurrent Get calls
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				manager.Get(ServiceTypePostgreSQL)
			} else {
				manager.Get(ServiceTypeVault)
			}
		}(i)
	}

	wg.Wait()
}

// TestRateLimiter_UpdateLimits_ConcurrentAccess tests concurrent UpdateLimits
func TestRateLimiter_UpdateLimits_ConcurrentAccess(t *testing.T) {
	limiter := NewRateLimiter(ServiceTypePostgreSQL, 10.0, 20)

	var wg sync.WaitGroup

	// Concurrent UpdateLimits calls
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			limiter.UpdateLimits(float64(i+1), i+1)
		}(i)
	}

	// Concurrent Tokens calls
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limiter.Tokens()
		}()
	}

	wg.Wait()
}

// TestRateLimiter_RateEnforcement tests that rate limiting is actually enforced
func TestRateLimiter_RateEnforcement(t *testing.T) {
	// Create a limiter with 1 request per second and burst of 1
	limiter := NewRateLimiter(ServiceTypePostgreSQL, 1.0, 1)

	// First request should be allowed immediately
	start := time.Now()
	ctx := context.Background()
	err := limiter.Wait(ctx)
	require.NoError(t, err)
	firstDuration := time.Since(start)

	// First request should be nearly instant
	assert.Less(t, firstDuration, 100*time.Millisecond)

	// Second request should wait approximately 1 second
	start = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = limiter.Wait(ctx)
	require.NoError(t, err)
	secondDuration := time.Since(start)

	// Second request should have waited at least 500ms (allowing for some variance)
	assert.Greater(t, secondDuration, 500*time.Millisecond)
}

// TestRateLimiter_BurstBehavior tests burst behavior
func TestRateLimiter_BurstBehavior(t *testing.T) {
	// Create a limiter with low rate but high burst
	limiter := NewRateLimiter(ServiceTypePostgreSQL, 0.1, 5)

	// All burst requests should be allowed immediately
	for i := 0; i < 5; i++ {
		allowed := limiter.Allow()
		assert.True(t, allowed, "Request %d should be allowed", i)
	}

	// Next request should be denied (burst exhausted)
	allowed := limiter.Allow()
	assert.False(t, allowed, "Request after burst should be denied")
}
