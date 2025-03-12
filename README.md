# APM (Atomic Package Manager)

APM — это универсальное приложение для управления как системными пакетами, так и пакетами из distrobox. 
Оно объединяет все функции в единое API, обеспечивает взаимодействие через DBUS-сервис и предоставляет опциональную поддержку атомарных образов на базе ALT Linux.

Программа поддерживает три режима работы:
* DBUS-сервис
* Консольное приложение
* Поддержка атомарных образов (функционал и модель поведения определяется автоматически)

Два формата ответов :
* форматированный text (Стандартное значение)
* json (Опционально, флаг -f json)

**Внимание!**

При работе с APM из атомарного образа форматированный текстовый ответ (text) может быть изменён.


Для подробной справки после установки вызовите:
```
apm -help
```

## Установка
Для установки вручную выполните в консоли:

```
curl -fsSL https://raw.githubusercontent.com/alt-atomic/apm/main/data/install.sh | sudo bash
```

Общая справка:
```
apm -h

NAME:
   apm - Atomic Package Manager

USAGE:
   apm [global options] [command [command options]]

COMMANDS:
   dbus-user     Запуск DBUS-сервиса com.application.APM
   dbus-system   Запуск DBUS-сервиса com.application.APM
   system, s     Управление системными пакетами
   distrobox, d  Управление пакетами и контейнерами distrobox
   help, h       Показать список команд или справку по каждой команде

GLOBAL OPTIONS:
   --format value, -f value       Формат вывода: json, text (default: "text")
   --transaction value, -t value  Внутреннее свойство, добавляет транзакцию к выводу
   --help, -h                     show help
```


## Пользовательская сессия DBUS
При запуске в пользовательской сессии сервис регистрируется в сессионной шине DBUS, что не требует дополнительных привилегий.
В этом режиме софт работает с контейнерами distrobox, для просмотра всех методов установите например D-SPY и найдите там сервис APM

```
apm dbus-user
```

## Системная сессия DBUS
Для корректной работы необходимо применить политику доступа путём копирования файла apm.conf.
В этом режиме софт работает с системными пакетами, для просмотра всех методов установите например D-SPY и найдите там сервис APM
```
sudo cp data/dbus-config/apm.conf /etc/dbus-1/system.d/

sudo apm dbus-system
```

## Пример работы с системными пакетами
```
apm s

NAME:
   apm system - Управление системными пакетами

USAGE:
   apm system [command [command options]] 

COMMANDS:
   install     Список пакетов на установку
   remove, rm  Список пакетов на удаление
   update      Обновление пакетной базы
   info        Информация о пакете
   search      Быстрый поиск пакетов по названию
   list        Построение запроса для получения списка пакетов
   image, i    Модуль для работы с образом

OPTIONS:
   --help, -h  show help

```

### Установка
При работе из атомарной системы становится доступен флаг -apply/-a. При указании данного флага пакет будет добавлен в систему, а образ пересобран.

Пример запроса:
```
apm s install zip

⚛
├── zip успешно удалён.
╰── Информация
    ├── Дополнительно установлено: нет
    ├── Количество новых установок: 0
    ├── Новые установленные пакеты: нет
    ├── Количество не обновленных: 64
    ├── Количество удаленных: 1
    ├── Удаленные пакеты
    │   ╰── 1) zip
    ├── Количество обновленных: 0
    ╰── Обновленные пакеты: нет
```

Если формат ответ не указан как json и источником запроса не является DBUS - запускается диалог предварительного анализа пакетов для установки 

![img.png](data/img/install.png)


Результат выполнения в формате json:
```
apm s install zip -f json

{
  "data": {
    "info": {
      "extraInstalled": null,
      "upgradedPackages": null,
      "newInstalledPackages": [
        "zip"
      ],
      "removedPackages": null,
      "upgradedCount": 0,
      "newInstalledCount": 1,
      "removedCount": 0,
      "notUpgradedCount": 64
    }
  },
  "error": false
}
```

### Удаление
При работе из атомарной системы становится доступен флаг -apply/-a. При указании данного флага пакет будет добавлен в систему, а образ пересобран.

Пример запроса:
```
apm s remove zip

⚛
├── zip успешно удалён.
╰── Информация
    ├── Дополнительно установлено: нет
    ├── Количество новых установок: 0
    ├── Новые установленные пакеты: нет
    ├── Количество не обновленных: 64
    ├── Количество удаленных: 1
    ├── Удаленные пакеты
    │   ╰── 1) zip
    ├── Количество обновленных: 0
    ╰── Обновленные пакеты: нет
    
```

Если формат ответ не указан как json и источником запроса не является DBUS - запускается диалог предварительного анализа пакетов для удаления

![img.png](data/img/remove.png)


Результат выполнения в формате json:
```
apm s remove zip -f json

{
  "data": {
    "info": {
      "extraInstalled": null,
      "upgradedPackages": null,
      "newInstalledPackages": null,
      "removedPackages": [
        "zip"
      ],
      "upgradedCount": 0,
      "newInstalledCount": 0,
      "removedCount": 1,
      "notUpgradedCount": 64
    }
  },
  "error": false
}

```

### Списки
Списки позволяют выстраивать сложные запросы путём фильтрации и сортировки

```
apm s list -h
     
NAME:
   apm system list - Построение запроса для получения списка пакетов

USAGE:
   apm system list [command [command options]]

OPTIONS:
   --sort value                      Поле для сортировки, например: name, installed
   --order value                     Порядок сортировки: ASC или DESC (default: "ASC")
   --limit value                     Лимит выборки (default: 10)
   --offset value                    Смещение выборки (default: 0)
   --filter-field value, --ff value  Название поля для фильтрации, например: name, version, manager, section
   --filter-value value, --fv value  Значение для фильтрации по указанному полю
   --force-update                    Принудительно обновить все пакеты перед запросом (default: false)
   --help, -h                        show help

GLOBAL OPTIONS:
   --format value, -f value       Формат вывода: json, text (default: "text")
   --transaction value, -t value  Внутреннее свойство, добавляет транзакцию к выводу
```

Например, что бы достать самый "тяжелый" пакет и отобразить только одну запись:
```
apm system list --sort="size" --order="DESC" -limit 1

⚛
├── Найдена 1 запись
├── Пакеты
│   ╰── 1)
│       ├── Зависимости
│       │   ╰── 1) speed-dreams
│       ├── Описание: Game data for Speed Dreams                     
│       │   Speed Dreams ia a fork of the racing car simulator Torcs,
│       │   with some new features.                                  
│       ├── Имя файла: speed-dreams-data-2.3.0-alt1.x86_64.rpm
│       ├── Установлено: нет
│       ├── Займёт на диске: 2722.10 MB
│       ├── Мэйнтейнер: Artyom Bystrov <arbars@altlinux.org>
│       ├── Название: speed-dreams-data
│       ├── Раздел: Games/Sports
│       ├── Размер: 1942.57 MB
│       ├── Версия: 2.3.0
│       ╰── Установленная версия: нет
╰── Всего записей: 53865
```

Или найти все пакеты установленные в системе и ограничить вывод одним пакетом:
```
apm system list -ff="installed" -fv="true" -limit 1

⚛
├── Найдена 1 запись
├── Пакеты
│   ╰── 1)
│       ├── Зависимости
│       │   ├── 9) perl
│       │   ├── 10) perl-base
│       │   ╰── 11) rtld
│       ├── Описание: Parts of the groff formatting system that is required for viewing manpages
│       │   A stripped-down groff package containing the components required                    
│       │   to view man pages in ASCII, Latin-1 and UTF-8.                                      
│       ├── Имя файла: groff-base-1.22.3-alt2.x86_64.rpm
│       ├── Установлено: да
│       ├── Займёт на диске: 3.19 MB
│       ├── Мэйнтейнер: Alexey Gladkov <legion@altlinux.ru>
│       ├── Название: groff-base
│       ├── Раздел: Text tools
│       ├── Размер: 0.83 MB
│       ├── Версия: 1.22.3
│       ╰── Установленная версия: 1.22.3
╰── Всего записей: 1594
```

Для построения запросов лучше посмотреть ответ в формате json что бы увидеть названия полей не отформатированные выводом.


## Пример работы с distrobox
```
apm d                                    
NAME:
   apm distrobox - Управление пакетами и контейнерами distrobox

USAGE:
   apm distrobox [command [command options]] 

COMMANDS:
   update        Обновить и синхронизировать списки установленных пакетов с хостом
   info          Информация о пакете
   search        Быстрый поиск пакетов по названию
   list          Построение запроса для получения списка пакетов
   install       Установить пакет
   remove, rm    Удалить пакет
   container, c  Модуль для работы с контейнерами

OPTIONS:
   --help, -h  show help
```

### Добавление контейнера

```
Поле image поддерживает три образа: alt, arch, ubuntu
Добавление контейнера alt:
apm distrobox c create --image alt
```

### Списки

Списки для distrobox построены схожим образом с системными пакетами, описание:

```
apm distrobox list -h       
             
NAME:
   apm distrobox list - Построение запроса для получения списка пакетов

USAGE:
   apm distrobox list [command [command options]]

OPTIONS:
   --container value, -c value       Название контейнера. Необходимо указать
   --sort value                      Поле для сортировки, например: packageName, version
   --order value                     Порядок сортировки: ASC или DESC (default: "ASC")
   --limit value                     Лимит выборки (default: 10)
   --offset value                    Смещение выборки (default: 0)
   --filter-field value, --ff value  Название поля для фильтрации, например: packageName, version, manager
   --filter-value value, --fv value  Значение для фильтрации по указанному полю
   --force-update                    Принудительно обновить все пакеты перед запросом (default: false)
   --help, -h                        show help

GLOBAL OPTIONS:
   --format value, -f value       Формат вывода: json, text (default: "text")
   --transaction value, -t value  Внутреннее свойство, добавляет транзакцию к выводу
```

Для получения всех пакетов контейнера atomic-alt:

```
apm distrobox list -c atomic-alt -limit 1

⚛
├── Найдена 1 запись
├── Пакеты
│   ╰── 1)
│       ├── Описание: Libraries and header files for LLVM                       
│       │   This package contains library and header files needed to develop new
│       │   native programs that use the LLVM infrastructure.                   
│       ├── Экспортировано: нет
│       ├── Установлено: нет
│       ├── Пакетный менеджер: apt-get
│       ├── Название пакета: llvm14.0-devel
│       ╰── Версия: 14.0.6
╰── Всего записей: 53865
```
