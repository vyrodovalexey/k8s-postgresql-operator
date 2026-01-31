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
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	_ "github.com/lib/pq"

	"github.com/vyrodovalexey/k8s-postgresql-operator/test/testutils"
)

var (
	testConfig  *testutils.TestConfig
	testDB      *sql.DB
	vaultClient *api.Client
)

func TestMain(m *testing.M) {
	testConfig = testutils.DefaultTestConfig()

	// Setup PostgreSQL connection
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s connect_timeout=10",
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		testConfig.PostgresSSLMode,
	)

	var err error
	testDB, err = sql.Open("postgres", connStr)
	if err != nil {
		fmt.Printf("Failed to connect to PostgreSQL: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := testDB.PingContext(ctx); err != nil {
		fmt.Printf("Failed to ping PostgreSQL: %v\n", err)
		os.Exit(1)
	}

	// Setup Vault connection
	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = testConfig.VaultAddr

	vaultClient, err = api.NewClient(vaultConfig)
	if err != nil {
		fmt.Printf("Failed to create Vault client: %v\n", err)
		os.Exit(1)
	}
	vaultClient.SetToken(testConfig.VaultToken)

	// Run tests
	code := m.Run()

	// Cleanup
	if testDB != nil {
		testDB.Close()
	}

	os.Exit(code)
}

// skipIfNoPostgres skips the test if PostgreSQL is not available
func skipIfNoPostgres(t *testing.T) {
	t.Helper()
	if testDB == nil {
		t.Skip("PostgreSQL not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := testDB.PingContext(ctx); err != nil {
		t.Skipf("PostgreSQL not reachable: %v", err)
	}
}

// skipIfNoVault skips the test if Vault is not available
func skipIfNoVault(t *testing.T) {
	t.Helper()
	if vaultClient == nil {
		t.Skip("Vault not available")
	}
	health, err := vaultClient.Sys().Health()
	if err != nil {
		t.Skipf("Vault not reachable: %v", err)
	}
	if !health.Initialized {
		t.Skip("Vault not initialized")
	}
}

// cleanupTestUser removes a test user from PostgreSQL
func cleanupTestUser(t *testing.T, username string) {
	t.Helper()
	if testDB == nil {
		return
	}
	_, _ = testDB.Exec(fmt.Sprintf("DROP USER IF EXISTS %q", username))
}

// cleanupTestDatabase removes a test database from PostgreSQL
func cleanupTestDatabase(t *testing.T, dbName string) {
	t.Helper()
	if testDB == nil {
		return
	}
	// Terminate connections
	_, _ = testDB.Exec(fmt.Sprintf(`
		SELECT pg_terminate_backend(pid) 
		FROM pg_stat_activity 
		WHERE datname = '%s' AND pid <> pg_backend_pid()
	`, dbName))
	_, _ = testDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %q", dbName))
}

// cleanupTestRole removes a test role from PostgreSQL
func cleanupTestRole(t *testing.T, roleName string) {
	t.Helper()
	if testDB == nil {
		return
	}
	_, _ = testDB.Exec(fmt.Sprintf("DROP ROLE IF EXISTS %q", roleName))
}

// cleanupTestSchema removes a test schema from a database
func cleanupTestSchema(t *testing.T, dbName, schemaName string) {
	t.Helper()
	if testDB == nil {
		return
	}
	// Connect to the specific database to drop the schema
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		testConfig.PostgresHost,
		testConfig.PostgresPort,
		testConfig.PostgresUser,
		testConfig.PostgresPassword,
		dbName,
		testConfig.PostgresSSLMode,
	)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return
	}
	defer db.Close()
	_, _ = db.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %q CASCADE", schemaName))
}
