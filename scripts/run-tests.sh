#!/bin/bash
# Enhanced test runner for APM project
# Supports multiple test modes and environments

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Configuration
USE_CONTAINER=${USE_CONTAINER:-"auto"}
VERBOSE=${VERBOSE:-false}
PARALLEL=${PARALLEL:-true}
TIMEOUT=${TIMEOUT:-1800}  # 30 minutes

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

print_error() {
    print_status "$RED" "❌ $1"
}

print_success() {
    print_status "$GREEN" "✅ $1"
}

print_info() {
    print_status "$BLUE" "ℹ️  $1"
}

print_warning() {
    print_status "$YELLOW" "⚠️  $1"
}

# Function to check if we're in a container
is_in_container() {
    [[ -f /.dockerenv ]] || [[ -f /run/.containerenv ]] || [[ -n "${container:-}" ]]
}

# Function to check if we have the necessary build tools
check_build_dependencies() {
    local missing_deps=()
    
    if ! command -v meson &> /dev/null; then
        missing_deps+=("meson")
    fi
    
    if ! command -v ninja &> /dev/null; then
        missing_deps+=("ninja-build")
    fi
    
    if ! command -v go &> /dev/null; then
        missing_deps+=("golang")
    fi
    
    if ! pkg-config --exists apt-pkg 2>/dev/null; then
        missing_deps+=("libapt-pkg-dev")
    fi
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        print_error "Missing build dependencies: ${missing_deps[*]}"
        print_info "Install with: apt-get install ${missing_deps[*]}"
        return 1
    fi
    
    return 0
}

# Function to setup build directory
setup_build() {
    print_info "Setting up build directory..."
    
    if [[ ! -d "${PROJECT_ROOT}/builddir" ]]; then
        cd "${PROJECT_ROOT}"
        meson setup builddir --prefix=/usr --buildtype=debug
    fi
    
    print_info "Compiling project..."
    meson compile -C "${PROJECT_ROOT}/builddir"
    
    print_success "Build setup completed"
}

# Function to run tests with meson
run_meson_tests() {
    local suite=${1:-""}
    local extra_args=()
    
    if [[ "$VERBOSE" == "true" ]]; then
        extra_args+=("--verbose")
    fi
    
    if [[ -n "$suite" ]]; then
        extra_args+=("--suite" "$suite")
    fi
    
    if [[ "$PARALLEL" == "false" ]]; then
        extra_args+=("--num-processes" "1")
    fi
    
    extra_args+=("--timeout" "$TIMEOUT")
    
    cd "${PROJECT_ROOT}"
    
    print_info "Running meson tests with suite: ${suite:-all}"
    if meson test -C builddir "${extra_args[@]}"; then
        print_success "Meson tests passed"
        return 0
    else
        print_error "Meson tests failed"
        return 1
    fi
}

# Function to run containerized tests
run_container_tests() {
    local suite=${1:-"all"}
    
    if ! command -v podman &> /dev/null; then
        print_error "Podman not found. Install podman to use containerized testing."
        return 1
    fi
    
    print_info "Running containerized tests..."
    exec "${PROJECT_ROOT}/scripts/test-container.sh" "$suite"
}

# Function to run specific test types
run_unit_tests() {
    print_info "Running unit tests..."
    run_meson_tests "unit"
}

run_integration_tests() {
    print_info "Running integration tests..."
    run_meson_tests "integration"
}

run_apt_tests() {
    if [[ $EUID -ne 0 ]]; then
        print_warning "APT tests require root privileges"
        print_info "Run with: sudo $0 apt"
        return 1
    fi
    
    print_info "Running APT binding tests..."
    run_meson_tests "apt"
}

run_system_tests() {
    print_info "Running system tests..."
    run_meson_tests "system"
}

run_distrobox_tests() {
    print_info "Running distrobox tests..."
    run_meson_tests "distrobox"
}

# Function to run all tests
run_all_tests() {
    local failed_suites=()
    
    print_info "Running comprehensive test suite..."
    
    # Unit tests (always safe to run)
    if ! run_unit_tests; then
        failed_suites+=("unit")
    fi
    
    # System tests (if we have proper environment)
    if ! run_system_tests; then
        failed_suites+=("system")
        print_warning "System tests failed - this may be expected in limited environments"
    fi
    
    # APT tests (only if root)
    if [[ $EUID -eq 0 ]]; then
        if ! run_apt_tests; then
            failed_suites+=("apt")
        fi
    else
        print_warning "Skipping APT tests (require root privileges)"
    fi
    
    # Integration tests
    if ! run_integration_tests; then
        failed_suites+=("integration")
        print_warning "Integration tests failed - this may be expected without DBUS service"
    fi
    
    # Distrobox tests
    if ! run_distrobox_tests; then
        failed_suites+=("distrobox")
        print_warning "Distrobox tests failed - this may be expected without distrobox"
    fi
    
    # Summary
    if [[ ${#failed_suites[@]} -eq 0 ]]; then
        print_success "All test suites passed!"
        return 0
    else
        print_error "Failed test suites: ${failed_suites[*]}"
        print_info "Some failures may be expected in limited test environments"
        return 1
    fi
}

# Function to display usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS] [COMMAND]

COMMANDS:
    all         Run all test suites (default)
    unit        Run unit tests only
    system      Run system integration tests
    apt         Run APT binding tests (requires root)
    integration Run DBUS integration tests
    distrobox   Run distrobox tests
    container   Run tests in container environment

OPTIONS:
    -c, --container     Force container mode
    -n, --no-container  Force native mode
    -v, --verbose       Enable verbose output
    -s, --sequential    Run tests sequentially (no parallel)
    -t, --timeout SEC   Set test timeout (default: 1800)
    -h, --help          Show this help

ENVIRONMENT VARIABLES:
    USE_CONTAINER       auto|yes|no (default: auto)
    VERBOSE             true|false (default: false)
    PARALLEL            true|false (default: true)
    TIMEOUT             timeout in seconds (default: 1800)

EXAMPLES:
    # Run all tests
    $0

    # Run only unit tests
    $0 unit

    # Run tests in container with verbose output
    $0 -c -v

    # Run APT tests as root
    sudo $0 apt

    # Run with custom timeout
    TIMEOUT=3600 $0 integration
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--container)
            USE_CONTAINER="yes"
            shift
            ;;
        -n|--no-container)
            USE_CONTAINER="no"
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -s|--sequential)
            PARALLEL=false
            shift
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -*)
            print_error "Unknown option: $1"
            usage
            exit 1
            ;;
        *)
            break
            ;;
    esac
done

# Determine command
COMMAND=${1:-"all"}

# Main execution logic
main() {
    print_info "APM Test Runner"
    print_info "Project root: $PROJECT_ROOT"
    print_info "Command: $COMMAND"
    print_info "Container mode: $USE_CONTAINER"
    print_info "Verbose: $VERBOSE"
    
    # Determine if we should use container
    local use_container_resolved="no"
    
    case "$USE_CONTAINER" in
        "yes")
            use_container_resolved="yes"
            ;;
        "no")
            use_container_resolved="no"
            ;;
        "auto")
            if is_in_container; then
                use_container_resolved="no"  # Already in container
            elif ! check_build_dependencies; then
                print_warning "Missing build dependencies, trying container mode..."
                use_container_resolved="yes"
            else
                use_container_resolved="no"
            fi
            ;;
    esac
    
    # Handle container command specially
    if [[ "$COMMAND" == "container" ]]; then
        use_container_resolved="yes"
        COMMAND="all"
    fi
    
    # Execute based on mode
    if [[ "$use_container_resolved" == "yes" ]]; then
        print_info "Using containerized testing environment"
        run_container_tests "$COMMAND"
    else
        print_info "Using native testing environment"
        
        # Check dependencies and setup build
        if ! check_build_dependencies; then
            print_error "Build dependencies not available and container mode disabled"
            exit 1
        fi
        
        setup_build
        
        # Execute the requested command
        case "$COMMAND" in
            "all")
                run_all_tests
                ;;
            "unit")
                run_unit_tests
                ;;
            "system")
                run_system_tests
                ;;
            "apt")
                run_apt_tests
                ;;
            "integration")
                run_integration_tests
                ;;
            "distrobox")
                run_distrobox_tests
                ;;
            *)
                print_error "Unknown command: $COMMAND"
                usage
                exit 1
                ;;
        esac
    fi
}

# Run main function
main "$@"