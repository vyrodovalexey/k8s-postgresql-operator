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
	"fmt"
	"github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/kubernetes"
	"os"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("vault")

// Client wraps the Vault API client
type Client struct {
	client *api.Client
}

// NewClient creates a new Vault client using Kubernetes authentication
// VAULT_ADDR: Vault server address (required)
// VAULT_ROLE: Vault role for Kubernetes authentication (required)
// VAULT_TOKEN_PATH: Path to Kubernetes service account token file (optional, defaults to standard path)
func NewClient() (*Client, error) {
	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		return nil, fmt.Errorf("VAULT_ADDR environment variable is not set")
	}

	vaultRole := os.Getenv("VAULT_ROLE")
	if vaultRole == "" {
		return nil, fmt.Errorf("VAULT_ROLE environment variable is not set (required for Kubernetes auth)")
	}
	log.Info("Vault ENV", "VAULT_ADDR", vaultAddr, "VAULT_ROLE", vaultRole)
	config := api.DefaultConfig()
	config.Address = vaultAddr

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	// Determine the path to the Kubernetes service account token
	tokenPath := os.Getenv("VAULT_TOKEN_PATH")
	if tokenPath == "" {
		// Default to the standard Kubernetes service account token path
		tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	}

	// Create Kubernetes auth method
	// Use WithServiceAccountTokenPath to read the token from the file
	k8sAuth, err := auth.NewKubernetesAuth(vaultRole,
		auth.WithServiceAccountTokenPath(tokenPath),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes auth method: %w", err)
	}

	// Authenticate with Vault using Kubernetes auth
	authInfo, err := client.Auth().Login(context.Background(), k8sAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with Vault using Kubernetes auth: %w", err)
	}

	if authInfo == nil {
		return nil, fmt.Errorf("authentication returned empty auth info")
	}

	log.Info("Successfully authenticated with Vault using Kubernetes auth", "role", vaultRole)

	return &Client{client: client}, nil
}

// StorePostgresqlCredentials stores PostgreSQL credentials in Vault KV store
// Path: /secret/{postgresqlID}
// Keys: admin_username, admin_password
func (c *Client) StorePostgresqlCredentials(ctx context.Context, postgresqlID, username, password string) error {

	// Prepare the data to store
	data := map[string]interface{}{
		"admin_username": username,
		"admin_password": password,
	}

	// Write to KV v2 (if using KV v2, the path should be secret/data/{postgresqlID})
	// For KV v1, use the path as-is
	// Try KV v2 first (most common)
	kv2Path := fmt.Sprintf("pdb/%s", postgresqlID)
	_, err := c.client.KVv2("secret").Put(context.Background(), kv2Path, data)

	if err != nil {
		// If KV v2 fails, try KV v1
		log.Info("KV v2 write failed", "error", err, "path", kv2Path)
	} else {
		log.Info("Credentials stored in Vault KV v2", "path", kv2Path)
	}

	return nil
}

// GetPostgresqlCredentials retrieves PostgreSQL credentials from Vault KV store
func (c *Client) GetPostgresqlCredentials(ctx context.Context, postgresqlID string) (username, password string, err error) {
	kv2Path := fmt.Sprintf("pdb/%s/admin", postgresqlID)

	// Use KVv2 to read the secret
	secret, err := c.client.KVv2("secret").Get(ctx, kv2Path)
	if err != nil {
		return "", "", fmt.Errorf("failed to read secret from Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return "", "", fmt.Errorf("secret not found at path: %s", kv2Path)
	}

	if u, ok := secret.Data["admin_username"].(string); ok {
		username = u
	}
	if p, ok := secret.Data["admin_password"].(string); ok {
		password = p
	}

	if username == "" || password == "" {
		return "", "", fmt.Errorf("credentials not found in secret at path: %s", kv2Path)
	}

	return username, password, nil
}

// GetPostgresqlUserCredentials retrieves PostgreSQL credentials from Vault KV store
func (c *Client) GetPostgresqlUserCredentials(ctx context.Context, postgresqlID, username string) (password string, err error) {
	kv2Path := fmt.Sprintf("pdb/%s/%s", postgresqlID, username)

	// Use KVv2 to read the secret
	secret, err := c.client.KVv2("secret").Get(ctx, kv2Path)
	if err != nil {
		return "", fmt.Errorf("failed to read secret from Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("secret not found at path: %s", kv2Path)
	}

	if p, ok := secret.Data["password"].(string); ok {
		password = p
	}

	if password == "" {
		return "", fmt.Errorf("credentials not found in secret at path: %s", kv2Path)
	}

	return password, nil
}

// StorePostgresqlUserCredentials
func (c *Client) StorePostgresqlUserCredentials(ctx context.Context, postgresqlID, username, password string) error {

	// Prepare the data to store
	data := map[string]interface{}{
		"password": password,
	}

	// Write to KV v2 (if using KV v2, the path should be secret/data/{postgresqlID})
	// For KV v1, use the path as-is
	// Try KV v2 first (most common)
	kv2Path := fmt.Sprintf("pdb/%s/%s", postgresqlID, username)
	_, err := c.client.KVv2("secret").Put(context.Background(), kv2Path, data)

	if err != nil {
		// If KV v2 fails, try KV v1
		log.Info("KV v2 write failed", "error", err, "path", kv2Path)
	} else {
		log.Info("Credentials stored in Vault KV v2", "path", kv2Path)
	}

	return nil
}
