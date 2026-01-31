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

// Package events provides standard event reasons and messages for Kubernetes event recording
package events

import (
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Event types
const (
	// EventTypeNormal represents a normal event
	EventTypeNormal = "Normal"
	// EventTypeWarning represents a warning event
	EventTypeWarning = "Warning"
)

// Event reasons for successful operations
const (
	// ReasonCreated indicates a resource was successfully created
	ReasonCreated = "Created"
	// ReasonUpdated indicates a resource was successfully updated
	ReasonUpdated = "Updated"
	// ReasonDeleted indicates a resource was successfully deleted
	ReasonDeleted = "Deleted"
	// ReasonConnected indicates a successful connection
	ReasonConnected = "Connected"
	// ReasonSynced indicates a resource was successfully synced
	ReasonSynced = "Synced"
	// ReasonApplied indicates grants were successfully applied
	ReasonApplied = "Applied"
)

// Event reasons for failures
const (
	// ReasonCreateFailed indicates a resource creation failed
	ReasonCreateFailed = "CreateFailed"
	// ReasonUpdateFailed indicates a resource update failed
	ReasonUpdateFailed = "UpdateFailed"
	// ReasonDeleteFailed indicates a resource deletion failed
	ReasonDeleteFailed = "DeleteFailed"
	// ReasonConnectionFailed indicates a connection failure
	ReasonConnectionFailed = "ConnectionFailed"
	// ReasonSyncFailed indicates a sync failure
	ReasonSyncFailed = "SyncFailed"
	// ReasonApplyFailed indicates grants application failed
	ReasonApplyFailed = "ApplyFailed"
	// ReasonVaultError indicates a Vault-related error
	ReasonVaultError = "VaultError"
	// ReasonInvalidConfiguration indicates an invalid configuration
	ReasonInvalidConfiguration = "InvalidConfiguration"
	// ReasonPostgresqlNotFound indicates PostgreSQL instance not found
	ReasonPostgresqlNotFound = "PostgresqlNotFound"
	// ReasonPostgresqlNotConnected indicates PostgreSQL instance not connected
	ReasonPostgresqlNotConnected = "PostgresqlNotConnected"
)

// EventRecorder wraps the Kubernetes event recorder with helper methods
type EventRecorder struct {
	recorder record.EventRecorder
}

// NewEventRecorder creates a new EventRecorder
func NewEventRecorder(recorder record.EventRecorder) *EventRecorder {
	if recorder == nil {
		return nil
	}
	return &EventRecorder{recorder: recorder}
}

// RecordNormal records a normal event
func (e *EventRecorder) RecordNormal(obj client.Object, reason, message string) {
	if e == nil || e.recorder == nil {
		return
	}
	e.recorder.Event(obj, EventTypeNormal, reason, message)
}

// RecordWarning records a warning event
func (e *EventRecorder) RecordWarning(obj client.Object, reason, message string) {
	if e == nil || e.recorder == nil {
		return
	}
	e.recorder.Event(obj, EventTypeWarning, reason, message)
}

// RecordCreated records a successful creation event
func (e *EventRecorder) RecordCreated(obj client.Object, resourceType, resourceName string) {
	e.RecordNormal(obj, ReasonCreated, resourceType+" "+resourceName+" successfully created")
}

// RecordUpdated records a successful update event
func (e *EventRecorder) RecordUpdated(obj client.Object, resourceType, resourceName string) {
	e.RecordNormal(obj, ReasonUpdated, resourceType+" "+resourceName+" successfully updated")
}

// RecordDeleted records a successful deletion event
func (e *EventRecorder) RecordDeleted(obj client.Object, resourceType, resourceName string) {
	e.RecordNormal(obj, ReasonDeleted, resourceType+" "+resourceName+" successfully deleted")
}

// RecordConnected records a successful connection event
func (e *EventRecorder) RecordConnected(obj client.Object, target string) {
	e.RecordNormal(obj, ReasonConnected, "Successfully connected to "+target)
}

// RecordSynced records a successful sync event
func (e *EventRecorder) RecordSynced(obj client.Object, resourceType, resourceName string) {
	e.RecordNormal(obj, ReasonSynced, resourceType+" "+resourceName+" successfully synced")
}

// RecordApplied records a successful grants application event
func (e *EventRecorder) RecordApplied(obj client.Object, message string) {
	e.RecordNormal(obj, ReasonApplied, message)
}

// RecordCreateFailed records a creation failure event
func (e *EventRecorder) RecordCreateFailed(obj client.Object, resourceType, resourceName, errMsg string) {
	e.RecordWarning(obj, ReasonCreateFailed, "Failed to create "+resourceType+" "+resourceName+": "+errMsg)
}

// RecordUpdateFailed records an update failure event
func (e *EventRecorder) RecordUpdateFailed(obj client.Object, resourceType, resourceName, errMsg string) {
	e.RecordWarning(obj, ReasonUpdateFailed, "Failed to update "+resourceType+" "+resourceName+": "+errMsg)
}

// RecordDeleteFailed records a deletion failure event
func (e *EventRecorder) RecordDeleteFailed(obj client.Object, resourceType, resourceName, errMsg string) {
	e.RecordWarning(obj, ReasonDeleteFailed, "Failed to delete "+resourceType+" "+resourceName+": "+errMsg)
}

// RecordConnectionFailed records a connection failure event
func (e *EventRecorder) RecordConnectionFailed(obj client.Object, target, errMsg string) {
	e.RecordWarning(obj, ReasonConnectionFailed, "Failed to connect to "+target+": "+errMsg)
}

// RecordSyncFailed records a sync failure event
func (e *EventRecorder) RecordSyncFailed(obj client.Object, resourceType, resourceName, errMsg string) {
	e.RecordWarning(obj, ReasonSyncFailed, "Failed to sync "+resourceType+" "+resourceName+": "+errMsg)
}

// RecordApplyFailed records a grants application failure event
func (e *EventRecorder) RecordApplyFailed(obj client.Object, errMsg string) {
	e.RecordWarning(obj, ReasonApplyFailed, "Failed to apply grants: "+errMsg)
}

// RecordVaultError records a Vault-related error event
func (e *EventRecorder) RecordVaultError(obj client.Object, errMsg string) {
	e.RecordWarning(obj, ReasonVaultError, "Vault error: "+errMsg)
}

// RecordInvalidConfiguration records an invalid configuration event
func (e *EventRecorder) RecordInvalidConfiguration(obj client.Object, errMsg string) {
	e.RecordWarning(obj, ReasonInvalidConfiguration, "Invalid configuration: "+errMsg)
}

// RecordPostgresqlNotFound records a PostgreSQL not found event
func (e *EventRecorder) RecordPostgresqlNotFound(obj client.Object, postgresqlID string) {
	e.RecordWarning(obj, ReasonPostgresqlNotFound, "PostgreSQL instance with ID "+postgresqlID+" not found")
}

// RecordPostgresqlNotConnected records a PostgreSQL not connected event
func (e *EventRecorder) RecordPostgresqlNotConnected(obj client.Object, postgresqlID string) {
	e.RecordWarning(obj, ReasonPostgresqlNotConnected, "PostgreSQL instance with ID "+postgresqlID+" is not connected")
}
