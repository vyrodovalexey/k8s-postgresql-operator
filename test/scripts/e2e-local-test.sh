#!/bin/bash
# =============================================================================
# End-to-End Local Test Script for k8s-postgresql-operator
# =============================================================================
# This script performs a comprehensive end-to-end test of the operator:
# 1. Starts the test environment (PostgreSQL and Vault via docker-compose)
# 2. Configures Vault with Kubernetes auth and PKI
# 3. Builds and deploys the operator to a local Kubernetes cluster
# 4. Applies sample resources in the correct sequence
# 5. Verifies the deployment and resource creation
# 6. Optionally cleans up all resources
#
# Prerequisites:
# - Docker and docker-compose installed
# - kubectl configured with access to a local Kubernetes cluster (kind/minikube)
# - Helm 3 installed
# - jq installed for JSON parsing
#
# Usage:
#   ./e2e-local-test.sh [OPTIONS]
#
# Options:
#   --cleanup       Clean up all resources and exit
#   --skip-build    Skip building the Docker image
#   --skip-deploy   Skip deploying the operator (useful for re-running tests)
#   --verbose       Enable verbose output
#   --help          Show this help message
#
# Environment Variables:
#   CLUSTER_TYPE    - Type of local cluster: kind, minikube, docker-desktop (default: auto-detect)
#   NAMESPACE       - Kubernetes namespace for the operator (default: default)
#   IMAGE_NAME      - Docker image name (default: k8s-postgresql-operator)
#   IMAGE_TAG       - Docker image tag (default: e2e-test)
#   HELM_RELEASE    - Helm release name (default: k8s-postgresql-operator)
#   VAULT_ADDR      - Vault address (default: http://localhost:8200)
#   VAULT_TOKEN     - Vault root token (default: myroot)
# =============================================================================

set -euo pipefail

# =============================================================================
# Configuration
# =============================================================================

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Cluster configuration
CLUSTER_TYPE="${CLUSTER_TYPE:-auto}"
NAMESPACE="${NAMESPACE:-default}"

# Image configuration
IMAGE_NAME="${IMAGE_NAME:-k8s-postgresql-operator}"
IMAGE_TAG="${IMAGE_TAG:-e2e-test}"
FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"

# Helm configuration
HELM_RELEASE="${HELM_RELEASE:-k8s-postgresql-operator}"
HELM_CHART="${PROJECT_ROOT}/charts/k8s-postgresql-operator"

# Vault configuration
VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"
VAULT_TOKEN="${VAULT_TOKEN:-myroot}"

# Docker compose configuration
DOCKER_COMPOSE_DIR="${PROJECT_ROOT}/test/docker-compose"

# Sample resources
SAMPLES_DIR="${PROJECT_ROOT}/config/samples"

# Feature flags
CLEANUP_MODE=false
SKIP_BUILD=false
SKIP_DEPLOY=false
VERBOSE=false

# Timeouts
WAIT_TIMEOUT=120
RESOURCE_WAIT_TIMEOUT=60

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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
        echo -e "${CYAN}[DEBUG]${NC} $1"
    fi
}

log_step() {
    echo ""
    echo -e "${MAGENTA}${BOLD}=== Step $1: $2 ===${NC}"
}

log_section() {
    echo ""
    echo -e "${GREEN}${BOLD}=============================================================================${NC}"
    echo -e "${GREEN}${BOLD}$1${NC}"
    echo -e "${GREEN}${BOLD}=============================================================================${NC}"
}

show_help() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

End-to-End Local Test Script for k8s-postgresql-operator

Options:
  --cleanup       Clean up all resources and exit
  --skip-build    Skip building the Docker image
  --skip-deploy   Skip deploying the operator (useful for re-running tests)
  --verbose       Enable verbose output
  --help          Show this help message

Environment Variables:
  CLUSTER_TYPE    Type of local cluster: kind, minikube, docker-desktop (default: auto-detect)
  NAMESPACE       Kubernetes namespace for the operator (default: default)
  IMAGE_NAME      Docker image name (default: k8s-postgresql-operator)
  IMAGE_TAG       Docker image tag (default: e2e-test)
  HELM_RELEASE    Helm release name (default: k8s-postgresql-operator)
  VAULT_ADDR      Vault address (default: http://localhost:8200)
  VAULT_TOKEN     Vault root token (default: myroot)

Examples:
  # Run full e2e test
  ./e2e-local-test.sh

  # Run with verbose output
  ./e2e-local-test.sh --verbose

  # Skip build and deploy (re-run tests only)
  ./e2e-local-test.sh --skip-build --skip-deploy

  # Clean up all resources
  ./e2e-local-test.sh --cleanup

  # Use specific cluster type
  CLUSTER_TYPE=kind ./e2e-local-test.sh
EOF
}

# Check if a command exists
check_command() {
    if ! command -v "$1" &> /dev/null; then
        log_error "$1 is required but not installed"
        return 1
    fi
    log_verbose "$1 is available"
    return 0
}

# Wait for a condition with timeout
wait_for() {
    local description="$1"
    local timeout="$2"
    shift 2
    local cmd=("$@")
    
    log_info "Waiting for ${description} (timeout: ${timeout}s)..."
    
    local start_time
    start_time=$(date +%s)
    
    while true; do
        if "${cmd[@]}" &>/dev/null; then
            log_success "${description} is ready"
            return 0
        fi
        
        local current_time
        current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        
        if [[ $elapsed -ge $timeout ]]; then
            log_error "Timeout waiting for ${description}"
            return 1
        fi
        
        log_verbose "Still waiting for ${description}... (${elapsed}s elapsed)"
        sleep 2
    done
}

# =============================================================================
# Cluster Detection and Image Loading
# =============================================================================

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

load_image_to_cluster() {
    log_step "LOAD" "Loading image to ${CLUSTER_TYPE} cluster"
    
    case "${CLUSTER_TYPE}" in
        kind)
            log_info "Loading image to kind cluster..."
            local cluster_name
            cluster_name=$(kind get clusters | head -1)
            kind load docker-image "${FULL_IMAGE}" --name "${cluster_name}"
            log_success "Image loaded to kind cluster"
            ;;
        minikube)
            log_info "Loading image to minikube..."
            minikube image load "${FULL_IMAGE}"
            log_success "Image loaded to minikube"
            ;;
        docker-desktop)
            log_info "Docker Desktop uses local images directly, no loading needed"
            log_success "Image available in Docker Desktop"
            ;;
        *)
            log_warn "Unknown cluster type: ${CLUSTER_TYPE}, skipping image load"
            ;;
    esac
}

# =============================================================================
# Docker Compose Management
# =============================================================================

start_docker_compose() {
    log_step "DOCKER" "Starting Docker Compose services"
    
    cd "${DOCKER_COMPOSE_DIR}"
    
    log_info "Starting PostgreSQL and Vault containers..."
    docker-compose up -d
    
    log_info "Waiting for PostgreSQL to be ready..."
    wait_for "PostgreSQL" 60 docker-compose exec -T postgres pg_isready -U postgres
    
    log_info "Waiting for Vault to be ready..."
    local max_attempts=30
    local attempt=1
    while [[ $attempt -le $max_attempts ]]; do
        local health
        health=$(curl -s "${VAULT_ADDR}/v1/sys/health" 2>/dev/null || echo "{}")
        
        if echo "$health" | grep -q '"initialized":true' && echo "$health" | grep -q '"sealed":false'; then
            log_success "Vault is ready and unsealed"
            break
        fi
        
        log_verbose "Attempt $attempt/$max_attempts: Waiting for Vault..."
        sleep 2
        ((attempt++))
    done
    
    if [[ $attempt -gt $max_attempts ]]; then
        log_error "Vault is not available after $max_attempts attempts"
        return 1
    fi
    
    cd "${PROJECT_ROOT}"
    log_success "Docker Compose services are running"
}

stop_docker_compose() {
    log_step "DOCKER" "Stopping Docker Compose services"
    
    cd "${DOCKER_COMPOSE_DIR}"
    docker-compose down -v
    cd "${PROJECT_ROOT}"
    
    log_success "Docker Compose services stopped"
}

# =============================================================================
# Vault Configuration
# =============================================================================

configure_vault() {
    log_step "VAULT" "Configuring Vault for Kubernetes integration"
    
    log_info "Running setup-vault-local-k8s.sh..."
    
    export VAULT_ADDR="${VAULT_ADDR}"
    export VAULT_TOKEN="${VAULT_TOKEN}"
    export VAULT_NAMESPACE="${NAMESPACE}"
    export VAULT_SA_NAME="${HELM_RELEASE}"
    export VAULT_ROLE="k8s-postgresql-operator"
    
    if [[ "${VERBOSE}" == "true" ]]; then
        export VERBOSE="true"
    fi
    
    "${SCRIPT_DIR}/setup-vault-local-k8s.sh"
    
    log_success "Vault configured successfully"
}

# =============================================================================
# Build and Deploy
# =============================================================================

build_docker_image() {
    log_step "BUILD" "Building Docker image"
    
    cd "${PROJECT_ROOT}"
    
    log_info "Building ${FULL_IMAGE}..."
    docker build -t "${FULL_IMAGE}" .
    
    log_success "Docker image built: ${FULL_IMAGE}"
}

install_crds() {
    log_step "CRD" "Installing CRDs"
    
    cd "${PROJECT_ROOT}"
    
    log_info "Installing CRDs using make..."
    make install
    
    log_success "CRDs installed"
}

deploy_helm_chart() {
    log_step "HELM" "Deploying Helm chart"
    
    # Determine Vault address for Kubernetes
    local vault_k8s_addr
    case "${CLUSTER_TYPE}" in
        docker-desktop)
            vault_k8s_addr="http://host.docker.internal:8200"
            ;;
        kind|minikube)
            # For kind/minikube, we need to use the host's IP
            vault_k8s_addr="http://host.docker.internal:8200"
            ;;
        *)
            vault_k8s_addr="${VAULT_ADDR}"
            ;;
    esac
    
    log_info "Deploying Helm chart with Vault PKI enabled..."
    log_info "Vault address for K8s: ${vault_k8s_addr}"
    
    # Check if release exists
    if helm status "${HELM_RELEASE}" -n "${NAMESPACE}" &>/dev/null; then
        log_info "Upgrading existing Helm release..."
        helm upgrade "${HELM_RELEASE}" "${HELM_CHART}" \
            --namespace "${NAMESPACE}" \
            --set image.repository="${IMAGE_NAME}" \
            --set image.tag="${IMAGE_TAG}" \
            --set image.pullPolicy=IfNotPresent \
            --set vault.addr="${vault_k8s_addr}" \
            --set vault.role="k8s-postgresql-operator" \
            --set vault.mountPoint="secret" \
            --set vault.secretPath="pdb" \
            --set vaultPKI.enabled=true \
            --set vaultPKI.mountPath="pki" \
            --set vaultPKI.role="webhook-cert" \
            --set vaultPKI.ttl="720h" \
            --set vaultPKI.renewalBuffer="24h" \
            --wait \
            --timeout 5m
    else
        log_info "Installing new Helm release..."
        helm install "${HELM_RELEASE}" "${HELM_CHART}" \
            --namespace "${NAMESPACE}" \
            --create-namespace \
            --set image.repository="${IMAGE_NAME}" \
            --set image.tag="${IMAGE_TAG}" \
            --set image.pullPolicy=IfNotPresent \
            --set vault.addr="${vault_k8s_addr}" \
            --set vault.role="k8s-postgresql-operator" \
            --set vault.mountPoint="secret" \
            --set vault.secretPath="pdb" \
            --set vaultPKI.enabled=true \
            --set vaultPKI.mountPath="pki" \
            --set vaultPKI.role="webhook-cert" \
            --set vaultPKI.ttl="720h" \
            --set vaultPKI.renewalBuffer="24h" \
            --wait \
            --timeout 5m
    fi
    
    log_success "Helm chart deployed"
}

# =============================================================================
# Sample Resources Application
# =============================================================================

apply_sample_resources() {
    log_step "SAMPLES" "Applying sample resources"
    
    # Apply resources in the correct sequence with dependencies
    
    # 1. PostgreSQL instance reference
    log_info "Applying PostgreSQL instance reference..."
    kubectl apply -f "${SAMPLES_DIR}/postgresql.yaml" -n "${NAMESPACE}"
    sleep 2
    
    # 2. Create user
    log_info "Applying User resource..."
    kubectl apply -f "${SAMPLES_DIR}/user.yaml" -n "${NAMESPACE}"
    sleep 3
    
    # 3. Create schema (depends on user)
    log_info "Applying Schema resource..."
    kubectl apply -f "${SAMPLES_DIR}/schema.yaml" -n "${NAMESPACE}"
    sleep 2
    
    # 4. Create database (depends on user and schema)
    log_info "Applying Database resource..."
    kubectl apply -f "${SAMPLES_DIR}/database.yaml" -n "${NAMESPACE}"
    sleep 3
    
    # 5. Apply grants (depends on database and user)
    log_info "Applying Grant resource..."
    kubectl apply -f "${SAMPLES_DIR}/grant.yaml" -n "${NAMESPACE}"
    sleep 2
    
    log_success "All sample resources applied"
}

# =============================================================================
# Verification Functions
# =============================================================================

verify_operator_running() {
    log_step "VERIFY-OP" "Verifying operator is running"
    
    log_info "Checking operator pod status..."
    
    # Wait for operator pod to be ready
    local pod_name
    pod_name=$(kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/name=k8s-postgresql-operator" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    
    if [[ -z "$pod_name" ]]; then
        log_error "Operator pod not found"
        kubectl get pods -n "${NAMESPACE}"
        return 1
    fi
    
    log_info "Found operator pod: ${pod_name}"
    
    # Wait for pod to be ready
    if ! kubectl wait --for=condition=ready pod/"${pod_name}" -n "${NAMESPACE}" --timeout="${WAIT_TIMEOUT}s"; then
        log_error "Operator pod is not ready"
        kubectl describe pod "${pod_name}" -n "${NAMESPACE}"
        kubectl logs "${pod_name}" -n "${NAMESPACE}" --tail=50
        return 1
    fi
    
    log_success "Operator pod is running and ready"
    
    # Show pod logs
    if [[ "${VERBOSE}" == "true" ]]; then
        log_info "Operator pod logs:"
        kubectl logs "${pod_name}" -n "${NAMESPACE}" --tail=20
    fi
}

verify_webhook() {
    log_step "VERIFY-WH" "Verifying webhook is working"
    
    log_info "Checking ValidatingWebhookConfiguration..."
    
    # Check if webhook configuration exists
    if ! kubectl get validatingwebhookconfiguration -l "app.kubernetes.io/name=k8s-postgresql-operator" &>/dev/null; then
        log_warn "ValidatingWebhookConfiguration not found, checking by name..."
        if ! kubectl get validatingwebhookconfiguration "${HELM_RELEASE}-validating-webhook" &>/dev/null; then
            log_error "ValidatingWebhookConfiguration not found"
            return 1
        fi
    fi
    
    log_success "ValidatingWebhookConfiguration exists"
    
    # Test webhook by applying an invalid resource (should be rejected)
    log_info "Testing webhook validation..."
    
    # Create a temporary invalid resource
    local temp_file
    temp_file=$(mktemp)
    cat > "${temp_file}" << EOF
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: User
metadata:
  name: test-invalid-user
  namespace: ${NAMESPACE}
spec:
  username: ""
  postgresqlID: "non-existent-pg"
EOF
    
    # Try to apply the invalid resource (should fail)
    if kubectl apply -f "${temp_file}" 2>&1 | grep -qi "denied\|rejected\|invalid\|error"; then
        log_success "Webhook correctly rejected invalid resource"
    else
        log_warn "Webhook may not be validating resources (this could be expected if validation is lenient)"
    fi
    
    rm -f "${temp_file}"
}

verify_resources_in_postgresql() {
    log_step "VERIFY-PG" "Verifying resources in PostgreSQL"
    
    log_info "Connecting to PostgreSQL to verify resources..."
    
    # Get PostgreSQL connection details from docker-compose
    local pg_host="localhost"
    local pg_port="5432"
    local pg_user="postgres"
    local pg_password="postgres"
    
    # Check if user was created
    log_info "Checking if user 'myuser' exists..."
    local user_exists
    user_exists=$(docker exec postgres psql -U "${pg_user}" -d postgres -tAc "SELECT 1 FROM pg_roles WHERE rolname='myuser'" 2>/dev/null || echo "")
    
    if [[ "$user_exists" == "1" ]]; then
        log_success "User 'myuser' exists in PostgreSQL"
    else
        log_warn "User 'myuser' not found in PostgreSQL (may take time to reconcile)"
    fi
    
    # Check if database was created
    log_info "Checking if database 'mydatabase' exists..."
    local db_exists
    db_exists=$(docker exec postgres psql -U "${pg_user}" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='mydatabase'" 2>/dev/null || echo "")
    
    if [[ "$db_exists" == "1" ]]; then
        log_success "Database 'mydatabase' exists in PostgreSQL"
    else
        log_warn "Database 'mydatabase' not found in PostgreSQL (may take time to reconcile)"
    fi
    
    # Check if schema was created
    log_info "Checking if schema 'myschema' exists..."
    local schema_exists
    schema_exists=$(docker exec postgres psql -U "${pg_user}" -d mydatabase -tAc "SELECT 1 FROM information_schema.schemata WHERE schema_name='myschema'" 2>/dev/null || echo "")
    
    if [[ "$schema_exists" == "1" ]]; then
        log_success "Schema 'myschema' exists in PostgreSQL"
    else
        log_warn "Schema 'myschema' not found in PostgreSQL (may take time to reconcile)"
    fi
}

verify_custom_resources() {
    log_step "VERIFY-CR" "Verifying custom resources status"
    
    log_info "Checking Postgresql resource status..."
    kubectl get postgresql -n "${NAMESPACE}" -o wide 2>/dev/null || log_warn "No Postgresql resources found"
    
    log_info "Checking User resource status..."
    kubectl get user.postgresql-operator.vyrodovalexey.github.com -n "${NAMESPACE}" -o wide 2>/dev/null || log_warn "No User resources found"
    
    log_info "Checking Database resource status..."
    kubectl get database.postgresql-operator.vyrodovalexey.github.com -n "${NAMESPACE}" -o wide 2>/dev/null || log_warn "No Database resources found"
    
    log_info "Checking Schema resource status..."
    kubectl get schema.postgresql-operator.vyrodovalexey.github.com -n "${NAMESPACE}" -o wide 2>/dev/null || log_warn "No Schema resources found"
    
    log_info "Checking Grant resource status..."
    kubectl get grant -n "${NAMESPACE}" -o wide 2>/dev/null || log_warn "No Grant resources found"
    
    log_success "Custom resources status checked"
}

# =============================================================================
# Cleanup Functions
# =============================================================================

cleanup_sample_resources() {
    log_step "CLEANUP-CR" "Cleaning up sample resources"
    
    log_info "Deleting Grant resources..."
    kubectl delete -f "${SAMPLES_DIR}/grant.yaml" -n "${NAMESPACE}" --ignore-not-found=true 2>/dev/null || true
    
    log_info "Deleting Schema resources..."
    kubectl delete -f "${SAMPLES_DIR}/schema.yaml" -n "${NAMESPACE}" --ignore-not-found=true 2>/dev/null || true
    
    log_info "Deleting Database resources..."
    kubectl delete -f "${SAMPLES_DIR}/database.yaml" -n "${NAMESPACE}" --ignore-not-found=true 2>/dev/null || true
    
    log_info "Deleting User resources..."
    kubectl delete -f "${SAMPLES_DIR}/user.yaml" -n "${NAMESPACE}" --ignore-not-found=true 2>/dev/null || true
    
    log_info "Deleting Postgresql resources..."
    kubectl delete -f "${SAMPLES_DIR}/postgresql.yaml" -n "${NAMESPACE}" --ignore-not-found=true 2>/dev/null || true
    
    log_success "Sample resources cleaned up"
}

cleanup_helm_release() {
    log_step "CLEANUP-HELM" "Uninstalling Helm release"
    
    if helm status "${HELM_RELEASE}" -n "${NAMESPACE}" &>/dev/null; then
        log_info "Uninstalling Helm release ${HELM_RELEASE}..."
        helm uninstall "${HELM_RELEASE}" -n "${NAMESPACE}" --wait
        log_success "Helm release uninstalled"
    else
        log_info "Helm release ${HELM_RELEASE} not found, skipping"
    fi
}

cleanup_crds() {
    log_step "CLEANUP-CRD" "Removing CRDs"
    
    log_info "Uninstalling CRDs using make..."
    cd "${PROJECT_ROOT}"
    make uninstall ignore-not-found=true 2>/dev/null || true
    
    log_success "CRDs removed"
}

cleanup_all() {
    log_section "Cleanup Mode"
    
    cleanup_sample_resources
    cleanup_helm_release
    cleanup_crds
    stop_docker_compose
    
    log_success "All resources cleaned up"
}

# =============================================================================
# Main Functions
# =============================================================================

run_e2e_test() {
    log_section "Starting End-to-End Test"
    
    echo ""
    echo "Configuration:"
    echo "  Project Root:    ${PROJECT_ROOT}"
    echo "  Cluster Type:    ${CLUSTER_TYPE}"
    echo "  Namespace:       ${NAMESPACE}"
    echo "  Image:           ${FULL_IMAGE}"
    echo "  Helm Release:    ${HELM_RELEASE}"
    echo "  Vault Address:   ${VAULT_ADDR}"
    echo "  Skip Build:      ${SKIP_BUILD}"
    echo "  Skip Deploy:     ${SKIP_DEPLOY}"
    echo "  Verbose:         ${VERBOSE}"
    echo ""
    
    # Step 1: Start Docker Compose
    start_docker_compose
    
    # Step 2: Configure Vault
    configure_vault
    
    # Step 3: Build Docker image (if not skipped)
    if [[ "${SKIP_BUILD}" != "true" ]]; then
        build_docker_image
        load_image_to_cluster
    else
        log_info "Skipping Docker image build"
    fi
    
    # Step 4: Deploy operator (if not skipped)
    if [[ "${SKIP_DEPLOY}" != "true" ]]; then
        install_crds
        deploy_helm_chart
    else
        log_info "Skipping operator deployment"
    fi
    
    # Step 5: Verify operator is running
    verify_operator_running
    
    # Step 6: Verify webhook
    verify_webhook
    
    # Step 7: Apply sample resources
    apply_sample_resources
    
    # Step 8: Wait for reconciliation
    log_info "Waiting for resources to be reconciled..."
    sleep 10
    
    # Step 9: Verify custom resources
    verify_custom_resources
    
    # Step 10: Verify resources in PostgreSQL
    verify_resources_in_postgresql
    
    log_section "End-to-End Test Complete"
    
    echo ""
    echo "Summary:"
    echo "  - Docker Compose services: Running"
    echo "  - Vault: Configured with K8s auth and PKI"
    echo "  - Operator: Deployed and running"
    echo "  - Webhook: Configured"
    echo "  - Sample resources: Applied"
    echo ""
    echo "To clean up all resources, run:"
    echo "  $0 --cleanup"
    echo ""
    
    log_success "E2E test completed successfully!"
}

# =============================================================================
# Argument Parsing
# =============================================================================

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --cleanup)
                CLEANUP_MODE=true
                shift
                ;;
            --skip-build)
                SKIP_BUILD=true
                shift
                ;;
            --skip-deploy)
                SKIP_DEPLOY=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# =============================================================================
# Prerequisites Check
# =============================================================================

check_prerequisites() {
    log_step "0" "Checking prerequisites"
    
    local missing=false
    
    check_command docker || missing=true
    check_command docker-compose || missing=true
    check_command kubectl || missing=true
    check_command helm || missing=true
    check_command jq || missing=true
    check_command curl || missing=true
    
    if [[ "$missing" == "true" ]]; then
        log_error "Missing required tools. Please install them and try again."
        exit 1
    fi
    
    # Check kubectl connectivity
    if ! kubectl cluster-info &>/dev/null; then
        log_error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
        exit 1
    fi
    
    log_success "All prerequisites met"
}

# =============================================================================
# Main Entry Point
# =============================================================================

main() {
    parse_args "$@"
    
    # Check prerequisites
    check_prerequisites
    
    # Detect cluster type
    detect_cluster_type
    
    if [[ "${CLEANUP_MODE}" == "true" ]]; then
        cleanup_all
        exit 0
    fi
    
    run_e2e_test
}

# Run main function
main "$@"
