# Testing Guide for APM (Atomic Package Manager)

В директории tests интеграционные тесты, unit тесты лежат рядом с оригинальными файлами

### В контейнере

```bash
Просто войти в контейнер
./scripts/test-container.sh exec
Запустить интеграционные тесты
./scripts/test-container.sh integration 
Запустить все тесты
./scripts/test-container.sh all
```
