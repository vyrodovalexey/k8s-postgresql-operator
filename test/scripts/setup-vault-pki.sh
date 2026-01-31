#!/bin/bash
# =============================================================================
# setup-vault-pki.sh - PKI Setup for Webhook Certificates
# =============================================================================
# This script configures Vault PKI for webhook TLS certificates:
# - Enables PKI secrets engine
# - Generates root CA
# - Configures PKI role for webhook certificates
# - Creates policy for PKI access
#
# Prerequisites:
# - Vault server running
# - curl and jq installed
#
# Environment Variables:
#   VAULT_ADDR          - Vault server address (default: http://localhost:8200)
#   VAULT_TOKEN         - Vault root token (default: myroot)
#   VAULT_PKI_PATH      - PKI secrets engine mount path (default: pki)
#   VAULT_PKI_ROLE      - PKI role name (default: webhook-cert)
#   VAULT_PKI_TTL       - Certificate TTL (default: 720h = 30 days)
#   VAULT_PKI_MAX_TTL   - Maximum certificate TTL (default: 87600h = 10 years)
#   CA_COMMON_NAME      - CA common name (default: K8s PostgreSQL Operator CA)
#
# Usage:
#   ./setup-vault-pki.sh
#   VAULT_PKI_PATH=pki-operator ./setup-vault-pki.sh
# =============================================================================

set -euo pipefail

# =============================================================================
# Configuration
# =============================================================================

VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"
VAULT_TOKEN="${VAULT_TOKEN:-myroot}"
VAULT_PKI_PATH="${VAULT_PKI_PATH:-pki}"
VAULT_PKI_ROLE="${VAULT_PKI_ROLE:-webhook-cert}"
VAULT_PKI_TTL="${VAULT_PKI_TTL:-720h}"
VAULT_PKI_MAX_TTL="${VAULT_PKI_MAX_TTL:-87600h}"
CA_COMMON_NAME="${CA_COMMON_NAME:-K8s PostgreSQL Operator CA}"
VERBOSE="${VERBOSE:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# =============================================================================
# Helper Functions
# =============================================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_verbose() {
    if [[ "${VERBOSE}" == "true" ]]; then
        echo -e "${BLUE}[DEBUG]${NC} $1"
    fi
}

# Make a Vault API request
vault_request() {
    local method="$1"
    local path="$2"
    local data="${3:-}"
    
    local url="${VAULT_ADDR}/v1/${path}"
    local args=(-s -X "${method}" -H "X-Vault-Token: ${VAULT_TOKEN}")
    
    if [[ -n "${data}" ]]; then
        args+=(-H "Content-Type: application/json" -d "${data}")
    fi
    
    log_verbose "Vault request: ${method} ${url}"
    curl "${args[@]}" "${url}"
}

# Check if Vault is ready
check_vault_ready() {
    log_info "Checking Vault availability at ${VAULT_ADDR}..."
    
    local max_attempts=30
    local attempt=1
    
    while [[ $attempt -le $max_attempts ]]; do
        local health
        health=$(curl -s "${VAULT_ADDR}/v1/sys/health" 2>/dev/null || echo "{}")
        
        if echo "$health" | grep -q '"initialized":true' && echo "$health" | grep -q '"sealed":false'; then
            log_success "Vault is ready and unsealed"
            return 0
        fi
        
        log_verbose "Attempt $attempt/$max_attempts: Waiting for Vault..."
        sleep 2
        ((attempt++))
    done
    
    log_error "Vault is not available after $max_attempts attempts"
    return 1
}

# =============================================================================
# Main Functions
# =============================================================================

# Enable PKI secrets engine (idempotent)
enable_pki_engine() {
    log_info "Enabling PKI secrets engine at ${VAULT_PKI_PATH}/..."
    
    # Check if already enabled
    local mounts
    mounts=$(vault_request GET "sys/mounts" 2>/dev/null || echo "{}")
    
    if echo "$mounts" | grep -q "\"${VAULT_PKI_PATH}/\""; then
        log_info "PKI secrets engine already enabled at ${VAULT_PKI_PATH}/"
        return 0
    fi
    
    # Enable PKI secrets engine
    local data
    data=$(cat <<EOF
{
    "type": "pki",
    "config": {
        "max_lease_ttl": "${VAULT_PKI_MAX_TTL}"
    }
}
EOF
)
    
    local result
    result=$(vault_request POST "sys/mounts/${VAULT_PKI_PATH}" "${data}" 2>&1)
    
    if echo "$result" | grep -q "error"; then
        if echo "$result" | grep -q "path is already in use"; then
            log_info "PKI secrets engine already enabled at ${VAULT_PKI_PATH}/"
            return 0
        fi
        log_error "Failed to enable PKI secrets engine: $result"
        return 1
    fi
    
    log_success "PKI secrets engine enabled at ${VAULT_PKI_PATH}/"
}

# Configure PKI max lease TTL
configure_pki_ttl() {
    log_info "Configuring PKI max lease TTL..."
    
    local data
    data=$(cat <<EOF
{
    "max_lease_ttl": "${VAULT_PKI_MAX_TTL}"
}
EOF
)
    
    local result
    result=$(vault_request POST "sys/mounts/${VAULT_PKI_PATH}/tune" "${data}" 2>&1)
    
    if echo "$result" | grep -q "error"; then
        log_warn "Could not configure PKI TTL: $result"
    else
        log_success "PKI max lease TTL configured: ${VAULT_PKI_MAX_TTL}"
    fi
}

# Generate root CA
generate_root_ca() {
    log_info "Generating root CA certificate..."
    
    # Check if CA already exists
    local ca_result
    ca_result=$(vault_request GET "${VAULT_PKI_PATH}/ca/pem" 2>&1)
    
    if [[ -n "${ca_result}" ]] && ! echo "$ca_result" | grep -q "error"; then
        log_info "Root CA already exists"
        return 0
    fi
    
    local data
    data=$(cat <<EOF
{
    "common_name": "${CA_COMMON_NAME}",
    "ttl": "${VAULT_PKI_MAX_TTL}",
    "key_type": "rsa",
    "key_bits": 4096
}
EOF
)
    
    local result
    result=$(vault_request POST "${VAULT_PKI_PATH}/root/generate/internal" "${data}" 2>&1)
    
    if echo "$result" | grep -q "error"; then
        log_error "Failed to generate root CA: $result"
        return 1
    fi
    
    log_success "Root CA generated: ${CA_COMMON_NAME}"
}

# Configure CA and CRL URLs
configure_ca_urls() {
    log_info "Configuring CA and CRL URLs..."
    
    local data
    data=$(cat <<EOF
{
    "issuing_certificates": "${VAULT_ADDR}/v1/${VAULT_PKI_PATH}/ca",
    "crl_distribution_points": "${VAULT_ADDR}/v1/${VAULT_PKI_PATH}/crl"
}
EOF
)
    
    local result
    result=$(vault_request POST "${VAULT_PKI_PATH}/config/urls" "${data}" 2>&1)
    
    if echo "$result" | grep -q "error"; then
        log_warn "Could not configure CA URLs: $result"
    else
        log_success "CA and CRL URLs configured"
    fi
}

# Create PKI role for webhook certificates
create_pki_role() {
    log_info "Creating PKI role for webhook certificates..."
    
    local data
    data=$(cat <<EOF
{
    "allowed_domains": ["svc", "svc.cluster.local", "localhost"],
    "allow_subdomains": true,
    "allow_bare_domains": true,
    "allow_localhost": true,
    "allow_ip_sans": true,
    "max_ttl": "${VAULT_PKI_TTL}",
    "ttl": "${VAULT_PKI_TTL}",
    "key_type": "rsa",
    "key_bits": 2048,
    "key_usage": ["DigitalSignature", "KeyEncipherment"],
    "ext_key_usage": ["ServerAuth", "ClientAuth"],
    "require_cn": true,
    "allow_any_name": false,
    "enforce_hostnames": false
}
EOF
)
    
    local result
    result=$(vault_request POST "${VAULT_PKI_PATH}/roles/${VAULT_PKI_ROLE}" "${data}" 2>&1)
    
    if echo "$result" | grep -q "error"; then
        log_error "Failed to create PKI role: $result"
        return 1
    fi
    
    log_success "PKI role created: ${VAULT_PKI_ROLE}"
}

# Create policy for PKI access
create_pki_policy() {
    log_info "Creating PKI access policy..."
    
    local policy
    policy=$(cat <<EOF
# Allow issuing certificates
path "${VAULT_PKI_PATH}/issue/${VAULT_PKI_ROLE}" {
  capabilities = ["create", "update"]
}

# Allow reading CA certificate
path "${VAULT_PKI_PATH}/ca/pem" {
  capabilities = ["read"]
}

# Allow reading CA chain
path "${VAULT_PKI_PATH}/ca_chain" {
  capabilities = ["read"]
}

# Allow reading certificate
path "${VAULT_PKI_PATH}/cert/*" {
  capabilities = ["read"]
}

# Allow checking Vault health
path "sys/health" {
  capabilities = ["read"]
}
EOF
)
    
    local data
    data=$(jq -n --arg policy "$policy" '{"policy": $policy}')
    
    local result
    result=$(vault_request PUT "sys/policies/acl/k8s-postgresql-operator-pki" "${data}" 2>&1)
    
    if echo "$result" | grep -q "error"; then
        log_error "Failed to create PKI policy: $result"
        return 1
    fi
    
    log_success "PKI policy created: k8s-postgresql-operator-pki"
}

# Verify PKI setup by issuing a test certificate
verify_pki_setup() {
    log_info "Verifying PKI setup by issuing a test certificate..."
    
    local data
    data=$(cat <<EOF
{
    "common_name": "test.svc.cluster.local",
    "ttl": "1h"
}
EOF
)
    
    local result
    result=$(vault_request POST "${VAULT_PKI_PATH}/issue/${VAULT_PKI_ROLE}" "${data}" 2>&1)
    
    if echo "$result" | grep -q '"certificate"'; then
        log_success "PKI setup verified - test certificate issued successfully"
    else
        log_error "Failed to issue test certificate: $result"
        return 1
    fi
}

# =============================================================================
# Main Entry Point
# =============================================================================

main() {
    echo ""
    echo "============================================================================="
    echo "Vault PKI Setup for k8s-postgresql-operator Webhooks"
    echo "============================================================================="
    echo ""
    echo "Configuration:"
    echo "  Vault Address:     ${VAULT_ADDR}"
    echo "  PKI Mount Path:    ${VAULT_PKI_PATH}"
    echo "  PKI Role:          ${VAULT_PKI_ROLE}"
    echo "  Certificate TTL:   ${VAULT_PKI_TTL}"
    echo "  Max TTL:           ${VAULT_PKI_MAX_TTL}"
    echo "  CA Common Name:    ${CA_COMMON_NAME}"
    echo ""
    
    # Check Vault is ready
    check_vault_ready
    
    # Enable PKI secrets engine
    enable_pki_engine
    
    # Configure PKI TTL
    configure_pki_ttl
    
    # Generate root CA
    generate_root_ca
    
    # Configure CA URLs
    configure_ca_urls
    
    # Create PKI role
    create_pki_role
    
    # Create PKI policy
    create_pki_policy
    
    # Verify PKI setup
    verify_pki_setup
    
    echo ""
    echo "============================================================================="
    log_success "Vault PKI setup complete!"
    echo "============================================================================="
    echo ""
    echo "PKI Configuration:"
    echo "  - Mount Path:  ${VAULT_PKI_PATH}"
    echo "  - Role:        ${VAULT_PKI_ROLE}"
    echo "  - TTL:         ${VAULT_PKI_TTL}"
    echo ""
    echo "To issue a certificate:"
    echo "  curl -X POST -H \"X-Vault-Token: \${VAULT_TOKEN}\" \\"
    echo "    ${VAULT_ADDR}/v1/${VAULT_PKI_PATH}/issue/${VAULT_PKI_ROLE} \\"
    echo "    -d '{\"common_name\": \"my-service.svc.cluster.local\"}'"
    echo ""
}

main "$@"
