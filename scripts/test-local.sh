#!/bin/bash
# Простой локальный тест-раннер для APM
# Запускает тесты прямо на локальной машине

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Проверка зависимостей для сборки
check_dependencies() {
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
    
    # APT development headers check - skip if installed
     if ! find /usr/include -name "apt-pkg" -type d &>/dev/null; then
         missing_deps+=("libapt-devel")
     fi
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        print_error "Отсутствуют зависимости: ${missing_deps[*]}"
        print_info "Установите с помощью: apt-get install ${missing_deps[*]}"
        return 1
    fi
    
    return 0
}

# Настройка и сборка проекта
setup_build() {
    print_info "Настройка сборки..."
    
    cd "${PROJECT_ROOT}"
    
    if [[ ! -d "builddir" ]]; then
        meson setup builddir --prefix=/usr --buildtype=debug
    fi
    
    print_info "Компиляция..."
    meson compile -C builddir
    
    print_success "Сборка завершена"
}

# Запуск конкретного набора тестов
run_test_suite() {
    local suite="$1"
    
    print_info "Запуск тестов: ${suite}"
    
    cd "${PROJECT_ROOT}"
    
    if meson test -C builddir --suite "$suite" --verbose; then
        print_success "Тесты $suite прошли успешно"
        return 0
    else
        print_error "Тесты $suite провалились"
        return 1
    fi
}

# Запуск всех тестов
run_all_tests() {
    print_info "Запуск всех тестов..."
    
    cd "${PROJECT_ROOT}"
    
    if meson test -C builddir --verbose; then
        print_success "Все тесты прошли успешно"
        return 0
    else
        print_error "Некоторые тесты провалились"
        return 1
    fi
}

# Показать справку
usage() {
    cat << EOF
Использование: $0 [КОМАНДА]

КОМАНДЫ:
    all         Запустить все тесты (по умолчанию)
    unit        Только юнит тесты
    system      Системные тесты
    apt         APT биндинг тесты (требует root)
    integration Интеграционные тесты (требует DBUS)
    distrobox   Distrobox тесты
    help        Показать эту справку

ПРИМЕРЫ:
    $0                 # Все тесты
    $0 unit           # Только юнит тесты
    sudo $0 apt       # APT тесты с root правами
    $0 distrobox      # Distrobox тесты (создают контейнеры)

ПРИМЕЧАНИЯ:
    - Тесты запускаются напрямую через meson
    - APT тесты требуют root права
    - Distrobox тесты создают реальные контейнеры
    - Интеграционные тесты требуют системный DBUS
EOF
}

# Парсинг аргументов
COMMAND=${1:-"all"}

case "$COMMAND" in
    "help"|"-h"|"--help")
        usage
        exit 0
        ;;
esac

# Основная логика
main() {
    print_info "APM Local Test Runner"
    print_info "Корневая директория: $PROJECT_ROOT"
    
    # Проверка зависимостей
    if ! check_dependencies; then
        print_error "Не удалось проверить зависимости"
        print_info "Используйте test-container.sh для контейнеризованного тестирования"
        exit 1
    fi
    
    # Настройка сборки
    setup_build
    
    # Выполнение команды
    case "$COMMAND" in
        "all")
            run_all_tests
            ;;
        "unit"|"system"|"apt"|"distrobox")
            run_test_suite "$COMMAND"
            ;;
        *)
            print_error "Неизвестная команда: $COMMAND"
            usage
            exit 1
            ;;
    esac
}

# Запуск
main "$@"
