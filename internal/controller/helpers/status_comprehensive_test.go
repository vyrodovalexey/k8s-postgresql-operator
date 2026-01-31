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

package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// MockStatusClient is a mock implementation of client.StatusClient
type MockStatusClient struct {
	mock.Mock
}

func (m *MockStatusClient) Status() client.StatusWriter {
	args := m.Called()
	return args.Get(0).(client.StatusWriter)
}

// MockStatusWriter is a mock implementation of client.StatusWriter
type MockStatusWriter struct {
	mock.Mock
}

func (m *MockStatusWriter) Create(
	ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	args := m.Called(ctx, obj, subResource, opts)
	return args.Error(0)
}

func (m *MockStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockStatusWriter) Patch(
	ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	args := m.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

func TestUpdateStatusWithRetry_Success(t *testing.T) {
	// Arrange
	mockStatusClient := new(MockStatusClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	obj := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "default",
		},
	}

	mockStatusClient.On("Status").Return(mockStatusWriter)
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Act
	err := UpdateStatusWithRetry(context.Background(), mockStatusClient, obj, logger)

	// Assert
	assert.NoError(t, err)
	mockStatusClient.AssertExpectations(t)
	mockStatusWriter.AssertExpectations(t)
}

func TestUpdateStatusWithRetry_ConflictThenSuccess(t *testing.T) {
	// Arrange
	mockStatusClient := new(MockStatusClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	obj := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "default",
		},
	}

	conflictErr := errors.NewConflict(schema.GroupResource{Group: "test", Resource: "test"}, "test", nil)

	mockStatusClient.On("Status").Return(mockStatusWriter)
	// First call returns conflict, second call succeeds
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(conflictErr).Once()
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()

	// Act
	opts := StatusUpdateOptions{
		Retries:    3,
		RetryDelay: 1 * time.Millisecond, // Short delay for testing
	}
	err := UpdateStatusWithRetryAndOptions(context.Background(), mockStatusClient, obj, logger, opts)

	// Assert
	assert.NoError(t, err)
	mockStatusClient.AssertExpectations(t)
	mockStatusWriter.AssertExpectations(t)
}

func TestUpdateStatusWithRetry_AllRetriesFail(t *testing.T) {
	// Arrange
	mockStatusClient := new(MockStatusClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	obj := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "default",
		},
	}

	conflictErr := errors.NewConflict(schema.GroupResource{Group: "test", Resource: "test"}, "test", nil)

	mockStatusClient.On("Status").Return(mockStatusWriter)
	// All calls return conflict
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(conflictErr).Times(3)

	// Act
	opts := StatusUpdateOptions{
		Retries:    3,
		RetryDelay: 1 * time.Millisecond,
	}
	err := UpdateStatusWithRetryAndOptions(context.Background(), mockStatusClient, obj, logger, opts)

	// Assert
	assert.Error(t, err)
	assert.True(t, errors.IsConflict(err))
	mockStatusClient.AssertExpectations(t)
	mockStatusWriter.AssertExpectations(t)
}

func TestUpdateStatusWithRetry_NonRetryableError(t *testing.T) {
	// Arrange
	mockStatusClient := new(MockStatusClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	obj := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "default",
		},
	}

	notFoundErr := errors.NewNotFound(schema.GroupResource{Group: "test", Resource: "test"}, "test")

	mockStatusClient.On("Status").Return(mockStatusWriter)
	// First call returns non-retryable error
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(notFoundErr).Once()

	// Act
	opts := StatusUpdateOptions{
		Retries:    3,
		RetryDelay: 1 * time.Millisecond,
	}
	err := UpdateStatusWithRetryAndOptions(context.Background(), mockStatusClient, obj, logger, opts)

	// Assert
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
	// Should only be called once since error is not retryable
	mockStatusWriter.AssertNumberOfCalls(t, "Update", 1)
}

func TestUpdateStatusWithRetry_ContextCancelled(t *testing.T) {
	// Arrange
	mockStatusClient := new(MockStatusClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	obj := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "default",
		},
	}

	conflictErr := errors.NewConflict(schema.GroupResource{Group: "test", Resource: "test"}, "test", nil)

	mockStatusClient.On("Status").Return(mockStatusWriter)
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(conflictErr)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Act
	opts := StatusUpdateOptions{
		Retries:    3,
		RetryDelay: 100 * time.Millisecond, // Longer delay to ensure context cancellation is detected
	}
	err := UpdateStatusWithRetryAndOptions(ctx, mockStatusClient, obj, logger, opts)

	// Assert
	assert.Error(t, err)
	// The error should be context.Canceled
	assert.Equal(t, context.Canceled, err)
}

func TestUpdateStatusWithRetry_ServerTimeout(t *testing.T) {
	// Arrange
	mockStatusClient := new(MockStatusClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	obj := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "default",
		},
	}

	timeoutErr := errors.NewServerTimeout(schema.GroupResource{Group: "test", Resource: "test"}, "test", 1)

	mockStatusClient.On("Status").Return(mockStatusWriter)
	// First call returns timeout, second call succeeds
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(timeoutErr).Once()
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()

	// Act
	opts := StatusUpdateOptions{
		Retries:    3,
		RetryDelay: 1 * time.Millisecond,
	}
	err := UpdateStatusWithRetryAndOptions(context.Background(), mockStatusClient, obj, logger, opts)

	// Assert
	assert.NoError(t, err)
	mockStatusWriter.AssertNumberOfCalls(t, "Update", 2)
}

func TestUpdateStatusWithRetry_ServiceUnavailable(t *testing.T) {
	// Arrange
	mockStatusClient := new(MockStatusClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	obj := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "default",
		},
	}

	unavailableErr := errors.NewServiceUnavailable("service unavailable")

	mockStatusClient.On("Status").Return(mockStatusWriter)
	// First call returns unavailable, second call succeeds
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(unavailableErr).Once()
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()

	// Act
	opts := StatusUpdateOptions{
		Retries:    3,
		RetryDelay: 1 * time.Millisecond,
	}
	err := UpdateStatusWithRetryAndOptions(context.Background(), mockStatusClient, obj, logger, opts)

	// Assert
	assert.NoError(t, err)
	mockStatusWriter.AssertNumberOfCalls(t, "Update", 2)
}

func TestUpdateStatusWithRetry_TooManyRequests(t *testing.T) {
	// Arrange
	mockStatusClient := new(MockStatusClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	obj := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "default",
		},
	}

	tooManyErr := errors.NewTooManyRequests("too many requests", 1)

	mockStatusClient.On("Status").Return(mockStatusWriter)
	// First call returns too many requests, second call succeeds
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(tooManyErr).Once()
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()

	// Act
	opts := StatusUpdateOptions{
		Retries:    3,
		RetryDelay: 1 * time.Millisecond,
	}
	err := UpdateStatusWithRetryAndOptions(context.Background(), mockStatusClient, obj, logger, opts)

	// Assert
	assert.NoError(t, err)
	mockStatusWriter.AssertNumberOfCalls(t, "Update", 2)
}

func TestUpdateStatusWithRetry_InternalError(t *testing.T) {
	// Arrange
	mockStatusClient := new(MockStatusClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	obj := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "default",
		},
	}

	internalErr := errors.NewInternalError(assert.AnError)

	mockStatusClient.On("Status").Return(mockStatusWriter)
	// First call returns internal error, second call succeeds
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(internalErr).Once()
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()

	// Act
	opts := StatusUpdateOptions{
		Retries:    3,
		RetryDelay: 1 * time.Millisecond,
	}
	err := UpdateStatusWithRetryAndOptions(context.Background(), mockStatusClient, obj, logger, opts)

	// Assert
	assert.NoError(t, err)
	mockStatusWriter.AssertNumberOfCalls(t, "Update", 2)
}

func TestUpdateStatusWithRetryAndOptions_CustomOptions(t *testing.T) {
	tests := []struct {
		name        string
		opts        StatusUpdateOptions
		numFailures int
		expectError bool
	}{
		{
			name: "Single retry succeeds on second attempt",
			opts: StatusUpdateOptions{
				Retries:    2,
				RetryDelay: 1 * time.Millisecond,
			},
			numFailures: 1,
			expectError: false,
		},
		{
			name: "All retries exhausted",
			opts: StatusUpdateOptions{
				Retries:    2,
				RetryDelay: 1 * time.Millisecond,
			},
			numFailures: 3,
			expectError: true,
		},
		{
			name: "Success on first attempt",
			opts: StatusUpdateOptions{
				Retries:    5,
				RetryDelay: 1 * time.Millisecond,
			},
			numFailures: 0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockStatusClient := new(MockStatusClient)
			mockStatusWriter := new(MockStatusWriter)
			logger := zap.NewNop().Sugar()

			obj := &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-db",
					Namespace: "default",
				},
			}

			conflictErr := errors.NewConflict(schema.GroupResource{Group: "test", Resource: "test"}, "test", nil)

			mockStatusClient.On("Status").Return(mockStatusWriter)

			// Set up mock calls based on number of failures
			for i := 0; i < tt.numFailures && i < tt.opts.Retries; i++ {
				mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(conflictErr).Once()
			}
			if tt.numFailures < tt.opts.Retries {
				mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()
			}

			// Act
			err := UpdateStatusWithRetryAndOptions(context.Background(), mockStatusClient, obj, logger, tt.opts)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateStatusWithRetry_BadRequestNotRetryable(t *testing.T) {
	// Arrange
	mockStatusClient := new(MockStatusClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	obj := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "default",
		},
	}

	badRequestErr := errors.NewBadRequest("bad request")

	mockStatusClient.On("Status").Return(mockStatusWriter)
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(badRequestErr).Once()

	// Act
	opts := StatusUpdateOptions{
		Retries:    3,
		RetryDelay: 1 * time.Millisecond,
	}
	err := UpdateStatusWithRetryAndOptions(context.Background(), mockStatusClient, obj, logger, opts)

	// Assert
	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
	// Should only be called once since error is not retryable
	mockStatusWriter.AssertNumberOfCalls(t, "Update", 1)
}

func TestUpdateStatusWithRetry_ForbiddenNotRetryable(t *testing.T) {
	// Arrange
	mockStatusClient := new(MockStatusClient)
	mockStatusWriter := new(MockStatusWriter)
	logger := zap.NewNop().Sugar()

	obj := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "default",
		},
	}

	forbiddenErr := errors.NewForbidden(schema.GroupResource{Group: "test", Resource: "test"}, "test", nil)

	mockStatusClient.On("Status").Return(mockStatusWriter)
	mockStatusWriter.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Return(forbiddenErr).Once()

	// Act
	opts := StatusUpdateOptions{
		Retries:    3,
		RetryDelay: 1 * time.Millisecond,
	}
	err := UpdateStatusWithRetryAndOptions(context.Background(), mockStatusClient, obj, logger, opts)

	// Assert
	assert.Error(t, err)
	assert.True(t, errors.IsForbidden(err))
	// Should only be called once since error is not retryable
	mockStatusWriter.AssertNumberOfCalls(t, "Update", 1)
}
