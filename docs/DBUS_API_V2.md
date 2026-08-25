# APM D-Bus API v2

APM экспортирует API v2 под именем `org.altlinux.APM` на объекте `/org/altlinux/APM2`.

| Шина    | Интерфейсы                                                                                   |
|---------|----------------------------------------------------------------------------------------------|
| System  | `Packages`, `Image` (только atomic), `Applications`, `Kernel` (кроме atomic), `Repo`, `Jobs` |
| Session | `Distrobox`, `Icons`, `Jobs`                                                                 |

Полные имена интерфейсов имеют префикс `org.altlinux.APM2.`, например
`org.altlinux.APM2.Packages`. Сервисы поднимаются через D-Bus activation при
первом обращении; вручную — `sudo apm dbus-system` и `apm dbus-session`.

## Формат данных

Скаляры, массивы строк и байтов передаются нативными типами D-Bus. Всё сложное —
JSON-строкой, без обёртки: в ответе лежит ровно тот же объект, что отдаёт HTTP API.

| Аргумент       | Смысл                                                        |
|----------------|--------------------------------------------------------------|
| `json`         | Ответ метода, сериализованный DTO домена                     |
| `request_json` | Запрос списка: `sort`, `order`, `limit`, `offset`, `filters` |
| `options_json` | Необязательные параметры метода                              |
| `config_json`  | Конфигурация образа                                          |

Разбор `options_json` строгий — неизвестное поле считается ошибкой клиента,
чтобы опечатка в имени опции не проходила молча. Пустая строка равнозначна `{}`.

Постраничные методы принимают `limit` (по умолчанию 100, максимум 1000) и `offset`.

```bash
busctl call org.altlinux.APM /org/altlinux/APM2 \
    org.altlinux.APM2.Packages Install ass 1 hello '{"noUpdate":true}'
```

## Фоновые задачи

Методы, возвращающие `job: u`, ставят задачу в очередь и немедленно отдают её id.
Результат приходит **только** сигналом, поэтому подписываться нужно до вызова.

Реестр хранит лишь работающие задачи: запись удаляется в момент завершения, и
`Jobs.Get` для завершённой вернёт `NotFound`.

Отмена: владелец задачи отменяет свободно, чужой — через polkit-действие того
домена, которому задача принадлежит. Транзакции rpm/apt отменять нельзя,
`Jobs.Cancel` для них вернёт ошибку валидации.

В `JobFinished` поле `status` принимает значения `ok`, `error` и `canceled`.
При ошибке в `json` приходит `{"errorType":"..."}` с машинным типом из таблицы
ниже, а человекочитаемый текст — в `message`.

```bash
gdbus monitor --system --dest org.altlinux.APM --object-path /org/altlinux/APM2
```

## Права (Polkit)

У каждого домена своё действие, применяется к изменяющим методам и к их
`Check*`-симуляциям; читающие методы доступны без авторизации.

| Интерфейс      | Действие                                |
|----------------|-----------------------------------------|
| `Packages`     | `org.altlinux.APM2.packages.manage`     |
| `Image`        | `org.altlinux.APM2.image.manage`        |
| `Applications` | `org.altlinux.APM2.applications.manage` |
| `Kernel`       | `org.altlinux.APM2.kernel.manage`       |
| `Repo`         | `org.altlinux.APM2.repo.manage`         |

Сессионная шина работает без polkit: `Distrobox` и `Icons` и так исполняются от
имени пользователя.

## Ошибки

Ошибки возвращаются как D-Bus Error с типизированным именем. Имена общие с API v1 —
таксономия одна на оба интерфейса.

| Тип            | D-Bus имя ошибки                          |
|----------------|-------------------------------------------|
| `DATABASE`     | `org.altlinux.APM.Error.Database`         |
| `REPOSITORY`   | `org.altlinux.APM.Error.Repository`       |
| `APT`          | `org.altlinux.APM.Error.Apt`              |
| `VALIDATION`   | `org.altlinux.APM.Error.Validation`       |
| `CANCELED`     | `org.altlinux.APM.Error.Canceled`         |
| `IMAGE`        | `org.altlinux.APM.Error.Image`            |
| `KERNEL`       | `org.altlinux.APM.Error.Kernel`           |
| `CONTAINER`    | `org.altlinux.APM.Error.Container`        |
| `NO_OPERATION` | `org.altlinux.APM.Error.NoOperation`      |
| `NOT_FOUND`    | `org.altlinux.APM.Error.NotFound`         |
| `PERMISSION`   | `org.freedesktop.DBus.Error.AccessDenied` |

Отказ в правах отдаётся стандартным `AccessDenied`, а неклассифицированная
ошибка — стандартным `org.freedesktop.DBus.Error.Failed`.

---

## Пакеты

Шина: **system**.

### `org.altlinux.APM2.Packages`

| Метод          | Аргументы                         | Ответ          |
|----------------|-----------------------------------|----------------|
| `Install`      | `packages: as`, `options_json: s` | `job: u`       |
| `Remove`       | `packages: as`, `options_json: s` | `job: u`       |
| `Reinstall`    | `packages: as`                    | `job: u`       |
| `Upgrade`      | `options_json: s`                 | `job: u`       |
| `Update`       | `options_json: s`                 | `job: u`       |
| `CheckInstall` | `packages: as`                    | `json: s`      |
| `CheckRemove`  | `packages: as`, `options_json: s` | `json: s`      |
| `CheckUpgrade` | —                                 | `json: s`      |
| `List`         | `request_json: s`                 | `json: s`      |
| `Info`         | `name: s`                         | `json: s`      |
| `MultiInfo`    | `names: as`                       | `json: s`      |
| `Search`       | `text: s`, `installed: b`         | `json: s`      |
| `Sections`     | —                                 | `sections: as` |
| `FilterFields` | —                                 | `json: s`      |
| `AptConfig`    | —                                 | `json: s`      |
| `SetAptConfig` | `options_json: s`                 | —              |

**Свойства**

| Свойство          | Тип | Доступ |
|-------------------|-----|--------|
| `Version`         | `s` | read   |
| `IsAtomic`        | `b` | read   |
| `KernelSupported` | `b` | read   |

## Образ системы

Шина: **system**.

### `org.altlinux.APM2.Image`

| Метод        | Аргументы                     | Ответ     |
|--------------|-------------------------------|-----------|
| `Status`     | —                             | `json: s` |
| `Update`     | `options_json: s`             | `job: u`  |
| `Apply`      | `options_json: s`             | `job: u`  |
| `Switch`     | `image: s`, `options_json: s` | `job: u`  |
| `History`    | `image: s`, `request_json: s` | `json: s` |
| `GetConfig`  | —                             | `json: s` |
| `SaveConfig` | `config_json: s`              | —         |
| `SyncGroups` | —                             | `json: s` |
| `FixNss`     | —                             | `json: s` |

## Каталог приложений

Шина: **system**.

### `org.altlinux.APM2.Applications`

| Метод          | Аргументы         | Ответ            |
|----------------|-------------------|------------------|
| `Update`       | —                 | `job: u`         |
| `Info`         | `name: s`         | `json: s`        |
| `List`         | `request_json: s` | `json: s`        |
| `Categories`   | —                 | `categories: as` |
| `FilterFields` | —                 | `json: s`        |

## Ядра

Шина: **system**.

### `org.altlinux.APM2.Kernel`

| Метод                 | Аргументы                                      | Ответ     |
|-----------------------|------------------------------------------------|-----------|
| `ListKernels`         | `flavour: s`, `installed_only: b`              | `json: s` |
| `Current`             | —                                              | `json: s` |
| `Install`             | `flavour: s`, `modules: as`, `options_json: s` | `job: u`  |
| `Update`              | `flavour: s`, `modules: as`, `options_json: s` | `job: u`  |
| `CheckInstall`        | `flavour: s`, `modules: as`, `options_json: s` | `json: s` |
| `CheckUpdate`         | `flavour: s`, `modules: as`, `options_json: s` | `json: s` |
| `CleanOld`            | `options_json: s`                              | `job: u`  |
| `CheckCleanOld`       | `options_json: s`                              | `json: s` |
| `ListModules`         | `flavour: s`                                   | `json: s` |
| `InstallModules`      | `flavour: s`, `modules: as`                    | `job: u`  |
| `CheckInstallModules` | `flavour: s`, `modules: as`                    | `json: s` |
| `RemoveModules`       | `flavour: s`, `modules: as`                    | `job: u`  |
| `CheckRemoveModules`  | `flavour: s`, `modules: as`                    | `json: s` |

## Репозитории

Шина: **system**.

### `org.altlinux.APM2.Repo`

| Метод            | Аргументы                | Ответ          |
|------------------|--------------------------|----------------|
| `List`           | `all: b`                 | `json: s`      |
| `Branches`       | —                        | `branches: as` |
| `TaskPackages`   | `task: s`                | `job: u`       |
| `TestTask`       | `task: s`                | `job: u`       |
| `Add`            | `sources: as`, `date: s` | `json: s`      |
| `Remove`         | `sources: as`, `date: s` | `json: s`      |
| `SetBranch`      | `branch: s`, `date: s`   | `json: s`      |
| `Clean`          | —                        | `json: s`      |
| `CheckAdd`       | `sources: as`, `date: s` | `json: s`      |
| `CheckRemove`    | `sources: as`, `date: s` | `json: s`      |
| `CheckSetBranch` | `branch: s`, `date: s`   | `json: s`      |
| `CheckClean`     | —                        | `json: s`      |

## Контейнеры

Шина: **session**.

### `org.altlinux.APM2.Distrobox`

| Метод             | Аргументы                                    | Ответ     |
|-------------------|----------------------------------------------|-----------|
| `ContainerList`   | —                                            | `json: s` |
| `ContainerAdd`    | `image: s`, `name: s`, `options_json: s`     | `job: u`  |
| `ContainerRemove` | `name: s`                                    | `job: u`  |
| `Update`          | `container: s`                               | `job: u`  |
| `Install`         | `container: s`, `name: s`, `options_json: s` | `job: u`  |
| `Remove`          | `container: s`, `name: s`, `options_json: s` | `job: u`  |
| `Info`            | `container: s`, `name: s`                    | `json: s` |
| `Search`          | `container: s`, `text: s`                    | `json: s` |
| `List`            | `container: s`, `request_json: s`            | `json: s` |
| `FilterFields`    | —                                            | `json: s` |

### `org.altlinux.APM2.Icons`

| Метод  | Аргументы                 | Ответ      |
|--------|---------------------------|------------|
| `Icon` | `name: s`, `container: s` | `icon: ay` |

## Фоновые задачи

Шина: **system и session**.

### `org.altlinux.APM2.Jobs`

| Метод    | Аргументы | Ответ     |
|----------|-----------|-----------|
| `List`   | —         | `json: s` |
| `Get`    | `job: u`  | `json: s` |
| `Cancel` | `job: u`  | —         |

**Сигналы**

| Сигнал        | Аргументы                                                                      |
|---------------|--------------------------------------------------------------------------------|
| `JobStarted`  | `job: u`, `domain: s`, `kind: s`                                               |
| `JobProgress` | `job: u`, `event: s`, `type: s`, `state: s`, `progress: d`, `progress_done: s` |
| `JobFinished` | `job: u`, `status: s`, `message: s`, `json: s`                                 |
