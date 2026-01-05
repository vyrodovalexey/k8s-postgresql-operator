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
	"go.uber.org/zap"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestDefaultClient_ImplementsInterface(t *testing.T) {
	// This test ensures DefaultClient implements the Client interface
	// If it doesn't, the code won't compile
	var _ Client = (*DefaultClient)(nil)
}

func TestNewDefaultClient(t *testing.T) {
	client := NewDefaultClient()
	assert.NotNil(t, client)
	assert.IsType(t, &DefaultClient{}, client)
}

func TestDefaultClient_CreateOrUpdateDatabase(t *testing.T) {
	client := NewDefaultClient()

	ctx := context.Background()
	err := client.CreateOrUpdateDatabase(
		ctx, "localhost", 5432, "admin", "password", "disable",
		"testdb", "testowner", "", "")

	// Should fail with connection error (expected since we're not connecting to a real DB)
	assert.Error(t, err)
}

func TestDefaultClient_CreateOrUpdateUser(t *testing.T) {
	client := NewDefaultClient()

	ctx := context.Background()
	err := client.CreateOrUpdateUser(
		ctx, "localhost", 5432, "admin", "password", "disable", "testuser", "testpass")

	// Should fail with connection error (expected since we're not connecting to a real DB)
	assert.Error(t, err)
}

func TestDefaultClient_CreateOrUpdateRoleGroup(t *testing.T) {
	client := NewDefaultClient()

	ctx := context.Background()
	err := client.CreateOrUpdateRoleGroup(
		ctx, "localhost", 5432, "admin", "password", "disable", "testgroup", []string{"member1"})

	// Should fail with connection error (expected since we're not connecting to a real DB)
	assert.Error(t, err)
}

func TestDefaultClient_CreateOrUpdateSchema(t *testing.T) {
	client := NewDefaultClient()

	ctx := context.Background()
	err := client.CreateOrUpdateSchema(
		ctx, "localhost", 5432, "admin", "password", "disable", "testdb", "testschema", "testowner")

	// Should fail with connection error (expected since we're not connecting to a real DB)
	assert.Error(t, err)
}

func TestDefaultClient_ApplyGrants(t *testing.T) {
	client := NewDefaultClient()

	ctx := context.Background()
	// ApplyGrants with empty grants returns nil, so we need to test with actual grants
	// or test that the function exists and can be called
	err := client.ApplyGrants(
		ctx, "localhost", 5432, "admin", "password", "disable",
		"testdb", "testrole", []instancev1alpha1.GrantItem{}, []instancev1alpha1.DefaultPrivilegeItem{})

	// With empty grants, ApplyGrants returns nil (no operation needed)
	// This test just verifies the interface method exists and can be called
	assert.NoError(t, err)
}

func TestDefaultClient_DeleteDatabase(t *testing.T) {
	client := NewDefaultClient()

	ctx := context.Background()
	err := client.DeleteDatabase(
		ctx, "localhost", 5432, "admin", "password", "disable", "testdb")

	// Should fail with connection error (expected since we're not connecting to a real DB)
	assert.Error(t, err)
}

func TestDefaultClient_CloseAllSessions(t *testing.T) {
	client := NewDefaultClient()

	ctx := context.Background()
	err := client.CloseAllSessions(
		ctx, "localhost", 5432, "admin", "password", "disable", "testdb")

	// Should fail with connection error (expected since we're not connecting to a real DB)
	assert.Error(t, err)
}

func TestDefaultClient_TestConnection(t *testing.T) {
	client := NewDefaultClient()
	logger := zap.NewNop().Sugar()

	ctx := context.Background()
	connected, err := client.TestConnection(
		ctx, "localhost", 5432, "postgres", "admin", "password", "disable",
		logger, 1, 1*time.Second)

	// Should fail with connection error (expected since we're not connecting to a real DB)
	assert.Error(t, err)
	assert.False(t, connected)
}

func TestDefaultClient_ExecuteOperationWithRetry(t *testing.T) {
	client := NewDefaultClient()
	logger := zap.NewNop().Sugar()

	ctx := context.Background()
	err := client.ExecuteOperationWithRetry(
		ctx, func() error { return nil }, logger, 1, 1*time.Second, "testOperation")

	// Should succeed since operation returns nil
	assert.NoError(t, err)
}

func TestDefaultClient_ExecuteOperationWithRetry_WithError(t *testing.T) {
	client := NewDefaultClient()
	logger := zap.NewNop().Sugar()

	ctx := context.Background()
	testErr := assert.AnError
	err := client.ExecuteOperationWithRetry(
		ctx, func() error { return testErr }, logger, 1, 1*time.Second, "testOperation")

	// Should fail after retries exhausted
	assert.Error(t, err)
}
