#!/bin/bash
# =============================================================================
# setup-vault-local-k8s.sh - Combined Vault Setup for Local K8s Testing
# =============================================================================
# This script combines all Vault setup scripts for local Kubernetes testing:
# - Basic Vault setup (KV secrets, test credentials)
# - Kubernetes auth setup
# - PKI setup for webhook certificates
#
# Supports Docker Desktop, kind, and minikube clusters.
#
# Path structure: Mount Point (secret) / instance type (pdb) / UUID / User Name
# Example: secret/pdb/test-pg-001/admin
#
# Prerequisites:
# - Vault server running (docker-compose or external)
# - kubectl configured with access to local Kubernetes cluster
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
#   VAULT_PKI_PATH      - PKI secrets engine mount path (default: pki)
#   VAULT_PKI_ROLE      - PKI role name (default: webhook-cert)
#   PG_INSTANCE_ID      - PostgreSQL instance ID (default: test-pg-001)
#   CLUSTER_TYPE        - Cluster type: auto, kind, minikube, docker-desktop
#
# Usage:
#   ./setup-vault-local-k8s.sh
#   VAULT_NAMESPACE=my-namespace ./setup-vault-local-k8s.sh
# =============================================================================

set -euo pipefail

# =============================================================================
# Configuration
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"
VAULT_TOKEN="${VAULT_TOKEN:-myroot}"
VAULT_NAMESPACE="${VAULT_NAMESPACE:-k8s-postgresql-operator-test}"
VAULT_SA_NAME="${VAULT_SA_NAME:-k8s-postgresql-operator}"
VAULT_ROLE="${VAULT_ROLE:-k8s-postgresql-operator}"
VAULT_MOUNT="${VAULT_MOUNT:-secret}"
VAULT_PATH="${VAULT_PATH:-pdb}"
VAULT_PKI_PATH="${VAULT_PKI_PATH:-pki}"
VAULT_PKI_ROLE="${VAULT_PKI_ROLE:-webhook-cert}"
PG_INSTANCE_ID="${PG_INSTANCE_ID:-test-pg-001}"
CLUSTER_TYPE="${CLUSTER_TYPE:-auto}"
VERBOSE="${VERBOSE:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color
BOLD='\033[1m'

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

log_section() {
    echo ""
    echo -e "${MAGENTA}${BOLD}=== $1 ===${NC}"
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

# Detect cluster type
detect_cluster_type() {
    if [[ "${CLUSTER_TYPE}" != "auto" ]]; then
        log_info "Using specified cluster type: ${CLUSTER_TYPE}"
        return 0
    fi
    
    log_info "Auto-detecting cluster type..."
    
    # Check for kind
    if command -v kind &>/dev/null && kind get clusters 2>/dev/null | grep -q .; then
        CLUSTER_TYPE="kind"
        log_success "Detected kind cluster"
        return 0
    fi
    
    # Check for minikube
    if command -v minikube &>/dev/null && minikube status &>/dev/null; then
        CLUSTER_TYPE="minikube"
        log_success "Detected minikube cluster"
        return 0
    fi
    
    # Check for Docker Desktop Kubernetes
    local context
    context=$(kubectl config current-context 2>/dev/null || echo "")
    if [[ "$context" == "docker-desktop" ]]; then
        CLUSTER_TYPE="docker-desktop"
        log_success "Detected Docker Desktop Kubernetes"
        return 0
    fi
    
    log_warn "Could not auto-detect cluster type, assuming docker-desktop"
    CLUSTER_TYPE="docker-desktop"
}

# Get Kubernetes cluster information
get_k8s_info() {
    log_info "Getting Kubernetes cluster information..."
    
    # Get Kubernetes API server host
    K8S_HOST=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null || echo "")
    
    if [[ -z "${K8S_HOST}" ]]; then
        log_error "Could not determine Kubernetes API server host"
        return 1
    fi
    log_info "Kubernetes API server: ${K8S_HOST}"
    
    # Get Kubernetes CA certificate
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
        log_warn "Could not get Kubernetes CA certificate"
    fi
    
    log_success "Kubernetes cluster information retrieved"
}

# =============================================================================
# Vault Setup Functions
# =============================================================================

# Enable KV v2 secrets engine
setup_kv_secrets() {
    log_section "Setting up KV Secrets Engine"
    
    # Check if already enabled
    local mounts
    mounts=$(vault_request GET "sys/mounts" 2>/dev/null || echo "{}")
    
    if echo "$mounts" | grep -q "\"${VAULT_MOUNT}/\""; then
        log_info "KV secrets engine already enabled at ${VAULT_MOUNT}/"
    else
        local result
        result=$(vault_request POST "sys/mounts/${VAULT_MOUNT}" \
            '{"type":"kv","options":{"version":"2"}}' 2>&1)
        
        if echo "$result" | grep -q "error" && ! echo "$result" | grep -q "path is already in use"; then
            log_warn "Could not enable KV secrets engine: $result"
            log_info "Assuming it's already enabled (dev mode)"
        else
            log_success "KV v2 secrets engine enabled at ${VAULT_MOUNT}/"
        fi
    fi
    
    # Store admin credentials
    log_info "Storing admin credentials..."
    local admin_data
    admin_data=$(cat <<EOF
{
    "data": {
        "admin_username": "postgres",
        "admin_password": "postgres"
    }
}
EOF
)
    vault_request POST "${VAULT_MOUNT}/data/${VAULT_PATH}/${PG_INSTANCE_ID}/admin" "${admin_data}" > /dev/null
    log_success "Admin credentials stored at ${VAULT_PATH}/${PG_INSTANCE_ID}/admin"
    
    # Store test user credentials
    log_info "Storing test user credentials..."
    local user_data
    user_data=$(cat <<EOF
{
    "data": {
        "password": "testpassword123"
    }
}
EOF
)
    vault_request POST "${VAULT_MOUNT}/data/${VAULT_PATH}/${PG_INSTANCE_ID}/testuser" "${user_data}" > /dev/null
    log_success "User credentials stored at ${VAULT_PATH}/${PG_INSTANCE_ID}/testuser"
}

# Setup Kubernetes auth
setup_k8s_auth() {
    log_section "Setting up Kubernetes Auth"
    
    # Enable Kubernetes auth
    local auths
    auths=$(vault_request GET "sys/auth" 2>/dev/null || echo "{}")
    
    if echo "$auths" | grep -q '"kubernetes/"'; then
        log_info "Kubernetes auth method already enabled"
    else
        local result
        result=$(vault_request POST "sys/auth/kubernetes" '{"type":"kubernetes"}' 2>&1)
        
        if echo "$result" | grep -q "error" && ! echo "$result" | grep -q "path is already in use"; then
            log_error "Failed to enable Kubernetes auth: $result"
            return 1
        fi
        log_success "Kubernetes auth method enabled"
    fi
    
    # Configure Kubernetes auth
    log_info "Configuring Kubernetes auth..."
    local config_data
    if [[ -n "${K8S_CA_CERT:-}" ]]; then
        config_data=$(cat <<EOF
{
    "kubernetes_host": "${K8S_HOST}",
    "kubernetes_ca_cert": "$(echo "${K8S_CA_CERT}" | base64 -d 2>/dev/null || echo "${K8S_CA_CERT}")",
    "disable_local_ca_jwt": false
}
EOF
)
    else
        config_data=$(cat <<EOF
{
    "kubernetes_host": "${K8S_HOST}",
    "disable_local_ca_jwt": true
}
EOF
)
    fi
    
    vault_request POST "auth/kubernetes/config" "${config_data}" > /dev/null
    log_success "Kubernetes auth configured"
    
    # Create operator policy
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

# Allow PKI operations
path "pki/issue/webhook-cert" {
  capabilities = ["create", "update"]
}

path "pki/ca/pem" {
  capabilities = ["read"]
}

path "pki/ca_chain" {
  capabilities = ["read"]
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
    
    local policy_data
    policy_data=$(jq -n --arg policy "$policy" '{"policy": $policy}')
    vault_request PUT "sys/policies/acl/${VAULT_ROLE}" "${policy_data}" > /dev/null
    log_success "Operator policy created: ${VAULT_ROLE}"
    
    # Create role for operator service account
    log_info "Creating Kubernetes auth role..."
    local role_data
    role_data=$(cat <<EOF
{
    "bound_service_account_names": ["${VAULT_SA_NAME}", "${VAULT_SA_NAME}-*", "default"],
    "bound_service_account_namespaces": ["${VAULT_NAMESPACE}", "default", "kube-system"],
    "policies": ["${VAULT_ROLE}"],
    "ttl": "24h",
    "max_ttl": "48h"
}
EOF
)
    
    vault_request POST "auth/kubernetes/role/${VAULT_ROLE}" "${role_data}" > /dev/null
    log_success "Kubernetes auth role created: ${VAULT_ROLE}"
}

# Setup PKI for webhook certificates
setup_pki() {
    log_section "Setting up PKI for Webhook Certificates"
    
    # Enable PKI secrets engine
    local mounts
    mounts=$(vault_request GET "sys/mounts" 2>/dev/null || echo "{}")
    
    if echo "$mounts" | grep -q "\"${VAULT_PKI_PATH}/\""; then
        log_info "PKI secrets engine already enabled at ${VAULT_PKI_PATH}/"
    else
        local pki_data
        pki_data=$(cat <<EOF
{
    "type": "pki",
    "config": {
        "max_lease_ttl": "87600h"
    }
}
EOF
)
        local result
        result=$(vault_request POST "sys/mounts/${VAULT_PKI_PATH}" "${pki_data}" 2>&1)
        
        if echo "$result" | grep -q "error" && ! echo "$result" | grep -q "path is already in use"; then
            log_error "Failed to enable PKI secrets engine: $result"
            return 1
        fi
        log_success "PKI secrets engine enabled at ${VAULT_PKI_PATH}/"
    fi
    
    # Configure PKI TTL
    log_info "Configuring PKI max lease TTL..."
    vault_request POST "sys/mounts/${VAULT_PKI_PATH}/tune" '{"max_lease_ttl":"87600h"}' > /dev/null
    
    # Generate root CA (if not exists)
    log_info "Generating root CA certificate..."
    local ca_result
    ca_result=$(vault_request GET "${VAULT_PKI_PATH}/ca/pem" 2>&1)
    
    if [[ -z "${ca_result}" ]] || echo "$ca_result" | grep -q "error"; then
        local ca_data
        ca_data=$(cat <<EOF
{
    "common_name": "K8s PostgreSQL Operator CA",
    "ttl": "87600h",
    "key_type": "rsa",
    "key_bits": 4096
}
EOF
)
        vault_request POST "${VAULT_PKI_PATH}/root/generate/internal" "${ca_data}" > /dev/null
        log_success "Root CA generated"
    else
        log_info "Root CA already exists"
    fi
    
    # Configure CA URLs
    log_info "Configuring CA and CRL URLs..."
    local urls_data
    urls_data=$(cat <<EOF
{
    "issuing_certificates": "${VAULT_ADDR}/v1/${VAULT_PKI_PATH}/ca",
    "crl_distribution_points": "${VAULT_ADDR}/v1/${VAULT_PKI_PATH}/crl"
}
EOF
)
    vault_request POST "${VAULT_PKI_PATH}/config/urls" "${urls_data}" > /dev/null
    
    # Create PKI role
    log_info "Creating PKI role for webhook certificates..."
    local role_data
    role_data=$(cat <<EOF
{
    "allowed_domains": ["svc", "svc.cluster.local", "localhost"],
    "allow_subdomains": true,
    "allow_bare_domains": true,
    "allow_localhost": true,
    "allow_ip_sans": true,
    "max_ttl": "720h",
    "ttl": "720h",
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
    vault_request POST "${VAULT_PKI_PATH}/roles/${VAULT_PKI_ROLE}" "${role_data}" > /dev/null
    log_success "PKI role created: ${VAULT_PKI_ROLE}"
    
    # Verify PKI setup
    log_info "Verifying PKI setup..."
    local test_cert
    test_cert=$(vault_request POST "${VAULT_PKI_PATH}/issue/${VAULT_PKI_ROLE}" \
        '{"common_name": "test.svc.cluster.local", "ttl": "1h"}' 2>&1)
    
    if echo "$test_cert" | grep -q '"certificate"'; then
        log_success "PKI setup verified - test certificate issued successfully"
    else
        log_warn "PKI verification failed, but continuing..."
    fi
}

# Create namespace if it doesn't exist
ensure_namespace() {
    log_info "Ensuring namespace ${VAULT_NAMESPACE} exists..."
    
    if kubectl get namespace "${VAULT_NAMESPACE}" &>/dev/null; then
        log_info "Namespace ${VAULT_NAMESPACE} already exists"
    else
        kubectl create namespace "${VAULT_NAMESPACE}" 2>/dev/null || true
        log_success "Namespace ${VAULT_NAMESPACE} created"
    fi
}

# =============================================================================
# Main Entry Point
# =============================================================================

main() {
    echo ""
    echo "============================================================================="
    echo "Vault Local K8s Setup for k8s-postgresql-operator"
    echo "============================================================================="
    echo ""
    echo "Configuration:"
    echo "  Vault Address:     ${VAULT_ADDR}"
    echo "  Namespace:         ${VAULT_NAMESPACE}"
    echo "  Service Account:   ${VAULT_SA_NAME}"
    echo "  Vault Role:        ${VAULT_ROLE}"
    echo "  KV Mount Point:    ${VAULT_MOUNT}"
    echo "  Secret Path:       ${VAULT_PATH}"
    echo "  PKI Mount Path:    ${VAULT_PKI_PATH}"
    echo "  PKI Role:          ${VAULT_PKI_ROLE}"
    echo "  Instance ID:       ${PG_INSTANCE_ID}"
    echo ""
    
    # Check prerequisites
    log_section "Checking Prerequisites"
    
    if ! command -v kubectl &>/dev/null; then
        log_error "kubectl is required but not installed"
        exit 1
    fi
    
    if ! command -v curl &>/dev/null; then
        log_error "curl is required but not installed"
        exit 1
    fi
    
    if ! command -v jq &>/dev/null; then
        log_error "jq is required but not installed"
        exit 1
    fi
    
    log_success "All prerequisites met"
    
    # Detect cluster type
    detect_cluster_type
    
    # Check Vault is ready
    check_vault_ready
    
    # Get Kubernetes cluster information
    get_k8s_info
    
    # Ensure namespace exists
    ensure_namespace
    
    # Setup KV secrets
    setup_kv_secrets
    
    # Setup Kubernetes auth
    setup_k8s_auth
    
    # Setup PKI
    setup_pki
    
    echo ""
    echo "============================================================================="
    log_success "Vault local K8s setup complete!"
    echo "============================================================================="
    echo ""
    echo "Summary:"
    echo "  - KV secrets engine enabled at ${VAULT_MOUNT}/"
    echo "  - Test credentials stored at ${VAULT_PATH}/${PG_INSTANCE_ID}/"
    echo "  - Kubernetes auth configured for ${VAULT_NAMESPACE}"
    echo "  - PKI enabled at ${VAULT_PKI_PATH}/ with role ${VAULT_PKI_ROLE}"
    echo ""
    echo "The operator can now authenticate to Vault using:"
    echo "  - Service Account: ${VAULT_SA_NAME}"
    echo "  - Namespace:       ${VAULT_NAMESPACE}"
    echo "  - Vault Role:      ${VAULT_ROLE}"
    echo ""
    echo "To test Vault connectivity from a pod:"
    echo "  kubectl run vault-test --rm -it --restart=Never \\"
    echo "    --image=curlimages/curl:latest \\"
    echo "    --namespace=${VAULT_NAMESPACE} \\"
    echo "    -- curl -s ${VAULT_ADDR}/v1/sys/health"
    echo ""
}

main "$@"
