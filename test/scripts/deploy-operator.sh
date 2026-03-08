#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# deploy-operator.sh - Build and deploy the operator to local Kubernetes
# =============================================================================
# This script builds the operator Docker image and deploys it to the local
# docker-desktop Kubernetes cluster via Helm. It is idempotent and safe to
# run multiple times.
# =============================================================================

# Configuration with sensible defaults
OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-k8s-postgresql-operator-system}"
OPERATOR_IMAGE="${OPERATOR_IMAGE:-k8s-postgresql-operator}"
OPERATOR_TAG="${OPERATOR_TAG:-local}"
HELM_RELEASE_NAME="${HELM_RELEASE_NAME:-k8s-postgresql-operator}"
VAULT_ADDR="${VAULT_ADDR:-http://host.docker.internal:8200}"
VAULT_ROLE="${VAULT_ROLE:-k8s-postgresql-operator}"
VAULT_MOUNT_POINT="${VAULT_MOUNT_POINT:-secret}"
VAULT_SECRET_PATH="${VAULT_SECRET_PATH:-pdb}"
VAULT_PKI_ENABLED="${VAULT_PKI_ENABLED:-true}"
VAULT_PKI_MOUNT_PATH="${VAULT_PKI_MOUNT_PATH:-pki}"
VAULT_PKI_ROLE="${VAULT_PKI_ROLE:-webhook-cert}"
VAULT_PKI_TTL="${VAULT_PKI_TTL:-720h}"
VAULT_PKI_RENEWAL_BUFFER="${VAULT_PKI_RENEWAL_BUFFER:-24h}"
OTEL_ENDPOINT="${OTEL_ENDPOINT:-otel-collector.monitoring.svc.cluster.local:4317}"
OTEL_INSECURE="${OTEL_INSECURE:-true}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-120s}"
SKIP_BUILD="${SKIP_BUILD:-false}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CHART_PATH="${CHART_PATH:-${PROJECT_ROOT}/charts/k8s-postgresql-operator}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; }

# ---------------------------------------------------------------------------
# Check prerequisites
# ---------------------------------------------------------------------------
check_prerequisites() {
    info "Checking prerequisites..."

    if ! command -v kubectl >/dev/null 2>&1; then
        error "kubectl is not installed"
        exit 1
    fi

    if ! command -v helm >/dev/null 2>&1; then
        error "helm is not installed"
        exit 1
    fi

    if ! command -v docker >/dev/null 2>&1; then
        error "docker is not installed"
        exit 1
    fi

    if ! kubectl cluster-info >/dev/null 2>&1; then
        error "Cannot connect to Kubernetes cluster"
        exit 1
    fi

    info "Prerequisites satisfied"
}

# ---------------------------------------------------------------------------
# Create operator namespace
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
# Build Docker image
# ---------------------------------------------------------------------------
build_image() {
    if [ "${SKIP_BUILD}" = "true" ]; then
        info "Skipping Docker image build (SKIP_BUILD=true)"
        return
    fi

    info "Building Docker image '${OPERATOR_IMAGE}:${OPERATOR_TAG}'..."
    docker build -t "${OPERATOR_IMAGE}:${OPERATOR_TAG}" "${PROJECT_ROOT}"
    info "Docker image built successfully"

    # For docker-desktop, the image is automatically available to Kubernetes
    # No need to load it like with kind
    info "Image '${OPERATOR_IMAGE}:${OPERATOR_TAG}' is available to docker-desktop K8s"
}

# ---------------------------------------------------------------------------
# Deploy via Helm
# ---------------------------------------------------------------------------
deploy_helm() {
    info "Deploying operator via Helm..."

    if [ ! -d "${CHART_PATH}" ]; then
        error "Helm chart not found at '${CHART_PATH}'"
        exit 1
    fi

    helm upgrade --install "${HELM_RELEASE_NAME}" "${CHART_PATH}" \
        --namespace "${OPERATOR_NAMESPACE}" \
        --set image.repository="${OPERATOR_IMAGE}" \
        --set image.tag="${OPERATOR_TAG}" \
        --set image.pullPolicy=Never \
        --set vault.addr="${VAULT_ADDR}" \
        --set vault.role="${VAULT_ROLE}" \
        --set vault.mountPoint="${VAULT_MOUNT_POINT}" \
        --set vault.secretPath="${VAULT_SECRET_PATH}" \
        --set vaultPKI.enabled="${VAULT_PKI_ENABLED}" \
        --set vaultPKI.mountPath="${VAULT_PKI_MOUNT_PATH}" \
        --set vaultPKI.role="${VAULT_PKI_ROLE}" \
        --set vaultPKI.ttl="${VAULT_PKI_TTL}" \
        --set vaultPKI.renewalBuffer="${VAULT_PKI_RENEWAL_BUFFER}" \
        --set tracing.enabled=true \
        --set tracing.endpoint="${OTEL_ENDPOINT}" \
        --set tracing.insecure="${OTEL_INSECURE}" \
        --wait \
        --timeout "${WAIT_TIMEOUT}"

    info "Helm release '${HELM_RELEASE_NAME}' deployed successfully"
}

# ---------------------------------------------------------------------------
# Wait for deployment to be ready
# ---------------------------------------------------------------------------
wait_for_deployment() {
    info "Waiting for operator deployment to be ready..."

    if ! kubectl rollout status deployment "${HELM_RELEASE_NAME}" \
        --namespace "${OPERATOR_NAMESPACE}" \
        --timeout="${WAIT_TIMEOUT}" 2>/dev/null; then
        warn "Deployment rollout may not be complete. Checking status..."
    fi

    info "Deployment status:"
    kubectl get deployment -n "${OPERATOR_NAMESPACE}" -o wide
    echo
    info "Pod status:"
    kubectl get pods -n "${OPERATOR_NAMESPACE}" -o wide
}

# ---------------------------------------------------------------------------
# Check pod logs for errors
# ---------------------------------------------------------------------------
check_logs() {
    info "Checking operator pod logs for errors..."

    local pod_name
    pod_name=$(kubectl get pods -n "${OPERATOR_NAMESPACE}" \
        -l "app.kubernetes.io/name=k8s-postgresql-operator" \
        -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

    if [ -z "${pod_name}" ]; then
        warn "No operator pod found. It may still be starting."
        return
    fi

    info "Latest logs from pod '${pod_name}':"
    echo "---"
    kubectl logs "${pod_name}" -n "${OPERATOR_NAMESPACE}" --tail=20 2>/dev/null || warn "Could not retrieve logs"
    echo "---"

    # Check for error-level log entries
    local error_count
    error_count=$(kubectl logs "${pod_name}" -n "${OPERATOR_NAMESPACE}" 2>/dev/null | grep -ci '"level":"error"\|"level": "error"\|ERROR' || true)
    if [ "${error_count}" -gt 0 ]; then
        warn "Found ${error_count} error-level log entries. Review logs with:"
        warn "  kubectl logs ${pod_name} -n ${OPERATOR_NAMESPACE}"
    else
        info "✓ No error-level log entries found"
    fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    info "============================================="
    info "  Operator Deployment"
    info "============================================="
    info "Namespace:     ${OPERATOR_NAMESPACE}"
    info "Image:         ${OPERATOR_IMAGE}:${OPERATOR_TAG}"
    info "Helm release:  ${HELM_RELEASE_NAME}"
    info "Vault addr:    ${VAULT_ADDR}"
    info "Vault role:    ${VAULT_ROLE}"
    info "Vault PKI:     enabled=${VAULT_PKI_ENABLED}, mount=${VAULT_PKI_MOUNT_PATH}, role=${VAULT_PKI_ROLE}"
    info "OTLP:          endpoint=${OTEL_ENDPOINT}, insecure=${OTEL_INSECURE}"
    info "Chart path:    ${CHART_PATH}"
    echo

    check_prerequisites
    create_namespace
    build_image
    deploy_helm
    wait_for_deployment
    check_logs

    echo
    info "============================================="
    info "  Operator deployment complete!"
    info "============================================="
}

main "$@"
