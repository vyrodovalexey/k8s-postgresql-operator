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

package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// VaultTestClient wraps an HTTP client for Vault token-based authentication.
// This is used in functional and integration tests where K8s auth is not available.
type VaultTestClient struct {
	Addr       string
	Token      string
	HTTPClient *http.Client
}

// NewVaultTestClient creates a new VaultTestClient with the given address and token.
func NewVaultTestClient(addr, token string) *VaultTestClient {
	return &VaultTestClient{
		Addr:  addr,
		Token: token,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// vaultResponse represents a generic Vault API response.
type vaultResponse struct {
	Data      json.RawMessage `json:"data"`
	Errors    []string        `json:"errors,omitempty"`
	RequestID string          `json:"request_id"`
	LeaseID   string          `json:"lease_id"`
	Renewable bool            `json:"renewable"`
	LeaseDur  int             `json:"lease_duration"`
	WrapInfo  interface{}     `json:"wrap_info"`
	Warnings  []string        `json:"warnings"`
	Auth      interface{}     `json:"auth"`
	MountType string          `json:"mount_type"`
}

// kvV2Data represents the data wrapper for KV v2 secrets.
type kvV2Data struct {
	Data     map[string]interface{} `json:"data"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CheckHealth checks if Vault is available and healthy.
func (c *VaultTestClient) CheckHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Addr+"/v1/sys/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to check Vault health: %w", err)
	}
	defer resp.Body.Close()

	// Vault returns 200 for initialized, unsealed, active
	// 429 for unsealed, standby
	// 472 for disaster recovery mode replication secondary and target
	// 473 for performance standby
	// 501 for not initialized
	// 503 for sealed
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusTooManyRequests {
		return fmt.Errorf("vault health check returned status %d", resp.StatusCode)
	}

	return nil
}

// WriteSecret writes a secret to Vault KV v2 at the given path.
func (c *VaultTestClient) WriteSecret(ctx context.Context, mountPath, secretPath string, data map[string]interface{}) error {
	payload := map[string]interface{}{
		"data": data,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal secret data: %w", err)
	}

	url := fmt.Sprintf("%s/v1/%s/data/%s", c.Addr, mountPath, secretPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create write request: %w", err)
	}
	req.Header.Set("X-Vault-Token", c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to write secret: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to write secret, status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// ReadSecret reads a secret from Vault KV v2 at the given path.
func (c *VaultTestClient) ReadSecret(ctx context.Context, mountPath, secretPath string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/v1/%s/data/%s", c.Addr, mountPath, secretPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create read request: %w", err)
	}
	req.Header.Set("X-Vault-Token", c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("secret not found at path: %s/%s", mountPath, secretPath)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to read secret, status %d: %s", resp.StatusCode, string(respBody))
	}

	var vaultResp vaultResponse
	if err := json.NewDecoder(resp.Body).Decode(&vaultResp); err != nil {
		return nil, fmt.Errorf("failed to decode vault response: %w", err)
	}

	var kvData kvV2Data
	if err := json.Unmarshal(vaultResp.Data, &kvData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal KV v2 data: %w", err)
	}

	return kvData.Data, nil
}

// DeleteSecret deletes a secret from Vault KV v2 at the given path.
func (c *VaultTestClient) DeleteSecret(ctx context.Context, mountPath, secretPath string) error {
	// Use metadata endpoint for permanent deletion
	url := fmt.Sprintf("%s/v1/%s/metadata/%s", c.Addr, mountPath, secretPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	req.Header.Set("X-Vault-Token", c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete secret, status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// pkiCertResponse represents the response from Vault PKI certificate issuance.
type pkiCertResponse struct {
	Data struct {
		Certificate    string   `json:"certificate"`
		IssuingCA      string   `json:"issuing_ca"`
		PrivateKey     string   `json:"private_key"`
		PrivateKeyType string   `json:"private_key_type"`
		SerialNumber   string   `json:"serial_number"`
		CAChain        []string `json:"ca_chain"`
	} `json:"data"`
	Errors []string `json:"errors,omitempty"`
}

// IssuePKICert issues a certificate from Vault PKI.
func (c *VaultTestClient) IssuePKICert(ctx context.Context, mountPath, role, commonName string) (cert, key, ca string, err error) {
	payload := map[string]interface{}{
		"common_name": commonName,
		"ttl":         "24h",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to marshal PKI request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/%s/issue/%s", c.Addr, mountPath, role)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create PKI issue request: %w", err)
	}
	req.Header.Set("X-Vault-Token", c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to issue PKI certificate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("failed to issue PKI certificate, status %d: %s", resp.StatusCode, string(respBody))
	}

	var pkiResp pkiCertResponse
	if err := json.NewDecoder(resp.Body).Decode(&pkiResp); err != nil {
		return "", "", "", fmt.Errorf("failed to decode PKI response: %w", err)
	}

	if len(pkiResp.Errors) > 0 {
		return "", "", "", fmt.Errorf("PKI errors: %v", pkiResp.Errors)
	}

	return pkiResp.Data.Certificate, pkiResp.Data.PrivateKey, pkiResp.Data.IssuingCA, nil
}

// ReadPKICA reads the CA certificate from Vault PKI.
func (c *VaultTestClient) ReadPKICA(ctx context.Context, mountPath string) (string, error) {
	url := fmt.Sprintf("%s/v1/%s/ca/pem", c.Addr, mountPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create PKI CA request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to read PKI CA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to read PKI CA, status %d: %s", resp.StatusCode, string(respBody))
	}

	caBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read PKI CA body: %w", err)
	}

	return string(caBytes), nil
}

// SetupPKI configures a PKI secrets engine in Vault for testing.
// It enables the PKI engine, generates a root CA, and creates a role.
func (c *VaultTestClient) SetupPKI(ctx context.Context, mountPath, roleName, allowedDomains string) error {
	// Enable PKI secrets engine
	enablePayload := map[string]interface{}{
		"type": "pki",
	}
	body, _ := json.Marshal(enablePayload)
	url := fmt.Sprintf("%s/v1/sys/mounts/%s", c.Addr, mountPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create enable PKI request: %w", err)
	}
	req.Header.Set("X-Vault-Token", c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to enable PKI engine: %w", err)
	}
	resp.Body.Close()
	// Ignore 400 error if already mounted

	// Check if CA already exists by trying to read it
	caURL := fmt.Sprintf("%s/v1/%s/ca/pem", c.Addr, mountPath)
	caReq, err := http.NewRequestWithContext(ctx, http.MethodGet, caURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create CA check request: %w", err)
	}
	caResp, err := c.HTTPClient.Do(caReq)
	if err != nil {
		return fmt.Errorf("failed to check CA: %w", err)
	}
	caBody, _ := io.ReadAll(caResp.Body)
	caResp.Body.Close()

	// Only generate root CA if one doesn't exist yet
	if caResp.StatusCode != http.StatusOK || len(caBody) == 0 {
		rootPayload := map[string]interface{}{
			"common_name": "K8s PostgreSQL Operator CA",
			"ttl":         "87600h",
			"key_type":    "rsa",
			"key_bits":    2048,
		}
		body, _ = json.Marshal(rootPayload)
		url = fmt.Sprintf("%s/v1/%s/root/generate/internal", c.Addr, mountPath)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create root CA request: %w", err)
		}
		req.Header.Set("X-Vault-Token", c.Token)
		req.Header.Set("Content-Type", "application/json")

		resp, err = c.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to generate root CA: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to generate root CA, status %d", resp.StatusCode)
		}
	}

	// Create a role
	rolePayload := map[string]interface{}{
		"allowed_domains":  allowedDomains,
		"allow_subdomains": true,
		"max_ttl":          "72h",
		"key_type":         "rsa",
		"key_bits":         2048,
	}
	body, _ = json.Marshal(rolePayload)
	url = fmt.Sprintf("%s/v1/%s/roles/%s", c.Addr, mountPath, roleName)
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create role request: %w", err)
	}
	req.Header.Set("X-Vault-Token", c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err = c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create PKI role: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create PKI role, status %d", resp.StatusCode)
	}

	return nil
}
