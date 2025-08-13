# Testing Guide for APM (Atomic Package Manager)

## Озор

APM располагает комплексной системой тестирования, разработанной для решения сложных задач управления пакетами, привязок C++ и операций на системном уровне

### В контейнере

```bash
./scripts/test-container.sh all
./scripts/test-container.sh safe
./scripts/test-container.sh unit
./scripts/test-container.sh system
./scripts/test-container.sh apt
```

### Скриптом

```bash
./scripts/test-local.sh all
./scripts/test-local.sh unit
./scripts/test-local.sh system
./scripts/test-local.sh apt
./scripts/test-local.sh integration
./scripts/test-local.sh distrobox
```

### Ручные

```bash
# Setup build directory
meson setup builddir

# Run all tests
meson test -C builddir

# Только быстрые unit тесты (без системных зависимостей)
meson test -C builddir --suite unit
# Интеграционные тесты (DBUS, сервисы)
meson test -C builddir --suite integration
# Системные тесты (могут требовать root/системные ресурсы)  
meson test -C builddir --suite system
# APT биндинги (требуют root + APT)
meson test -C builddir --suite apt
# Distrobox тесты
meson test -C builddir --suite distrobox
# Verbose output
meson test -C builddir --verbose
```
