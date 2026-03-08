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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/kubernetes"
)

// ErrSecretNotFound indicates the secret path does not exist in Vault
var ErrSecretNotFound = fmt.Errorf("secret not found in Vault")

// IsSecretNotFound checks if the error indicates a missing secret
func IsSecretNotFound(err error) bool {
	return errors.Is(err, ErrSecretNotFound)
}

// isVaultNotFoundError checks if the error from Vault indicates a 404 (secret not found)
func isVaultNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var respErr *api.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == http.StatusNotFound
	}
	// Fallback: check error message for common "not found" patterns
	errMsg := err.Error()
	return strings.Contains(errMsg, "secret not found")
}

// Client wraps the Vault API client
type Client struct {
	client          *api.Client
	vaultMountPoint string
	vaultSecretPath string
}

// NewClient creates a new Vault client using Kubernetes authentication
// VAULT_ADDR: Vault server address (required)
// VAULT_ROLE: Vault role for Kubernetes authentication (required)
// VAULT_TOKEN_PATH: Path to Kubernetes service account token file (optional, defaults to standard path)
func NewClient(
	ctx context.Context, vaultAddr, vaultRole, tokenPath, vaultMountPoint, vaultSecretPath string,
) (*Client, error) {
	if vaultRole == "" {
		return nil, fmt.Errorf("environment variable VAULT_ROLE is not set (required for Kubernetes auth)")
	}

	config := api.DefaultConfig()
	config.Address = vaultAddr

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
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
	authInfo, err := client.Auth().Login(ctx, k8sAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with Vault using Kubernetes auth: %w", err)
	}

	if authInfo == nil {
		return nil, fmt.Errorf("authentication returned empty auth info")
	}

	return &Client{client: client, vaultMountPoint: vaultMountPoint, vaultSecretPath: vaultSecretPath}, nil
}

// CheckHealth checks if Vault is available and healthy
func (c *Client) CheckHealth(ctx context.Context) error {
	health, err := c.client.Sys().Health()
	if err != nil {
		return fmt.Errorf("failed to check Vault health: %w", err)
	}

	if health == nil {
		return fmt.Errorf("vault health check returned nil")
	}

	// Health check is successful if we get a response (even if sealed)
	// Being sealed is a different state than being unavailable
	return nil
}

// GetPostgresqlCredentials retrieves PostgreSQL instance admin credentials from Vault KV store
func (c *Client) GetPostgresqlCredentials(
	ctx context.Context, postgresqlID string) (login, password string, err error) {
	kv2Path := fmt.Sprintf("%s/%s/instance_admin", c.vaultSecretPath, postgresqlID)

	secret, err := c.client.KVv2(c.vaultMountPoint).Get(ctx, kv2Path)
	if err != nil {
		if isVaultNotFoundError(err) {
			return "", "", fmt.Errorf("instance admin credentials at path %s: %w", kv2Path, ErrSecretNotFound)
		}
		return "", "", fmt.Errorf("failed to read secret from Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return "", "", fmt.Errorf("instance admin credentials at path %s: %w", kv2Path, ErrSecretNotFound)
	}

	if u, ok := secret.Data["login"].(string); ok {
		login = u
	}
	if p, ok := secret.Data["password"].(string); ok {
		password = p
	}

	if login == "" || password == "" {
		return "", "", fmt.Errorf("credentials not found in secret at path: %s", kv2Path)
	}

	return login, password, nil
}

// GetDefaultCredentials retrieves default PostgreSQL credentials from Vault KV store
// These are used as fallback when instance-specific credentials don't exist
func (c *Client) GetDefaultCredentials(ctx context.Context) (login, password string, err error) {
	kv2Path := fmt.Sprintf("%s/default", c.vaultSecretPath)

	secret, err := c.client.KVv2(c.vaultMountPoint).Get(ctx, kv2Path)
	if err != nil {
		if isVaultNotFoundError(err) {
			return "", "", fmt.Errorf("default credentials at path %s: %w", kv2Path, ErrSecretNotFound)
		}
		return "", "", fmt.Errorf("failed to read default credentials from Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return "", "", fmt.Errorf("default credentials at path %s: %w", kv2Path, ErrSecretNotFound)
	}

	if u, ok := secret.Data["login"].(string); ok {
		login = u
	}
	if p, ok := secret.Data["password"].(string); ok {
		password = p
	}

	if login == "" || password == "" {
		return "", "", fmt.Errorf("credentials not found in default secret at path: %s", kv2Path)
	}

	return login, password, nil
}

// StorePostgresqlCredentials stores PostgreSQL instance admin credentials in Vault KV store
func (c *Client) StorePostgresqlCredentials(ctx context.Context, postgresqlID, login, password string) error {
	data := map[string]interface{}{
		"login":    login,
		"password": password,
	}

	kv2Path := fmt.Sprintf("%s/%s/instance_admin", c.vaultSecretPath, postgresqlID)
	_, err := c.client.KVv2(c.vaultMountPoint).Put(ctx, kv2Path, data)
	if err != nil {
		return fmt.Errorf("failed to store instance admin credentials in Vault: %w", err)
	}

	return nil
}

// GetInstanceAdminNewPassword retrieves the new_password field from instance admin secret
// Returns empty string and nil error if the field doesn't exist (no rotation needed)
// Returns error only if Vault is unavailable or the secret itself doesn't exist
func (c *Client) GetInstanceAdminNewPassword(ctx context.Context, postgresqlID string) (string, error) {
	kv2Path := fmt.Sprintf("%s/%s/instance_admin", c.vaultSecretPath, postgresqlID)

	secret, err := c.client.KVv2(c.vaultMountPoint).Get(ctx, kv2Path)
	if err != nil {
		if isVaultNotFoundError(err) {
			return "", fmt.Errorf("instance admin secret at path %s: %w", kv2Path, ErrSecretNotFound)
		}
		return "", fmt.Errorf("failed to read instance admin secret from Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("instance admin secret at path %s: %w", kv2Path, ErrSecretNotFound)
	}

	// new_password is optional - return empty string if not present
	if newPass, ok := secret.Data["new_password"].(string); ok && newPass != "" {
		return newPass, nil
	}

	return "", nil
}

// RotateInstanceAdminPassword atomically rotates the instance admin password in Vault
// It reads the current login, then writes login + new password (removing new_password field)
func (c *Client) RotateInstanceAdminPassword(ctx context.Context, postgresqlID, newPassword string) error {
	kv2Path := fmt.Sprintf("%s/%s/instance_admin", c.vaultSecretPath, postgresqlID)

	// Read current secret to preserve the login
	secret, err := c.client.KVv2(c.vaultMountPoint).Get(ctx, kv2Path)
	if err != nil {
		return fmt.Errorf("failed to read instance admin secret for rotation: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return fmt.Errorf("instance admin secret not found for rotation at path: %s", kv2Path)
	}

	login, ok := secret.Data["login"].(string)
	if !ok || login == "" {
		return fmt.Errorf("login not found in instance admin secret at path: %s", kv2Path)
	}

	// Write back with updated password and no new_password field
	data := map[string]interface{}{
		"login":    login,
		"password": newPassword,
	}

	_, err = c.client.KVv2(c.vaultMountPoint).Put(ctx, kv2Path, data)
	if err != nil {
		return fmt.Errorf("failed to rotate instance admin password in Vault: %w", err)
	}

	return nil
}

// GetPostgresqlUserCredentials retrieves PostgreSQL credentials from Vault KV store
func (c *Client) GetPostgresqlUserCredentials(
	ctx context.Context, postgresqlID, username string) (password string, err error) {
	kv2Path := fmt.Sprintf("%s/%s/%s", c.vaultSecretPath, postgresqlID, username)

	// Use KVv2 to read the secret
	secret, err := c.client.KVv2(c.vaultMountPoint).Get(ctx, kv2Path)
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

// StorePostgresqlUserCredentials stores PostgreSQL user credentials in Vault KV store
func (c *Client) StorePostgresqlUserCredentials(ctx context.Context, postgresqlID, username, password string) error {
	// Prepare the data to store
	data := map[string]interface{}{
		"password": password,
	}

	// Write to KV v2 (if using KV v2, the path should be secret/data/{postgresqlID})
	// For KV v1, use the path as-is
	// Try KV v2 first (most common)
	kv2Path := fmt.Sprintf("%s/%s/%s", c.vaultSecretPath, postgresqlID, username)
	_, err := c.client.KVv2(c.vaultMountPoint).Put(ctx, kv2Path, data)

	if err != nil {
		// If KV v2 fails, try KV v1
		return err
	}

	return nil
}

// PKICertificate holds the certificate data returned by Vault PKI
type PKICertificate struct {
	Certificate  string
	PrivateKey   string
	CAChain      []string
	SerialNumber string
	Expiration   int64
}

// IssueCertificate issues a new TLS certificate from Vault PKI
func (c *Client) IssueCertificate(
	ctx context.Context, mountPath, roleName, commonName string,
	altNames []string, ipSANs []string, ttl string,
) (*PKICertificate, error) {
	path := fmt.Sprintf("%s/issue/%s", mountPath, roleName)

	data := map[string]interface{}{
		"common_name": commonName,
		"ttl":         ttl,
	}
	if len(altNames) > 0 {
		data["alt_names"] = strings.Join(altNames, ",")
	}
	if len(ipSANs) > 0 {
		data["ip_sans"] = strings.Join(ipSANs, ",")
	}

	secret, err := c.client.Logical().WriteWithContext(ctx, path, data)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to issue certificate from Vault PKI: %w", err,
		)
	}
	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf(
			"empty response from Vault PKI at path: %s", path,
		)
	}

	return parsePKICertificateResponse(secret.Data, path)
}

// parsePKICertificateResponse parses the Vault PKI response data
func parsePKICertificateResponse(
	data map[string]interface{}, path string,
) (*PKICertificate, error) {
	result := &PKICertificate{}

	certVal, ok := data["certificate"].(string)
	if !ok || certVal == "" {
		return nil, fmt.Errorf(
			"certificate not found in Vault PKI response at: %s", path,
		)
	}
	result.Certificate = certVal

	keyVal, ok := data["private_key"].(string)
	if !ok || keyVal == "" {
		return nil, fmt.Errorf(
			"private_key not found in Vault PKI response at: %s", path,
		)
	}
	result.PrivateKey = keyVal

	if serial, ok := data["serial_number"].(string); ok {
		result.SerialNumber = serial
	}

	switch exp := data["expiration"].(type) {
	case json.Number:
		if v, err := exp.Int64(); err == nil {
			result.Expiration = v
		}
	case float64:
		result.Expiration = int64(exp)
	}

	if chain, ok := data["ca_chain"].([]interface{}); ok {
		for _, c := range chain {
			if s, ok := c.(string); ok {
				result.CAChain = append(result.CAChain, s)
			}
		}
	}

	return result, nil
}

// GetCACertificate retrieves the CA certificate from Vault PKI
func (c *Client) GetCACertificate(
	ctx context.Context, mountPath string,
) (string, error) {
	path := fmt.Sprintf("%s/cert/ca", mountPath)

	secret, err := c.client.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return "", fmt.Errorf(
			"failed to read CA certificate from Vault PKI: %w", err,
		)
	}
	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf(
			"empty CA response from Vault PKI at path: %s", path,
		)
	}

	certVal, ok := secret.Data["certificate"].(string)
	if !ok || certVal == "" {
		return "", fmt.Errorf(
			"CA certificate not found in Vault PKI response at: %s",
			path,
		)
	}

	return certVal, nil
}
