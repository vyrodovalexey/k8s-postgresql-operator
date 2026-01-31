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

package metrics

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdatePostgresqlCount(t *testing.T) {
	// Reset the metric before test
	PostgresqlCount.Set(0)

	tests := []struct {
		name  string
		count float64
	}{
		{
			name:  "zero count",
			count: 0,
		},
		{
			name:  "single instance",
			count: 1,
		},
		{
			name:  "multiple instances",
			count: 5,
		},
		{
			name:  "large count",
			count: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdatePostgresqlCount(tt.count)

			metric := &dto.Metric{}
			err := PostgresqlCount.Write(metric)
			require.NoError(t, err)
			assert.Equal(t, tt.count, *metric.Gauge.Value)
		})
	}
}

func TestUpdateObjectCount(t *testing.T) {
	// Reset metrics before test
	ObjectCountPerPostgresqlID.Reset()

	tests := []struct {
		name         string
		kind         string
		postgresqlID string
		count        float64
	}{
		{
			name:         "user count",
			kind:         "user",
			postgresqlID: "pg-1",
			count:        3,
		},
		{
			name:         "database count",
			kind:         "database",
			postgresqlID: "pg-1",
			count:        2,
		},
		{
			name:         "grant count",
			kind:         "grant",
			postgresqlID: "pg-2",
			count:        5,
		},
		{
			name:         "zero count",
			kind:         "schema",
			postgresqlID: "pg-3",
			count:        0,
		},
		{
			name:         "same kind different postgresqlID",
			kind:         "user",
			postgresqlID: "pg-2",
			count:        10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateObjectCount(tt.kind, tt.postgresqlID, tt.count)

			metric := &dto.Metric{}
			err := ObjectCountPerPostgresqlID.WithLabelValues(tt.kind, tt.postgresqlID).Write(metric)
			require.NoError(t, err)
			assert.Equal(t, tt.count, *metric.Gauge.Value)
		})
	}
}

func TestSetObjectInfo(t *testing.T) {
	// Reset metrics before test
	ObjectNames.Reset()

	tests := []struct {
		name         string
		kind         string
		postgresqlID string
		objName      string
		namespace    string
	}{
		{
			name:         "user object",
			kind:         "user",
			postgresqlID: "pg-1",
			objName:      "user1",
			namespace:    "default",
		},
		{
			name:         "database object",
			kind:         "database",
			postgresqlID: "pg-1",
			objName:      "db1",
			namespace:    "production",
		},
		{
			name:         "grant object",
			kind:         "grant",
			postgresqlID: "pg-2",
			objName:      "grant1",
			namespace:    "test",
		},
		{
			name:         "schema object",
			kind:         "schema",
			postgresqlID: "pg-3",
			objName:      "schema1",
			namespace:    "default",
		},
		{
			name:         "rolegroup object",
			kind:         "rolegroup",
			postgresqlID: "pg-1",
			objName:      "rg1",
			namespace:    "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var databasename, username string
			if tt.kind == "database" {
				databasename = "testdb"
			} else if tt.kind == "user" {
				username = "testuser"
			}
			SetObjectInfo(tt.kind, tt.postgresqlID, tt.objName, tt.namespace, databasename, username)

			metric := &dto.Metric{}
			err := ObjectNames.WithLabelValues(
				tt.kind, tt.postgresqlID, tt.objName, tt.namespace, databasename, username,
			).Write(metric)
			require.NoError(t, err)
			assert.Equal(t, float64(1), *metric.Gauge.Value)
		})
	}
}

func TestRemoveObjectInfo(t *testing.T) {
	// Reset metrics before test
	ObjectNames.Reset()

	kind := "user"
	postgresqlID := "pg-1"
	objName := "user1"
	namespace := "default"

	// First set the metric
	SetObjectInfo(kind, postgresqlID, objName, namespace, "", "testuser")

	// Verify it exists
	metric := &dto.Metric{}
	err := ObjectNames.WithLabelValues(kind, postgresqlID, objName, namespace, "", "testuser").Write(metric)
	require.NoError(t, err)
	assert.Equal(t, float64(1), *metric.Gauge.Value)

	// Remove it
	RemoveObjectInfo(kind, postgresqlID, objName, namespace, "", "testuser")

	// Verify it's removed (should return an error or zero value)
	metric2 := &dto.Metric{}
	err2 := ObjectNames.WithLabelValues(kind, postgresqlID, objName, namespace, "", "testuser").Write(metric2)
	// After deletion, the metric might still exist but with a zero value or error
	// The behavior depends on Prometheus implementation
	if err2 == nil {
		// If no error, the value should be 0 or not set
		if metric2.Gauge != nil {
			assert.Equal(t, float64(0), *metric2.Gauge.Value)
		}
	}
}

func TestRemoveObjectCount(t *testing.T) {
	// Reset metrics before test
	ObjectCountPerPostgresqlID.Reset()

	kind := "user"
	postgresqlID := "pg-1"
	count := float64(5)

	// First set the metric
	UpdateObjectCount(kind, postgresqlID, count)

	// Verify it exists
	metric := &dto.Metric{}
	err := ObjectCountPerPostgresqlID.WithLabelValues(kind, postgresqlID).Write(metric)
	require.NoError(t, err)
	assert.Equal(t, count, *metric.Gauge.Value)

	// Remove it
	RemoveObjectCount(kind, postgresqlID)

	// Verify it's removed
	metric2 := &dto.Metric{}
	err2 := ObjectCountPerPostgresqlID.WithLabelValues(kind, postgresqlID).Write(metric2)
	// After deletion, the metric might still exist but with a zero value
	if err2 == nil && metric2.Gauge != nil {
		assert.Equal(t, float64(0), *metric2.Gauge.Value)
	}
}

func TestMetricsRegistration(t *testing.T) {
	// Test that all metrics are properly registered
	// This is a basic check to ensure metrics are accessible

	assert.NotNil(t, PostgresqlCount)
	assert.NotNil(t, ObjectCountPerPostgresqlID)
	assert.NotNil(t, ObjectNames)

	// Verify metric names by checking they can be written
	metric := &dto.Metric{}
	err := PostgresqlCount.Write(metric)
	require.NoError(t, err)
	assert.NotNil(t, metric)

	metric2 := &dto.Metric{}
	err2 := ObjectCountPerPostgresqlID.WithLabelValues("test", "test").Write(metric2)
	require.NoError(t, err2)
	assert.NotNil(t, metric2)

	metric3 := &dto.Metric{}
	err3 := ObjectNames.WithLabelValues("test", "test", "test", "test", "", "").Write(metric3)
	require.NoError(t, err3)
	assert.NotNil(t, metric3)
}

func TestMultipleUpdates(t *testing.T) {
	// Test that multiple updates work correctly
	PostgresqlCount.Set(0)
	ObjectCountPerPostgresqlID.Reset()
	ObjectNames.Reset()

	// Update multiple times
	UpdatePostgresqlCount(1)
	UpdatePostgresqlCount(2)
	UpdatePostgresqlCount(3)

	metric := &dto.Metric{}
	err := PostgresqlCount.Write(metric)
	require.NoError(t, err)
	assert.Equal(t, float64(3), *metric.Gauge.Value)

	// Update object count multiple times
	UpdateObjectCount("user", "pg-1", 1)
	UpdateObjectCount("user", "pg-1", 2)
	UpdateObjectCount("user", "pg-1", 3)

	metric2 := &dto.Metric{}
	err2 := ObjectCountPerPostgresqlID.WithLabelValues("user", "pg-1").Write(metric2)
	require.NoError(t, err2)
	assert.Equal(t, float64(3), *metric2.Gauge.Value)

	// Set object info multiple times (should always be 1)
	SetObjectInfo("user", "pg-1", "user1", "default", "", "testuser")
	SetObjectInfo("user", "pg-1", "user1", "default", "", "testuser")
	SetObjectInfo("user", "pg-1", "user1", "default", "", "testuser")

	metric3 := &dto.Metric{}
	err3 := ObjectNames.WithLabelValues("user", "pg-1", "user1", "default", "", "testuser").Write(metric3)
	require.NoError(t, err3)
	assert.Equal(t, float64(1), *metric3.Gauge.Value)
}

func TestConcurrentUpdates(t *testing.T) {
	// Test concurrent metric updates
	PostgresqlCount.Set(0)
	ObjectCountPerPostgresqlID.Reset()
	ObjectNames.Reset()

	// Use channels to coordinate goroutines
	done := make(chan bool, 10)

	// Concurrent updates to PostgresqlCount
	for i := 0; i < 10; i++ {
		go func(val float64) {
			UpdatePostgresqlCount(val)
			done <- true
		}(float64(i))
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent updates to ObjectCountPerPostgresqlID
	for i := 0; i < 10; i++ {
		go func(val float64) {
			UpdateObjectCount("user", "pg-1", val)
			done <- true
		}(float64(i))
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent updates to ObjectNames
	for i := 0; i < 10; i++ {
		go func(name string) {
			SetObjectInfo("user", "pg-1", name, "default", "", "testuser")
			done <- true
		}("user" + string(rune(i)))
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final state (last value should be set)
	metric := &dto.Metric{}
	err := PostgresqlCount.Write(metric)
	require.NoError(t, err)
	// The value should be one of the values we set (0-9)
	assert.GreaterOrEqual(t, *metric.Gauge.Value, float64(0))
	assert.LessOrEqual(t, *metric.Gauge.Value, float64(9))
}

// TestRecordRateLimitWait_TableDriven tests RecordRateLimitWait with various scenarios
func TestRecordRateLimitWait_TableDriven(t *testing.T) {
	// Reset metrics before test
	RateLimitWaitTotal.Reset()

	tests := []struct {
		name    string
		service string
		result  string
	}{
		{
			name:    "postgresql success",
			service: "postgresql",
			result:  "success",
		},
		{
			name:    "postgresql error",
			service: "postgresql",
			result:  "error",
		},
		{
			name:    "vault success",
			service: "vault",
			result:  "success",
		},
		{
			name:    "vault error",
			service: "vault",
			result:  "error",
		},
		{
			name:    "custom service",
			service: "custom",
			result:  "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record the metric
			RecordRateLimitWait(tt.service, tt.result)

			// Verify the metric was recorded
			metric := &dto.Metric{}
			err := RateLimitWaitTotal.WithLabelValues(tt.service, tt.result).Write(metric)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, *metric.Counter.Value, float64(1))
		})
	}
}

// TestRecordRateLimitAllow_TableDriven tests RecordRateLimitAllow with various scenarios
func TestRecordRateLimitAllow_TableDriven(t *testing.T) {
	// Reset metrics before test
	RateLimitAllowTotal.Reset()

	tests := []struct {
		name    string
		service string
		result  string
	}{
		{
			name:    "postgresql allowed",
			service: "postgresql",
			result:  "allowed",
		},
		{
			name:    "postgresql denied",
			service: "postgresql",
			result:  "denied",
		},
		{
			name:    "vault allowed",
			service: "vault",
			result:  "allowed",
		},
		{
			name:    "vault denied",
			service: "vault",
			result:  "denied",
		},
		{
			name:    "custom service allowed",
			service: "custom",
			result:  "allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record the metric
			RecordRateLimitAllow(tt.service, tt.result)

			// Verify the metric was recorded
			metric := &dto.Metric{}
			err := RateLimitAllowTotal.WithLabelValues(tt.service, tt.result).Write(metric)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, *metric.Counter.Value, float64(1))
		})
	}
}

// TestRecordOperationDuration_TableDriven tests RecordOperationDuration with various scenarios
func TestRecordOperationDuration_TableDriven(t *testing.T) {
	// Reset metrics before test
	OperationDuration.Reset()

	tests := []struct {
		name            string
		service         string
		operation       string
		result          string
		durationSeconds float64
	}{
		{
			name:            "postgresql create success",
			service:         "postgresql",
			operation:       "create_database",
			result:          "success",
			durationSeconds: 0.5,
		},
		{
			name:            "postgresql create error",
			service:         "postgresql",
			operation:       "create_database",
			result:          "error",
			durationSeconds: 1.0,
		},
		{
			name:            "vault read success",
			service:         "vault",
			operation:       "read_secret",
			result:          "success",
			durationSeconds: 0.1,
		},
		{
			name:            "vault write error",
			service:         "vault",
			operation:       "write_secret",
			result:          "error",
			durationSeconds: 2.0,
		},
		{
			name:            "very fast operation",
			service:         "postgresql",
			operation:       "ping",
			result:          "success",
			durationSeconds: 0.001,
		},
		{
			name:            "slow operation",
			service:         "postgresql",
			operation:       "backup",
			result:          "success",
			durationSeconds: 10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record the metric - this should not panic
			RecordOperationDuration(tt.service, tt.operation, tt.result, tt.durationSeconds)
			// Histogram metrics are recorded successfully if no panic occurs
		})
	}
}

// TestRecordOperation_TableDriven tests RecordOperation with various scenarios
func TestRecordOperation_TableDriven(t *testing.T) {
	// Reset metrics before test
	OperationTotal.Reset()

	tests := []struct {
		name      string
		service   string
		operation string
		result    string
	}{
		{
			name:      "postgresql create success",
			service:   "postgresql",
			operation: "create_database",
			result:    "success",
		},
		{
			name:      "postgresql create error",
			service:   "postgresql",
			operation: "create_database",
			result:    "error",
		},
		{
			name:      "postgresql update success",
			service:   "postgresql",
			operation: "update_user",
			result:    "success",
		},
		{
			name:      "vault read success",
			service:   "vault",
			operation: "read_secret",
			result:    "success",
		},
		{
			name:      "vault write error",
			service:   "vault",
			operation: "write_secret",
			result:    "error",
		},
		{
			name:      "custom operation",
			service:   "custom",
			operation: "custom_op",
			result:    "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record the metric
			RecordOperation(tt.service, tt.operation, tt.result)

			// Verify the metric was recorded
			metric := &dto.Metric{}
			err := OperationTotal.WithLabelValues(tt.service, tt.operation, tt.result).Write(metric)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, *metric.Counter.Value, float64(1))
		})
	}
}

// TestRecordRateLimitWait_MultipleRecords tests multiple recordings
func TestRecordRateLimitWait_MultipleRecords(t *testing.T) {
	RateLimitWaitTotal.Reset()

	// Record multiple times
	for i := 0; i < 5; i++ {
		RecordRateLimitWait("postgresql", "success")
	}

	metric := &dto.Metric{}
	err := RateLimitWaitTotal.WithLabelValues("postgresql", "success").Write(metric)
	require.NoError(t, err)
	assert.Equal(t, float64(5), *metric.Counter.Value)
}

// TestRecordRateLimitAllow_MultipleRecords tests multiple recordings
func TestRecordRateLimitAllow_MultipleRecords(t *testing.T) {
	RateLimitAllowTotal.Reset()

	// Record multiple times
	for i := 0; i < 3; i++ {
		RecordRateLimitAllow("vault", "allowed")
	}
	for i := 0; i < 2; i++ {
		RecordRateLimitAllow("vault", "denied")
	}

	metricAllowed := &dto.Metric{}
	err := RateLimitAllowTotal.WithLabelValues("vault", "allowed").Write(metricAllowed)
	require.NoError(t, err)
	assert.Equal(t, float64(3), *metricAllowed.Counter.Value)

	metricDenied := &dto.Metric{}
	err = RateLimitAllowTotal.WithLabelValues("vault", "denied").Write(metricDenied)
	require.NoError(t, err)
	assert.Equal(t, float64(2), *metricDenied.Counter.Value)
}

// TestRecordOperationDuration_MultipleRecords tests multiple duration recordings
func TestRecordOperationDuration_MultipleRecords(t *testing.T) {
	OperationDuration.Reset()

	// Record multiple durations - should not panic
	durations := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	for _, d := range durations {
		RecordOperationDuration("postgresql", "query", "success", d)
	}
	// If we get here without panic, the test passes
}

// TestRecordOperation_MultipleRecords tests multiple operation recordings
func TestRecordOperation_MultipleRecords(t *testing.T) {
	OperationTotal.Reset()

	// Record multiple operations
	for i := 0; i < 10; i++ {
		RecordOperation("postgresql", "create_user", "success")
	}

	metric := &dto.Metric{}
	err := OperationTotal.WithLabelValues("postgresql", "create_user", "success").Write(metric)
	require.NoError(t, err)
	assert.Equal(t, float64(10), *metric.Counter.Value)
}

// TestMetrics_ConcurrentRecording tests concurrent metric recording
func TestMetrics_ConcurrentRecording(t *testing.T) {
	RateLimitWaitTotal.Reset()
	RateLimitAllowTotal.Reset()
	OperationTotal.Reset()
	OperationDuration.Reset()

	done := make(chan bool, 40)

	// Concurrent rate limit wait recordings
	for i := 0; i < 10; i++ {
		go func() {
			RecordRateLimitWait("postgresql", "success")
			done <- true
		}()
	}

	// Concurrent rate limit allow recordings
	for i := 0; i < 10; i++ {
		go func() {
			RecordRateLimitAllow("vault", "allowed")
			done <- true
		}()
	}

	// Concurrent operation recordings
	for i := 0; i < 10; i++ {
		go func() {
			RecordOperation("postgresql", "create", "success")
			done <- true
		}()
	}

	// Concurrent duration recordings
	for i := 0; i < 10; i++ {
		go func(d float64) {
			RecordOperationDuration("vault", "read", "success", d)
			done <- true
		}(float64(i) * 0.1)
	}

	// Wait for all goroutines
	for i := 0; i < 40; i++ {
		<-done
	}

	// Verify counts
	metric1 := &dto.Metric{}
	err := RateLimitWaitTotal.WithLabelValues("postgresql", "success").Write(metric1)
	require.NoError(t, err)
	assert.Equal(t, float64(10), *metric1.Counter.Value)

	metric2 := &dto.Metric{}
	err = RateLimitAllowTotal.WithLabelValues("vault", "allowed").Write(metric2)
	require.NoError(t, err)
	assert.Equal(t, float64(10), *metric2.Counter.Value)

	metric3 := &dto.Metric{}
	err = OperationTotal.WithLabelValues("postgresql", "create", "success").Write(metric3)
	require.NoError(t, err)
	assert.Equal(t, float64(10), *metric3.Counter.Value)

	// Duration recordings are verified by not panicking during concurrent access
}
