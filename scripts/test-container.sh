#!/bin/bash
# Test runner script for containerized testing using pre-built container
# Usage: ./scripts/test-container.sh [test-suite]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CONTAINER_IMAGE="${CONTAINER_IMAGE:-apm-test:latest}"
CONTAINER_NAME="${CONTAINER_NAME:-apm-test-$(date +%s)}"
TEST_SUITE="${1:-all}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# Function to check if container image exists
check_container_image() {
    if ! podman image exists "${CONTAINER_IMAGE}"; then
        print_warning "Container image ${CONTAINER_IMAGE} not found"
        print_info "Building container image from Dockerfile.test..."
        
        if ! podman build -f "${PROJECT_ROOT}/Dockerfile.test" -t "${CONTAINER_IMAGE}" "${PROJECT_ROOT}"; then
            print_error "Failed to build container image"
            return 1
        fi
        
        print_success "Container image built successfully"
    else
        print_info "Using existing container image: ${CONTAINER_IMAGE}"
    fi
}

print_info "APM Containerized Test Runner"
print_info "Test suite: ${TEST_SUITE}"

# Check if podman is available
if ! command -v podman &> /dev/null; then
    print_error "Podman not found. Please install podman."
    exit 1
fi

# Check/build container image
check_container_image

print_info "🐳 Starting test container..."

# Create test container with necessary mounts
if ! podman run -d \
    --name "${CONTAINER_NAME}" \
    --privileged \
    --security-opt label=disable \
    -v "${PROJECT_ROOT}:/workspace:Z" \
    -w /workspace \
    -e TEST_SUITE="${TEST_SUITE}" \
    "${CONTAINER_IMAGE}" \
    sleep infinity; then
    print_error "Failed to start container"
    exit 1
fi

print_success "Container started: ${CONTAINER_NAME}"

print_info "🔨 Building project in container..."

# Setup build directory and compile
if ! podman exec "${CONTAINER_NAME}" bash -c "
    # Copy source to tmp and build there to avoid permission issues
    cp -r /workspace /tmp/apm-src && \
    cd /tmp/apm-src && \
    # Set Go environment for build
    export GOCACHE=/tmp/go-cache && \
    export GOMODCACHE=/tmp/go-mod && \
    export GO111MODULE=on && \
    meson setup builddir --prefix=/usr --buildtype=debug --wipe && \
    meson compile -C builddir
"; then
    print_error "Build failed in container"
    exit 1
fi

print_success "Project built successfully"

print_info "🧪 Running tests..."

# Run different test suites based on parameter
case "${TEST_SUITE}" in
    "unit")
        print_info "Running unit tests only..."
        podman exec "${CONTAINER_NAME}" bash -c "
            cd /tmp/apm-src && \
            export GOCACHE=/tmp/go-cache && \
            export GOMODCACHE=/tmp/go-mod && \
            export GO111MODULE=on && \
            meson test -C builddir --suite unit --verbose
        "
        ;;
    "system")
        print_info "Running system tests only..."
        podman exec "${CONTAINER_NAME}" bash -c "
            cd /tmp/apm-src && \
            export GOCACHE=/tmp/go-cache && \
            export GOMODCACHE=/tmp/go-mod && \
            export GO111MODULE=on && \
            meson test -C builddir --suite integration --verbose
        "
        ;;
    "apt")
        print_info "Running APT binding tests only..."
        podman exec "${CONTAINER_NAME}" bash -c "
            cd /tmp/apm-src && \
            export GOCACHE=/tmp/go-cache && \
            export GOMODCACHE=/tmp/go-mod && \
            export GO111MODULE=on && \
            meson test -C builddir --suite apt --verbose
        "
        ;;
    "safe")
        print_info "Running safe tests (unit + apt + system, excluding distrobox and integration)..."
        podman exec "${CONTAINER_NAME}" bash -c "
            cd /tmp/apm-src && \
            export GOCACHE=/tmp/go-cache && \
            export GOMODCACHE=/tmp/go-mod && \
            export GO111MODULE=on && \
            meson test -C builddir --suite unit --verbose && \
            meson test -C builddir --suite apt --verbose && \
            meson test -C builddir --suite integration --verbose
        "
        ;;
    "all")
        print_warning "Running ALL tests including distrobox (may fail in container)..."
        podman exec "${CONTAINER_NAME}" bash -c "
            cd /tmp/apm-src && \
            export GOCACHE=/tmp/go-cache && \
            export GOMODCACHE=/tmp/go-mod && \
            export GO111MODULE=on && \
            meson test -C builddir --verbose
        "
        ;;
    *)
        print_error "Unknown test suite: ${TEST_SUITE}"
        print_info "Available test suites: unit, system, apt, safe, all"
        exit 1
        ;;
esac

# Cleanup function
cleanup() {
    print_info "🧹 Cleaning up container..."
    podman rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
}

# Set trap for cleanup
trap cleanup EXIT

# Check test results
TEST_EXIT_CODE=$?
if [ $TEST_EXIT_CODE -eq 0 ]; then
    print_success "All tests completed successfully!"
else
    print_error "Some tests failed (exit code: $TEST_EXIT_CODE)"
fi

print_info "Test summary:"
print_info "- Container image: ${CONTAINER_IMAGE}"
print_info "- Test suite: ${TEST_SUITE}"
print_info "- Container name: ${CONTAINER_NAME}"

exit $TEST_EXIT_CODE
