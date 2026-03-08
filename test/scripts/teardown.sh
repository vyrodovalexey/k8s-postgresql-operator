#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# teardown.sh - Clean up all test environment resources
# =============================================================================
# This script tears down the entire test environment including Helm releases,
# Kubernetes namespaces, and docker-compose services. It is idempotent and
# safe to run multiple times.
# =============================================================================

# Configuration with sensible defaults
OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-k8s-postgresql-operator-system}"
MONITORING_NAMESPACE="${MONITORING_NAMESPACE:-monitoring}"
HELM_RELEASE_NAME="${HELM_RELEASE_NAME:-k8s-postgresql-operator}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMPOSE_DIR="${COMPOSE_DIR:-${PROJECT_ROOT}/test/docker-compose}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; }

# ---------------------------------------------------------------------------
# Uninstall operator Helm release
# ---------------------------------------------------------------------------
uninstall_operator() {
    info "Uninstalling operator Helm release '${HELM_RELEASE_NAME}'..."
    if helm status "${HELM_RELEASE_NAME}" -n "${OPERATOR_NAMESPACE}" >/dev/null 2>&1; then
        helm uninstall "${HELM_RELEASE_NAME}" -n "${OPERATOR_NAMESPACE}" --wait
        info "Operator Helm release uninstalled"
    else
        info "Operator Helm release '${HELM_RELEASE_NAME}' not found, skipping"
    fi
}

# ---------------------------------------------------------------------------
# Delete operator namespace
# ---------------------------------------------------------------------------
delete_operator_namespace() {
    info "Deleting operator namespace '${OPERATOR_NAMESPACE}'..."

    # Clean up ClusterRoleBinding created by vault-k8s-auth setup
    if kubectl get clusterrolebinding vault-auth-tokenreview-binding >/dev/null 2>&1; then
        kubectl delete clusterrolebinding vault-auth-tokenreview-binding --ignore-not-found=true
        info "ClusterRoleBinding 'vault-auth-tokenreview-binding' deleted"
    fi

    if kubectl get namespace "${OPERATOR_NAMESPACE}" >/dev/null 2>&1; then
        kubectl delete namespace "${OPERATOR_NAMESPACE}" --ignore-not-found=true --timeout=60s
        info "Operator namespace deleted"
    else
        info "Operator namespace '${OPERATOR_NAMESPACE}' not found, skipping"
    fi
}

# ---------------------------------------------------------------------------
# Uninstall monitoring Helm releases
# ---------------------------------------------------------------------------
uninstall_monitoring() {
    info "Uninstalling monitoring Helm releases..."

    if helm status vmagent -n "${MONITORING_NAMESPACE}" >/dev/null 2>&1; then
        helm uninstall vmagent -n "${MONITORING_NAMESPACE}" --wait
        info "vmagent Helm release uninstalled"
    else
        info "vmagent Helm release not found, skipping"
    fi

    if helm status otel-collector -n "${MONITORING_NAMESPACE}" >/dev/null 2>&1; then
        helm uninstall otel-collector -n "${MONITORING_NAMESPACE}" --wait
        info "otel-collector Helm release uninstalled"
    else
        info "otel-collector Helm release not found, skipping"
    fi
}

# ---------------------------------------------------------------------------
# Delete monitoring namespace
# ---------------------------------------------------------------------------
delete_monitoring_namespace() {
    info "Deleting monitoring namespace '${MONITORING_NAMESPACE}'..."
    if kubectl get namespace "${MONITORING_NAMESPACE}" >/dev/null 2>&1; then
        kubectl delete namespace "${MONITORING_NAMESPACE}" --ignore-not-found=true --timeout=60s
        info "Monitoring namespace deleted"
    else
        info "Monitoring namespace '${MONITORING_NAMESPACE}' not found, skipping"
    fi
}

# ---------------------------------------------------------------------------
# Tear down docker-compose
# ---------------------------------------------------------------------------
teardown_compose() {
    info "Tearing down docker-compose services..."
    if [ -f "${COMPOSE_DIR}/docker-compose.yml" ]; then
        docker compose -f "${COMPOSE_DIR}/docker-compose.yml" down -v --remove-orphans
        info "Docker-compose services stopped and volumes removed"
    else
        warn "docker-compose.yml not found at '${COMPOSE_DIR}', skipping"
    fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    info "============================================="
    info "  Test Environment Teardown"
    info "============================================="
    info "Operator NS:     ${OPERATOR_NAMESPACE}"
    info "Monitoring NS:   ${MONITORING_NAMESPACE}"
    info "Helm release:    ${HELM_RELEASE_NAME}"
    info "Compose dir:     ${COMPOSE_DIR}"
    echo

    # Kubernetes teardown (only if cluster is reachable)
    if kubectl cluster-info >/dev/null 2>&1; then
        uninstall_operator
        delete_operator_namespace
        uninstall_monitoring
        delete_monitoring_namespace
    else
        warn "Kubernetes cluster not reachable, skipping K8s teardown"
    fi

    # Docker-compose teardown
    teardown_compose

    echo
    info "============================================="
    info "  Teardown complete!"
    info "============================================="
}

main "$@"
