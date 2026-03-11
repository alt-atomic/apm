# APT C Bindings

C-интерфейс поверх libapt для использования через CGO

## Структура файлов

```
include/              — публичные заголовки (C API)
  apt.h               — единый include (подключает всё)
  apt_common.h        — общие типы: AptPackageChanges, callback'и, opaque-указатели
  apt_error.h         — коды ошибок, AptResult, строковые константы ошибок
  apt_system.h        — инициализация APT
  apt_cache.h         — кеш пакетов
  apt_config.h        — конфигурация APT
  apt_transaction.h   — транзакции (install/remove/plan/execute)
  apt_package.h       — метаданные пакетов, поиск
  apt_lock.h          — проверка блокировок
  apt_logging.h       — callback'и логирования
  apt_ext_rpm.h       — обработка RPM-файлов в аргументах
src/                  — внутренние заголовки
*.cpp                 — реализация C API
```

## Глобальные типы состояний

```c
typedef struct AptSystem      AptSystem;       // глобальное состояние APT
typedef struct AptCache       AptCache;        // открытый кеш пакетов
typedef struct AptTransaction AptTransaction;  // набор операций для plan/execute
```

### Информация о пакетах (apt_package.h)

```c
AptResult apt_package_get(AptCache *cache, const char *name, AptPackageInfo *info);
void      apt_package_free(AptPackageInfo *info);

AptResult apt_packages_search(AptCache *cache, const char *pattern, AptPackageList *result);
void      apt_packages_free(AptPackageList *list);
```

Метолы для работы с информацией о пакетах сильно изменяют и дополняют её:

**apt_package_get** — резолвит пакет по имени, виртуальному имени или пути к RPM-файлу:
- Обычное имя (`vim`) — прямой поиск в кеше
- Суффикс `.32bit` (`vim.32bit`) — срезается, ищется базовый пакет
- Не найден — пробует `i586-` префикс (`i586-vim`)

**apt_packages_search** — поиск по расширенному regex (имя + описание):
- Матчит по имени пакета и по LongDesc/ShortDesc
- Пропускает `i586-` вариант если базовый пакет имеет кандидата (чтобы не дублировать)

**Алиасы (i586/32bit)**. Оба метода заполняют поле `aliases` в `AptPackageInfo`:
- Для пакета `libSDL2` с `i586-libSDL2` вариантом — алиасы `["i586-libSDL2", "i586-libSDL2.32bit"]`
- Для пакета `i586-libSDL2` с базовым `libSDL2` — алиасы `["i586-libSDL2", "i586-libSDL2.32bit"]`

**Список файлов**. Поле `files` заполняется из RPM hdlist — читается заголовок RPM из файлов `/var/lib/apt/lists/*_hdlist.*`, извлекаются пути файлов

В итоге мы получаем дистиллированные списки где i586 улетают в алиасы, добавляются "файлы" известные из реп и тд. Работа с i586 никуда не уходит, просто APM скрывает все эти префиксы/суффиксы и пакеты в этом виде уже готовы для отображения на фронтенде

Ремарка:
Возможно это неправильно и странно, но делать огромное количество дубликатов мне я не хочу, поэтому я не придумал ничего лучше, фактически эти списки нужны только для внутренней базы, а когда будет происходить установка/удаление APT сам выберет пакет под архитектуру поэтому это влияет только на отображение

## Жизненный цикл

```
1. apt_init_config()               — настройка APT
2. apt_init_system(&system)        — инициализация, получаем AptSystem*
3. apt_cache_open(system, &cache)  — открываем кеш, получаем AptCache*
4. apt_transaction_new(cache, &tx) — создаём транзакцию

5. Наполняем транзакцию:
   - apt_transaction_install(tx, names, count)
   - apt_transaction_remove(tx, names, count, purge, depends)
   - apt_transaction_reinstall(tx, names, count)
   - apt_transaction_dist_upgrade(tx)
   - apt_transaction_autoremove(tx)

6. Либо симулируем, либо выполняем:
   - apt_transaction_plan(tx, &changes)               — что произойдёт (без фактических изменений)
   - apt_transaction_execute(tx, callback, ud, false) — реальная установка/удаление

7. Освобождение всякого последовательно:
   - apt_free_package_changes(&changes)
   - apt_transaction_free(tx)
   - apt_cache_close(cache)
   - apt_cleanup_system(system)
```

## API по группам

### Инициализация (apt_system.h)

```c
AptResult apt_init_config();                    // настроить APT
AptResult apt_init_system(AptSystem **system);  // создать системный handle
void apt_cleanup_system(const AptSystem *sys);  // освободить
```

### Кеш (apt_cache.h)

```c
AptResult apt_cache_open(const AptSystem *sys, AptCache **cache, bool with_lock);
void      apt_cache_close(AptCache *cache);
AptResult apt_cache_refresh(AptCache *cache);   // переоткрыть кеш
AptResult apt_cache_update(AptCache *cache);    // скачать свежие индексы репозиториев
```

`with_lock=true` — блокирует доступ (нужно для update/execute).
`with_lock=false` — только чтение (для симуляции, поиска).

### Транзакции (apt_transaction.h)

```c
AptResult apt_transaction_new(AptCache *cache, AptTransaction **tx);
void      apt_transaction_free(const AptTransaction *tx);

// Наполнение
AptResult apt_transaction_install(AptTransaction *tx, const char **names, size_t count);
AptResult apt_transaction_remove(AptTransaction *tx, const char **names, size_t count,
                                  bool purge, bool remove_depends);
AptResult apt_transaction_reinstall(AptTransaction *tx, const char **names, size_t count);
AptResult apt_transaction_dist_upgrade(AptTransaction *tx);
AptResult apt_transaction_autoremove(AptTransaction *tx);

// Симуляция — заполняет changes, система не меняется
AptResult apt_transaction_plan(const AptTransaction *tx, AptPackageChanges *changes);

// Выполнение — реально устанавливает/удаляет пакеты
AptResult apt_transaction_execute(const AptTransaction *tx,
                                   AptProgressCallback callback, uintptr_t user_data,
                                   bool download_only);
```

Транзакцию можно наполнять несколькими вызовами (install + remove в одной транзакции).
После `plan()` или `execute()` транзакция отработана — для новой операции нужна новая транзакция

### Информация о пакетах (apt_package.h)

```c
AptResult apt_package_get(AptCache *cache, const char *name, AptPackageInfo *info);
void      apt_package_free(AptPackageInfo *info);

AptResult apt_packages_search(AptCache *cache, const char *pattern, AptPackageList *result);
void      apt_packages_free(AptPackageList *list);
```

### Блокировки (apt_lock.h)

```c
AptLockStatus apt_check_lock_status();
void          apt_free_lock_status(AptLockStatus *status);
```

### Конфигурация (apt_config.h)

```c
AptErrorCode apt_set_config(const char *key, const char *value);
char        *apt_config_dump(void);          // получить весь конфиг
void        *apt_config_snapshot(void);      // делает копию и сохраняет копию текущего конфига
void         apt_config_restore(void *snap); // Восстанавливает исходный конфиг
```

Работа с конфигами этими методами удобна тем что можно переопределить конфигурацию APT на фронтенде, например только для одной транзакции или в целом для всей сессии и затем откатить

### Логирование (apt_logging.h)

```c
void apt_set_log_callback(AptLogCallback cb, uintptr_t user_data);
void apt_use_go_progress_callback(uintptr_t user_data);
void apt_enable_go_log_callback(uintptr_t user_data);
void apt_capture_stdio(int enable);
```

Множество операции внутри либы включая работу с rpm ПЫТАЮТСЯ писать в вывод, что бы это запретить есть некоторые "васянские хаки" для перехвата и парсинга

### RPM-аргументы (apt_ext_rpm.h)

```c
AptResult apt_preprocess_install_arguments(const char **names, size_t count, bool *added_new);
void      apt_clear_install_arguments(void);
```

## Потокобезопасность

Библиотека не потокобезопасна (колбеки глобальные например) поэтому нельзя делать более одной операции одновременно. 
Go-обёртка сериализует вызовы через мьютекс, так как в apm своя база данных - ему не нужны info/search методы на каждую операцию, только при общем обновлении или в других редких случаях при работе с rpm файлом.
