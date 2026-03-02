# APM D-Bus API

APM экспортирует два D-Bus сервиса с именем `org.altlinux.APM`:

| Шина    | Объект              | Интерфейсы                                       |
|---------|---------------------|--------------------------------------------------|
| System  | `/org/altlinux/APM` | `system`, `kernel` (в atomic недоступен), `repo` |
| Session | `/org/altlinux/APM` | `distrobox`                                      |

Полные имена интерфейсов имеют префикс `org.altlinux.APM.` (например `org.altlinux.APM.system`).

### Запуск D-Bus сервисов

Сервисы запускаются автоматически через D-Bus activation при первом обращении к `org.altlinux.APM`:

- **System Bus** — `org.altlinux.APM.service` активирует `apm.service` (systemd, от root)
- **Session Bus** — `org.altlinux.APM.User.service` запускает `apm dbus-session`

Для ручного запуска или отладки:

```bash
# System Bus — system, kernel, repo (требует root)
sudo apm dbus-system

# Session Bus — distrobox (пользовательский)
apm dbus-session
```

Оба поддерживают флаг `-v` / `--verbose` для логирования в stdout.

## Интерактивная документация

Каждый модуль имеет встроенную интерактивную документацию с перечислением всех методов, параметров, примерами команд `dbus-send` и структурами ответов. Документация генерируется автоматически из исходного кода.

```bash
apm system dbus-doc

apm distrobox dbus-doc

apm kernel dbus-doc

apm repo dbus-doc
```

---

## Формат ответа

Все методы возвращают JSON-строку с единой структурой `APIResponse`:

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

Ошибки возвращаются как D-Bus Error с типизированным именем. Если ошибка не относится к APM (например, ошибка сериализации JSON или Polkit), возвращается стандартная ошибка `org.freedesktop.DBus.Error.Failed`.

**APM ошибки:**

| Код             | D-Bus имя ошибки                     | Описание                                   |
|-----------------|--------------------------------------|---------------------------------------------|
| `DATABASE`      | `org.altlinux.APM.Error.Database`    | Ошибка базы данных                          |
| `REPOSITORY`    | `org.altlinux.APM.Error.Repository`  | Ошибка репозитория                          |
| `APT`           | `org.altlinux.APM.Error.Apt`         | Ошибка APT                                  |
| `VALIDATION`    | `org.altlinux.APM.Error.Validation`  | Ошибка валидации параметров                 |
| `PERMISSION`    | `org.altlinux.APM.Error.Permission`  | Нет прав                                    |
| `CANCELED`      | `org.altlinux.APM.Error.Canceled`    | Операция отменена                           |
| `IMAGE`         | `org.altlinux.APM.Error.Image`       | Ошибка работы с образом                     |
| `KERNEL`        | `org.altlinux.APM.Error.Kernel`      | Ошибка работы с ядром                       |
| `CONTAINER`     | `org.altlinux.APM.Error.Container`   | Ошибка контейнера                           |
| `NO_OPERATION`  | `org.altlinux.APM.Error.NoOperation` | Нечего делать (уже в нужном состоянии)      |
| `NOT_FOUND`     | `org.altlinux.APM.Error.NotFound`    | Ресурс не найден                            |

**Стандартная D-Bus ошибка:**

| D-Bus имя ошибки                       | Описание                                         |
|-----------------------------------------|--------------------------------------------------|
| `org.freedesktop.DBus.Error.Failed`     | Внутренняя ошибка (сериализация, Polkit и т.д.)  |

## Transaction ID

Каждый метод принимает параметр `transaction` (string) - идентификатор для отслеживания операции.

- Пустая строка `""` - ID генерируется автоматически (для background-методов).
- Формат: `{UnixNano}-{16 hex символов}`, например `1740907234567890123-a1b2c3d4e5f6g7h8`.
- Transaction ID связывает запрос с сигналами прогресса и результата.

## Права (Polkit)

Методы, изменяющие систему, требуют авторизации через PolicyKit.

- Действие: **`org.altlinux.APM.manage`**
- Применяется к: модулям `system`, `kernel`, `repo` (System Bus)
- Модуль `distrobox` работает на Session Bus без Polkit

---

## Фоновое выполнение (background)

Долгие операции (установка, удаление, обновление) поддерживают фоновое выполнение через параметр `background bool`.

### Синхронный режим (background = false)

Метод блокируется до завершения и возвращает результат в ответе.

### Фоновый режим (background = true)

Метод немедленно возвращает:

```json
{
  "data": {
    "message": "Task started in background",
    "transaction": "1740907234567890123-a1b2c3d4e5f6g7h8"
  },
  "error": null
}
```

Результат придёт через D-Bus сигнал `org.altlinux.APM.Notification`.

---

## Сигналы

Все сигналы отправляются на объект `/org/altlinux/APM` с именем `org.altlinux.APM.Notification`. Payload — JSON-строка.

Через один сигнал приходят три типа сообщений, различаемых по полю `type`:

| Тип              | Описание                                   |
|------------------|--------------------------------------------|
| `NOTIFICATION`   | Уведомление о начале/завершении этапа      |
| `PROGRESS`       | Прогресс выполнения с процентами           |
| `TASK_RESULT`    | Финальный результат фоновой задачи         |

### Подписка на сигналы

```bash
# system, kernel, repo (System Bus)
gdbus monitor --system --dest org.altlinux.APM

# distrobox (Session Bus)
gdbus monitor --session --dest org.altlinux.APM
```

### NOTIFICATION / PROGRESS

Уведомления о ходе выполнения операции. За одну операцию приходит множество таких сигналов - они соответствуют тем же этапам, которые отображаются в CLI (спиннер, прогресс-бар).

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
| `name`         | string | Имя события (см. константы ниже)              |
| `message`      | string | Человекочитаемое описание                     |
| `state`        | string | `BEFORE` — начало, `AFTER` — завершение этапа |
| `type`         | string | `NOTIFICATION` или `PROGRESS`                 |
| `progress`     | float  | Процент выполнения (0–100)                    |
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

## Константы событий

### System

| Константа                          | Значение                           |
|------------------------------------|------------------------------------|
| `EventSystemInstall`               | `system.Install`                   |
| `EventSystemRemove`                | `system.Remove`                    |
| `EventSystemUpdate`                | `system.Update`                    |
| `EventSystemUpgrade`               | `system.Upgrade`                   |
| `EventSystemCheckInstall`          | `system.CheckInstall`              |
| `EventSystemCheckRemove`           | `system.CheckRemove`               |
| `EventSystemCheckUpgrade`          | `system.CheckUpgrade`              |
| `EventSystemImageUpdate`           | `system.ImageUpdate`               |
| `EventSystemImageApply`            | `system.ImageApply`                |
| `EventSystemAptUpdate`             | `system.AptUpdate`                 |
| `EventSystemSavePackagesToDB`      | `system.SavePackagesToDB`          |
| `EventSystemSaveImageToDB`         | `system.SaveImageToDB`             |
| `EventSystemBuildImage`            | `system.BuildImage`                |
| `EventSystemSwitchImage`           | `system.SwitchImage`               |
| `EventSystemCheckUpdateBaseImage`  | `system.CheckAndUpdateBaseImage`   |
| `EventSystemBootcUpgrade`          | `system.bootcUpgrade`              |
| `EventSystemPruneOldImages`        | `system.pruneOldImages`            |
| `EventSystemUpdateAllPackagesDB`   | `system.updateAllPackagesDB`       |
| `EventSystemUpdateAppStream`       | `system.UpdateAppStream`           |
| `EventSystemDownloadProgress`      | `system.downloadProgress`          |
| `EventSystemPullImage`             | `system.pullImage`                 |

### Kernel

| Константа                       | Значение                             |
|---------------------------------|--------------------------------------|
| `EventKernelInstall`            | `kernel.InstallKernel`               |
| `EventKernelCheckInstall`       | `kernel.CheckInstallKernel`          |
| `EventKernelUpdate`             | `kernel.UpdateKernel`                |
| `EventKernelCheckUpdate`        | `kernel.CheckUpdateKernel`           |
| `EventKernelClean`              | `kernel.CleanOldKernels`             |
| `EventKernelCheckClean`         | `kernel.CheckCleanOldKernels`        |
| `EventKernelInstallMods`        | `kernel.InstallKernelModules`        |
| `EventKernelCheckInstallMods`   | `kernel.CheckInstallKernelModules`   |
| `EventKernelRemoveMods`         | `kernel.RemoveKernelModules`         |
| `EventKernelCheckRemoveMods`    | `kernel.CheckRemoveKernelModules`    |

### Distrobox

| Константа                      | Значение                   |
|--------------------------------|----------------------------|
| `EventDistroUpdate`            | `distrobox.Update`         |
| `EventDistroContainerAdd`      | `distrobox.ContainerAdd`   |
| `EventDistroSavePackagesToDB`  | `distro.SavePackagesToDB`  |
| `EventDistroCreateContainer`   | `distro.CreateContainer`   |
| `EventDistroRemoveContainer`   | `distro.RemoveContainer`   |
| `EventDistroInstallPackage`    | `distro.InstallPackage`    |
| `EventDistroRemovePackage`     | `distro.RemovePackage`     |
| `EventDistroUpdatePackages`    | `distro.UpdatePackages`    |
| `EventDistroGetPackages`       | `distro.GetPackages`       |
