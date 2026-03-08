#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# publish-dashboards.sh - Upload Grafana dashboards via HTTP API
# =============================================================================
# This script uploads all JSON dashboard files from the monitoring/grafana/
# dashboards directory to the Grafana instance running in docker-compose.
# It is idempotent and safe to run multiple times.
# =============================================================================

# Configuration with sensible defaults
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"
GRAFANA_USER="${GRAFANA_USER:-admin}"
GRAFANA_PASSWORD="${GRAFANA_PASSWORD:-admin}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DASHBOARDS_DIR="${DASHBOARDS_DIR:-${PROJECT_ROOT}/monitoring/grafana/dashboards}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; }

# ---------------------------------------------------------------------------
# Wait for Grafana to be ready
# ---------------------------------------------------------------------------
wait_for_grafana() {
    info "Waiting for Grafana to be ready at ${GRAFANA_URL}..."
    local retries=30
    local count=0
    until curl -sf "${GRAFANA_URL}/api/health" >/dev/null 2>&1; do
        count=$((count + 1))
        if [ "${count}" -ge "${retries}" ]; then
            error "Grafana did not become ready after ${retries} attempts"
            exit 1
        fi
        warn "Grafana not ready yet (attempt ${count}/${retries}), waiting..."
        sleep 2
    done
    info "Grafana is ready"
}

# ---------------------------------------------------------------------------
# Upload a single dashboard
# ---------------------------------------------------------------------------
upload_dashboard() {
    local file="$1"
    local filename
    filename=$(basename "${file}")

    info "Uploading dashboard: ${filename}..."

    # Wrap the dashboard JSON in the required API payload
    local payload
    payload=$(jq -c '{
        dashboard: .,
        overwrite: true,
        message: "Uploaded by publish-dashboards.sh"
    }' < "${file}")

    local response
    local http_code
    http_code=$(curl -sf -o /tmp/grafana_response.json -w "%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
        -d "${payload}" \
        "${GRAFANA_URL}/api/dashboards/db" 2>/dev/null || echo "000")

    if [ "${http_code}" = "200" ]; then
        local dashboard_url
        dashboard_url=$(jq -r '.url // "unknown"' /tmp/grafana_response.json 2>/dev/null || echo "unknown")
        info "✓ ${filename} uploaded successfully (URL: ${GRAFANA_URL}${dashboard_url})"
    else
        response=$(cat /tmp/grafana_response.json 2>/dev/null || echo "no response")
        error "✗ ${filename} upload failed (HTTP ${http_code}): ${response}"
        return 1
    fi
}

# ---------------------------------------------------------------------------
# Upload all dashboards
# ---------------------------------------------------------------------------
upload_all_dashboards() {
    if [ ! -d "${DASHBOARDS_DIR}" ]; then
        warn "Dashboards directory not found at '${DASHBOARDS_DIR}'"
        warn "No dashboards to upload. Create JSON dashboard files in:"
        warn "  ${DASHBOARDS_DIR}/"
        return 0
    fi

    local dashboard_files
    dashboard_files=$(find "${DASHBOARDS_DIR}" -name "*.json" -type f 2>/dev/null || true)

    if [ -z "${dashboard_files}" ]; then
        warn "No JSON dashboard files found in '${DASHBOARDS_DIR}'"
        return 0
    fi

    local total=0
    local success=0
    local failed=0

    while IFS= read -r file; do
        total=$((total + 1))
        if upload_dashboard "${file}"; then
            success=$((success + 1))
        else
            failed=$((failed + 1))
        fi
    done <<< "${dashboard_files}"

    echo
    info "Dashboard upload summary: ${success}/${total} succeeded, ${failed} failed"
}

# ---------------------------------------------------------------------------
# Verify dashboards are accessible
# ---------------------------------------------------------------------------
verify_dashboards() {
    info "Verifying dashboards are accessible..."

    local response
    response=$(curl -sf \
        -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
        "${GRAFANA_URL}/api/search?type=dash-db" 2>/dev/null || echo "[]")

    local count
    count=$(echo "${response}" | jq 'length' 2>/dev/null || echo "0")

    info "Total dashboards in Grafana: ${count}"

    if [ "${count}" -gt 0 ]; then
        echo "${response}" | jq -r '.[] | "  - \(.title) (\(.uri))"' 2>/dev/null || true
    fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    info "============================================="
    info "  Grafana Dashboard Publisher"
    info "============================================="
    info "Grafana URL:      ${GRAFANA_URL}"
    info "Dashboards dir:   ${DASHBOARDS_DIR}"
    echo

    wait_for_grafana
    upload_all_dashboards
    verify_dashboards

    echo
    info "============================================="
    info "  Dashboard publishing complete!"
    info "============================================="
}

main "$@"
