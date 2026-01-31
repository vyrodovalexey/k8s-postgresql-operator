#!/bin/bash
# =============================================================================
# setup-vault-k8s.sh - Kubernetes Auth Setup for Vault
# =============================================================================
# This script configures Vault with Kubernetes authentication:
# - Enables Kubernetes auth method
# - Configures Kubernetes auth with local cluster
# - Creates policy for operator access
# - Creates role for operator service account
#
# Path structure: Mount Point (secret) / instance type (pdb) / UUID / User Name
#
# Prerequisites:
# - Vault server running
# - kubectl configured with access to Kubernetes cluster
# - curl and jq installed
#
# Environment Variables:
#   VAULT_ADDR          - Vault server address (default: http://localhost:8200)
#   VAULT_TOKEN         - Vault root token (default: myroot)
#   VAULT_NAMESPACE     - Kubernetes namespace (default: k8s-postgresql-operator-test)
#   VAULT_SA_NAME       - Service account name (default: k8s-postgresql-operator)
#   VAULT_ROLE          - Vault role name (default: k8s-postgresql-operator)
#   VAULT_MOUNT         - KV secrets engine mount point (default: secret)
#   VAULT_PATH          - Secret path prefix (default: pdb)
#   K8S_HOST            - Kubernetes API server host (auto-detected if not set)
#   K8S_CA_CERT         - Kubernetes CA certificate (auto-detected if not set)
#
# Usage:
#   ./setup-vault-k8s.sh
#   VAULT_NAMESPACE=my-namespace ./setup-vault-k8s.sh
# =============================================================================

set -euo pipefail

# =============================================================================
# Configuration
# =============================================================================

VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"
VAULT_TOKEN="${VAULT_TOKEN:-myroot}"
VAULT_NAMESPACE="${VAULT_NAMESPACE:-k8s-postgresql-operator-test}"
VAULT_SA_NAME="${VAULT_SA_NAME:-k8s-postgresql-operator}"
VAULT_ROLE="${VAULT_ROLE:-k8s-postgresql-operator}"
VAULT_MOUNT="${VAULT_MOUNT:-secret}"
VAULT_PATH="${VAULT_PATH:-pdb}"
K8S_HOST="${K8S_HOST:-}"
K8S_CA_CERT="${K8S_CA_CERT:-}"
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

# Get Kubernetes cluster information
get_k8s_info() {
    log_info "Getting Kubernetes cluster information..."
    
    # Get Kubernetes API server host
    if [[ -z "${K8S_HOST}" ]]; then
        K8S_HOST=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null || echo "")
        
        if [[ -z "${K8S_HOST}" ]]; then
            log_error "Could not determine Kubernetes API server host"
            return 1
        fi
    fi
    log_info "Kubernetes API server: ${K8S_HOST}"
    
    # Get Kubernetes CA certificate
    if [[ -z "${K8S_CA_CERT}" ]]; then
        # Try to get CA from kubeconfig
        K8S_CA_CERT=$(kubectl config view --raw --minify -o jsonpath='{.clusters[0].cluster.certificate-authority-data}' 2>/dev/null || echo "")
        
        if [[ -z "${K8S_CA_CERT}" ]]; then
            # Try to read from file
            local ca_file
            ca_file=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.certificate-authority}' 2>/dev/null || echo "")
            if [[ -n "${ca_file}" ]] && [[ -f "${ca_file}" ]]; then
                K8S_CA_CERT=$(base64 < "${ca_file}" | tr -d '\n')
            fi
        fi
        
        if [[ -z "${K8S_CA_CERT}" ]]; then
            log_warn "Could not get Kubernetes CA certificate, using empty value"
            K8S_CA_CERT=""
        fi
    fi
    
    log_success "Kubernetes cluster information retrieved"
}

# =============================================================================
# Main Functions
# =============================================================================

# Enable Kubernetes auth method (idempotent)
enable_k8s_auth() {
    log_info "Enabling Kubernetes auth method..."
    
    # Check if already enabled
    local auths
    auths=$(vault_request GET "sys/auth" 2>/dev/null || echo "{}")
    
    if echo "$auths" | grep -q '"kubernetes/"'; then
        log_info "Kubernetes auth method already enabled"
        return 0
    fi
    
    # Enable Kubernetes auth
    local result
    result=$(vault_request POST "sys/auth/kubernetes" '{"type":"kubernetes"}' 2>&1)
    
    if echo "$result" | grep -q "error"; then
        if echo "$result" | grep -q "path is already in use"; then
            log_info "Kubernetes auth method already enabled"
            return 0
        fi
        log_error "Failed to enable Kubernetes auth: $result"
        return 1
    fi
    
    log_success "Kubernetes auth method enabled"
}

# Configure Kubernetes auth
configure_k8s_auth() {
    log_info "Configuring Kubernetes auth..."
    
    local data
    if [[ -n "${K8S_CA_CERT}" ]]; then
        data=$(cat <<EOF
{
    "kubernetes_host": "${K8S_HOST}",
    "kubernetes_ca_cert": "$(echo "${K8S_CA_CERT}" | base64 -d 2>/dev/null || echo "${K8S_CA_CERT}")",
    "disable_local_ca_jwt": false
}
EOF
)
    else
        # For local development, we can skip CA verification
        data=$(cat <<EOF
{
    "kubernetes_host": "${K8S_HOST}",
    "disable_local_ca_jwt": true
}
EOF
)
    fi
    
    local result
    result=$(vault_request POST "auth/kubernetes/config" "${data}" 2>&1)
    
    if echo "$result" | grep -q "error"; then
        log_error "Failed to configure Kubernetes auth: $result"
        return 1
    fi
    
    log_success "Kubernetes auth configured"
}

# Create policy for operator access
create_operator_policy() {
    log_info "Creating operator policy..."
    
    local policy
    policy=$(cat <<'EOF'
# Allow reading and writing secrets for PostgreSQL instances
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

# Allow token self-lookup
path "auth/token/lookup-self" {
  capabilities = ["read"]
}
EOF
)
    
    local data
    data=$(jq -n --arg policy "$policy" '{"policy": $policy}')
    
    local result
    result=$(vault_request PUT "sys/policies/acl/${VAULT_ROLE}" "${data}" 2>&1)
    
    if echo "$result" | grep -q "error"; then
        log_error "Failed to create policy: $result"
        return 1
    fi
    
    log_success "Operator policy created: ${VAULT_ROLE}"
}

# Create role for operator service account
create_operator_role() {
    log_info "Creating Kubernetes auth role for operator..."
    
    local data
    data=$(cat <<EOF
{
    "bound_service_account_names": ["${VAULT_SA_NAME}", "${VAULT_SA_NAME}-*"],
    "bound_service_account_namespaces": ["${VAULT_NAMESPACE}", "default", "kube-system"],
    "policies": ["${VAULT_ROLE}"],
    "ttl": "24h",
    "max_ttl": "48h"
}
EOF
)
    
    local result
    result=$(vault_request POST "auth/kubernetes/role/${VAULT_ROLE}" "${data}" 2>&1)
    
    if echo "$result" | grep -q "error"; then
        log_error "Failed to create role: $result"
        return 1
    fi
    
    log_success "Kubernetes auth role created: ${VAULT_ROLE}"
}

# Verify Kubernetes auth configuration
verify_k8s_auth() {
    log_info "Verifying Kubernetes auth configuration..."
    
    # Check role exists
    local role_result
    role_result=$(vault_request GET "auth/kubernetes/role/${VAULT_ROLE}" 2>&1)
    
    if echo "$role_result" | grep -q "bound_service_account_names"; then
        log_success "Kubernetes auth role verified"
    else
        log_error "Failed to verify Kubernetes auth role"
        return 1
    fi
    
    # Check policy exists
    local policy_result
    policy_result=$(vault_request GET "sys/policies/acl/${VAULT_ROLE}" 2>&1)
    
    if echo "$policy_result" | grep -q "policy"; then
        log_success "Operator policy verified"
    else
        log_error "Failed to verify operator policy"
        return 1
    fi
}

# =============================================================================
# Main Entry Point
# =============================================================================

main() {
    echo ""
    echo "============================================================================="
    echo "Vault Kubernetes Auth Setup for k8s-postgresql-operator"
    echo "============================================================================="
    echo ""
    echo "Configuration:"
    echo "  Vault Address:     ${VAULT_ADDR}"
    echo "  Namespace:         ${VAULT_NAMESPACE}"
    echo "  Service Account:   ${VAULT_SA_NAME}"
    echo "  Vault Role:        ${VAULT_ROLE}"
    echo "  Mount Point:       ${VAULT_MOUNT}"
    echo "  Secret Path:       ${VAULT_PATH}"
    echo ""
    
    # Check Vault is ready
    check_vault_ready
    
    # Get Kubernetes cluster information
    get_k8s_info
    
    # Enable Kubernetes auth
    enable_k8s_auth
    
    # Configure Kubernetes auth
    configure_k8s_auth
    
    # Create policy
    create_operator_policy
    
    # Create role
    create_operator_role
    
    # Verify configuration
    verify_k8s_auth
    
    echo ""
    echo "============================================================================="
    log_success "Vault Kubernetes auth setup complete!"
    echo "============================================================================="
    echo ""
    echo "The operator can now authenticate to Vault using:"
    echo "  - Service Account: ${VAULT_SA_NAME}"
    echo "  - Namespace:       ${VAULT_NAMESPACE}"
    echo "  - Vault Role:      ${VAULT_ROLE}"
    echo ""
}

main "$@"
