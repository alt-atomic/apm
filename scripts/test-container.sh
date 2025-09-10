#!/bin/bash
# Test runner script for containerized testing using pre-built container
# Usage: ./scripts/test-container.sh [test-suite]
# Usage: ./scripts/test-container.sh exec - to enter container interactively

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
    rsync -av --exclude='.cache' /workspace/ /tmp/apm-src/ && \
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

# Handle exec mode for interactive container access
if [ "${TEST_SUITE}" = "exec" ]; then
    print_info "🚀 Entering container interactively as root..."
    print_info "Container has been built and is ready for manual testing."
    print_info "Your project is available at /tmp/apm-src (built) and /workspace (source)"
    print_info "Use 'exit' to leave the container"
    
    # Remove the cleanup trap for exec mode so container stays running
    trap - EXIT
    
    # Enter container interactively as root
    if podman exec -it --user root "${CONTAINER_NAME}" bash -c "cd /tmp/apm-src && exec bash"; then
        print_success "Exited container successfully"
    else
        print_warning "Container session ended"
    fi
    
    # Manual cleanup for exec mode
    print_info "🧹 Cleaning up container ${CONTAINER_NAME} ..."
    podman rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
    exit 0
fi

# Run different test suites based on parameter
case "${TEST_SUITE}" in
    "system")
        print_info "Running system tests only..."
        podman exec --user root "${CONTAINER_NAME}" bash -c "
            cd /tmp/apm-src && \
            export GOCACHE=/tmp/go-cache && \
            export GOMODCACHE=/tmp/go-mod && \
            export GO111MODULE=on && \
            meson test -C builddir --suite system --verbose
        "
        ;;
    "apt")
        print_info "Running APT binding tests only..."
        podman exec --user root "${CONTAINER_NAME}" bash -c "
            cd /tmp/apm-src && \
            export GOCACHE=/tmp/go-cache && \
            export GOMODCACHE=/tmp/go-mod && \
            export GO111MODULE=on && \
            meson test -C builddir --suite apt --verbose
        "
        ;;
    *)
        print_error "Unknown test suite: ${TEST_SUITE}"
        print_info "Available test suites: system, apt, exec"
        exit 1
        ;;
esac

# Cleanup function
cleanup() {
    print_info "🧹 Cleaning up container ${CONTAINER_NAME} ..."
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
