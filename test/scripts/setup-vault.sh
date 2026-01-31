#!/bin/bash
# =============================================================================
# setup-vault.sh - Basic Vault Setup for Testing
# =============================================================================
# This script sets up Vault with basic configuration for testing:
# - Enables KV v2 secrets engine at secret/ path
# - Stores test PostgreSQL admin credentials
# - Stores test user credentials
#
# Path structure: Mount Point (secret) / instance type (pdb) / UUID / User Name
# Example: secret/pdb/test-pg-001/admin
#
# Prerequisites:
# - Vault server running (dev mode or configured)
# - curl installed
#
# Environment Variables:
#   VAULT_ADDR      - Vault server address (default: http://localhost:8200)
#   VAULT_TOKEN     - Vault root token (default: myroot)
#   VAULT_MOUNT     - KV secrets engine mount point (default: secret)
#   VAULT_PATH      - Secret path prefix (default: pdb)
#   PG_INSTANCE_ID  - PostgreSQL instance ID (default: test-pg-001)
#   PG_ADMIN_USER   - PostgreSQL admin username (default: postgres)
#   PG_ADMIN_PASS   - PostgreSQL admin password (default: postgres)
#   TEST_USER       - Test user name (default: testuser)
#   TEST_USER_PASS  - Test user password (default: testpassword123)
#
# Usage:
#   ./setup-vault.sh
#   VAULT_ADDR=http://vault:8200 ./setup-vault.sh
# =============================================================================

set -euo pipefail

# =============================================================================
# Configuration
# =============================================================================

VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"
VAULT_TOKEN="${VAULT_TOKEN:-myroot}"
VAULT_MOUNT="${VAULT_MOUNT:-secret}"
VAULT_PATH="${VAULT_PATH:-pdb}"
PG_INSTANCE_ID="${PG_INSTANCE_ID:-test-pg-001}"
PG_ADMIN_USER="${PG_ADMIN_USER:-postgres}"
PG_ADMIN_PASS="${PG_ADMIN_PASS:-postgres}"
TEST_USER="${TEST_USER:-testuser}"
TEST_USER_PASS="${TEST_USER_PASS:-testpassword123}"
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

# Enable KV v2 secrets engine (idempotent)
enable_kv_secrets_engine() {
    log_info "Enabling KV v2 secrets engine at ${VAULT_MOUNT}/..."
    
    # Check if already enabled
    local mounts
    mounts=$(vault_request GET "sys/mounts" 2>/dev/null || echo "{}")
    
    if echo "$mounts" | grep -q "\"${VAULT_MOUNT}/\""; then
        log_info "KV secrets engine already enabled at ${VAULT_MOUNT}/"
        return 0
    fi
    
    # Enable KV v2 secrets engine
    local result
    result=$(vault_request POST "sys/mounts/${VAULT_MOUNT}" \
        '{"type":"kv","options":{"version":"2"}}' 2>&1)
    
    if echo "$result" | grep -q "error"; then
        # Check if it's already enabled (dev mode has it by default)
        if echo "$result" | grep -q "path is already in use"; then
            log_info "KV secrets engine already enabled at ${VAULT_MOUNT}/"
            return 0
        fi
        log_warn "Could not enable KV secrets engine: $result"
        log_info "Assuming it's already enabled (dev mode)"
    else
        log_success "KV v2 secrets engine enabled at ${VAULT_MOUNT}/"
    fi
}

# Store PostgreSQL admin credentials
store_admin_credentials() {
    local secret_path="${VAULT_MOUNT}/data/${VAULT_PATH}/${PG_INSTANCE_ID}/admin"
    
    log_info "Storing admin credentials at ${secret_path}..."
    
    local data
    data=$(cat <<EOF
{
    "data": {
        "admin_username": "${PG_ADMIN_USER}",
        "admin_password": "${PG_ADMIN_PASS}"
    }
}
EOF
)
    
    local result
    result=$(vault_request POST "${secret_path}" "${data}" 2>&1)
    
    if echo "$result" | grep -q "error"; then
        log_error "Failed to store admin credentials: $result"
        return 1
    fi
    
    log_success "Admin credentials stored at ${VAULT_PATH}/${PG_INSTANCE_ID}/admin"
}

# Store test user credentials
store_user_credentials() {
    local secret_path="${VAULT_MOUNT}/data/${VAULT_PATH}/${PG_INSTANCE_ID}/${TEST_USER}"
    
    log_info "Storing user credentials at ${secret_path}..."
    
    local data
    data=$(cat <<EOF
{
    "data": {
        "password": "${TEST_USER_PASS}"
    }
}
EOF
)
    
    local result
    result=$(vault_request POST "${secret_path}" "${data}" 2>&1)
    
    if echo "$result" | grep -q "error"; then
        log_error "Failed to store user credentials: $result"
        return 1
    fi
    
    log_success "User credentials stored at ${VAULT_PATH}/${PG_INSTANCE_ID}/${TEST_USER}"
}

# Verify stored credentials
verify_credentials() {
    log_info "Verifying stored credentials..."
    
    # Verify admin credentials
    local admin_path="${VAULT_MOUNT}/data/${VAULT_PATH}/${PG_INSTANCE_ID}/admin"
    local admin_result
    admin_result=$(vault_request GET "${admin_path}" 2>&1)
    
    if echo "$admin_result" | grep -q "admin_username"; then
        log_success "Admin credentials verified"
    else
        log_error "Failed to verify admin credentials"
        return 1
    fi
    
    # Verify user credentials
    local user_path="${VAULT_MOUNT}/data/${VAULT_PATH}/${PG_INSTANCE_ID}/${TEST_USER}"
    local user_result
    user_result=$(vault_request GET "${user_path}" 2>&1)
    
    if echo "$user_result" | grep -q "password"; then
        log_success "User credentials verified"
    else
        log_error "Failed to verify user credentials"
        return 1
    fi
}

# Create policy for operator access
create_operator_policy() {
    log_info "Creating operator policy for secret access..."
    
    local policy
    policy=$(cat <<'EOF'
# Allow reading secrets for PostgreSQL instances
path "secret/data/pdb/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "secret/metadata/pdb/*" {
  capabilities = ["list", "read", "delete"]
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
    result=$(vault_request PUT "sys/policies/acl/k8s-postgresql-operator" "${data}" 2>&1)
    
    if echo "$result" | grep -q "error"; then
        log_warn "Could not create policy: $result"
    else
        log_success "Operator policy created: k8s-postgresql-operator"
    fi
}

# =============================================================================
# Main Entry Point
# =============================================================================

main() {
    echo ""
    echo "============================================================================="
    echo "Vault Basic Setup for k8s-postgresql-operator"
    echo "============================================================================="
    echo ""
    echo "Configuration:"
    echo "  Vault Address:     ${VAULT_ADDR}"
    echo "  Mount Point:       ${VAULT_MOUNT}"
    echo "  Secret Path:       ${VAULT_PATH}"
    echo "  Instance ID:       ${PG_INSTANCE_ID}"
    echo "  Admin User:        ${PG_ADMIN_USER}"
    echo "  Test User:         ${TEST_USER}"
    echo ""
    
    # Check Vault is ready
    check_vault_ready
    
    # Enable KV secrets engine
    enable_kv_secrets_engine
    
    # Store credentials
    store_admin_credentials
    store_user_credentials
    
    # Create operator policy
    create_operator_policy
    
    # Verify credentials
    verify_credentials
    
    echo ""
    echo "============================================================================="
    log_success "Vault basic setup complete!"
    echo "============================================================================="
    echo ""
    echo "Stored secrets:"
    echo "  - ${VAULT_PATH}/${PG_INSTANCE_ID}/admin (admin credentials)"
    echo "  - ${VAULT_PATH}/${PG_INSTANCE_ID}/${TEST_USER} (user credentials)"
    echo ""
}

main "$@"
