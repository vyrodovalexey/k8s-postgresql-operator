#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# deploy-monitoring.sh - Deploy monitoring stack to local Kubernetes
# =============================================================================
# This script deploys vmagent and otel-collector Helm charts to the local
# docker-desktop Kubernetes cluster. It is idempotent and safe to run multiple
# times.
# =============================================================================

# Configuration with sensible defaults
MONITORING_NAMESPACE="${MONITORING_NAMESPACE:-monitoring}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
VMAGENT_CHART_PATH="${VMAGENT_CHART_PATH:-${PROJECT_ROOT}/test/monitoring/vmagent}"
OTEL_CHART_PATH="${OTEL_CHART_PATH:-${PROJECT_ROOT}/test/monitoring/otel-collector}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-120s}"

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

    if ! kubectl cluster-info >/dev/null 2>&1; then
        error "Cannot connect to Kubernetes cluster"
        exit 1
    fi

    info "Prerequisites satisfied"
}

# ---------------------------------------------------------------------------
# Create monitoring namespace
# ---------------------------------------------------------------------------
create_namespace() {
    info "Ensuring namespace '${MONITORING_NAMESPACE}' exists..."
    if kubectl get namespace "${MONITORING_NAMESPACE}" >/dev/null 2>&1; then
        info "Namespace '${MONITORING_NAMESPACE}' already exists"
    else
        kubectl create namespace "${MONITORING_NAMESPACE}"
        info "Namespace '${MONITORING_NAMESPACE}' created"
    fi
}

# ---------------------------------------------------------------------------
# Deploy vmagent
# ---------------------------------------------------------------------------
deploy_vmagent() {
    info "Deploying vmagent chart..."

    if [ ! -d "${VMAGENT_CHART_PATH}" ]; then
        error "vmagent chart not found at '${VMAGENT_CHART_PATH}'"
        exit 1
    fi

    helm upgrade --install vmagent "${VMAGENT_CHART_PATH}" \
        --namespace "${MONITORING_NAMESPACE}" \
        --wait \
        --timeout "${WAIT_TIMEOUT}"

    info "vmagent deployed successfully"
}

# ---------------------------------------------------------------------------
# Deploy otel-collector
# ---------------------------------------------------------------------------
deploy_otel_collector() {
    info "Deploying otel-collector chart..."

    if [ ! -d "${OTEL_CHART_PATH}" ]; then
        error "otel-collector chart not found at '${OTEL_CHART_PATH}'"
        exit 1
    fi

    helm upgrade --install otel-collector "${OTEL_CHART_PATH}" \
        --namespace "${MONITORING_NAMESPACE}" \
        --wait \
        --timeout "${WAIT_TIMEOUT}"

    info "otel-collector deployed successfully"
}

# ---------------------------------------------------------------------------
# Wait for pods to be ready
# ---------------------------------------------------------------------------
wait_for_pods() {
    info "Waiting for all pods in '${MONITORING_NAMESPACE}' to be ready..."

    if ! kubectl wait --for=condition=ready pod \
        --all \
        --namespace "${MONITORING_NAMESPACE}" \
        --timeout="${WAIT_TIMEOUT}" 2>/dev/null; then
        warn "Some pods may not be ready yet. Checking status..."
    fi

    info "Pod status in '${MONITORING_NAMESPACE}':"
    kubectl get pods -n "${MONITORING_NAMESPACE}" -o wide
}

# ---------------------------------------------------------------------------
# Verify deployments
# ---------------------------------------------------------------------------
verify_deployments() {
    info "Verifying deployments..."

    local all_ready=true

    # Check vmagent
    if kubectl get deployment vmagent -n "${MONITORING_NAMESPACE}" >/dev/null 2>&1; then
        local vmagent_ready
        vmagent_ready=$(kubectl get deployment vmagent -n "${MONITORING_NAMESPACE}" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        if [ "${vmagent_ready}" -ge 1 ] 2>/dev/null; then
            info "✓ vmagent: ${vmagent_ready} replica(s) ready"
        else
            warn "✗ vmagent: not ready (${vmagent_ready} replicas)"
            all_ready=false
        fi
    else
        warn "✗ vmagent deployment not found"
        all_ready=false
    fi

    # Check otel-collector
    if kubectl get deployment otel-collector -n "${MONITORING_NAMESPACE}" >/dev/null 2>&1; then
        local otel_ready
        otel_ready=$(kubectl get deployment otel-collector -n "${MONITORING_NAMESPACE}" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        if [ "${otel_ready}" -ge 1 ] 2>/dev/null; then
            info "✓ otel-collector: ${otel_ready} replica(s) ready"
        else
            warn "✗ otel-collector: not ready (${otel_ready} replicas)"
            all_ready=false
        fi
    else
        warn "✗ otel-collector deployment not found"
        all_ready=false
    fi

    if [ "${all_ready}" = true ]; then
        info "All monitoring deployments are ready"
    else
        warn "Some monitoring deployments are not ready. Check pod logs for details."
    fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    info "============================================="
    info "  Monitoring Stack Deployment"
    info "============================================="
    info "Namespace:       ${MONITORING_NAMESPACE}"
    info "vmagent chart:   ${VMAGENT_CHART_PATH}"
    info "otel chart:      ${OTEL_CHART_PATH}"
    echo

    check_prerequisites
    create_namespace
    deploy_vmagent
    deploy_otel_collector
    wait_for_pods
    verify_deployments

    echo
    info "============================================="
    info "  Monitoring stack deployment complete!"
    info "============================================="
}

main "$@"
