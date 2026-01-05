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

package postgresql

import (
	"context"
	"time"

	"go.uber.org/zap"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// Client defines the interface for PostgreSQL operations
// This interface abstracts PostgreSQL operations to enable dependency injection and testing
type Client interface {
	// CreateOrUpdateDatabase creates or updates a PostgreSQL database
	CreateOrUpdateDatabase(
		ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode,
		databaseName, owner, schemaName, templateDatabase string) error

	// CreateOrUpdateUser creates or updates a PostgreSQL user
	CreateOrUpdateUser(
		ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, username, password string) error

	// CreateOrUpdateRoleGroup creates or updates a PostgreSQL role group
	CreateOrUpdateRoleGroup(
		ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode,
		groupRole string, memberRoles []string) error

	// CreateOrUpdateSchema creates or updates a PostgreSQL schema
	CreateOrUpdateSchema(
		ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode,
		databaseName, schemaName, owner string) error

	// ApplyGrants applies grants to a PostgreSQL role on a database
	ApplyGrants(
		ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode,
		databaseName, roleName string, grants []instancev1alpha1.GrantItem,
		defaultPrivileges []instancev1alpha1.DefaultPrivilegeItem) error

	// DeleteDatabase deletes a PostgreSQL database
	DeleteDatabase(
		ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, databaseName string) error

	// CloseAllSessions closes all active sessions/connections to a database
	CloseAllSessions(
		ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, databaseName string) error

	// TestConnection tests the connection to a PostgreSQL instance with retry logic
	TestConnection(
		ctx context.Context, host string, port int32, database, username, password, sslMode string,
		log *zap.SugaredLogger, retries int, retryTimeout time.Duration) (bool, error)

	// TestConnectionFromPostgresql tests the connection to a PostgreSQL instance from a Postgresql CRD
	TestConnectionFromPostgresql(
		ctx context.Context, postgresql *instancev1alpha1.Postgresql, vaultClient *vault.Client,
		log *zap.SugaredLogger, retries int, retryTimeout time.Duration) error

	// ExecuteOperationWithRetry executes a PostgreSQL operation with retry logic
	ExecuteOperationWithRetry(
		ctx context.Context, operation func() error, log *zap.SugaredLogger,
		retries int, retryDelay time.Duration, operationName string) error
}

// DefaultClient is the default implementation of the Client interface
// It uses the package-level functions for PostgreSQL operations
type DefaultClient struct{}

// NewDefaultClient creates a new default PostgreSQL client
func NewDefaultClient() Client {
	return &DefaultClient{}
}

// CreateOrUpdateDatabase implements Client.CreateOrUpdateDatabase
func (c *DefaultClient) CreateOrUpdateDatabase(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode,
	databaseName, owner, schemaName, templateDatabase string) error {
	return CreateOrUpdateDatabase(ctx, host, port, adminUser, adminPassword, sslMode,
		databaseName, owner, schemaName, templateDatabase)
}

// CreateOrUpdateUser implements Client.CreateOrUpdateUser
func (c *DefaultClient) CreateOrUpdateUser(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, username, password string) error {
	return CreateOrUpdateUser(ctx, host, port, adminUser, adminPassword, sslMode, username, password)
}

// CreateOrUpdateRoleGroup implements Client.CreateOrUpdateRoleGroup
func (c *DefaultClient) CreateOrUpdateRoleGroup(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode,
	groupRole string, memberRoles []string) error {
	return CreateOrUpdateRoleGroup(ctx, host, port, adminUser, adminPassword, sslMode, groupRole, memberRoles)
}

// CreateOrUpdateSchema implements Client.CreateOrUpdateSchema
func (c *DefaultClient) CreateOrUpdateSchema(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode,
	databaseName, schemaName, owner string) error {
	return CreateOrUpdateSchema(ctx, host, port, adminUser, adminPassword, sslMode, databaseName, schemaName, owner)
}

// ApplyGrants implements Client.ApplyGrants
func (c *DefaultClient) ApplyGrants(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode,
	databaseName, roleName string, grants []instancev1alpha1.GrantItem,
	defaultPrivileges []instancev1alpha1.DefaultPrivilegeItem) error {
	return ApplyGrants(
		ctx, host, port, adminUser, adminPassword, sslMode,
		databaseName, roleName, grants, defaultPrivileges)
}

// DeleteDatabase implements Client.DeleteDatabase
func (c *DefaultClient) DeleteDatabase(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, databaseName string) error {
	return DeleteDatabase(ctx, host, port, adminUser, adminPassword, sslMode, databaseName)
}

// CloseAllSessions implements Client.CloseAllSessions
func (c *DefaultClient) CloseAllSessions(
	ctx context.Context, host string, port int32, adminUser, adminPassword, sslMode, databaseName string) error {
	return CloseAllSessions(ctx, host, port, adminUser, adminPassword, sslMode, databaseName)
}

// TestConnection implements Client.TestConnection
func (c *DefaultClient) TestConnection(
	ctx context.Context, host string, port int32, database, username, password, sslMode string,
	log *zap.SugaredLogger, retries int, retryTimeout time.Duration) (bool, error) {
	return TestConnection(ctx, host, port, database, username, password, sslMode, log, retries, retryTimeout)
}

// TestConnectionFromPostgresql implements Client.TestConnectionFromPostgresql
func (c *DefaultClient) TestConnectionFromPostgresql(
	ctx context.Context, postgresql *instancev1alpha1.Postgresql, vaultClient *vault.Client,
	log *zap.SugaredLogger, retries int, retryTimeout time.Duration) error {
	return TestConnectionFromPostgresql(ctx, postgresql, vaultClient, log, retries, retryTimeout)
}

// ExecuteOperationWithRetry implements Client.ExecuteOperationWithRetry
func (c *DefaultClient) ExecuteOperationWithRetry(
	ctx context.Context, operation func() error, log *zap.SugaredLogger,
	retries int, retryDelay time.Duration, operationName string) error {
	return ExecuteOperationWithRetry(ctx, operation, log, retries, retryDelay, operationName)
}

// Ensure DefaultClient implements Client interface
var _ Client = (*DefaultClient)(nil)
