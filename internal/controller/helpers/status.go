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
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultStatusUpdateRetries is the default number of retries for status updates
	DefaultStatusUpdateRetries = 3
	// DefaultStatusUpdateRetryDelay is the default delay between status update retries
	DefaultStatusUpdateRetryDelay = 100 * time.Millisecond
)

// StatusUpdateOptions contains options for status update with retry
type StatusUpdateOptions struct {
	Retries    int
	RetryDelay time.Duration
}

// DefaultStatusUpdateOptions returns the default options for status updates
func DefaultStatusUpdateOptions() StatusUpdateOptions {
	return StatusUpdateOptions{
		Retries:    DefaultStatusUpdateRetries,
		RetryDelay: DefaultStatusUpdateRetryDelay,
	}
}

// UpdateStatusWithRetry updates the status of a Kubernetes object with retry logic for transient failures.
// It handles conflict errors by retrying the update operation.
func UpdateStatusWithRetry(
	ctx context.Context,
	statusClient client.StatusClient,
	obj client.Object,
	log *zap.SugaredLogger,
) error {
	return UpdateStatusWithRetryAndOptions(ctx, statusClient, obj, log, DefaultStatusUpdateOptions())
}

// UpdateStatusWithRetryAndOptions updates the status with custom retry options
func UpdateStatusWithRetryAndOptions(
	ctx context.Context,
	statusClient client.StatusClient,
	obj client.Object,
	log *zap.SugaredLogger,
	opts StatusUpdateOptions,
) error {
	var lastErr error

	for attempt := 1; attempt <= opts.Retries; attempt++ {
		err := statusClient.Status().Update(ctx, obj)
		if err == nil {
			if attempt > 1 {
				log.Debugw("Status update succeeded after retry",
					"kind", obj.GetObjectKind().GroupVersionKind().Kind,
					"name", obj.GetName(),
					"namespace", obj.GetNamespace(),
					"attempt", attempt)
			}
			return nil
		}

		lastErr = err

		// Check if the error is retryable
		if !isRetryableStatusError(err) {
			log.Errorw("Status update failed with non-retryable error",
				"kind", obj.GetObjectKind().GroupVersionKind().Kind,
				"name", obj.GetName(),
				"namespace", obj.GetNamespace(),
				"error", err)
			return err
		}

		// Log the retry attempt
		if attempt < opts.Retries {
			log.Debugw("Status update failed, retrying",
				"kind", obj.GetObjectKind().GroupVersionKind().Kind,
				"name", obj.GetName(),
				"namespace", obj.GetNamespace(),
				"attempt", attempt,
				"maxRetries", opts.Retries,
				"error", err)

			// Wait before retrying
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(opts.RetryDelay):
				// Continue to next attempt
			}
		}
	}

	log.Errorw("Status update failed after all retries",
		"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"name", obj.GetName(),
		"namespace", obj.GetNamespace(),
		"attempts", opts.Retries,
		"error", lastErr)

	return lastErr
}

// isRetryableStatusError determines if an error is retryable for status updates
func isRetryableStatusError(err error) bool {
	// Conflict errors are retryable (resource was modified by another process)
	if errors.IsConflict(err) {
		return true
	}

	// Server timeout errors are retryable
	if errors.IsServerTimeout(err) {
		return true
	}

	// Service unavailable errors are retryable
	if errors.IsServiceUnavailable(err) {
		return true
	}

	// Too many requests errors are retryable
	if errors.IsTooManyRequests(err) {
		return true
	}

	// Internal server errors may be transient
	if errors.IsInternalError(err) {
		return true
	}

	return false
}
