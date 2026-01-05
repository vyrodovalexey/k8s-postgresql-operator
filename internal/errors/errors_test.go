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

package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	err := New("test error")
	assert.NotNil(t, err)
	assert.Equal(t, "UNKNOWN", err.Code)
	assert.Equal(t, "test error", err.Message)
	assert.Empty(t, err.Op)
	assert.Nil(t, err.Err)
}

func TestError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *Error
		expected string
	}{
		{
			name: "Simple error",
			err: &Error{
				Code:    "TEST",
				Message: "test message",
			},
			expected: "test message",
		},
		{
			name: "Error with operation",
			err: &Error{
				Code:    "TEST",
				Message: "test message",
				Op:      "testOp",
			},
			expected: "testOp: test message",
		},
		{
			name: "Error with underlying error",
			err: &Error{
				Code:    "TEST",
				Message: "test message",
				Err:     fmt.Errorf("underlying error"),
			},
			expected: "test message: underlying error",
		},
		{
			name: "Error with operation and underlying error",
			err: &Error{
				Code:    "TEST",
				Message: "test message",
				Op:      "testOp",
				Err:     fmt.Errorf("underlying error"),
			},
			expected: "testOp: test message: underlying error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestError_Unwrap(t *testing.T) {
	underlyingErr := fmt.Errorf("underlying error")
	err := &Error{
		Code:    "TEST",
		Message: "test message",
		Err:     underlyingErr,
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, underlyingErr, unwrapped)
}

func TestError_WithOp(t *testing.T) {
	err := New("test error")
	errWithOp := err.WithOp("testOperation")

	assert.NotEqual(t, err, errWithOp)
	assert.Equal(t, "testOperation", errWithOp.Op)
	assert.Equal(t, err.Code, errWithOp.Code)
	assert.Equal(t, err.Message, errWithOp.Message)
	assert.Equal(t, err.Err, errWithOp.Err)
}

func TestError_WithCode(t *testing.T) {
	err := New("test error")
	errWithCode := err.WithCode("TEST_CODE")

	assert.NotEqual(t, err, errWithCode)
	assert.Equal(t, "TEST_CODE", errWithCode.Code)
	assert.Equal(t, err.Message, errWithCode.Message)
	assert.Equal(t, err.Op, errWithCode.Op)
	assert.Equal(t, err.Err, errWithCode.Err)
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name          string
		baseErr       *Error
		underlyingErr error
		expectNil     bool
		checkCode     string
		checkMessage  string
	}{
		{
			name:          "Both nil",
			baseErr:       nil,
			underlyingErr: nil,
			expectNil:     true,
		},
		{
			name:          "Base nil, underlying not nil",
			baseErr:       nil,
			underlyingErr: fmt.Errorf("underlying"),
			expectNil:     false,
			checkCode:     "UNKNOWN",
			checkMessage:  "underlying",
		},
		{
			name:          "Base not nil, underlying nil",
			baseErr:       New("base error"),
			underlyingErr: nil,
			expectNil:     false,
			checkCode:     "UNKNOWN",
			checkMessage:  "base error",
		},
		{
			name:          "Both not nil",
			baseErr:       New("base error").WithCode("BASE_CODE"),
			underlyingErr: fmt.Errorf("underlying"),
			expectNil:     false,
			checkCode:     "BASE_CODE",
			checkMessage:  "base error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Wrap(tt.baseErr, tt.underlyingErr)
			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.checkCode, result.Code)
				assert.Equal(t, tt.checkMessage, result.Message)
				if tt.underlyingErr != nil && tt.baseErr != nil {
					assert.Equal(t, tt.underlyingErr, result.Err)
				}
			}
		})
	}
}

func TestWrapf(t *testing.T) {
	baseErr := New("base error").WithCode("BASE_CODE")
	underlyingErr := fmt.Errorf("underlying")

	result := Wrapf(baseErr, underlyingErr, "formatted: %s", "value")
	assert.NotNil(t, result)
	assert.Equal(t, "BASE_CODE", result.Code)
	assert.Equal(t, "formatted: value", result.Message)
	assert.Equal(t, underlyingErr, result.Err)
}

func TestError_Is(t *testing.T) {
	err1 := New("test error").WithCode("TEST_CODE")
	err2 := New("test error").WithCode("TEST_CODE")
	err3 := New("different error").WithCode("TEST_CODE")
	err4 := New("test error").WithCode("DIFFERENT_CODE")

	// Same code and message
	assert.True(t, err1.Is(err2))
	// Different message
	assert.False(t, err1.Is(err3))
	// Different code
	assert.False(t, err1.Is(err4))

	// Test with underlying error
	underlyingErr := fmt.Errorf("underlying")
	wrappedErr := Wrap(err1, underlyingErr)
	assert.True(t, errors.Is(wrappedErr, underlyingErr))
}

func TestPostgreSQLError(t *testing.T) {
	tests := []struct {
		name      string
		op        string
		baseErr   *Error
		checkOp   string
		checkCode string
	}{
		{
			name:      "With base error",
			op:        "testOp",
			baseErr:   ErrPostgresqlConnectionFailed,
			checkOp:   "testOp",
			checkCode: "POSTGRESQL_CONNECTION_FAILED",
		},
		{
			name:      "Without base error",
			op:        "testOp",
			baseErr:   nil,
			checkOp:   "testOp",
			checkCode: "POSTGRESQL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PostgreSQLError(tt.op, tt.baseErr)
			assert.NotNil(t, result)
			assert.Equal(t, tt.checkOp, result.Op)
			assert.Equal(t, tt.checkCode, result.Code)
		})
	}
}

func TestVaultError(t *testing.T) {
	tests := []struct {
		name      string
		op        string
		baseErr   *Error
		checkOp   string
		checkCode string
	}{
		{
			name:      "With base error",
			op:        "testOp",
			baseErr:   ErrVaultUnavailable,
			checkOp:   "testOp",
			checkCode: "VAULT_UNAVAILABLE",
		},
		{
			name:      "Without base error",
			op:        "testOp",
			baseErr:   nil,
			checkOp:   "testOp",
			checkCode: "VAULT_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VaultError(tt.op, tt.baseErr)
			assert.NotNil(t, result)
			assert.Equal(t, tt.checkOp, result.Op)
			assert.Equal(t, tt.checkCode, result.Code)
		})
	}
}

func TestKubernetesError(t *testing.T) {
	tests := []struct {
		name      string
		op        string
		baseErr   *Error
		checkOp   string
		checkCode string
	}{
		{
			name:      "With base error",
			op:        "testOp",
			baseErr:   ErrKubernetesResourceNotFound,
			checkOp:   "testOp",
			checkCode: "K8S_RESOURCE_NOT_FOUND",
		},
		{
			name:      "Without base error",
			op:        "testOp",
			baseErr:   nil,
			checkOp:   "testOp",
			checkCode: "K8S_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := KubernetesError(tt.op, tt.baseErr)
			assert.NotNil(t, result)
			assert.Equal(t, tt.checkOp, result.Op)
			assert.Equal(t, tt.checkCode, result.Code)
		})
	}
}

func TestValidationError(t *testing.T) {
	result := ValidationError("invalid input")
	assert.NotNil(t, result)
	assert.Equal(t, "VALIDATION_ERROR", result.Code)
	assert.Equal(t, "invalid input", result.Message)
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrNotFound", ErrNotFound, true},
		{"ErrPostgresqlNotFound", ErrPostgresqlNotFound, true},
		{"ErrVaultSecretNotFound", ErrVaultSecretNotFound, true},
		{"ErrKubernetesResourceNotFound", ErrKubernetesResourceNotFound, true},
		{"Wrapped ErrNotFound", Wrap(ErrNotFound, fmt.Errorf("wrapped")), true},
		{"Other error", ErrConnectionFailed, false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsNotFound(tt.err))
		})
	}
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrConnectionFailed", ErrConnectionFailed, true},
		{"ErrPostgresqlConnectionFailed", ErrPostgresqlConnectionFailed, true},
		{"ErrVaultUnavailable", ErrVaultUnavailable, true},
		{"Wrapped connection error", Wrap(ErrConnectionFailed, fmt.Errorf("wrapped")), true},
		{"Other error", ErrNotFound, false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsConnectionError(tt.err))
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"Connection error", ErrConnectionFailed, true},
		{"PostgreSQL connection error", ErrPostgresqlConnectionFailed, true},
		{"Vault unavailable", ErrVaultUnavailable, true},
		{"Timeout error", ErrTimeout, true},
		{"Wrapped retryable error", Wrap(ErrTimeout, fmt.Errorf("wrapped")), true},
		{"Non-retryable error", ErrNotFound, false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsRetryable(tt.err))
		})
	}
}

func TestAsError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		expected  bool
		checkCode string
	}{
		{"*Error", New("test"), true, "UNKNOWN"},
		{"Wrapped *Error", Wrap(ErrNotFound, fmt.Errorf("wrapped")), true, "UNKNOWN"},
		{"Standard error", fmt.Errorf("standard error"), false, ""},
		{"Nil error", nil, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := AsError(tt.err)
			assert.Equal(t, tt.expected, ok)
			if tt.expected {
				assert.NotNil(t, result)
				assert.Equal(t, tt.checkCode, result.Code)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestErrorConstants(t *testing.T) {
	// Test that all error constants are properly initialized
	assert.NotNil(t, ErrNotFound)
	assert.NotNil(t, ErrInvalidConfiguration)
	assert.NotNil(t, ErrConnectionFailed)
	assert.NotNil(t, ErrOperationFailed)
	assert.NotNil(t, ErrUnauthorized)
	assert.NotNil(t, ErrTimeout)
	assert.NotNil(t, ErrRetryExhausted)

	assert.NotNil(t, ErrPostgresqlNotFound)
	assert.NotNil(t, ErrPostgresqlConnectionFailed)
	assert.NotNil(t, ErrPostgresqlOperationFailed)
	assert.NotNil(t, ErrPostgresqlDatabaseExists)
	assert.NotNil(t, ErrPostgresqlUserExists)
	assert.NotNil(t, ErrPostgresqlSchemaExists)

	assert.NotNil(t, ErrVaultUnavailable)
	assert.NotNil(t, ErrVaultSecretNotFound)
	assert.NotNil(t, ErrVaultAuthenticationFailed)
	assert.NotNil(t, ErrVaultOperationFailed)

	assert.NotNil(t, ErrKubernetesResourceNotFound)
	assert.NotNil(t, ErrKubernetesOperationFailed)
	assert.NotNil(t, ErrKubernetesDuplicateResource)

	assert.NotNil(t, ErrValidationFailed)
	assert.NotNil(t, ErrInvalidInput)
}

func TestError_WithOp_Chaining(t *testing.T) {
	err := New("test error").WithOp("op1").WithOp("op2")
	assert.Equal(t, "op2", err.Op)
}

func TestError_WithCode_Chaining(t *testing.T) {
	err := New("test error").WithCode("CODE1").WithCode("CODE2")
	assert.Equal(t, "CODE2", err.Code)
}

func TestWrap_WithOp(t *testing.T) {
	baseErr := New("base error").WithCode("BASE_CODE").WithOp("baseOp")
	underlyingErr := fmt.Errorf("underlying")
	wrapped := Wrap(baseErr, underlyingErr)

	assert.Equal(t, "BASE_CODE", wrapped.Code)
	assert.Equal(t, "base error", wrapped.Message)
	assert.Equal(t, "baseOp", wrapped.Op)
	assert.Equal(t, underlyingErr, wrapped.Err)
}
