#!/bin/bash
# Test runner script for containerized testing
# Usage: ./scripts/test-container.sh [test-suite]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CONTAINER_IMAGE="${CONTAINER_IMAGE:-registry.altlinux.org/sisyphus/base:latest}"
CONTAINER_NAME="${CONTAINER_NAME:-apm-test-$(date +%s)}"
TEST_SUITE="${1:-all}"

echo "🐳 Setting up test container..."

# Create test container with necessary mounts and privileges
podman run -d \
    --name "${CONTAINER_NAME}" \
    --privileged \
    --security-opt label=disable \
    -v "${PROJECT_ROOT}:/workspace:Z" \
    -w /workspace \
    "${CONTAINER_IMAGE}" \
    sleep infinity

echo "📦 Installing dependencies in container..."

# Install build dependencies
podman exec "${CONTAINER_NAME}" bash -c "
    apt-get update -qq
    apt-get install -y \
        build-essential \
        meson \
        ninja-build \
        systemd-devel \
        golang \
        libapt-devel \
        apt-utils \
        gcc \
        git \
        ca-certificates
"

echo "🔨 Building project in container..."

# Setup build directory and compile
podman exec "${CONTAINER_NAME}" bash -c "
    cd /workspace
    meson setup builddir --prefix=/usr --buildtype=debug
    meson compile -C builddir
"

echo "🧪 Running tests..."

# Run different test suites based on parameter
case "${TEST_SUITE}" in
    "unit")
        echo "Running unit tests only..."
        podman exec "${CONTAINER_NAME}" bash -c "
            cd /workspace
            meson test -C builddir --suite unit --verbose
        "
        ;;
    "integration")
        echo "Running integration tests only..."
        podman exec "${CONTAINER_NAME}" bash -c "
            cd /workspace
            meson test -C builddir --suite integration --verbose
        "
        ;;
    "apt")
        echo "Running APT binding tests only..."
        podman exec "${CONTAINER_NAME}" bash -c "
            cd /workspace
            meson test -C builddir --suite apt --verbose
        "
        ;;
    "all"|*)
        echo "Running all tests..."
        podman exec "${CONTAINER_NAME}" bash -c "
            cd /workspace
            meson test -C builddir --verbose
        "
        ;;
esac

# Cleanup function
cleanup() {
    echo "🧹 Cleaning up container..."
    podman rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
}

# Set trap for cleanup
trap cleanup EXIT

echo "✅ Tests completed!"
