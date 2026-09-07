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

Методы, возвращающие `job: s`, ставят задачу в очередь и немедленно отдают её id.
Результат приходит **только** сигналом, поэтому подписываться нужно до вызова.

Id задачи — строка вида `:1.42-3`: уникальное имя демона на шине и счётчик,
так что id не повторяются и после перезапуска сервиса.

Завершённая задача остаётся в реестре ещё 20 минут: `Jobs.Get` отдаёт её
итоговое состояние и результат, если клиент пропустил сигнал. После этого
`Jobs.Get` вернёт `NotFound`.

Отмена: владелец задачи отменяет свободно, чужой — через polkit-действие того
домена, которому задача принадлежит. Транзакции rpm/apt отменять нельзя,
`Jobs.Cancel` для них вернёт ошибку валидации.

В `JobFinished` поле `status` принимает значения `ok`, `error` и `canceled`.
При `ok` в `json` лежит ответ метода, при `error` и `canceled` — `{}`. Машинный
тип ошибки приходит в `error_type` (значения из таблицы ниже, пусто для
неклассифицированной ошибки), человекочитаемый текст — в `message`.

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
| `Install`      | `packages: as`, `options_json: s` | `job: s`       |
| `Remove`       | `packages: as`, `options_json: s` | `job: s`       |
| `Reinstall`    | `packages: as`                    | `job: s`       |
| `Upgrade`      | `options_json: s`                 | `job: s`       |
| `Update`       | `options_json: s`                 | `job: s`       |
| `CheckInstall` | `packages: as`                    | `job: s`       |
| `CheckRemove`  | `packages: as`, `options_json: s` | `job: s`       |
| `CheckUpgrade` | —                                 | `job: s`       |
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
| `Update`     | `options_json: s`             | `job: s`  |
| `Apply`      | `options_json: s`             | `job: s`  |
| `Switch`     | `image: s`, `options_json: s` | `job: s`  |
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
| `Update`       | —                 | `job: s`         |
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
| `Install`             | `flavour: s`, `modules: as`, `options_json: s` | `job: s`  |
| `Update`              | `flavour: s`, `modules: as`, `options_json: s` | `job: s`  |
| `CheckInstall`        | `flavour: s`, `modules: as`, `options_json: s` | `job: s`  |
| `CheckUpdate`         | `flavour: s`, `modules: as`, `options_json: s` | `job: s`  |
| `CleanOld`            | `options_json: s`                              | `job: s`  |
| `CheckCleanOld`       | `options_json: s`                              | `job: s`  |
| `ListModules`         | `flavour: s`                                   | `json: s` |
| `InstallModules`      | `flavour: s`, `modules: as`                    | `job: s`  |
| `CheckInstallModules` | `flavour: s`, `modules: as`                    | `job: s`  |
| `RemoveModules`       | `flavour: s`, `modules: as`                    | `job: s`  |
| `CheckRemoveModules`  | `flavour: s`, `modules: as`                    | `job: s`  |

## Репозитории

Шина: **system**.

### `org.altlinux.APM2.Repo`

| Метод            | Аргументы                | Ответ          |
|------------------|--------------------------|----------------|
| `List`           | `all: b`                 | `json: s`      |
| `Branches`       | —                        | `branches: as` |
| `TaskPackages`   | `task: s`                | `job: s`       |
| `TestTask`       | `task: s`                | `job: s`       |
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
| `ContainerAdd`    | `image: s`, `name: s`, `options_json: s`     | `job: s`  |
| `ContainerRemove` | `name: s`                                    | `job: s`  |
| `Update`          | `container: s`                               | `job: s`  |
| `Install`         | `container: s`, `name: s`, `options_json: s` | `job: s`  |
| `Remove`          | `container: s`, `name: s`, `options_json: s` | `job: s`  |
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
| `Get`    | `job: s`  | `json: s` |
| `Cancel` | `job: s`  | —         |

**Сигналы**

| Сигнал        | Аргументы                                                                                    |
|---------------|----------------------------------------------------------------------------------------------|
| `JobStarted`  | `job: s`, `domain: s`, `kind: s`                                                             |
| `JobProgress` | `job: s`, `event: s`, `type: s`, `state: s`, `message: s`, `progress: d`, `progress_done: s` |
| `JobFinished` | `job: s`, `status: s`, `error_type: s`, `message: s`, `json: s`                              |
