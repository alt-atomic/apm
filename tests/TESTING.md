# Testing Guide for APM (Atomic Package Manager)

В директории tests интеграционные тесты, unit тесты лежат рядом с оригинальными файлами

### В контейнере

```bash
# Тест для distrobox, требует обычного пользователя (не root) и сам distrobox
go test -tags distrobox ./tests/integration/distrobox/...

# Просто войти в контейнер
./scripts/test-container.sh exec

# Проверить Packages/Repo D-Bus v2 через systemd/D-Bus activation и polkit
./scripts/test-dbus-e2e.sh

# Запустить интеграционные тесты
./scripts/test-container.sh integration 

# Запустить тесты сборки
./scripts/test-container.sh build 

# Запустить все тесты
./scripts/test-container.sh all
```
