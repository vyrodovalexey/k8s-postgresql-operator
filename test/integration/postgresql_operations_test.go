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

package integration_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
)

func TestIntegration_PostgreSQLFullLifecycle(t *testing.T) {
	pgCfg := newPGConfig(t)
	log := testLogger()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	suffix := time.Now().UnixNano()
	dbName := fmt.Sprintf("integ_lifecycle_db_%d", suffix)
	username := fmt.Sprintf("integ_lifecycle_user_%d", suffix)
	groupRole := fmt.Sprintf("integ_lifecycle_group_%d", suffix)
	schemaName := "lifecycle_schema"
	password := "lifecycle_pass_123"

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = pgCfg.DropTestDatabase(cleanupCtx, dbName)
		_ = pgCfg.DropTestUser(cleanupCtx, username)
		_ = pgCfg.DropTestUser(cleanupCtx, groupRole)
	})

	// Step 1: Create user
	t.Run("Step1_CreateUser", func(t *testing.T) {
		err := postgresql.CreateOrUpdateUser(
			ctx, pgCfg.Host, pgCfg.Port,
			pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
			username, password,
		)
		require.NoError(t, err, "Creating user should succeed")

		exists, err := pgCfg.UserExists(ctx, username)
		require.NoError(t, err)
		assert.True(t, exists, "User should exist")
	})

	// Step 2: Create database with owner
	t.Run("Step2_CreateDatabase", func(t *testing.T) {
		err := postgresql.CreateOrUpdateDatabase(
			ctx, pgCfg.Host, pgCfg.Port,
			pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
			dbName, username, "", "",
		)
		require.NoError(t, err, "Creating database should succeed")

		exists, err := pgCfg.DatabaseExists(ctx, dbName)
		require.NoError(t, err)
		assert.True(t, exists, "Database should exist")
	})

	// Step 3: Create schema
	t.Run("Step3_CreateSchema", func(t *testing.T) {
		err := postgresql.CreateOrUpdateSchema(
			ctx, pgCfg.Host, pgCfg.Port,
			pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
			dbName, schemaName, username,
		)
		require.NoError(t, err, "Creating schema should succeed")

		exists, err := pgCfg.SchemaExists(ctx, dbName, schemaName)
		require.NoError(t, err)
		assert.True(t, exists, "Schema should exist")
	})

	// Step 4: Apply grants
	t.Run("Step4_ApplyGrants", func(t *testing.T) {
		grants := []instancev1alpha1.GrantItem{
			{
				Type:       instancev1alpha1.GrantTypeDatabase,
				Privileges: []string{"CONNECT"},
			},
			{
				Type:       instancev1alpha1.GrantTypeSchema,
				Schema:     schemaName,
				Privileges: []string{"USAGE", "CREATE"},
			},
			{
				Type:       instancev1alpha1.GrantTypeAllTables,
				Schema:     schemaName,
				Privileges: []string{"SELECT", "INSERT", "UPDATE", "DELETE"},
			},
			{
				Type:       instancev1alpha1.GrantTypeAllSequences,
				Schema:     schemaName,
				Privileges: []string{"USAGE", "SELECT"},
			},
		}

		defaultPrivileges := []instancev1alpha1.DefaultPrivilegeItem{
			{
				ObjectType: "tables",
				Schema:     schemaName,
				Privileges: []string{"SELECT", "INSERT", "UPDATE", "DELETE"},
			},
			{
				ObjectType: "sequences",
				Schema:     schemaName,
				Privileges: []string{"USAGE", "SELECT"},
			},
		}

		err := postgresql.ApplyGrants(
			ctx, pgCfg.Host, pgCfg.Port,
			pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
			dbName, username, grants, defaultPrivileges,
		)
		require.NoError(t, err, "Applying grants should succeed")

		// Verify CONNECT privilege
		hasConnect, err := pgCfg.RoleHasPrivilege(ctx, dbName, username, "CONNECT")
		require.NoError(t, err)
		assert.True(t, hasConnect, "User should have CONNECT privilege")
	})

	// Step 5: Create role group
	t.Run("Step5_CreateRoleGroup", func(t *testing.T) {
		err := postgresql.CreateOrUpdateRoleGroup(
			ctx, pgCfg.Host, pgCfg.Port,
			pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
			groupRole, []string{username},
		)
		require.NoError(t, err, "Creating role group should succeed")

		exists, err := pgCfg.UserExists(ctx, groupRole)
		require.NoError(t, err)
		assert.True(t, exists, "Group role should exist")

		hasMember, err := pgCfg.RoleGroupHasMember(ctx, groupRole, username)
		require.NoError(t, err)
		assert.True(t, hasMember, "Group should have the user as member")
	})

	// Step 6: Verify user can connect to the database
	// Note: CreateOrUpdateUser uses pq.QuoteIdentifier for the password, which wraps it
	// in double quotes. The actual password stored in PostgreSQL includes these quotes.
	t.Run("Step6_VerifyUserConnection", func(t *testing.T) {
		quotedPassword := fmt.Sprintf("%q", password)
		connected, err := postgresql.TestConnection(
			ctx, pgCfg.Host, pgCfg.Port, dbName,
			username, quotedPassword, pgCfg.SSLMode,
			log, 3, 2*time.Second,
		)
		require.NoError(t, err, "User should be able to connect to the database")
		assert.True(t, connected)
	})

	// Step 7: Delete database
	t.Run("Step7_DeleteDatabase", func(t *testing.T) {
		err := postgresql.DeleteDatabase(
			ctx, pgCfg.Host, pgCfg.Port,
			pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
			dbName,
		)
		require.NoError(t, err, "Deleting database should succeed")

		exists, err := pgCfg.DatabaseExists(ctx, dbName)
		require.NoError(t, err)
		assert.False(t, exists, "Database should not exist after deletion")
	})
}

func TestIntegration_PostgreSQLConcurrentOperations(t *testing.T) {
	pgCfg := newPGConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	suffix := time.Now().UnixNano()
	numDatabases := 5

	// Create databases concurrently
	var wg sync.WaitGroup
	errors := make([]error, numDatabases)
	dbNames := make([]string, numDatabases)

	for i := 0; i < numDatabases; i++ {
		dbNames[i] = fmt.Sprintf("integ_concurrent_db_%d_%d", suffix, i)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		for _, dbName := range dbNames {
			_ = pgCfg.DropTestDatabase(cleanupCtx, dbName)
		}
	})

	// Create users for each database first (sequentially, as they share the same connection)
	for i := 0; i < numDatabases; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errors[idx] = postgresql.CreateOrUpdateDatabase(
				ctx, pgCfg.Host, pgCfg.Port,
				pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
				dbNames[idx], pgCfg.User, "", "",
			)
		}(i)
	}

	wg.Wait()

	// Verify all databases were created successfully
	for i := 0; i < numDatabases; i++ {
		assert.NoError(t, errors[i], "Concurrent database creation %d should succeed", i)

		exists, err := pgCfg.DatabaseExists(ctx, dbNames[i])
		require.NoError(t, err)
		assert.True(t, exists, "Database %s should exist", dbNames[i])
	}
}

func TestIntegration_PostgreSQLRetryLogic(t *testing.T) {
	pgCfg := newPGConfig(t)
	log := testLogger()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test retry with a valid connection (should succeed on first attempt)
	t.Run("SuccessfulRetry", func(t *testing.T) {
		callCount := 0
		operation := func() error {
			callCount++
			return postgresql.CreateOrUpdateUser(
				ctx, pgCfg.Host, pgCfg.Port,
				pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
				fmt.Sprintf("integ_retry_user_%d", time.Now().UnixNano()), "pass123",
			)
		}

		err := postgresql.ExecuteOperationWithRetry(
			ctx, operation, log, 3, 1*time.Second, "TestRetrySuccess",
		)
		require.NoError(t, err, "Operation should succeed")
		assert.Equal(t, 1, callCount, "Should succeed on first attempt")
	})

	// Test retry with a failing operation
	t.Run("FailingRetry", func(t *testing.T) {
		callCount := 0
		operation := func() error {
			callCount++
			// Try to connect to a non-existent host
			return postgresql.CreateOrUpdateUser(
				ctx, "nonexistent-host", pgCfg.Port,
				pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
				"retry_user", "pass123",
			)
		}

		err := postgresql.ExecuteOperationWithRetry(
			ctx, operation, log, 3, 500*time.Millisecond, "TestRetryFail",
		)
		require.Error(t, err, "Operation should fail after retries")
		assert.Equal(t, 3, callCount, "Should have attempted 3 times")
	})

	// Test retry with eventual success
	t.Run("EventualSuccess", func(t *testing.T) {
		callCount := 0
		username := fmt.Sprintf("integ_eventual_user_%d", time.Now().UnixNano())

		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cleanupCancel()
			_ = pgCfg.DropTestUser(cleanupCtx, username)
		})

		operation := func() error {
			callCount++
			if callCount < 3 {
				// Simulate failure on first two attempts
				return fmt.Errorf("simulated failure attempt %d", callCount)
			}
			// Succeed on third attempt
			return postgresql.CreateOrUpdateUser(
				ctx, pgCfg.Host, pgCfg.Port,
				pgCfg.User, pgCfg.Password, pgCfg.SSLMode,
				username, "pass123",
			)
		}

		err := postgresql.ExecuteOperationWithRetry(
			ctx, operation, log, 5, 500*time.Millisecond, "TestEventualSuccess",
		)
		require.NoError(t, err, "Operation should eventually succeed")
		assert.Equal(t, 3, callCount, "Should have attempted 3 times before succeeding")

		exists, err := pgCfg.UserExists(ctx, username)
		require.NoError(t, err)
		assert.True(t, exists, "User should exist after eventual success")
	})
}
