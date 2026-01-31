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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// ============================================================================
// Tests for actual functions with invalid connections
// These tests cover the initial code paths before database operations
// ============================================================================

// TestDeleteUser_ActualFunctionCoverage tests the actual DeleteUser function
func TestDeleteUser_ActualFunctionCoverage(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		port          int32
		adminUser     string
		adminPassword string
		sslMode       string
		username      string
		expectError   bool
		errorContains string
	}{
		{
			name:          "Invalid host - connection fails",
			host:          "invalid-host-that-does-not-exist-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			username:      "testuser",
			expectError:   true,
			errorContains: "checkUserExists",
		},
		{
			name:          "Empty host - connection fails",
			host:          "",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			username:      "testuser",
			expectError:   true,
		},
		{
			name:          "Invalid port - connection fails",
			host:          "localhost",
			port:          59999,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			username:      "testuser",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := DeleteUser(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode, tt.username)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCreateOrUpdateDatabase_ActualFunctionCoverage tests the actual CreateOrUpdateDatabase function
func TestCreateOrUpdateDatabase_ActualFunctionCoverage(t *testing.T) {
	tests := []struct {
		name             string
		host             string
		port             int32
		adminUser        string
		adminPassword    string
		sslMode          string
		databaseName     string
		owner            string
		schemaName       string
		templateDatabase string
		expectError      bool
		errorContains    string
	}{
		{
			name:          "Invalid host - connection fails",
			host:          "invalid-host-that-does-not-exist-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			owner:         "testowner",
			expectError:   true,
			errorContains: "checkDatabaseExists",
		},
		{
			name:          "Invalid port - connection fails",
			host:          "localhost",
			port:          59999,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			owner:         "testowner",
			expectError:   true,
		},
		{
			name:             "With template database - connection fails",
			host:             "invalid-host-12345",
			port:             5432,
			adminUser:        "admin",
			adminPassword:    "password",
			sslMode:          "disable",
			databaseName:     "testdb",
			owner:            "testowner",
			templateDatabase: "template1",
			expectError:      true,
		},
		{
			name:          "With schema - connection fails",
			host:          "invalid-host-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			owner:         "testowner",
			schemaName:    "myschema",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := CreateOrUpdateDatabase(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode,
				tt.databaseName, tt.owner, tt.schemaName, tt.templateDatabase)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCreateOrUpdateUser_ActualFunctionCoverage tests the actual CreateOrUpdateUser function
func TestCreateOrUpdateUser_ActualFunctionCoverage(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		port          int32
		adminUser     string
		adminPassword string
		sslMode       string
		username      string
		password      string
		expectError   bool
		errorContains string
	}{
		{
			name:          "Invalid host - connection fails",
			host:          "invalid-host-that-does-not-exist-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			username:      "testuser",
			password:      "testpass",
			expectError:   true,
			errorContains: "checkUserExists",
		},
		{
			name:          "Invalid port - connection fails",
			host:          "localhost",
			port:          59999,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			username:      "testuser",
			password:      "testpass",
			expectError:   true,
		},
		{
			name:          "Special characters in password",
			host:          "invalid-host-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			username:      "testuser",
			password:      "p@ss'word\"with$pecial",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := CreateOrUpdateUser(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode, tt.username, tt.password)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCreateOrUpdateRoleGroup_ActualFunctionCoverage tests the actual CreateOrUpdateRoleGroup function
func TestCreateOrUpdateRoleGroup_ActualFunctionCoverage(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		port          int32
		adminUser     string
		adminPassword string
		sslMode       string
		groupRole     string
		memberRoles   []string
		expectError   bool
		errorContains string
	}{
		{
			name:          "Invalid host - connection fails",
			host:          "invalid-host-that-does-not-exist-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			groupRole:     "testgroup",
			memberRoles:   []string{"member1"},
			expectError:   true,
			errorContains: "checkGroupRoleExists",
		},
		{
			name:          "Invalid port - connection fails",
			host:          "localhost",
			port:          59999,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			groupRole:     "testgroup",
			memberRoles:   []string{"member1", "member2"},
			expectError:   true,
		},
		{
			name:          "Empty member roles",
			host:          "invalid-host-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			groupRole:     "testgroup",
			memberRoles:   []string{},
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := CreateOrUpdateRoleGroup(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode, tt.groupRole, tt.memberRoles)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCreateOrUpdateSchema_ActualFunctionCoverage tests the actual CreateOrUpdateSchema function
func TestCreateOrUpdateSchema_ActualFunctionCoverage(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		port          int32
		adminUser     string
		adminPassword string
		sslMode       string
		databaseName  string
		schemaName    string
		owner         string
		expectError   bool
		errorContains string
	}{
		{
			name:          "Invalid host - connection fails",
			host:          "invalid-host-that-does-not-exist-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			schemaName:    "testschema",
			owner:         "testowner",
			expectError:   true,
			errorContains: "failed to check if schema exists",
		},
		{
			name:          "Invalid port - connection fails",
			host:          "localhost",
			port:          59999,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			schemaName:    "testschema",
			owner:         "testowner",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := CreateOrUpdateSchema(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode, tt.databaseName, tt.schemaName, tt.owner)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestApplyGrants_ActualFunctionCoverage tests the actual ApplyGrants function
func TestApplyGrants_ActualFunctionCoverage(t *testing.T) {
	tests := []struct {
		name              string
		host              string
		port              int32
		adminUser         string
		adminPassword     string
		sslMode           string
		databaseName      string
		roleName          string
		grants            []instancev1alpha1.GrantItem
		defaultPrivileges []instancev1alpha1.DefaultPrivilegeItem
		expectError       bool
		errorContains     string
	}{
		{
			name:          "Invalid privilege - validation fails",
			host:          "invalid-host",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			roleName:      "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeDatabase,
					Privileges: []string{"INVALID_PRIVILEGE"},
				},
			},
			expectError:   true,
			errorContains: "invalid privilege",
		},
		{
			name:          "Invalid default privilege - validation fails",
			host:          "invalid-host",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			roleName:      "testrole",
			defaultPrivileges: []instancev1alpha1.DefaultPrivilegeItem{
				{
					Schema:     "public",
					ObjectType: "tables",
					Privileges: []string{"INVALID_PRIVILEGE"},
				},
			},
			expectError:   true,
			errorContains: "invalid privilege",
		},
		{
			name:          "Empty grants - no connection needed",
			host:          "invalid-host",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			roleName:      "testrole",
			grants:        []instancev1alpha1.GrantItem{},
			expectError:   false,
		},
		{
			name:          "Database grant - connection fails",
			host:          "invalid-host-that-does-not-exist-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			roleName:      "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeDatabase,
					Privileges: []string{"CONNECT"},
				},
			},
			expectError:   true,
			errorContains: "failed to ping postgres database",
		},
		{
			name:          "Schema grant - connection fails",
			host:          "invalid-host-that-does-not-exist-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			roleName:      "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeSchema,
					Schema:     "public",
					Privileges: []string{"USAGE"},
				},
			},
			expectError:   true,
			errorContains: "failed to ping database",
		},
		{
			name:          "Default privileges - connection fails",
			host:          "invalid-host-that-does-not-exist-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			roleName:      "testrole",
			defaultPrivileges: []instancev1alpha1.DefaultPrivilegeItem{
				{
					Schema:     "public",
					ObjectType: "tables",
					Privileges: []string{"SELECT"},
				},
			},
			expectError:   true,
			errorContains: "failed to ping database",
		},
		{
			name:          "Multiple grant types",
			host:          "invalid-host-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			roleName:      "testrole",
			grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeDatabase,
					Privileges: []string{"CONNECT", "CREATE"},
				},
				{
					Type:       instancev1alpha1.GrantTypeSchema,
					Schema:     "public",
					Privileges: []string{"USAGE"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := ApplyGrants(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode,
				tt.databaseName, tt.roleName, tt.grants, tt.defaultPrivileges)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestDeleteDatabase_ActualFunctionCoverage tests the actual DeleteDatabase function
func TestDeleteDatabase_ActualFunctionCoverage(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		port          int32
		adminUser     string
		adminPassword string
		sslMode       string
		databaseName  string
		expectError   bool
		errorContains string
	}{
		{
			name:          "Invalid host - CloseAllSessions fails",
			host:          "invalid-host-that-does-not-exist-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			expectError:   true,
			errorContains: "failed to close sessions",
		},
		{
			name:          "Invalid port - connection fails",
			host:          "localhost",
			port:          59999,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := DeleteDatabase(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode, tt.databaseName)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCloseAllSessions_ActualFunctionCoverage tests the actual CloseAllSessions function
func TestCloseAllSessions_ActualFunctionCoverage(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		port          int32
		adminUser     string
		adminPassword string
		sslMode       string
		databaseName  string
		expectError   bool
		errorContains string
	}{
		{
			name:          "Invalid host - connection fails",
			host:          "invalid-host-that-does-not-exist-12345",
			port:          5432,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			expectError:   true,
			errorContains: "failed to terminate connections",
		},
		{
			name:          "Invalid port - connection fails",
			host:          "localhost",
			port:          59999,
			adminUser:     "admin",
			adminPassword: "password",
			sslMode:       "disable",
			databaseName:  "testdb",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := CloseAllSessions(ctx, tt.host, tt.port, tt.adminUser, tt.adminPassword, tt.sslMode, tt.databaseName)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ============================================================================
// Context cancellation tests for actual functions
// ============================================================================

func TestDeleteUser_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := DeleteUser(ctx, "localhost", 5432, "admin", "password", "disable", "testuser")
	assert.Error(t, err)
}

func TestCreateOrUpdateDatabase_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := CreateOrUpdateDatabase(ctx, "localhost", 5432, "admin", "password", "disable", "testdb", "owner", "", "")
	assert.Error(t, err)
}

func TestCreateOrUpdateUser_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := CreateOrUpdateUser(ctx, "localhost", 5432, "admin", "password", "disable", "testuser", "testpass")
	assert.Error(t, err)
}

func TestCreateOrUpdateRoleGroup_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := CreateOrUpdateRoleGroup(ctx, "localhost", 5432, "admin", "password", "disable", "testgroup", []string{"member1"})
	assert.Error(t, err)
}

func TestCreateOrUpdateSchema_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := CreateOrUpdateSchema(ctx, "localhost", 5432, "admin", "password", "disable", "testdb", "testschema", "owner")
	assert.Error(t, err)
}

func TestApplyGrants_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	grants := []instancev1alpha1.GrantItem{
		{
			Type:       instancev1alpha1.GrantTypeDatabase,
			Privileges: []string{"CONNECT"},
		},
	}

	err := ApplyGrants(ctx, "localhost", 5432, "admin", "password", "disable", "testdb", "testrole", grants, nil)
	assert.Error(t, err)
}

func TestDeleteDatabase_ContextCancellationActual(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := DeleteDatabase(ctx, "localhost", 5432, "admin", "password", "disable", "testdb")
	assert.Error(t, err)
}

func TestCloseAllSessions_ContextCancellationActual(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := CloseAllSessions(ctx, "localhost", 5432, "admin", "password", "disable", "testdb")
	assert.Error(t, err)
}
