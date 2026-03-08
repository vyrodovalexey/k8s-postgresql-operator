#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# setup-vault-k8s-auth.sh - Configure Vault Kubernetes authentication
# =============================================================================
# This script configures Vault Kubernetes auth method for the docker-desktop
# cluster. It is idempotent and safe to run multiple times.
# =============================================================================

# Configuration with sensible defaults
VAULT_ADDR="${VAULT_ADDR:-http://127.0.0.1:8200}"
VAULT_TOKEN="${VAULT_TOKEN:-myroot}"
VAULT_ROLE_NAME="${VAULT_ROLE_NAME:-k8s-postgresql-operator}"
OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-k8s-postgresql-operator-system}"
OPERATOR_SERVICE_ACCOUNT="${OPERATOR_SERVICE_ACCOUNT:-k8s-postgresql-operator}"
VAULT_AUTH_PATH="${VAULT_AUTH_PATH:-kubernetes}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; }

# Export for vault CLI
export VAULT_ADDR
export VAULT_TOKEN

# ---------------------------------------------------------------------------
# Wait for Vault to be ready
# ---------------------------------------------------------------------------
wait_for_vault() {
    info "Waiting for Vault to be ready at ${VAULT_ADDR}..."
    local retries=30
    local count=0
    until vault status >/dev/null 2>&1; do
        count=$((count + 1))
        if [ "${count}" -ge "${retries}" ]; then
            error "Vault did not become ready after ${retries} attempts"
            exit 1
        fi
        warn "Vault not ready yet (attempt ${count}/${retries}), waiting..."
        sleep 2
    done
    info "Vault is ready"
}

# ---------------------------------------------------------------------------
# Ensure kubectl is available and cluster is reachable
# ---------------------------------------------------------------------------
check_k8s() {
    info "Checking Kubernetes cluster connectivity..."
    if ! kubectl cluster-info >/dev/null 2>&1; then
        error "Cannot connect to Kubernetes cluster. Ensure docker-desktop K8s is running."
        exit 1
    fi
    local context
    context=$(kubectl config current-context)
    info "Connected to Kubernetes cluster (context: ${context})"
}

# ---------------------------------------------------------------------------
# Create operator namespace if not exists
# ---------------------------------------------------------------------------
create_namespace() {
    info "Ensuring namespace '${OPERATOR_NAMESPACE}' exists..."
    if kubectl get namespace "${OPERATOR_NAMESPACE}" >/dev/null 2>&1; then
        info "Namespace '${OPERATOR_NAMESPACE}' already exists"
    else
        kubectl create namespace "${OPERATOR_NAMESPACE}"
        info "Namespace '${OPERATOR_NAMESPACE}' created"
    fi
}

# ---------------------------------------------------------------------------
# Create service account for Vault auth
# ---------------------------------------------------------------------------
create_service_account() {
    local sa_name="vault-auth"
    info "Ensuring service account '${sa_name}' exists in '${OPERATOR_NAMESPACE}'..."

    if kubectl get serviceaccount "${sa_name}" -n "${OPERATOR_NAMESPACE}" >/dev/null 2>&1; then
        info "Service account '${sa_name}' already exists"
    else
        kubectl create serviceaccount "${sa_name}" -n "${OPERATOR_NAMESPACE}"
        info "Service account '${sa_name}' created"
    fi

    # Create ClusterRoleBinding for token review (required for Vault K8s auth)
    info "Ensuring ClusterRoleBinding for vault-auth token review..."
    kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: vault-auth-tokenreview-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
  - kind: ServiceAccount
    name: ${sa_name}
    namespace: ${OPERATOR_NAMESPACE}
EOF
    info "ClusterRoleBinding configured"

    # Create a long-lived token secret for the vault-auth service account
    info "Ensuring long-lived token secret for vault-auth..."
    kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: vault-auth-token
  namespace: ${OPERATOR_NAMESPACE}
  annotations:
    kubernetes.io/service-account.name: ${sa_name}
type: kubernetes.io/service-account-token
EOF
    info "Token secret configured"

    # Wait for the token to be populated
    info "Waiting for token to be populated..."
    local retries=15
    local count=0
    until kubectl get secret vault-auth-token -n "${OPERATOR_NAMESPACE}" -o jsonpath='{.data.token}' 2>/dev/null | base64 -d >/dev/null 2>&1; do
        count=$((count + 1))
        if [ "${count}" -ge "${retries}" ]; then
            error "Token was not populated after ${retries} attempts"
            exit 1
        fi
        sleep 2
    done
    info "Token is ready"
}

# ---------------------------------------------------------------------------
# Get Kubernetes cluster information
# ---------------------------------------------------------------------------
get_k8s_info() {
    info "Retrieving Kubernetes cluster information..."

    # Get the K8s API server CA certificate
    K8S_CA_CERT=$(kubectl get secret vault-auth-token -n "${OPERATOR_NAMESPACE}" -o jsonpath='{.data.ca\.crt}' | base64 -d)
    info "✓ Retrieved K8s CA certificate"

    # Get the reviewer JWT token
    REVIEWER_JWT=$(kubectl get secret vault-auth-token -n "${OPERATOR_NAMESPACE}" -o jsonpath='{.data.token}' | base64 -d)
    info "✓ Retrieved reviewer JWT token"

    # Determine the K8s API endpoint that Vault can reach.
    # Vault runs in docker-compose, so it cannot reach 127.0.0.1 on the host.
    # For docker-desktop, we use kubernetes.docker.internal because:
    #   1. It resolves to the docker-desktop VM IP (192.168.65.3)
    #   2. The K8s API server TLS cert includes it as a SAN
    # Using host.docker.internal would fail TLS verification since it's not in the cert SANs.
    K8S_HOST="${K8S_HOST:-}"

    if [ -z "${K8S_HOST}" ]; then
        local cluster_server
        cluster_server=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null || true)
        if [ -n "${cluster_server}" ]; then
            # Replace 127.0.0.1 or localhost with kubernetes.docker.internal
            # which is in the K8s API server TLS cert SANs
            K8S_HOST=$(echo "${cluster_server}" | sed -e 's|127\.0\.0\.1|kubernetes.docker.internal|g' -e 's|localhost|kubernetes.docker.internal|g')
            info "✓ Using K8s API endpoint: ${K8S_HOST} (derived from ${cluster_server})"
        else
            K8S_HOST="https://kubernetes.docker.internal:6443"
            info "✓ Using default K8s API endpoint: ${K8S_HOST}"
        fi
    else
        info "✓ Using K8s API endpoint from env: ${K8S_HOST}"
    fi
}

# ---------------------------------------------------------------------------
# Enable Vault Kubernetes auth method
# ---------------------------------------------------------------------------
enable_k8s_auth() {
    info "Checking if Kubernetes auth method is enabled at '${VAULT_AUTH_PATH}'..."
    if vault auth list -format=json | jq -e ".\"${VAULT_AUTH_PATH}/\"" >/dev/null 2>&1; then
        info "Kubernetes auth method already enabled at '${VAULT_AUTH_PATH}/'"
    else
        info "Enabling Kubernetes auth method at '${VAULT_AUTH_PATH}'..."
        vault auth enable -path="${VAULT_AUTH_PATH}" kubernetes
        info "Kubernetes auth method enabled"
    fi
}

# ---------------------------------------------------------------------------
# Configure Vault Kubernetes auth
# ---------------------------------------------------------------------------
configure_k8s_auth() {
    info "Configuring Vault Kubernetes auth..."

    # Write the K8s auth config
    vault write "auth/${VAULT_AUTH_PATH}/config" \
        token_reviewer_jwt="${REVIEWER_JWT}" \
        kubernetes_host="${K8S_HOST}" \
        kubernetes_ca_cert="${K8S_CA_CERT}" \
        disable_local_ca_jwt="true"

    info "Vault Kubernetes auth configured"
}

# ---------------------------------------------------------------------------
# Create Vault role for the operator
# ---------------------------------------------------------------------------
create_vault_role() {
    info "Creating Vault role '${VAULT_ROLE_NAME}'..."

    vault write "auth/${VAULT_AUTH_PATH}/role/${VAULT_ROLE_NAME}" \
        bound_service_account_names="${OPERATOR_SERVICE_ACCOUNT}" \
        bound_service_account_namespaces="${OPERATOR_NAMESPACE}" \
        policies="k8s-postgresql-operator-kv,k8s-postgresql-operator-pki" \
        ttl="1h" \
        max_ttl="24h"

    info "Vault role '${VAULT_ROLE_NAME}' created with policies: k8s-postgresql-operator-kv, k8s-postgresql-operator-pki"
}

# ---------------------------------------------------------------------------
# Verify configuration
# ---------------------------------------------------------------------------
verify_config() {
    info "Verifying Vault Kubernetes auth configuration..."

    # Read back the config
    local config
    config=$(vault read -format=json "auth/${VAULT_AUTH_PATH}/config" 2>&1)
    if echo "${config}" | jq -e '.data.kubernetes_host' >/dev/null 2>&1; then
        local host
        host=$(echo "${config}" | jq -r '.data.kubernetes_host')
        info "✓ Kubernetes host: ${host}"
    else
        error "✗ Failed to read Kubernetes auth config"
        exit 1
    fi

    # Read back the role
    local role
    role=$(vault read -format=json "auth/${VAULT_AUTH_PATH}/role/${VAULT_ROLE_NAME}" 2>&1)
    if echo "${role}" | jq -e '.data.bound_service_account_names' >/dev/null 2>&1; then
        local sa_names
        sa_names=$(echo "${role}" | jq -r '.data.bound_service_account_names | join(",")')
        info "✓ Bound service accounts: ${sa_names}"
        local policies
        policies=$(echo "${role}" | jq -r '.data.token_policies | join(",")')
        info "✓ Token policies: ${policies}"
    else
        error "✗ Failed to read Vault role"
        exit 1
    fi

    info "Vault Kubernetes auth verification complete"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    info "============================================="
    info "  Vault Kubernetes Auth Setup"
    info "============================================="
    info "Vault address:     ${VAULT_ADDR}"
    info "Auth path:         ${VAULT_AUTH_PATH}"
    info "Vault role:        ${VAULT_ROLE_NAME}"
    info "Operator NS:       ${OPERATOR_NAMESPACE}"
    info "Operator SA:       ${OPERATOR_SERVICE_ACCOUNT}"
    echo

    wait_for_vault
    check_k8s
    create_namespace
    create_service_account
    get_k8s_info
    enable_k8s_auth
    configure_k8s_auth
    create_vault_role
    verify_config

    echo
    info "============================================="
    info "  Vault Kubernetes auth setup complete!"
    info "============================================="
}

main "$@"
