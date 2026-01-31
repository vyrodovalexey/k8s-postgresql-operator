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

package vault

import (
	"context"
)

// ClientInterface defines the interface for Vault operations
// This interface abstracts Vault operations to enable dependency injection and testing
type ClientInterface interface {
	// CheckHealth checks if Vault is available and healthy
	CheckHealth(ctx context.Context) error

	// GetPostgresqlCredentials retrieves PostgreSQL admin credentials from Vault KV store
	GetPostgresqlCredentials(ctx context.Context, postgresqlID string) (username, password string, err error)

	// GetPostgresqlUserCredentials retrieves PostgreSQL user credentials from Vault KV store
	GetPostgresqlUserCredentials(ctx context.Context, postgresqlID, username string) (password string, err error)

	// StorePostgresqlUserCredentials stores PostgreSQL user credentials in Vault KV store
	StorePostgresqlUserCredentials(ctx context.Context, postgresqlID, username, password string) error
}

// Ensure Client implements ClientInterface
var _ ClientInterface = (*Client)(nil)
