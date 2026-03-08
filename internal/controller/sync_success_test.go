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

package controller

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	// Import lib/pq driver for sql.Open
	_ "github.com/lib/pq"
)

// TestMockPGServer_BasicConnection verifies the mock PG server can accept
// connections from the lib/pq driver.
func TestMockPGServer_BasicConnection(t *testing.T) {
	srv := newMockPGServer(t, false)
	defer srv.close()

	connStr := fmt.Sprintf(
		"host=127.0.0.1 port=%d user=admin password=secret dbname=postgres sslmode=disable connect_timeout=5",
		srv.port)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()

	err = db.Ping()
	assert.NoError(t, err, "Ping should succeed against mock PG server")
}

// TestMockPGServer_SimpleQuery verifies simple queries work.
func TestMockPGServer_SimpleQuery(t *testing.T) {
	srv := newMockPGServer(t, false)
	defer srv.close()

	connStr := fmt.Sprintf(
		"host=127.0.0.1 port=%d user=admin password=secret dbname=postgres sslmode=disable connect_timeout=5",
		srv.port)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()

	// Test a CREATE statement (simple query)
	_, err = db.Exec("CREATE DATABASE testdb OWNER testowner")
	assert.NoError(t, err, "CREATE DATABASE should succeed")
}

// TestMockPGServer_ParameterizedQuery verifies parameterized queries work
// (extended query protocol used by lib/pq for QueryRow with $1 params).
func TestMockPGServer_ParameterizedQuery(t *testing.T) {
	srv := newMockPGServer(t, false)
	defer srv.close()

	connStr := fmt.Sprintf(
		"host=127.0.0.1 port=%d user=admin password=secret dbname=postgres sslmode=disable connect_timeout=5",
		srv.port)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()

	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", "testdb").Scan(&exists)
	assert.NoError(t, err, "Parameterized query should succeed")
	assert.False(t, exists, "defaultSuccess=false should return false")
}

// TestMockPGServer_ParameterizedQuery_DefaultTrue verifies parameterized queries
// return true when defaultSuccess=true.
func TestMockPGServer_ParameterizedQuery_DefaultTrue(t *testing.T) {
	srv := newMockPGServer(t, true)
	defer srv.close()

	connStr := fmt.Sprintf(
		"host=127.0.0.1 port=%d user=admin password=secret dbname=postgres sslmode=disable connect_timeout=5",
		srv.port)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()

	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", "testdb").Scan(&exists)
	assert.NoError(t, err, "Parameterized query should succeed")
	assert.True(t, exists, "defaultSuccess=true should return true")
}
