#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# setup-vault.sh - Configure Vault KV v2 secrets engine for testing
# =============================================================================
# This script configures Vault with test secrets for the k8s-postgresql-operator.
# It is idempotent and safe to run multiple times.
# =============================================================================

# Configuration with sensible defaults
VAULT_ADDR="${VAULT_ADDR:-http://127.0.0.1:8200}"
VAULT_TOKEN="${VAULT_TOKEN:-myroot}"
VAULT_MOUNT_POINT="${VAULT_MOUNT_POINT:-secret}"
VAULT_SECRET_PATH="${VAULT_SECRET_PATH:-pdb}"
PG_INSTANCE_ID="${PG_INSTANCE_ID:-test-pg-001}"

# Admin credentials
ADMIN_USERNAME="${ADMIN_USERNAME:-postgres}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-postgres}"

# Test user credentials
TEST_USERNAME="${TEST_USERNAME:-testuser}"
TEST_PASSWORD="${TEST_PASSWORD:-testpassword123}"

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
# Verify KV v2 secrets engine is enabled
# ---------------------------------------------------------------------------
verify_kv_engine() {
    info "Verifying KV v2 secrets engine at '${VAULT_MOUNT_POINT}'..."
    if vault secrets list -format=json | jq -e ".\"${VAULT_MOUNT_POINT}/\"" >/dev/null 2>&1; then
        info "KV v2 secrets engine already enabled at '${VAULT_MOUNT_POINT}/'"
    else
        info "Enabling KV v2 secrets engine at '${VAULT_MOUNT_POINT}'..."
        vault secrets enable -path="${VAULT_MOUNT_POINT}" -version=2 kv
        info "KV v2 secrets engine enabled at '${VAULT_MOUNT_POINT}/'"
    fi
}

# ---------------------------------------------------------------------------
# Store test secrets
# ---------------------------------------------------------------------------
store_secrets() {
    local admin_path="${VAULT_MOUNT_POINT}/${VAULT_SECRET_PATH}/${PG_INSTANCE_ID}/admin"
    local user_path="${VAULT_MOUNT_POINT}/${VAULT_SECRET_PATH}/${PG_INSTANCE_ID}/${TEST_USERNAME}"

    info "Storing admin credentials at '${admin_path}'..."
    vault kv put "${admin_path}" \
        admin_username="${ADMIN_USERNAME}" \
        admin_password="${ADMIN_PASSWORD}"
    info "Admin credentials stored successfully"

    info "Storing test user credentials at '${user_path}'..."
    vault kv put "${user_path}" \
        password="${TEST_PASSWORD}"
    info "Test user credentials stored successfully"
}

# ---------------------------------------------------------------------------
# Create Vault policy for the operator
# ---------------------------------------------------------------------------
create_policy() {
    info "Creating Vault policy 'k8s-postgresql-operator-kv'..."
    vault policy write k8s-postgresql-operator-kv - <<EOF
# Allow read and write access to KV v2 secrets under ${VAULT_SECRET_PATH}/*
path "${VAULT_MOUNT_POINT}/data/${VAULT_SECRET_PATH}/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "${VAULT_MOUNT_POINT}/metadata/${VAULT_SECRET_PATH}/*" {
  capabilities = ["read", "list", "delete"]
}

path "${VAULT_MOUNT_POINT}/delete/${VAULT_SECRET_PATH}/*" {
  capabilities = ["update"]
}

path "${VAULT_MOUNT_POINT}/undelete/${VAULT_SECRET_PATH}/*" {
  capabilities = ["update"]
}

path "${VAULT_MOUNT_POINT}/destroy/${VAULT_SECRET_PATH}/*" {
  capabilities = ["update"]
}
EOF
    info "Policy 'k8s-postgresql-operator-kv' created successfully"
}

# ---------------------------------------------------------------------------
# Verify secrets
# ---------------------------------------------------------------------------
verify_secrets() {
    local admin_path="${VAULT_MOUNT_POINT}/${VAULT_SECRET_PATH}/${PG_INSTANCE_ID}/admin"
    local user_path="${VAULT_MOUNT_POINT}/${VAULT_SECRET_PATH}/${PG_INSTANCE_ID}/${TEST_USERNAME}"

    info "Verifying stored secrets..."

    local admin_user
    admin_user=$(vault kv get -field=admin_username "${admin_path}")
    if [ "${admin_user}" = "${ADMIN_USERNAME}" ]; then
        info "✓ Admin username verified: ${admin_user}"
    else
        error "✗ Admin username mismatch: expected '${ADMIN_USERNAME}', got '${admin_user}'"
        exit 1
    fi

    local admin_pass
    admin_pass=$(vault kv get -field=admin_password "${admin_path}")
    if [ "${admin_pass}" = "${ADMIN_PASSWORD}" ]; then
        info "✓ Admin password verified"
    else
        error "✗ Admin password mismatch"
        exit 1
    fi

    local user_pass
    user_pass=$(vault kv get -field=password "${user_path}")
    if [ "${user_pass}" = "${TEST_PASSWORD}" ]; then
        info "✓ Test user password verified"
    else
        error "✗ Test user password mismatch"
        exit 1
    fi

    info "All secrets verified successfully"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    info "============================================="
    info "  Vault KV v2 Setup for Testing"
    info "============================================="
    info "Vault address: ${VAULT_ADDR}"
    info "Mount point:   ${VAULT_MOUNT_POINT}"
    info "Secret path:   ${VAULT_SECRET_PATH}"
    info "Instance ID:   ${PG_INSTANCE_ID}"
    echo

    wait_for_vault
    verify_kv_engine
    store_secrets
    create_policy
    verify_secrets

    echo
    info "============================================="
    info "  Vault KV v2 setup complete!"
    info "============================================="
}

main "$@"
