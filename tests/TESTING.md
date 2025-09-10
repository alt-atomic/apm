# Testing Guide for APM (Atomic Package Manager)

В директории tests интеграционные тесты, unit тесты лежат рядом с оригинальными файлами

### В контейнере

```bash
./scripts/test-container.sh exec
./scripts/test-container.sh system
./scripts/test-container.sh apt
```

### На машине

```bash
./scripts/test-local.sh all
./scripts/test-local.sh system
./scripts/test-local.sh apt
./scripts/test-local.sh distrobox
```
