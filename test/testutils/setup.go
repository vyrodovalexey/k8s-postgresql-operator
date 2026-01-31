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

// Package testutils provides shared test utilities for functional tests
package testutils

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/vault/api"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// TestConfig holds configuration for functional tests
type TestConfig struct {
	// PostgreSQL configuration
	PostgresHost     string
	PostgresPort     int32
	PostgresUser     string
	PostgresPassword string
	PostgresSSLMode  string

	// Vault configuration
	VaultAddr       string
	VaultToken      string
	VaultMountPoint string
	VaultSecretPath string

	// Test configuration
	TestTimeout    time.Duration
	CleanupTimeout time.Duration
	RetryInterval  time.Duration
	MaxRetries     int
	TestNamespace  string
	PostgresqlID   string
}

// DefaultTestConfig returns a test configuration with default values from environment
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		PostgresHost:     getEnvOrDefault("TEST_POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnvOrDefaultInt32("TEST_POSTGRES_PORT", 5432),
		PostgresUser:     getEnvOrDefault("TEST_POSTGRES_USER", "postgres"),
		PostgresPassword: getEnvOrDefault("TEST_POSTGRES_PASSWORD", "postgres"),
		PostgresSSLMode:  getEnvOrDefault("TEST_POSTGRES_SSLMODE", "disable"),
		VaultAddr:        getEnvOrDefault("TEST_VAULT_ADDR", "http://localhost:8200"),
		VaultToken:       getEnvOrDefault("TEST_VAULT_TOKEN", "myroot"),
		VaultMountPoint:  getEnvOrDefault("TEST_VAULT_MOUNT_POINT", "secret"),
		VaultSecretPath:  getEnvOrDefault("TEST_VAULT_SECRET_PATH", "pdb"),
		TestTimeout:      getDurationOrDefault("TEST_TIMEOUT", 60*time.Second),
		CleanupTimeout:   getDurationOrDefault("TEST_CLEANUP_TIMEOUT", 30*time.Second),
		RetryInterval:    getDurationOrDefault("TEST_RETRY_INTERVAL", 2*time.Second),
		MaxRetries:       getEnvOrDefaultInt("TEST_MAX_RETRIES", 10),
		TestNamespace:    getEnvOrDefault("TEST_NAMESPACE", "default"),
		PostgresqlID:     getEnvOrDefault("TEST_POSTGRESQL_ID", "test-pg-001"),
	}
}

// TestEnvironment provides a complete test environment for functional tests
type TestEnvironment struct {
	Config      *TestConfig
	Client      client.Client
	Scheme      *runtime.Scheme
	Logger      *zap.SugaredLogger
	VaultClient *api.Client
	PostgresDB  *sql.DB
	Ctx         context.Context
	Cancel      context.CancelFunc
}

// NewTestEnvironment creates a new test environment
func NewTestEnvironment(cfg *TestConfig) (*TestEnvironment, error) {
	if cfg == nil {
		cfg = DefaultTestConfig()
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.TestTimeout)

	// Create logger
	logger := zap.NewNop().Sugar()

	// Create scheme
	scheme := runtime.NewScheme()
	if err := instancev1alpha1.AddToScheme(scheme); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to add scheme: %w", err)
	}

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&instancev1alpha1.Postgresql{}).
		WithStatusSubresource(&instancev1alpha1.User{}).
		WithStatusSubresource(&instancev1alpha1.Database{}).
		WithStatusSubresource(&instancev1alpha1.Grant{}).
		WithStatusSubresource(&instancev1alpha1.RoleGroup{}).
		WithStatusSubresource(&instancev1alpha1.Schema{}).
		Build()

	env := &TestEnvironment{
		Config: cfg,
		Client: fakeClient,
		Scheme: scheme,
		Logger: logger,
		Ctx:    ctx,
		Cancel: cancel,
	}

	return env, nil
}

// SetupPostgresConnection establishes a connection to PostgreSQL
func (e *TestEnvironment) SetupPostgresConnection() error {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s connect_timeout=10",
		e.Config.PostgresHost,
		e.Config.PostgresPort,
		e.Config.PostgresUser,
		e.Config.PostgresPassword,
		e.Config.PostgresSSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open postgres connection: %w", err)
	}

	// Test connection
	if err := db.PingContext(e.Ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping postgres: %w", err)
	}

	e.PostgresDB = db
	return nil
}

// SetupVaultConnection establishes a connection to Vault
func (e *TestEnvironment) SetupVaultConnection() error {
	config := api.DefaultConfig()
	config.Address = e.Config.VaultAddr

	client, err := api.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create vault client: %w", err)
	}

	client.SetToken(e.Config.VaultToken)

	// Test connection
	_, err = client.Sys().Health()
	if err != nil {
		return fmt.Errorf("failed to check vault health: %w", err)
	}

	e.VaultClient = client
	return nil
}

// Cleanup cleans up test resources
func (e *TestEnvironment) Cleanup() {
	if e.PostgresDB != nil {
		e.PostgresDB.Close()
	}
	if e.Cancel != nil {
		e.Cancel()
	}
}

// CreatePostgresqlCRD creates a Postgresql CRD in the fake client
func (e *TestEnvironment) CreatePostgresqlCRD(name, namespace, postgresqlID string, connected bool) (*instancev1alpha1.Postgresql, error) {
	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: postgresqlID,
				Address:      e.Config.PostgresHost,
				Port:         e.Config.PostgresPort,
				SSLMode:      e.Config.PostgresSSLMode,
			},
		},
	}

	if err := e.Client.Create(e.Ctx, postgresql); err != nil {
		return nil, fmt.Errorf("failed to create postgresql: %w", err)
	}

	// Update status
	postgresql.Status.Connected = connected
	postgresql.Status.ConnectionAddress = fmt.Sprintf("%s:%d", e.Config.PostgresHost, e.Config.PostgresPort)
	now := metav1.Now()
	postgresql.Status.LastConnectionAttempt = &now

	if err := e.Client.Status().Update(e.Ctx, postgresql); err != nil {
		return nil, fmt.Errorf("failed to update postgresql status: %w", err)
	}

	return postgresql, nil
}

// StoreVaultCredentials stores credentials in Vault for testing
func (e *TestEnvironment) StoreVaultCredentials(postgresqlID, username, password string) error {
	if e.VaultClient == nil {
		return fmt.Errorf("vault client not initialized")
	}

	path := fmt.Sprintf("%s/data/%s/%s", e.Config.VaultMountPoint, e.Config.VaultSecretPath, postgresqlID)
	if username == "admin" {
		// Store admin credentials
		data := map[string]interface{}{
			"data": map[string]interface{}{
				"admin_username": username,
				"admin_password": password,
			},
		}
		_, err := e.VaultClient.Logical().Write(path+"/admin", data)
		return err
	}

	// Store user credentials
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"password": password,
		},
	}
	_, err := e.VaultClient.Logical().Write(path+"/"+username, data)
	return err
}

// CleanupPostgresUser removes a user from PostgreSQL
func (e *TestEnvironment) CleanupPostgresUser(username string) error {
	if e.PostgresDB == nil {
		return fmt.Errorf("postgres connection not initialized")
	}

	// Drop user if exists
	query := fmt.Sprintf("DROP USER IF EXISTS %q", username)
	_, err := e.PostgresDB.ExecContext(e.Ctx, query)
	return err
}

// CleanupPostgresDatabase removes a database from PostgreSQL
func (e *TestEnvironment) CleanupPostgresDatabase(dbName string) error {
	if e.PostgresDB == nil {
		return fmt.Errorf("postgres connection not initialized")
	}

	// Terminate connections to the database
	terminateQuery := fmt.Sprintf(`
		SELECT pg_terminate_backend(pid) 
		FROM pg_stat_activity 
		WHERE datname = '%s' AND pid <> pg_backend_pid()
	`, dbName)
	_, _ = e.PostgresDB.ExecContext(e.Ctx, terminateQuery)

	// Drop database if exists
	query := fmt.Sprintf("DROP DATABASE IF EXISTS %q", dbName)
	_, err := e.PostgresDB.ExecContext(e.Ctx, query)
	return err
}

// CleanupPostgresRole removes a role from PostgreSQL
func (e *TestEnvironment) CleanupPostgresRole(roleName string) error {
	if e.PostgresDB == nil {
		return fmt.Errorf("postgres connection not initialized")
	}

	// Drop role if exists
	query := fmt.Sprintf("DROP ROLE IF EXISTS %q", roleName)
	_, err := e.PostgresDB.ExecContext(e.Ctx, query)
	return err
}

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}

func getEnvOrDefaultInt32(key string, defaultValue int32) int32 {
	return int32(getEnvOrDefaultInt(key, int(defaultValue)))
}

func getDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

// SetupNopLogger returns a no-op logger for testing
func SetupNopLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

// SetupTestLogger returns a development logger for testing with debug output
func SetupTestLogger() *zap.SugaredLogger {
	logger, _ := zap.NewDevelopment()
	return logger.Sugar()
}
