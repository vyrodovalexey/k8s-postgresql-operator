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
)

// Common error types that can be checked with errors.Is()
var (
	// ErrNotFound indicates a resource was not found
	ErrNotFound = New("resource not found")

	// ErrInvalidConfiguration indicates invalid configuration
	ErrInvalidConfiguration = New("invalid configuration")

	// ErrConnectionFailed indicates a connection failure
	ErrConnectionFailed = New("connection failed")

	// ErrOperationFailed indicates an operation failed
	ErrOperationFailed = New("operation failed")

	// ErrUnauthorized indicates unauthorized access
	ErrUnauthorized = New("unauthorized")

	// ErrTimeout indicates an operation timed out
	ErrTimeout = New("operation timed out")

	// ErrRetryExhausted indicates all retries were exhausted
	ErrRetryExhausted = New("retry attempts exhausted")
)

// Error represents a structured error with context
type Error struct {
	// Code is a machine-readable error code
	Code string
	// Message is a human-readable error message
	Message string
	// Op is the operation that failed
	Op string
	// Err is the underlying error
	Err error
}

// New creates a new error with the given message
func New(message string) *Error {
	return &Error{
		Code:    "UNKNOWN",
		Message: message,
	}
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Op != "" {
		if e.Err != nil {
			return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
		}
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error for error unwrapping
func (e *Error) Unwrap() error {
	return e.Err
}

// WithOp adds an operation context to the error
func (e *Error) WithOp(op string) *Error {
	return &Error{
		Code:    e.Code,
		Message: e.Message,
		Op:      op,
		Err:     e.Err,
	}
}

// WithCode sets the error code
func (e *Error) WithCode(code string) *Error {
	return &Error{
		Code:    code,
		Message: e.Message,
		Op:      e.Op,
		Err:     e.Err,
	}
}

// Wrap wraps an error with additional context
func Wrap(baseErr *Error, underlyingErr error) *Error {
	if baseErr == nil {
		if underlyingErr == nil {
			return nil
		}
		return &Error{
			Code:    "UNKNOWN",
			Message: underlyingErr.Error(),
			Err:     underlyingErr,
		}
	}
	if underlyingErr == nil {
		return baseErr
	}
	return &Error{
		Code:    baseErr.Code,
		Message: baseErr.Message,
		Op:      baseErr.Op,
		Err:     underlyingErr,
	}
}

// Wrapf wraps an error with a formatted message
func Wrapf(baseErr *Error, underlyingErr error, format string, args ...interface{}) *Error {
	if baseErr == nil {
		if underlyingErr == nil {
			return nil
		}
		return &Error{
			Code:    "UNKNOWN",
			Message: fmt.Sprintf(format, args...),
			Err:     underlyingErr,
		}
	}
	if underlyingErr == nil {
		return baseErr
	}
	return &Error{
		Code:    baseErr.Code,
		Message: fmt.Sprintf(format, args...),
		Op:      baseErr.Op,
		Err:     underlyingErr,
	}
}

// Is checks if the error matches the target error
func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.Code == t.Code && e.Message == t.Message
	}
	return errors.Is(e.Err, target)
}

// PostgreSQL errors
var (
	// ErrPostgresqlNotFound indicates a PostgreSQL instance was not found
	ErrPostgresqlNotFound = New("postgresql instance not found").WithCode("POSTGRESQL_NOT_FOUND")

	// ErrPostgresqlConnectionFailed indicates a PostgreSQL connection failed
	ErrPostgresqlConnectionFailed = New("postgresql connection failed").WithCode("POSTGRESQL_CONNECTION_FAILED")

	// ErrPostgresqlOperationFailed indicates a PostgreSQL operation failed
	ErrPostgresqlOperationFailed = New("postgresql operation failed").WithCode("POSTGRESQL_OPERATION_FAILED")

	// ErrPostgresqlDatabaseExists indicates a database already exists
	ErrPostgresqlDatabaseExists = New("database already exists").WithCode("POSTGRESQL_DATABASE_EXISTS")

	// ErrPostgresqlUserExists indicates a user already exists
	ErrPostgresqlUserExists = New("user already exists").WithCode("POSTGRESQL_USER_EXISTS")

	// ErrPostgresqlSchemaExists indicates a schema already exists
	ErrPostgresqlSchemaExists = New("schema already exists").WithCode("POSTGRESQL_SCHEMA_EXISTS")
)

// PostgreSQLError wraps PostgreSQL-related errors
func PostgreSQLError(op string, baseErr *Error) *Error {
	if baseErr == nil {
		return New("postgresql error").WithOp(op).WithCode("POSTGRESQL_ERROR")
	}
	return &Error{
		Code:    baseErr.Code,
		Message: baseErr.Message,
		Op:      op,
		Err:     baseErr,
	}
}

// Vault errors
var (
	// ErrVaultUnavailable indicates Vault is unavailable
	ErrVaultUnavailable = New("vault unavailable").WithCode("VAULT_UNAVAILABLE")

	// ErrVaultSecretNotFound indicates a secret was not found in Vault
	ErrVaultSecretNotFound = New("vault secret not found").WithCode("VAULT_SECRET_NOT_FOUND")

	// ErrVaultAuthenticationFailed indicates Vault authentication failed
	ErrVaultAuthenticationFailed = New("vault authentication failed").WithCode("VAULT_AUTH_FAILED")

	// ErrVaultOperationFailed indicates a Vault operation failed
	ErrVaultOperationFailed = New("vault operation failed").WithCode("VAULT_OPERATION_FAILED")
)

// VaultError wraps Vault-related errors
func VaultError(op string, baseErr *Error) *Error {
	if baseErr == nil {
		return New("vault error").WithOp(op).WithCode("VAULT_ERROR")
	}
	return &Error{
		Code:    baseErr.Code,
		Message: baseErr.Message,
		Op:      op,
		Err:     baseErr,
	}
}

// Kubernetes errors
var (
	// ErrKubernetesResourceNotFound indicates a Kubernetes resource was not found
	ErrKubernetesResourceNotFound = New("kubernetes resource not found").WithCode("K8S_RESOURCE_NOT_FOUND")

	// ErrKubernetesOperationFailed indicates a Kubernetes operation failed
	ErrKubernetesOperationFailed = New("kubernetes operation failed").WithCode("K8S_OPERATION_FAILED")

	// ErrKubernetesDuplicateResource indicates a duplicate Kubernetes resource exists
	ErrKubernetesDuplicateResource = New("duplicate kubernetes resource").WithCode("K8S_DUPLICATE_RESOURCE")
)

// KubernetesError wraps Kubernetes-related errors
func KubernetesError(op string, baseErr *Error) *Error {
	if baseErr == nil {
		return New("kubernetes error").WithOp(op).WithCode("K8S_ERROR")
	}
	return &Error{
		Code:    baseErr.Code,
		Message: baseErr.Message,
		Op:      op,
		Err:     baseErr,
	}
}

// Validation errors
var (
	// ErrValidationFailed indicates validation failed
	ErrValidationFailed = New("validation failed").WithCode("VALIDATION_FAILED")

	// ErrInvalidInput indicates invalid input
	ErrInvalidInput = New("invalid input").WithCode("INVALID_INPUT")
)

// ValidationError wraps validation-related errors
func ValidationError(message string) *Error {
	return New(message).WithCode("VALIDATION_ERROR")
}

// Helper functions for common error patterns

// IsNotFound checks if an error is a "not found" error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) ||
		errors.Is(err, ErrPostgresqlNotFound) ||
		errors.Is(err, ErrVaultSecretNotFound) ||
		errors.Is(err, ErrKubernetesResourceNotFound)
}

// IsConnectionError checks if an error is a connection error
func IsConnectionError(err error) bool {
	return errors.Is(err, ErrConnectionFailed) ||
		errors.Is(err, ErrPostgresqlConnectionFailed) ||
		errors.Is(err, ErrVaultUnavailable)
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	return IsConnectionError(err) ||
		errors.Is(err, ErrTimeout) ||
		errors.Is(err, ErrVaultUnavailable)
}

// AsError extracts the structured Error from an error chain
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
