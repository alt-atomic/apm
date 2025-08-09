# Тесты APM (Atomic Package Manager)

Этот каталог содержит тесты для проекта APM, организованные по логическим группам.

## Запуск тестов

### Все тесты без root прав
```bash
go test ./tests/... -v
```

### Тесты, требующие root прав
```bash
sudo go test ./tests/... -v
```

### Конкретные группы тестов

#### Unit тесты (без зависимостей)
```bash
go test ./tests/system/ -run TestUnit -v
```

#### Тесты без root прав
```bash
go test ./tests/system/ -run TestNonRoot -v
```

#### Тесты с root правами
```bash
sudo go test ./tests/system/ -run TestRoot -v
```

#### Тесты для атомарной системы
```bash
sudo go test ./tests/system/ -run TestAtomic -v
```

#### Тесты для неатомарной системы
```bash
sudo go test ./tests/system/ -run TestNonAtomic -v
```

#### Тесты distrobox
```bash
go test ./tests/distrobox/ -v
```

### Полный набор тестов
```bash
# Сначала запускаем тесты без root
go test ./tests/... -v

# Затем тесты с root
sudo go test ./tests/... -v
```
