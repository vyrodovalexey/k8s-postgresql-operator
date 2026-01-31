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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestDefaultStatusUpdateOptions(t *testing.T) {
	opts := DefaultStatusUpdateOptions()

	assert.Equal(t, DefaultStatusUpdateRetries, opts.Retries)
	assert.Equal(t, DefaultStatusUpdateRetryDelay, opts.RetryDelay)
}

func TestIsRetryableStatusError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "conflict error is retryable",
			err:       errors.NewConflict(schema.GroupResource{Group: "test", Resource: "test"}, "test", nil),
			retryable: true,
		},
		{
			name:      "server timeout is retryable",
			err:       errors.NewServerTimeout(schema.GroupResource{Group: "test", Resource: "test"}, "test", 1),
			retryable: true,
		},
		{
			name:      "service unavailable is retryable",
			err:       errors.NewServiceUnavailable("service unavailable"),
			retryable: true,
		},
		{
			name:      "too many requests is retryable",
			err:       errors.NewTooManyRequests("too many requests", 1),
			retryable: true,
		},
		{
			name:      "internal error is retryable",
			err:       errors.NewInternalError(fmt.Errorf("internal error")),
			retryable: true,
		},
		{
			name:      "not found is not retryable",
			err:       errors.NewNotFound(schema.GroupResource{Group: "test", Resource: "test"}, "test"),
			retryable: false,
		},
		{
			name:      "bad request is not retryable",
			err:       errors.NewBadRequest("bad request"),
			retryable: false,
		},
		{
			name:      "forbidden is not retryable",
			err:       errors.NewForbidden(schema.GroupResource{Group: "test", Resource: "test"}, "test", nil),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableStatusError(tt.err)
			assert.Equal(t, tt.retryable, result)
		})
	}
}

func TestStatusUpdateOptions(t *testing.T) {
	opts := StatusUpdateOptions{
		Retries:    5,
		RetryDelay: 200 * time.Millisecond,
	}

	assert.Equal(t, 5, opts.Retries)
	assert.Equal(t, 200*time.Millisecond, opts.RetryDelay)
}
