//go:build integration

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

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
	"github.com/vyrodovalexey/k8s-postgresql-operator/test/testutils"
)

// TestIntegration_CreateUser tests creating a user in PostgreSQL
func TestIntegration_CreateUser(t *testing.T) {
	skipIfNoPostgres(t)

	ctx := context.Background()
	username := testutils.GenerateTestUsername()
	password := testutils.GenerateTestPassword(16)

	t.Cleanup(func() {
		cleanupTestUser(t, username)
	})

	// Create user
	err := pg.CreateOrUpdateUser(
		ctx,
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		testConfig.PostgresSSLMode,
		username,
		password,
	)
	require.NoError(t, err)

	// Verify user exists
	testutils.AssertPostgresUserExists(t, ctx, testDB, username)
}

// TestIntegration_UpdateUserPassword tests updating a user's password
func TestIntegration_UpdateUserPassword(t *testing.T) {
	skipIfNoPostgres(t)

	ctx := context.Background()
	username := testutils.GenerateTestUsername()
	password1 := testutils.GenerateTestPassword(16)
	password2 := testutils.GenerateTestPassword(16)

	t.Cleanup(func() {
		cleanupTestUser(t, username)
	})

	// Create user with initial password
	err := pg.CreateOrUpdateUser(
		ctx,
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		testConfig.PostgresSSLMode,
		username,
		password1,
	)
	require.NoError(t, err)

	// Update password
	err = pg.CreateOrUpdateUser(
		ctx,
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		testConfig.PostgresSSLMode,
		username,
		password2,
	)
	require.NoError(t, err)

	// Verify user still exists
	testutils.AssertPostgresUserExists(t, ctx, testDB, username)
}

// TestIntegration_CreateDatabase tests creating a database in PostgreSQL
func TestIntegration_CreateDatabase(t *testing.T) {
	skipIfNoPostgres(t)

	ctx := context.Background()
	dbName := testutils.SanitizeName(testutils.GenerateTestDatabaseName())
	owner := testConfig.PostgresUser

	t.Cleanup(func() {
		cleanupTestDatabase(t, dbName)
	})

	// Create database
	err := pg.CreateOrUpdateDatabase(
		ctx,
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		testConfig.PostgresSSLMode,
		dbName,
		owner,
		"", // schema
		"", // template
	)
	require.NoError(t, err)

	// Verify database exists
	testutils.AssertPostgresDatabaseExists(t, ctx, testDB, dbName)
}

// TestIntegration_CreateDatabaseWithOwner tests creating a database with a specific owner
func TestIntegration_CreateDatabaseWithOwner(t *testing.T) {
	skipIfNoPostgres(t)

	ctx := context.Background()
	username := testutils.SanitizeName(testutils.GenerateTestUsername())
	dbName := testutils.SanitizeName(testutils.GenerateTestDatabaseName())
	password := testutils.GenerateTestPassword(16)

	t.Cleanup(func() {
		cleanupTestDatabase(t, dbName)
		cleanupTestUser(t, username)
	})

	// Create owner user first
	err := pg.CreateOrUpdateUser(
		ctx,
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		testConfig.PostgresSSLMode,
		username,
		password,
	)
	require.NoError(t, err)

	// Create database with owner
	err = pg.CreateOrUpdateDatabase(
		ctx,
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		testConfig.PostgresSSLMode,
		dbName,
		username,
		"",
		"",
	)
	require.NoError(t, err)

	// Verify database exists and has correct owner
	testutils.AssertPostgresDatabaseExists(t, ctx, testDB, dbName)
	testutils.AssertDatabaseOwner(t, ctx, testDB, dbName, username)
}

// TestIntegration_DeleteDatabase tests deleting a database from PostgreSQL
func TestIntegration_DeleteDatabase(t *testing.T) {
	skipIfNoPostgres(t)

	ctx := context.Background()
	dbName := testutils.SanitizeName(testutils.GenerateTestDatabaseName())

	// Create database first
	err := pg.CreateOrUpdateDatabase(
		ctx,
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		testConfig.PostgresSSLMode,
		dbName,
		testConfig.PostgresUser,
		"",
		"",
	)
	require.NoError(t, err)

	// Verify database exists
	testutils.AssertPostgresDatabaseExists(t, ctx, testDB, dbName)

	// Close all sessions to the database
	err = pg.CloseAllSessions(
		ctx,
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		testConfig.PostgresSSLMode,
		dbName,
	)
	require.NoError(t, err)

	// Delete database
	err = pg.DeleteDatabase(
		ctx,
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		testConfig.PostgresSSLMode,
		dbName,
	)
	require.NoError(t, err)

	// Verify database no longer exists
	testutils.AssertPostgresDatabaseNotExists(t, ctx, testDB, dbName)
}

// TestIntegration_CreateRoleGroup tests creating a role group in PostgreSQL
func TestIntegration_CreateRoleGroup(t *testing.T) {
	skipIfNoPostgres(t)

	ctx := context.Background()
	groupRole := testutils.SanitizeName(testutils.GenerateTestRoleName())
	memberRole := testutils.SanitizeName(testutils.GenerateTestUsername())
	password := testutils.GenerateTestPassword(16)

	t.Cleanup(func() {
		cleanupTestRole(t, groupRole)
		cleanupTestUser(t, memberRole)
	})

	// Create member user first
	err := pg.CreateOrUpdateUser(
		ctx,
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		testConfig.PostgresSSLMode,
		memberRole,
		password,
	)
	require.NoError(t, err)

	// Create role group with member
	err = pg.CreateOrUpdateRoleGroup(
		ctx,
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		testConfig.PostgresSSLMode,
		groupRole,
		[]string{memberRole},
	)
	require.NoError(t, err)

	// Verify role group exists
	testutils.AssertPostgresRoleExists(t, ctx, testDB, groupRole)

	// Verify membership
	testutils.AssertRoleMembership(t, ctx, testDB, memberRole, groupRole)
}

// TestIntegration_TestConnection tests the connection test function
func TestIntegration_TestConnection(t *testing.T) {
	skipIfNoPostgres(t)

	ctx := context.Background()
	logger := testutils.SetupNopLogger()

	// Test successful connection
	connected, err := pg.TestConnection(
		ctx,
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		"postgres",
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		testConfig.PostgresSSLMode,
		logger,
		3,
		5*time.Second,
	)
	require.NoError(t, err)
	assert.True(t, connected)
}

// TestIntegration_TestConnectionFailure tests connection failure handling
func TestIntegration_TestConnectionFailure(t *testing.T) {
	ctx := context.Background()
	logger := testutils.SetupNopLogger()

	// Test failed connection to invalid host
	connected, err := pg.TestConnection(
		ctx,
		"invalid-host-that-does-not-exist",
		5432,
		"postgres",
		"postgres",
		"postgres",
		"disable",
		logger,
		1,
		2*time.Second,
	)
	assert.Error(t, err)
	assert.False(t, connected)
}

// TestIntegration_ExecuteOperationWithRetry tests the retry logic
func TestIntegration_ExecuteOperationWithRetry(t *testing.T) {
	skipIfNoPostgres(t)

	ctx := context.Background()
	logger := testutils.SetupNopLogger()

	// Test successful operation
	callCount := 0
	err := pg.ExecuteOperationWithRetry(
		ctx,
		func() error {
			callCount++
			return nil
		},
		logger,
		3,
		100*time.Millisecond,
		"test-operation",
	)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

// TestIntegration_ExecuteOperationWithRetryFailure tests retry on failure
func TestIntegration_ExecuteOperationWithRetryFailure(t *testing.T) {
	ctx := context.Background()
	logger := testutils.SetupNopLogger()

	// Test operation that always fails
	// The retry logic uses exponential backoff with max elapsed time
	// calculated as: retries * retryDelay * 3 = 3 * 100ms * 3 = 900ms
	// With exponential backoff, it will retry multiple times until max elapsed time
	callCount := 0
	err := pg.ExecuteOperationWithRetry(
		ctx,
		func() error {
			callCount++
			return assert.AnError
		},
		logger,
		3,
		100*time.Millisecond,
		"test-failing-operation",
	)
	assert.Error(t, err)
	// Should retry at least once (more than 1 attempt)
	assert.Greater(t, callCount, 1, "Should have retried at least once")
	// The error message should indicate the operation failed
	assert.Contains(t, err.Error(), "test-failing-operation")
}
