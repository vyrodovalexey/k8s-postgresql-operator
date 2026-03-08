#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# setup-vault-pki.sh - Configure Vault PKI secrets engine for webhook certs
# =============================================================================
# This script configures Vault PKI for issuing TLS certificates used by the
# operator's webhook server. It is idempotent and safe to run multiple times.
# =============================================================================

# Configuration with sensible defaults
VAULT_ADDR="${VAULT_ADDR:-http://127.0.0.1:8200}"
VAULT_TOKEN="${VAULT_TOKEN:-myroot}"
PKI_MOUNT="${PKI_MOUNT:-pki}"
PKI_MAX_LEASE_TTL="${PKI_MAX_LEASE_TTL:-87600h}"
PKI_CA_CN="${PKI_CA_CN:-K8s PostgreSQL Operator CA}"
PKI_ROLE_NAME="${PKI_ROLE_NAME:-webhook-cert}"

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
# Enable PKI secrets engine
# ---------------------------------------------------------------------------
enable_pki() {
    info "Checking if PKI secrets engine is enabled at '${PKI_MOUNT}'..."
    if vault secrets list -format=json | jq -e ".\"${PKI_MOUNT}/\"" >/dev/null 2>&1; then
        info "PKI secrets engine already enabled at '${PKI_MOUNT}/'"
        # Tune the max lease TTL in case it changed
        info "Tuning max lease TTL to ${PKI_MAX_LEASE_TTL}..."
        vault secrets tune -max-lease-ttl="${PKI_MAX_LEASE_TTL}" "${PKI_MOUNT}/"
    else
        info "Enabling PKI secrets engine at '${PKI_MOUNT}'..."
        vault secrets enable -path="${PKI_MOUNT}" -max-lease-ttl="${PKI_MAX_LEASE_TTL}" pki
        info "PKI secrets engine enabled at '${PKI_MOUNT}/'"
    fi
}

# ---------------------------------------------------------------------------
# Generate internal root CA
# ---------------------------------------------------------------------------
generate_root_ca() {
    info "Checking for existing root CA..."
    local ca_cert
    ca_cert=$(vault read -format=json "${PKI_MOUNT}/cert/ca" 2>/dev/null | jq -r '.data.certificate // empty' || true)

    if [ -n "${ca_cert}" ] && [ "${ca_cert}" != "null" ]; then
        info "Root CA already exists, skipping generation"
    else
        info "Generating internal root CA with CN '${PKI_CA_CN}'..."
        vault write "${PKI_MOUNT}/root/generate/internal" \
            common_name="${PKI_CA_CN}" \
            ttl="${PKI_MAX_LEASE_TTL}" \
            key_type="rsa" \
            key_bits=4096
        info "Root CA generated successfully"
    fi
}

# ---------------------------------------------------------------------------
# Configure CA and CRL URLs
# ---------------------------------------------------------------------------
configure_urls() {
    info "Configuring CA and CRL URLs..."
    vault write "${PKI_MOUNT}/config/urls" \
        issuing_certificates="${VAULT_ADDR}/v1/${PKI_MOUNT}/ca" \
        crl_distribution_points="${VAULT_ADDR}/v1/${PKI_MOUNT}/crl"
    info "CA and CRL URLs configured"
}

# ---------------------------------------------------------------------------
# Create PKI role for webhook certificates
# ---------------------------------------------------------------------------
create_pki_role() {
    info "Creating PKI role '${PKI_ROLE_NAME}'..."
    vault write "${PKI_MOUNT}/roles/${PKI_ROLE_NAME}" \
        allowed_domains="svc,svc.cluster.local,localhost" \
        allow_subdomains=true \
        allow_bare_domains=true \
        allow_localhost=true \
        allow_ip_sans=true \
        allow_any_name=true \
        server_flag=true \
        client_flag=true \
        max_ttl="8760h" \
        ttl="720h" \
        key_type="rsa" \
        key_bits=2048 \
        require_cn=true \
        enforce_hostnames=false
    info "PKI role '${PKI_ROLE_NAME}' created successfully"
}

# ---------------------------------------------------------------------------
# Create PKI policy
# ---------------------------------------------------------------------------
create_pki_policy() {
    info "Creating Vault policy 'k8s-postgresql-operator-pki'..."
    vault policy write k8s-postgresql-operator-pki - <<EOF
# Allow issuing certificates
path "${PKI_MOUNT}/issue/${PKI_ROLE_NAME}" {
  capabilities = ["create", "update"]
}

# Allow reading CA certificate
path "${PKI_MOUNT}/cert/ca" {
  capabilities = ["read"]
}

# Allow reading CA chain
path "${PKI_MOUNT}/ca/pem" {
  capabilities = ["read"]
}

# Allow reading CRL
path "${PKI_MOUNT}/crl" {
  capabilities = ["read"]
}

# Allow signing certificates
path "${PKI_MOUNT}/sign/${PKI_ROLE_NAME}" {
  capabilities = ["create", "update"]
}
EOF
    info "Policy 'k8s-postgresql-operator-pki' created successfully"
}

# ---------------------------------------------------------------------------
# Verify by issuing a test certificate
# ---------------------------------------------------------------------------
verify_pki() {
    info "Verifying PKI setup by issuing a test certificate..."
    local result
    result=$(vault write -format=json "${PKI_MOUNT}/issue/${PKI_ROLE_NAME}" \
        common_name="test-webhook.default.svc" \
        alt_names="test-webhook.default.svc.cluster.local,localhost" \
        ip_sans="127.0.0.1" \
        ttl="1h" 2>&1)

    if echo "${result}" | jq -e '.data.certificate' >/dev/null 2>&1; then
        info "✓ Test certificate issued successfully"
        local serial
        serial=$(echo "${result}" | jq -r '.data.serial_number')
        info "  Serial number: ${serial}"
    else
        error "✗ Failed to issue test certificate"
        error "${result}"
        exit 1
    fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    info "============================================="
    info "  Vault PKI Setup for Webhook Certificates"
    info "============================================="
    info "Vault address:   ${VAULT_ADDR}"
    info "PKI mount:       ${PKI_MOUNT}"
    info "Max lease TTL:   ${PKI_MAX_LEASE_TTL}"
    info "CA common name:  ${PKI_CA_CN}"
    info "PKI role:        ${PKI_ROLE_NAME}"
    echo

    wait_for_vault
    enable_pki
    generate_root_ca
    configure_urls
    create_pki_role
    create_pki_policy
    verify_pki

    echo
    info "============================================="
    info "  Vault PKI setup complete!"
    info "============================================="
}

main "$@"
