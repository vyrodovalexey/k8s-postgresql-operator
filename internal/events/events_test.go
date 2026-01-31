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

package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

// newTestPod creates a Pod for testing
func newTestPod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// TestEventConstants tests that event constants are defined correctly
func TestEventConstants(t *testing.T) {
	// Event types
	assert.Equal(t, "Normal", EventTypeNormal)
	assert.Equal(t, "Warning", EventTypeWarning)

	// Success reasons
	assert.Equal(t, "Created", ReasonCreated)
	assert.Equal(t, "Updated", ReasonUpdated)
	assert.Equal(t, "Deleted", ReasonDeleted)
	assert.Equal(t, "Connected", ReasonConnected)
	assert.Equal(t, "Synced", ReasonSynced)
	assert.Equal(t, "Applied", ReasonApplied)

	// Failure reasons
	assert.Equal(t, "CreateFailed", ReasonCreateFailed)
	assert.Equal(t, "UpdateFailed", ReasonUpdateFailed)
	assert.Equal(t, "DeleteFailed", ReasonDeleteFailed)
	assert.Equal(t, "ConnectionFailed", ReasonConnectionFailed)
	assert.Equal(t, "SyncFailed", ReasonSyncFailed)
	assert.Equal(t, "ApplyFailed", ReasonApplyFailed)
	assert.Equal(t, "VaultError", ReasonVaultError)
	assert.Equal(t, "InvalidConfiguration", ReasonInvalidConfiguration)
	assert.Equal(t, "PostgresqlNotFound", ReasonPostgresqlNotFound)
	assert.Equal(t, "PostgresqlNotConnected", ReasonPostgresqlNotConnected)
}

// TestNewEventRecorder tests NewEventRecorder function
func TestNewEventRecorder(t *testing.T) {
	tests := []struct {
		name      string
		recorder  record.EventRecorder
		expectNil bool
	}{
		{
			name:      "nil recorder returns nil",
			recorder:  nil,
			expectNil: true,
		},
		{
			name:      "valid recorder returns EventRecorder",
			recorder:  record.NewFakeRecorder(10),
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewEventRecorder(tt.recorder)
			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
			}
		})
	}
}

// TestEventRecorder_RecordNormal tests RecordNormal method
func TestEventRecorder_RecordNormal(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordNormal(obj, ReasonCreated, "Test message")

	// Check that event was recorded
	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeNormal)
		assert.Contains(t, event, ReasonCreated)
		assert.Contains(t, event, "Test message")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordWarning tests RecordWarning method
func TestEventRecorder_RecordWarning(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordWarning(obj, ReasonCreateFailed, "Test error message")

	// Check that event was recorded
	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeWarning)
		assert.Contains(t, event, ReasonCreateFailed)
		assert.Contains(t, event, "Test error message")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordCreated tests RecordCreated method
func TestEventRecorder_RecordCreated(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordCreated(obj, "Database", "testdb")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeNormal)
		assert.Contains(t, event, ReasonCreated)
		assert.Contains(t, event, "Database")
		assert.Contains(t, event, "testdb")
		assert.Contains(t, event, "successfully created")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordUpdated tests RecordUpdated method
func TestEventRecorder_RecordUpdated(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordUpdated(obj, "User", "testuser")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeNormal)
		assert.Contains(t, event, ReasonUpdated)
		assert.Contains(t, event, "User")
		assert.Contains(t, event, "testuser")
		assert.Contains(t, event, "successfully updated")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordDeleted tests RecordDeleted method
func TestEventRecorder_RecordDeleted(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordDeleted(obj, "Schema", "testschema")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeNormal)
		assert.Contains(t, event, ReasonDeleted)
		assert.Contains(t, event, "Schema")
		assert.Contains(t, event, "testschema")
		assert.Contains(t, event, "successfully deleted")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordConnected tests RecordConnected method
func TestEventRecorder_RecordConnected(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordConnected(obj, "PostgreSQL instance pg-1")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeNormal)
		assert.Contains(t, event, ReasonConnected)
		assert.Contains(t, event, "Successfully connected to")
		assert.Contains(t, event, "PostgreSQL instance pg-1")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordSynced tests RecordSynced method
func TestEventRecorder_RecordSynced(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordSynced(obj, "Grant", "testgrant")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeNormal)
		assert.Contains(t, event, ReasonSynced)
		assert.Contains(t, event, "Grant")
		assert.Contains(t, event, "testgrant")
		assert.Contains(t, event, "successfully synced")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordApplied tests RecordApplied method
func TestEventRecorder_RecordApplied(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordApplied(obj, "Grants applied successfully")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeNormal)
		assert.Contains(t, event, ReasonApplied)
		assert.Contains(t, event, "Grants applied successfully")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordCreateFailed tests RecordCreateFailed method
func TestEventRecorder_RecordCreateFailed(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordCreateFailed(obj, "Database", "testdb", "connection refused")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeWarning)
		assert.Contains(t, event, ReasonCreateFailed)
		assert.Contains(t, event, "Failed to create")
		assert.Contains(t, event, "Database")
		assert.Contains(t, event, "testdb")
		assert.Contains(t, event, "connection refused")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordUpdateFailed tests RecordUpdateFailed method
func TestEventRecorder_RecordUpdateFailed(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordUpdateFailed(obj, "User", "testuser", "permission denied")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeWarning)
		assert.Contains(t, event, ReasonUpdateFailed)
		assert.Contains(t, event, "Failed to update")
		assert.Contains(t, event, "User")
		assert.Contains(t, event, "testuser")
		assert.Contains(t, event, "permission denied")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordDeleteFailed tests RecordDeleteFailed method
func TestEventRecorder_RecordDeleteFailed(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordDeleteFailed(obj, "Schema", "testschema", "schema not empty")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeWarning)
		assert.Contains(t, event, ReasonDeleteFailed)
		assert.Contains(t, event, "Failed to delete")
		assert.Contains(t, event, "Schema")
		assert.Contains(t, event, "testschema")
		assert.Contains(t, event, "schema not empty")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordConnectionFailed tests RecordConnectionFailed method
func TestEventRecorder_RecordConnectionFailed(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordConnectionFailed(obj, "PostgreSQL pg-1", "timeout")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeWarning)
		assert.Contains(t, event, ReasonConnectionFailed)
		assert.Contains(t, event, "Failed to connect to")
		assert.Contains(t, event, "PostgreSQL pg-1")
		assert.Contains(t, event, "timeout")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordSyncFailed tests RecordSyncFailed method
func TestEventRecorder_RecordSyncFailed(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordSyncFailed(obj, "Grant", "testgrant", "invalid privilege")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeWarning)
		assert.Contains(t, event, ReasonSyncFailed)
		assert.Contains(t, event, "Failed to sync")
		assert.Contains(t, event, "Grant")
		assert.Contains(t, event, "testgrant")
		assert.Contains(t, event, "invalid privilege")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordApplyFailed tests RecordApplyFailed method
func TestEventRecorder_RecordApplyFailed(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordApplyFailed(obj, "role does not exist")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeWarning)
		assert.Contains(t, event, ReasonApplyFailed)
		assert.Contains(t, event, "Failed to apply grants")
		assert.Contains(t, event, "role does not exist")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordVaultError tests RecordVaultError method
func TestEventRecorder_RecordVaultError(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordVaultError(obj, "secret not found")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeWarning)
		assert.Contains(t, event, ReasonVaultError)
		assert.Contains(t, event, "Vault error")
		assert.Contains(t, event, "secret not found")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordInvalidConfiguration tests RecordInvalidConfiguration method
func TestEventRecorder_RecordInvalidConfiguration(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordInvalidConfiguration(obj, "missing required field")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeWarning)
		assert.Contains(t, event, ReasonInvalidConfiguration)
		assert.Contains(t, event, "Invalid configuration")
		assert.Contains(t, event, "missing required field")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordPostgresqlNotFound tests RecordPostgresqlNotFound method
func TestEventRecorder_RecordPostgresqlNotFound(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordPostgresqlNotFound(obj, "pg-1")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeWarning)
		assert.Contains(t, event, ReasonPostgresqlNotFound)
		assert.Contains(t, event, "PostgreSQL instance with ID")
		assert.Contains(t, event, "pg-1")
		assert.Contains(t, event, "not found")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_RecordPostgresqlNotConnected tests RecordPostgresqlNotConnected method
func TestEventRecorder_RecordPostgresqlNotConnected(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	eventRecorder.RecordPostgresqlNotConnected(obj, "pg-2")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeWarning)
		assert.Contains(t, event, ReasonPostgresqlNotConnected)
		assert.Contains(t, event, "PostgreSQL instance with ID")
		assert.Contains(t, event, "pg-2")
		assert.Contains(t, event, "is not connected")
	default:
		t.Error("Expected event to be recorded")
	}
}

// TestEventRecorder_NilRecorder tests that nil recorder doesn't panic
func TestEventRecorder_NilRecorder(t *testing.T) {
	var eventRecorder *EventRecorder = nil
	obj := newTestPod("test-obj", "default")

	// These should not panic
	eventRecorder.RecordNormal(obj, ReasonCreated, "test")
	eventRecorder.RecordWarning(obj, ReasonCreateFailed, "test")
	eventRecorder.RecordCreated(obj, "Database", "testdb")
	eventRecorder.RecordUpdated(obj, "User", "testuser")
	eventRecorder.RecordDeleted(obj, "Schema", "testschema")
	eventRecorder.RecordConnected(obj, "target")
	eventRecorder.RecordSynced(obj, "Grant", "testgrant")
	eventRecorder.RecordApplied(obj, "message")
	eventRecorder.RecordCreateFailed(obj, "Database", "testdb", "error")
	eventRecorder.RecordUpdateFailed(obj, "User", "testuser", "error")
	eventRecorder.RecordDeleteFailed(obj, "Schema", "testschema", "error")
	eventRecorder.RecordConnectionFailed(obj, "target", "error")
	eventRecorder.RecordSyncFailed(obj, "Grant", "testgrant", "error")
	eventRecorder.RecordApplyFailed(obj, "error")
	eventRecorder.RecordVaultError(obj, "error")
	eventRecorder.RecordInvalidConfiguration(obj, "error")
	eventRecorder.RecordPostgresqlNotFound(obj, "pg-1")
	eventRecorder.RecordPostgresqlNotConnected(obj, "pg-1")
}

// TestEventRecorder_NilInternalRecorder tests that nil internal recorder doesn't panic
func TestEventRecorder_NilInternalRecorder(t *testing.T) {
	eventRecorder := &EventRecorder{recorder: nil}
	obj := newTestPod("test-obj", "default")

	// These should not panic
	eventRecorder.RecordNormal(obj, ReasonCreated, "test")
	eventRecorder.RecordWarning(obj, ReasonCreateFailed, "test")
}

// TestEventRecorder_MultipleEvents tests recording multiple events
func TestEventRecorder_MultipleEvents(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)
	obj := newTestPod("test-obj", "default")

	// Record multiple events
	eventRecorder.RecordCreated(obj, "Database", "db1")
	eventRecorder.RecordCreated(obj, "User", "user1")
	eventRecorder.RecordUpdated(obj, "Schema", "schema1")

	// Verify all events were recorded
	eventCount := 0
	for {
		select {
		case <-fakeRecorder.Events:
			eventCount++
		default:
			assert.Equal(t, 3, eventCount)
			return
		}
	}
}

// TestEventRecorder_WithPod tests recording events on a real Kubernetes object
func TestEventRecorder_WithPod(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	eventRecorder := NewEventRecorder(fakeRecorder)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	eventRecorder.RecordNormal(pod, ReasonCreated, "Pod created")

	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, EventTypeNormal)
		assert.Contains(t, event, ReasonCreated)
		assert.Contains(t, event, "Pod created")
	default:
		t.Error("Expected event to be recorded")
	}
}
