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
		[]string{"kind", "postgresql_id", "name", "namespace"},
	)
)

func init() {
	// Register custom metrics with controller-runtime's metrics registry
	metrics.Registry.MustRegister(PostgresqlCount)
	metrics.Registry.MustRegister(ObjectCountPerPostgresqlID)
	metrics.Registry.MustRegister(ObjectNames)
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
func SetObjectInfo(kind, postgresqlID, name, namespace string) {
	ObjectNames.WithLabelValues(kind, postgresqlID, name, namespace).Set(1)
}

// RemoveObjectInfo removes the info metric for an object
func RemoveObjectInfo(kind, postgresqlID, name, namespace string) {
	ObjectNames.DeleteLabelValues(kind, postgresqlID, name, namespace)
}

// RemoveObjectCount removes the count metric for a specific kind and postgresqlID
func RemoveObjectCount(kind, postgresqlID string) {
	ObjectCountPerPostgresqlID.DeleteLabelValues(kind, postgresqlID)
}
