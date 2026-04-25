#!/bin/bash
# Test runner script for containerized testing using pre-built container
# Builds and installs APM inside container, then runs go test directly (no meson)
# Usage: ./scripts/test-container.sh [test-suite]
# Test suites: integration, all, exec
# Usage: ./scripts/test-container.sh exec - to enter container interactively

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CONTAINER_NAME="${CONTAINER_NAME:-apm-test}"
IMAGE="${1:-registry.altlinux.org/alt/alt:sisyphus}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

print_info() {
    print_status "$BLUE" "ℹ️  $1"
}

print_success() {
    print_status "$GREEN" "✅ $1"
}

print_warning() {
    print_status "$YELLOW" "⚠️  $1"
}

print_error() {
    print_status "$RED" "❌ $1"
}

# Check if podman is available
if ! command -v podman &> /dev/null; then
    print_error "Podman not found. Please install it."
    exit 1
fi

# Check if meson is available
if ! command -v meson &> /dev/null; then
    print_error "Meson not found. Please install it."
    exit 1
fi

TMPDIR=$(mktemp -d)

# Cleanup function
cleanup() {
    print_info "Cleaning up container ${CONTAINER_NAME} ..."
    podman stop -t 0 "${CONTAINER_NAME}" >/dev/null 2>&1 || true
    rm -rf "$TMPDIR"
}

# Set trap for cleanup
trap cleanup EXIT

# Create test container with necessary mounts
if ! podman run --rm -d \
    --name "${CONTAINER_NAME}" \
    --privileged \
    --replace \
    --pull=newer \
    "${IMAGE}" \
    sleep infinity; then
    print_error "Failed to start container"
    exit 1
fi

print_success "Container started: ${CONTAINER_NAME}"

print_info "Compiling APM..."

if [ ! -d "$PROJECT_ROOT/_build" ]; then
    meson setup -C _build --prefix=/usr -Dprofile=dev
fi

meson install --destdir "$TMPDIR" -C "$PROJECT_ROOT/_build"

print_info "Copy APM to container..."

# Copy source, build and install inside container
if ! podman cp "$TMPDIR"/. "${CONTAINER_NAME}:/tmp/stage"; then
    print_error "Build and install failed in container"
    exit 1
fi
podman exec "${CONTAINER_NAME}" sh -c "cp -rfL /tmp/stage/* /" || true

# Remove the cleanup trap for exec mode so container stays running
trap - EXIT

# Enter container interactively as root
if podman exec -it --user root "${CONTAINER_NAME}" bash; then
    print_success "Exited container successfully"
else
    print_warning "Container session ended"
fi

# Manual cleanup for exec mode
cleanup
