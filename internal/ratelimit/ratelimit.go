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

// Package ratelimit provides rate limiting functionality for external service calls
package ratelimit

import (
	"context"
	"sync"

	"golang.org/x/time/rate"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/metrics"
)

// ServiceType represents the type of external service being rate limited
type ServiceType string

const (
	// ServiceTypePostgreSQL represents PostgreSQL service
	ServiceTypePostgreSQL ServiceType = "postgresql"
	// ServiceTypeVault represents Vault service
	ServiceTypeVault ServiceType = "vault"
)

// RateLimiter provides rate limiting for external service calls
type RateLimiter struct {
	limiter     *rate.Limiter
	serviceType ServiceType
	mu          sync.RWMutex
}

// NewRateLimiter creates a new rate limiter for the specified service
// ratePerSecond is the number of requests allowed per second
// burst is the maximum number of requests that can be made at once
func NewRateLimiter(serviceType ServiceType, ratePerSecond float64, burst int) *RateLimiter {
	return &RateLimiter{
		limiter:     rate.NewLimiter(rate.Limit(ratePerSecond), burst),
		serviceType: serviceType,
	}
}

// Wait blocks until the rate limiter allows an event to happen
// It returns an error if the context is cancelled
func (r *RateLimiter) Wait(ctx context.Context) error {
	if r == nil || r.limiter == nil {
		return nil
	}

	r.mu.RLock()
	limiter := r.limiter
	r.mu.RUnlock()

	err := limiter.Wait(ctx)
	if err != nil {
		metrics.RecordRateLimitWait(string(r.serviceType), "error")
		return err
	}

	metrics.RecordRateLimitWait(string(r.serviceType), "success")
	return nil
}

// Allow reports whether an event may happen now
// It does not block and returns immediately
func (r *RateLimiter) Allow() bool {
	if r == nil || r.limiter == nil {
		return true
	}

	r.mu.RLock()
	limiter := r.limiter
	r.mu.RUnlock()

	allowed := limiter.Allow()
	if allowed {
		metrics.RecordRateLimitAllow(string(r.serviceType), "allowed")
	} else {
		metrics.RecordRateLimitAllow(string(r.serviceType), "denied")
	}

	return allowed
}

// UpdateLimits updates the rate limiter with new limits
func (r *RateLimiter) UpdateLimits(ratePerSecond float64, burst int) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.limiter.SetLimit(rate.Limit(ratePerSecond))
	r.limiter.SetBurst(burst)
}

// Tokens returns the current number of available tokens
func (r *RateLimiter) Tokens() float64 {
	if r == nil || r.limiter == nil {
		return 0
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.limiter.Tokens()
}

// Manager manages rate limiters for different services
type Manager struct {
	limiters map[ServiceType]*RateLimiter
	mu       sync.RWMutex
}

// NewManager creates a new rate limiter manager
func NewManager() *Manager {
	return &Manager{
		limiters: make(map[ServiceType]*RateLimiter),
	}
}

// GetOrCreate returns an existing rate limiter or creates a new one
func (m *Manager) GetOrCreate(serviceType ServiceType, ratePerSecond float64, burst int) *RateLimiter {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limiter, exists := m.limiters[serviceType]; exists {
		return limiter
	}

	limiter := NewRateLimiter(serviceType, ratePerSecond, burst)
	m.limiters[serviceType] = limiter
	return limiter
}

// Get returns an existing rate limiter or nil if not found
func (m *Manager) Get(serviceType ServiceType) *RateLimiter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.limiters[serviceType]
}

// DefaultManager is the default rate limiter manager
var DefaultManager = NewManager()

// GetPostgreSQLLimiter returns the PostgreSQL rate limiter from the default manager
func GetPostgreSQLLimiter(ratePerSecond float64, burst int) *RateLimiter {
	return DefaultManager.GetOrCreate(ServiceTypePostgreSQL, ratePerSecond, burst)
}

// GetVaultLimiter returns the Vault rate limiter from the default manager
func GetVaultLimiter(ratePerSecond float64, burst int) *RateLimiter {
	return DefaultManager.GetOrCreate(ServiceTypeVault, ratePerSecond, burst)
}
