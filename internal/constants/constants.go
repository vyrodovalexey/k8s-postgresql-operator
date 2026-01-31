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

// Package constants provides shared constant values used throughout the operator
package constants

import "time"

const (
	// DefaultRequeueDelay is the default delay before requeuing a reconciliation
	DefaultRequeueDelay = 30 * time.Second

	// DefaultConnectionTimeout is the default timeout for database connections
	DefaultConnectionTimeout = 10 * time.Second

	// DefaultPostgresPort is the default PostgreSQL port
	DefaultPostgresPort int32 = 5432

	// DefaultPasswordLength is the default length for generated passwords
	DefaultPasswordLength = 32

	// DefaultMetricsPort is the default port for the metrics server
	DefaultMetricsPort = 8080

	// DefaultWebhookPort is the default port for the webhook server
	DefaultWebhookPort = 8443

	// DefaultSSLMode is the default SSL mode for PostgreSQL connections
	DefaultSSLMode = "require"

	// DefaultMetricsCollectionInterval is the default interval for metrics collection
	DefaultMetricsCollectionInterval = 30 * time.Second

	// DefaultDatabaseName is the default PostgreSQL database name
	DefaultDatabaseName = "postgres"

	// DefaultSchemaName is the default PostgreSQL schema name
	DefaultSchemaName = "public"
)

// Condition types for CRD status
const (
	// ConditionTypeReady indicates the resource is ready
	ConditionTypeReady = "Ready"
)

// Condition reasons for CRD status
const (
	// ConditionReasonPostgresqlNotFound indicates the PostgreSQL instance was not found
	ConditionReasonPostgresqlNotFound = "PostgresqlNotFound"

	// ConditionReasonPostgresqlNotConnected indicates the PostgreSQL instance is not connected
	ConditionReasonPostgresqlNotConnected = "PostgresqlNotConnected"

	// ConditionReasonInvalidConfiguration indicates invalid configuration
	ConditionReasonInvalidConfiguration = "InvalidConfiguration"

	// ConditionReasonVaultNotAvailable indicates Vault is not available
	ConditionReasonVaultNotAvailable = "VaultNotAvailable"

	// ConditionReasonVaultError indicates a Vault error occurred
	ConditionReasonVaultError = "VaultError"

	// ConditionReasonCreated indicates the resource was created successfully
	ConditionReasonCreated = "Created"

	// ConditionReasonCreateFailed indicates the resource creation failed
	ConditionReasonCreateFailed = "CreateFailed"

	// ConditionReasonApplied indicates the resource was applied successfully
	ConditionReasonApplied = "Applied"

	// ConditionReasonApplyFailed indicates the resource application failed
	ConditionReasonApplyFailed = "ApplyFailed"

	// ConditionReasonDeleted indicates the resource was deleted successfully
	ConditionReasonDeleted = "Deleted"

	// ConditionReasonDeleteFailed indicates the resource deletion failed
	ConditionReasonDeleteFailed = "DeleteFailed"

	// ConditionReasonConnected indicates successful connection
	ConditionReasonConnected = "Connected"

	// ConditionReasonConnectionFailed indicates connection failure
	ConditionReasonConnectionFailed = "ConnectionFailed"
)

// Valid SSL modes for PostgreSQL connections
var ValidSSLModes = []string{"disable", "require", "verify-ca", "verify-full"}
