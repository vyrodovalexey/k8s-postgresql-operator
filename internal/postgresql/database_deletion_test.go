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

	"github.com/stretchr/testify/assert"
)

func TestCloseAllSessions_InvalidConnection(t *testing.T) {
	ctx := context.Background()

	// Test with invalid connection parameters
	err := CloseAllSessions(ctx, "invalid-host", 5432, "user", "password", "require", "testdb")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to")
}

func TestDeleteDatabase_InvalidConnection(t *testing.T) {
	ctx := context.Background()

	// Test with invalid connection parameters
	err := DeleteDatabase(ctx, "invalid-host", 5432, "user", "password", "require", "testdb")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to")
}

func TestDeleteDatabase_ClosesSessionsFirst(t *testing.T) {
	ctx := context.Background()

	// This test verifies that DeleteDatabase calls CloseAllSessions first
	// Since we can't easily test the actual database operations without a real DB,
	// we test that the function structure is correct
	err := DeleteDatabase(ctx, "invalid-host", 5432, "user", "password", "require", "testdb")

	// Should fail at CloseAllSessions step
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to close sessions")
}
