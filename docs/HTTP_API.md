# APM HTTP API

APM предоставляет два HTTP-сервера:

| Сервер         | Адрес по умолчанию  | Модули                       |
|----------------|---------------------|------------------------------|
| System (root)  | `127.0.0.1:8080`    | `system`, `repo`, `image`    |
| Session (user) | `127.0.0.1:8082`    | `distrobox`                  |

## Запуск

Для ручного запуска или отладки:

```bash
# System — system, repo, image (требует root)
sudo apm http-server

# Session — distrobox (пользовательский)
apm http-session
```

Параметры:

| Флаг             | Описание                            | Пример                                                   |
|------------------|-------------------------------------|----------------------------------------------------------|
| `-l`, `--listen` | Адрес и порт                        | `-l 0.0.0.0:8080`                                        |
| `--api-token`    | Токен авторизации (`[права:]токен`) | `--api-token manage:secret`, `--api-token read:readonly` |
| `-v`, `--verbose`| Логирование в stdout                |                                                          |

## Интерактивная документация

Каждый сервер автоматически предоставляет Swagger UI и OpenAPI-спецификацию:

```
GET /api/v1/docs         — Swagger UI
GET /api/v1/openapi.json — OpenAPI спецификация
```

---

## Формат ответа

Все эндпоинты возвращают единую структуру `APIResponse`:

```json
{
  "data": { },
  "error": null
}
```

При ошибке:

```json
{
  "data": null,
  "error": {
    "errorCode": "NOT_FOUND",
    "message": "Не удалось найти пакет example"
  }
}
```

## Коды ошибок

| Код             | HTTP статус | Описание                                   |
|-----------------|-------------|---------------------------------------------|
| `VALIDATION`    | 400         | Ошибка валидации параметров                 |
| `PERMISSION`    | 403         | Нет прав                                    |
| `NOT_FOUND`     | 404         | Ресурс не найден                            |
| `CANCELED`      | 409         | Операция отменена                           |
| `NO_OPERATION`  | 409         | Нечего делать (уже в нужном состоянии)      |
| `DATABASE`      | 500         | Ошибка базы данных                          |
| `REPOSITORY`    | 500         | Ошибка репозитория                          |
| `APT`           | 500         | Ошибка APT                                  |
| `IMAGE`         | 500         | Ошибка работы с образом                     |
| `KERNEL`        | 500         | Ошибка работы с ядром                       |
| `CONTAINER`     | 500         | Ошибка контейнера                           |

---

## Аутентификация

Если сервер запущен с `--api-token`, все эндпоинты (кроме публичных) требуют токен.

### Формат токена

```
Authorization: Bearer [permission:]token
```

| Право    | Доступ                          |
|----------|---------------------------------|
| `manage` | Полный доступ (по умолчанию)    |
| `read`   | Только чтение                   |

Примеры:

```
Authorization: Bearer manage:secrettoken   — полный доступ
Authorization: Bearer read:readonlytoken   — только чтение
Authorization: Bearer secrettoken          — полный доступ (manage по умолчанию)
```

### Публичные эндпоинты (без токена)

- `GET /api/v1` — информация об API
- `GET /api/v1/health` — проверка состояния
- `GET /api/v1/docs` — Swagger UI
- `GET /api/v1/openapi.json` — OpenAPI спецификация

---

## Transaction ID

Для отслеживания операций можно передать ID через заголовок или query-параметр:

```
X-Transaction-ID: my-transaction-id
```

Если не передан — генерируется автоматически для background-операций. Transaction ID связывает HTTP-запрос с событиями в WebSocket.

---

## Фоновое выполнение (background)

Долгие операции поддерживают фоновое выполнение через query-параметр `?background=true`.

### Синхронный режим (по умолчанию)

Запрос блокируется до завершения, результат в ответе.

### Фоновый режим

Сервер сразу возвращает **202 Accepted**:

```json
{
  "data": {
    "message": "Task started in background",
    "transaction": "1740907234567890123-a1b2c3d4e5f6g7h8"
  },
  "error": null
}
```

Результат и прогресс приходят через WebSocket.

---

## WebSocket (события)

Подключение:

```
ws://127.0.0.1:8080/api/v1/events
```

Аутентификация не требуется. Через WebSocket приходят те же три типа сообщений, что и через D-Bus сигналы:

| Тип              | Описание                                   |
|------------------|--------------------------------------------|
| `NOTIFICATION`   | Уведомление о начале/завершении этапа      |
| `PROGRESS`       | Прогресс выполнения с процентами           |
| `TASK_RESULT`    | Финальный результат фоновой задачи         |

### NOTIFICATION / PROGRESS

За одну операцию приходит множество таких сообщений — они соответствуют тем же этапам, которые отображаются в CLI (спиннер, прогресс-бар).

```json
{
  "name": "system.Install",
  "message": "Установка пакетов...",
  "state": "BEFORE",
  "type": "PROGRESS",
  "progress": 45.5,
  "progressDone": "2/5",
  "transaction": "..."
}
```

| Поле           | Тип    | Описание                                      |
|----------------|--------|-----------------------------------------------|
| `name`         | string | Имя события (константы см. в DBUS_API.md)     |
| `message`      | string | Человекочитаемое описание                     |
| `state`        | string | `BEFORE` — начало, `AFTER` — завершение этапа |
| `type`         | string | `NOTIFICATION` или `PROGRESS`                 |
| `progress`     | float  | Процент выполнения (0-100)                    |
| `progressDone` | string | Текстовый прогресс, например `"3/10"`         |
| `transaction`  | string | ID транзакции                                 |

### TASK_RESULT

Финальный результат фоновой задачи. Приходит один раз по завершении.

```json
{
  "type": "TASK_RESULT",
  "name": "system.Install",
  "transaction": "...",
  "data": { },
  "error": null
}
```

| Поле          | Тип    | Описание                                             |
|---------------|--------|------------------------------------------------------|
| `type`        | string | Всегда `TASK_RESULT`                                 |
| `name`        | string | Имя события                                          |
| `transaction` | string | ID транзакции из начального запроса                   |
| `data`        | object | Данные ответа (те же, что при синхронном вызове)      |
| `error`       | object | `null` или `{"errorCode": "...", "message": "..."}` |

---

## Middleware

Запросы проходят через цепочку middleware:

1. **CORS** — `Access-Control-Allow-Origin: *`, поддержка preflight (OPTIONS)
2. **Logging** — логирует `HTTP {METHOD} {PATH} {STATUS} {TIME}ms`
3. **Auth** — проверка токена и прав доступа

### CORS заголовки

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization, X-Transaction-ID
```

---

## Связь с D-Bus API

HTTP и D-Bus API используют один и тот же слой бизнес-логики. Различается только транспорт:

- HTTP — REST эндпоинты, Bearer-токен, WebSocket для событий
- D-Bus — методы интерфейсов, Polkit, сигналы для событий

Форматы событий (NOTIFICATION, PROGRESS, TASK_RESULT) и коды ошибок идентичны.
