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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// PostgresqlCount is the total number of registered PostgreSQL instances
	PostgresqlCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "postgresql_operator_postgresql_count",
			Help: "Total number of registered PostgreSQL instances",
		},
	)

	// ObjectCountPerPostgresqlID is the number of objects of each kind per postgresqlID
	ObjectCountPerPostgresqlID = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "postgresql_operator_object_count",
			Help: "Number of objects per kind and postgresqlID",
		},
		[]string{"kind", "postgresql_id"},
	)

	// ObjectNames is the names of each kind object per postgresqlID
	ObjectNames = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "postgresql_operator_object_info",
			Help: "Information about objects (name, namespace) per kind and postgresqlID. Value is always 1.",
		},
		[]string{"kind", "postgresql_id", "name", "namespace", "databasename", "username"},
	)

	// RateLimitWaitTotal is the total number of rate limit wait operations
	RateLimitWaitTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgresql_operator_rate_limit_wait_total",
			Help: "Total number of rate limit wait operations",
		},
		[]string{"service", "result"},
	)

	// RateLimitAllowTotal is the total number of rate limit allow checks
	RateLimitAllowTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgresql_operator_rate_limit_allow_total",
			Help: "Total number of rate limit allow checks",
		},
		[]string{"service", "result"},
	)

	// OperationDuration is a histogram for operation latencies
	OperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "postgresql_operator_operation_duration_seconds",
			Help:    "Duration of operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~16s
		},
		[]string{"service", "operation", "result"},
	)

	// OperationTotal is the total number of operations
	OperationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "postgresql_operator_operation_total",
			Help: "Total number of operations",
		},
		[]string{"service", "operation", "result"},
	)
)

func init() {
	// Register custom metrics with controller-runtime's metrics registry
	metrics.Registry.MustRegister(PostgresqlCount)
	metrics.Registry.MustRegister(ObjectCountPerPostgresqlID)
	metrics.Registry.MustRegister(ObjectNames)
	metrics.Registry.MustRegister(RateLimitWaitTotal)
	metrics.Registry.MustRegister(RateLimitAllowTotal)
	metrics.Registry.MustRegister(OperationDuration)
	metrics.Registry.MustRegister(OperationTotal)
}

// UpdatePostgresqlCount updates the total number of PostgreSQL instances
func UpdatePostgresqlCount(count float64) {
	PostgresqlCount.Set(count)
}

// UpdateObjectCount updates the count of objects for a specific kind and postgresqlID
func UpdateObjectCount(kind, postgresqlID string, count float64) {
	ObjectCountPerPostgresqlID.WithLabelValues(kind, postgresqlID).Set(count)
}

// SetObjectInfo sets the info metric for an object (value is always 1)
// databasename and username are optional labels (empty string for non-applicable kinds)
func SetObjectInfo(kind, postgresqlID, name, namespace, databasename, username string) {
	ObjectNames.WithLabelValues(kind, postgresqlID, name, namespace, databasename, username).Set(1)
}

// RemoveObjectInfo removes the info metric for an object
// databasename and username are optional labels (empty string for non-applicable kinds)
func RemoveObjectInfo(kind, postgresqlID, name, namespace, databasename, username string) {
	ObjectNames.DeleteLabelValues(kind, postgresqlID, name, namespace, databasename, username)
}

// RemoveObjectCount removes the count metric for a specific kind and postgresqlID
func RemoveObjectCount(kind, postgresqlID string) {
	ObjectCountPerPostgresqlID.DeleteLabelValues(kind, postgresqlID)
}

// RecordRateLimitWait records a rate limit wait operation
func RecordRateLimitWait(service, result string) {
	RateLimitWaitTotal.WithLabelValues(service, result).Inc()
}

// RecordRateLimitAllow records a rate limit allow check
func RecordRateLimitAllow(service, result string) {
	RateLimitAllowTotal.WithLabelValues(service, result).Inc()
}

// RecordOperationDuration records the duration of an operation
func RecordOperationDuration(service, operation, result string, durationSeconds float64) {
	OperationDuration.WithLabelValues(service, operation, result).Observe(durationSeconds)
}

// RecordOperation records an operation with its result
func RecordOperation(service, operation, result string) {
	OperationTotal.WithLabelValues(service, operation, result).Inc()
}
