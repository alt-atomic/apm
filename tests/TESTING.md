# Testing Guide for APM (Atomic Package Manager)

## Озор

APM располагает комплексной системой тестирования, разработанной для решения сложных задач управления пакетами, привязок C++ и операций на системном уровне

### В контейнере

```bash
# Запустить все тесты в контейнере
./scripts/test-container.sh all

# Специфичные тесты
./scripts/test-container.sh unit
./scripts/test-container.sh integration
./scripts/test-container.sh apt
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
