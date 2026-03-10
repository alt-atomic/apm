#!/bin/bash
# Интеграционные тесты совместимости apm repo и apt-repo
# Сравнивает поведение обоих инструментов на одних и тех же операциях
#
# Использование: sudo ./scripts/test-repo-compat.sh [TASK_NUM]
# По умолчанию TASK_NUM=410804

set -uo pipefail

TASK_NUM="${1:-410804}"
SOURCES_LIST="/etc/apt/sources.list"
SOURCES_DIR="/etc/apt/sources.list.d"
BACKUP_DIR="/tmp/apt-repo-test-backup-$$"
PASSED=0
FAILED=0
SKIPPED=0

# Цвета
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_header() { echo -e "\n${BLUE}══════════════════════════════════════════${NC}"; echo -e "${BLUE}  $1${NC}"; echo -e "${BLUE}══════════════════════════════════════════${NC}"; }
log_test()   { echo -e "\n${YELLOW}── TEST: $1${NC}"; }
log_pass()   { echo -e "${GREEN}   ✓ PASS: $1${NC}"; ((PASSED++)); }
log_fail()   { echo -e "${RED}   ✗ FAIL: $1${NC}"; ((FAILED++)); }
log_skip()   { echo -e "${YELLOW}   ⊘ SKIP: $1${NC}"; ((SKIPPED++)); }
log_info()   { echo -e "   $1"; }

# Проверка root
if [[ $EUID -ne 0 ]]; then
    echo "Требуется root. Запустите: sudo $0"
    exit 1
fi

# Проверка наличия инструментов
if ! command -v apt-repo &>/dev/null; then
    echo "apt-repo не найден"
    exit 1
fi
if ! command -v apm &>/dev/null; then
    echo "apm не найден"
    exit 1
fi

# Бэкап
backup() {
    mkdir -p "$BACKUP_DIR"
    cp -a "$SOURCES_LIST" "$BACKUP_DIR/sources.list.bak" 2>/dev/null || true
    cp -a "$SOURCES_DIR" "$BACKUP_DIR/sources.list.d.bak" 2>/dev/null || true
    log_info "Бэкап сохранён в $BACKUP_DIR"
}

# Восстановление
restore() {
    cp -a "$BACKUP_DIR/sources.list.bak" "$SOURCES_LIST" 2>/dev/null || true
    find "$SOURCES_DIR" -maxdepth 1 -name '*.list' -delete 2>/dev/null || true
    if ls "$BACKUP_DIR/sources.list.d.bak"/*.list &>/dev/null; then
        cp -a "$BACKUP_DIR/sources.list.d.bak"/*.list "$SOURCES_DIR/" 2>/dev/null || true
    fi
    log_info "Восстановлено из бэкапа"
}

# Очистка при выходе
cleanup() {
    echo ""
    log_header "Восстановление исходного состояния"
    restore
    rm -rf "$BACKUP_DIR"
    echo ""
    log_header "Результаты"
    echo -e "  ${GREEN}Пройдено: $PASSED${NC}"
    echo -e "  ${RED}Провалено: $FAILED${NC}"
    echo -e "  ${YELLOW}Пропущено: $SKIPPED${NC}"
    echo ""
    if [[ $FAILED -gt 0 ]]; then
        exit 1
    fi
}
trap cleanup EXIT

# Получить отсортированное содержимое sources.list (без пустых строк и комментариев)
get_active_repos() {
    grep -v '^\s*#' "$SOURCES_LIST" 2>/dev/null | grep -v '^\s*$' | sort || true
}

# Получить содержимое всех .list файлов
get_all_active_repos() {
    {
        grep -v '^\s*#' "$SOURCES_LIST" 2>/dev/null | grep -v '^\s*$' || true
        for f in "$SOURCES_DIR"/*.list; do
            [[ -f "$f" ]] && grep -v '^\s*#' "$f" 2>/dev/null | grep -v '^\s*$' || true
        done
    } | sort
}

# Сравнить два набора репо (канонизируя new_format → old_format)
canonicalize_repo_line() {
    # Приводим new_format к old_format для сравнения
    # rpm [key] http://host/path comp1/comp2/arch components → rpm [key] http://host/path/comp1/comp2 arch components
    local line="$1"
    echo "$line" | python3 -c "
import sys
for line in sys.stdin:
    parts = line.split()
    if len(parts) < 4:
        print(line.rstrip())
        continue
    idx = 1
    if parts[idx].startswith('['):
        idx += 1
    if idx+1 >= len(parts):
        print(line.rstrip())
        continue
    parts[idx] = parts[idx].rstrip('/')
    arch = parts[idx+1]
    if '/' not in arch:
        print(' '.join(parts))
        continue
    segs = arch.split('/')
    real_arch = segs[-1]
    prefix = '/'.join(segs[:-1])
    parts[idx] = parts[idx] + '/' + prefix
    parts[idx+1] = real_arch
    print(' '.join(parts))
" 2>/dev/null || echo "$line"
}

canonicalize_file() {
    local result=""
    while IFS= read -r line; do
        [[ -z "$line" ]] && continue
        canonicalize_repo_line "$line"
    done | sort
}

# Сравнить репозитории (с канонизацией)
compare_repos() {
    local label="$1"
    local repos_a
    local repos_b
    repos_a=$(get_active_repos | canonicalize_file)
    # repos_b передаётся через stdin или второй вызов
    repos_b="$2"

    if [[ "$repos_a" == "$repos_b" ]]; then
        log_pass "$label"
        return 0
    else
        log_fail "$label"
        log_info "Ожидалось:"
        echo "$repos_b" | sed 's/^/      /'
        log_info "Получено:"
        echo "$repos_a" | sed 's/^/      /'
        log_info "Diff:"
        diff <(echo "$repos_b") <(echo "$repos_a") | sed 's/^/      /' || true
        return 1
    fi
}

# ═══════════════════════════════════════
# Начало тестов
# ═══════════════════════════════════════

log_header "Интеграционные тесты: apm repo vs apt-repo"
log_info "Таск для тестирования: $TASK_NUM"
backup

# Сохраняем исходное состояние
INITIAL_REPOS=$(get_active_repos | canonicalize_file)

# ───────────────────────────────────────
# TEST 1: set sisyphus — сравнение результата
# ───────────────────────────────────────
log_test "1. apt-repo set sisyphus → запомнить результат"
apt-repo set sisyphus 2>/dev/null
APTREPO_SISYPHUS=$(get_active_repos | canonicalize_file)
log_info "apt-repo записал $(echo "$APTREPO_SISYPHUS" | wc -l) строк"

restore

log_test "1b. apm repo set sisyphus → сравнить"
apm repo set sisyphus -f json 2>/dev/null >/dev/null
APM_SISYPHUS=$(get_active_repos | canonicalize_file)
log_info "apm записал $(echo "$APM_SISYPHUS" | wc -l) строк"
compare_repos "set sisyphus: одинаковый набор репозиториев" "$APTREPO_SISYPHUS"

# ───────────────────────────────────────
# TEST 2: set p10 — сравнение результата
# ───────────────────────────────────────
restore

log_test "2. apt-repo set p10 → запомнить результат"
apt-repo set p10 2>/dev/null
APTREPO_P10=$(get_active_repos | canonicalize_file)
log_info "apt-repo записал $(echo "$APTREPO_P10" | wc -l) строк"

restore

log_test "2b. apm repo set p10 → сравнить"
apm repo set p10 -f json 2>/dev/null >/dev/null
APM_P10=$(get_active_repos | canonicalize_file)
log_info "apm записал $(echo "$APM_P10" | wc -l) строк"
compare_repos "set p10: одинаковый набор репозиториев" "$APTREPO_P10"

# ───────────────────────────────────────
# TEST 3: add task — сравнение результата
# ───────────────────────────────────────
restore

log_test "3. apt-repo set sisyphus + add task $TASK_NUM"
apt-repo set sisyphus 2>/dev/null
apt-repo add "$TASK_NUM" 2>/dev/null
APTREPO_WITH_TASK=$(get_active_repos | canonicalize_file)
APTREPO_TASK_COUNT=$(echo "$APTREPO_WITH_TASK" | wc -l)
log_info "apt-repo: $APTREPO_TASK_COUNT строк (с таском)"

restore

log_test "3b. apm repo set sisyphus + add task $TASK_NUM → сравнить"
apm repo set sisyphus -f json 2>/dev/null >/dev/null
apm repo add "$TASK_NUM" -f json 2>/dev/null >/dev/null
APM_WITH_TASK=$(get_active_repos | canonicalize_file)
APM_TASK_COUNT=$(echo "$APM_WITH_TASK" | wc -l)
log_info "apm: $APM_TASK_COUNT строк (с таском)"
compare_repos "add task $TASK_NUM: одинаковый набор репозиториев" "$APTREPO_WITH_TASK"

# ───────────────────────────────────────
# TEST 4: remove task — сравнение результата
# ───────────────────────────────────────
log_test "4. apm repo rm $TASK_NUM (после add)"
apm repo rm "$TASK_NUM" -f json 2>/dev/null >/dev/null
APM_AFTER_RM_TASK=$(get_active_repos | canonicalize_file)
# После удаления таска должны остаться только репозитории sisyphus
compare_repos "rm task: остались только репозитории ветки" "$APM_SISYPHUS"

# ───────────────────────────────────────
# TEST 5: кросс-совместимость — apt-repo add, apm rm
# ───────────────────────────────────────
restore

log_test "5. apt-repo add task → apm rm task (кросс-совместимость)"
apt-repo set sisyphus 2>/dev/null
BEFORE_TASK=$(get_active_repos | canonicalize_file)
apt-repo add "$TASK_NUM" 2>/dev/null
log_info "apt-repo добавил таск (new_format)"

apm repo rm "$TASK_NUM" -f json 2>/dev/null >/dev/null
AFTER_APM_RM=$(get_active_repos | canonicalize_file)
compare_repos "кросс: apt-repo add → apm rm = исходное состояние" "$BEFORE_TASK"

# ───────────────────────────────────────
# TEST 6: кросс-совместимость — apm add, apt-repo rm
# ───────────────────────────────────────
restore

log_test "6. apm add task → apt-repo rm task (кросс-совместимость)"
apt-repo set sisyphus 2>/dev/null
BEFORE_TASK2=$(get_active_repos | canonicalize_file)
apm repo add "$TASK_NUM" -f json 2>/dev/null >/dev/null
log_info "apm добавил таск (old_format)"

apt-repo rm "$TASK_NUM" 2>/dev/null
AFTER_APTREPO_RM=$(get_active_repos | canonicalize_file)
compare_repos "кросс: apm add → apt-repo rm = исходное состояние" "$BEFORE_TASK2"

# ───────────────────────────────────────
# TEST 7: кросс-совместимость — apt-repo set branch, apm set другой branch
# ───────────────────────────────────────
restore

log_test "7. apt-repo set sisyphus → apm set p10 (кросс-совместимость)"
apt-repo set sisyphus 2>/dev/null
log_info "apt-repo установил sisyphus (new_format)"
APTREPO_SISYPHUS_COUNT=$(get_active_repos | wc -l)
log_info "Строк в sources.list: $APTREPO_SISYPHUS_COUNT"

apm repo set p10 -f json 2>/dev/null >/dev/null
APM_AFTER_CROSS_SET=$(get_active_repos | canonicalize_file)
APM_CROSS_COUNT=$(get_active_repos | wc -l)
log_info "После apm set p10: $APM_CROSS_COUNT строк"

# Проверяем что нет остатков sisyphus
if echo "$APM_AFTER_CROSS_SET" | grep -qi sisyphus; then
    log_fail "кросс set: остались записи sisyphus после apm set p10"
else
    log_pass "кросс set: нет остатков sisyphus после apm set p10"
fi

# Проверяем что p10 присутствует
if echo "$APM_AFTER_CROSS_SET" | grep -qi "p10"; then
    log_pass "кросс set: записи p10 присутствуют"
else
    log_fail "кросс set: записи p10 отсутствуют"
fi

# ───────────────────────────────────────
# TEST 8: дубликаты — apt-repo add + apm add того же
# ───────────────────────────────────────
restore

log_test "8. apt-repo add task → apm add того же таска (проверка дубликатов)"
apt-repo set sisyphus 2>/dev/null
apt-repo add "$TASK_NUM" 2>/dev/null
BEFORE_DUP=$(get_active_repos | wc -l)
log_info "После apt-repo add: $BEFORE_DUP строк"

apm repo add "$TASK_NUM" -f json 2>/dev/null >/dev/null || true
AFTER_DUP=$(get_active_repos | wc -l)
log_info "После apm add (повторный): $AFTER_DUP строк"

if [[ "$BEFORE_DUP" -eq "$AFTER_DUP" ]]; then
    log_pass "нет дубликатов: количество строк не изменилось ($BEFORE_DUP)"
else
    log_fail "дубликаты: было $BEFORE_DUP, стало $AFTER_DUP"
fi

# ───────────────────────────────────────
# TEST 9: clean — удаление task репозиториев
# ───────────────────────────────────────
log_test "9. apm repo clean (удаление таска после apt-repo add)"
# Таск уже добавлен из теста 8
apm repo clean -f json 2>/dev/null >/dev/null
AFTER_CLEAN=$(get_active_repos | canonicalize_file)
AFTER_CLEAN_COUNT=$(get_active_repos | wc -l)
log_info "После clean: $AFTER_CLEAN_COUNT строк"

if echo "$AFTER_CLEAN" | grep -q "task"; then
    log_fail "clean: остались task-репозитории"
else
    log_pass "clean: все task-репозитории удалены"
fi

# ───────────────────────────────────────
# TEST 10: sources.list.d — комментирование vs удаление
# ───────────────────────────────────────
restore

log_test "10. sources.list.d: rm комментирует в .list файлах, удаляет из sources.list"

# Создаём .list файл в sources.list.d с репо таска
EXTRA_LIST="$SOURCES_DIR/test-task.list"
echo "rpm https://git.altlinux.org/repo/$TASK_NUM/ x86_64 task" > "$EXTRA_LIST"
echo "rpm https://git.altlinux.org/repo/$TASK_NUM/ x86_64-i586 task" >> "$EXTRA_LIST"

# Также добавим в sources.list
echo "rpm https://git.altlinux.org/repo/$TASK_NUM/ x86_64 task" >> "$SOURCES_LIST"

BEFORE_MAIN_LINES=$(wc -l < "$SOURCES_LIST")
BEFORE_EXTRA_LINES=$(wc -l < "$EXTRA_LIST")
log_info "До удаления: sources.list=$BEFORE_MAIN_LINES строк, test-task.list=$BEFORE_EXTRA_LINES строк"

apm repo rm "$TASK_NUM" -f json 2>/dev/null >/dev/null || true

AFTER_MAIN_LINES=$(wc -l < "$SOURCES_LIST")
log_info "После удаления: sources.list=$AFTER_MAIN_LINES строк"

# Проверяем: из sources.list строка должна быть УДАЛЕНА
if grep -q "git.altlinux.org/repo/$TASK_NUM" "$SOURCES_LIST" 2>/dev/null; then
    log_fail "sources.list: строка таска не удалена"
else
    log_pass "sources.list: строка таска удалена"
fi

# Проверяем: в .list файле строки должны быть ЗАКОММЕНТИРОВАНЫ (не удалены)
if [[ -f "$EXTRA_LIST" ]]; then
    COMMENTED=$(grep -c "^#" "$EXTRA_LIST" 2>/dev/null || true)
    ACTIVE=$(grep -v '^\s*#' "$EXTRA_LIST" 2>/dev/null | grep -c "rpm" || true)
    COMMENTED=${COMMENTED:-0}
    ACTIVE=${ACTIVE:-0}
    log_info "test-task.list: $COMMENTED закомментированных, $ACTIVE активных"
    if [[ "$COMMENTED" -eq 2 && "$ACTIVE" -eq 0 ]]; then
        log_pass "sources.list.d: строки закомментированы, не удалены"
    elif [[ "$COMMENTED" -gt 0 && "$ACTIVE" -eq 0 ]]; then
        log_pass "sources.list.d: строки закомментированы ($COMMENTED шт)"
    else
        log_fail "sources.list.d: ожидалось комментирование (commented=$COMMENTED, active=$ACTIVE)"
        cat "$EXTRA_LIST" | sed 's/^/      /'
    fi
else
    log_fail "sources.list.d: файл test-task.list удалён (должен был остаться)"
fi

rm -f "$EXTRA_LIST"

# ───────────────────────────────────────
# TEST 11: list — сравнение количества
# ───────────────────────────────────────
restore

log_test "10. list: сравнение количества репозиториев"
APTREPO_LIST_COUNT=$(apt-repo list 2>/dev/null | grep -c '^rpm' || echo 0)
APM_LIST_COUNT=$(apm repo list -f json 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('count',0))" 2>/dev/null || echo 0)

log_info "apt-repo list: $APTREPO_LIST_COUNT"
log_info "apm repo list: $APM_LIST_COUNT"

if [[ "$APTREPO_LIST_COUNT" -eq "$APM_LIST_COUNT" ]]; then
    log_pass "list: одинаковое количество ($APTREPO_LIST_COUNT)"
else
    log_fail "list: apt-repo=$APTREPO_LIST_COUNT, apm=$APM_LIST_COUNT"
fi

# ───────────────────────────────────────
# TEST 11: task packages — сравнение списка пакетов
# ───────────────────────────────────────
log_test "11. task packages: сравнение списка пакетов таска $TASK_NUM"
APTREPO_PKGS=$(apt-repo list "$TASK_NUM" 2>/dev/null | sort || true)
APM_PKGS=$(apm repo task "$TASK_NUM" -f json 2>/dev/null | python3 -c "
import sys, json
data = json.load(sys.stdin)
pkgs = data.get('data', {}).get('packages', [])
for p in sorted(pkgs):
    print(p)
" 2>/dev/null || true)

APTREPO_PKG_COUNT=$(echo "$APTREPO_PKGS" | grep -c . || echo 0)
APM_PKG_COUNT=$(echo "$APM_PKGS" | grep -c . || echo 0)
log_info "apt-repo list task: $APTREPO_PKG_COUNT пакетов"
log_info "apm repo task: $APM_PKG_COUNT пакетов"

if [[ "$APTREPO_PKG_COUNT" -eq "$APM_PKG_COUNT" ]]; then
    log_pass "task packages: одинаковое количество ($APTREPO_PKG_COUNT)"
elif [[ "$APM_PKG_COUNT" -gt "$APTREPO_PKG_COUNT" ]]; then
    # APM может включать больше пакетов (нет фильтрации kernel-modules)
    DIFF_PKGS=$(diff <(echo "$APTREPO_PKGS") <(echo "$APM_PKGS") || true)
    EXTRA=$(echo "$DIFF_PKGS" | grep '^>' | wc -l)
    MISSING=$(echo "$DIFF_PKGS" | grep '^<' | wc -l)
    log_info "APM: +$EXTRA пакетов, -$MISSING пакетов"
    if [[ "$MISSING" -eq 0 ]]; then
        log_pass "task packages: APM включает все пакеты apt-repo + $EXTRA дополнительных"
    else
        log_fail "task packages: APM не включает $MISSING пакетов из apt-repo"
    fi
    echo "$DIFF_PKGS" | head -20 | sed 's/^/      /'
else
    log_fail "task packages: apt-repo=$APTREPO_PKG_COUNT, apm=$APM_PKG_COUNT"
    diff <(echo "$APTREPO_PKGS") <(echo "$APM_PKGS") | head -20 | sed 's/^/      /' || true
fi
