#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CONTAINER_IMAGE="${CONTAINER_IMAGE:-apm-test:latest}"
CONTAINER_NAME="${CONTAINER_NAME:-apm-dbus-e2e-$$}"
REBUILD_IMAGE="${REBUILD_IMAGE:-0}"
KEEP_CONTAINER="${KEEP_CONTAINER:-0}"

run_test() {
    local timeout="$1"
    local test_name="$2"

    podman exec "${CONTAINER_NAME}" /tmp/apm-dbus-e2e.test \
        -test.v \
        -test.timeout="${timeout}" \
        -test.run="^${test_name}$"
}

run_test_as_user() {
    local timeout="$1"
    local test_name="$2"

    podman exec "${CONTAINER_NAME}" setpriv \
        --reuid=testuser \
        --regid=testuser \
        --init-groups \
        /tmp/apm-dbus-e2e.test \
        -test.v \
        -test.timeout="${timeout}" \
        -test.run="^${test_name}$"
}

run_activation_test() {
    local timeout="$1"
    local test_name="$2"

    podman exec --env APM_E2E_EXPECT_ACTIVATION=1 "${CONTAINER_NAME}" /tmp/apm-dbus-e2e.test \
        -test.v \
        -test.timeout="${timeout}" \
        -test.run="^${test_name}$"
}

cleanup() {
    local status=$?

    if [ "${status}" -ne 0 ]; then
        podman logs "${CONTAINER_NAME}" 2>/dev/null || true
        podman exec "${CONTAINER_NAME}" systemctl status apm.service --no-pager 2>/dev/null || true
        podman exec "${CONTAINER_NAME}" journalctl -u apm.service --no-pager 2>/dev/null || true
    fi
    if [ "${KEEP_CONTAINER}" = "1" ]; then
        echo "Keeping container ${CONTAINER_NAME} for debugging"
    else
        podman rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
    fi
}
trap cleanup EXIT

if ! command -v podman >/dev/null 2>&1; then
    echo "podman is required" >&2
    exit 1
fi

if [ "${REBUILD_IMAGE}" = "1" ] || ! podman image exists "${CONTAINER_IMAGE}"; then
    podman build -f "${PROJECT_ROOT}/Dockerfile.test" -t "${CONTAINER_IMAGE}" "${PROJECT_ROOT}"
fi

podman run -d \
    --name "${CONTAINER_NAME}" \
    --privileged \
    --security-opt label=disable \
    --systemd=always \
    --user root \
    -e GOCACHE=/go-cache \
    -e GOMODCACHE=/go-mod \
    -v apm-dbus-e2e-gocache:/go-cache \
    -v apm-dbus-e2e-gomod:/go-mod \
    -v "${PROJECT_ROOT}:/workspace:ro" \
    "${CONTAINER_IMAGE}" \
    /sbin/init

systemd_ready=0
systemd_state="unknown"
for _ in $(seq 1 30); do
    if [ "$(podman inspect --format '{{.State.Running}}' "${CONTAINER_NAME}" 2>/dev/null)" != "true" ]; then
        echo "container ${CONTAINER_NAME} stopped before systemd became ready" >&2
        exit 1
    fi
    systemd_state="$(podman exec "${CONTAINER_NAME}" systemctl is-system-running 2>/dev/null || true)"
    case "${systemd_state}" in
        running|degraded)
            systemd_ready=1
            break
            ;;
    esac
    sleep 1
done
if [ "${systemd_ready}" -ne 1 ]; then
    echo "systemd did not become ready; last state: ${systemd_state:-unknown}" >&2
    exit 1
fi

podman exec "${CONTAINER_NAME}" bash -c '
    set -euo pipefail
    required_commands=(busctl go install meson ninja rpm rsync setpriv systemctl systemd-tmpfiles)
    for command in "${required_commands[@]}"; do
        if ! command -v "${command}" >/dev/null 2>&1; then
            echo "required command is missing in the test image: ${command}" >&2
            exit 1
        fi
    done
    if ! id testuser >/dev/null 2>&1; then
        echo "required user is missing in the test image: testuser" >&2
        exit 1
    fi

    rsync -a \
        --exclude=/.cache/ \
        --exclude=/build/ \
        --exclude=/build-e2e/ \
        --exclude=/builddir/ \
        --exclude=/cmake-build-debug/ \
        --exclude=/pkg/apt/apt-source/ \
        /workspace/ /tmp/apm-src/
    cd /tmp/apm-src
    export GO111MODULE=on
    meson setup build-e2e --prefix /usr
    meson install -C build-e2e
    systemd-tmpfiles --create /usr/lib/tmpfiles.d/apm.conf
'

podman exec "${CONTAINER_NAME}" busctl call \
    org.freedesktop.DBus \
    /org/freedesktop/DBus \
    org.freedesktop.DBus \
    ReloadConfig
podman exec "${CONTAINER_NAME}" systemctl daemon-reload
podman exec "${CONTAINER_NAME}" systemctl restart polkit.service
podman exec "${CONTAINER_NAME}" systemctl stop apm.service

podman exec "${CONTAINER_NAME}" bash -c '
    set -euo pipefail
    cd /tmp/apm-src
    go test -c -tags=e2e -o /tmp/apm-dbus-e2e.test ./tests/e2e/dbus
    install -d /var/cache/apt/archives/partial
    if rpm --quiet -q hello; then
        rpm -e hello
    fi
'
run_test 10m TestPackagesV2InstallRemove
run_test_as_user 30s TestPackagesV2PolkitDenied

podman exec "${CONTAINER_NAME}" systemctl stop apm.service
podman exec "${CONTAINER_NAME}" install \
    -D \
    -m 0644 \
    /tmp/apm-src/tests/e2e/dbus/fixtures/apm-e2e.list \
    /etc/apt/sources.list.d/apm-e2e.list
run_activation_test 60s TestRepoV2ReadOnly
run_test 60s TestRepoV2Privileged
run_test_as_user 30s TestRepoV2PolkitDenied
