# Метаданные ОС в модуле branding

```yaml
modules:
  - type: branding
    body:
      name: alt-atomic-onyx
      build-type: stable
      metainfo:
        name:
          C: ALT Atomic Onyx
          ru: ALT Atomic Оникс
        summary:
          C: ALT Atomic image with GNOME
          ru: Образ ALT Atomic с GNOME
        description:
          C: |
            <p>ALT Atomic Onyx is an atomic operating system with GNOME.</p>
          ru: |
            <p>ALT Atomic Onyx — атомарная операционная система с GNOME.</p>
        variants:
          - image: altlinux.space/alt-atomic/onyx/stable:latest
            conditions:
              - type: archs
                value: x86_64,aarch64
            name:
              C: ALT Atomic Onyx
            summary:
              C: ALT Atomic Onyx default version
          - image: altlinux.space/alt-atomic/onyx-nvidia/stable:latest
            name:
              C: ALT Atomic Onyx (NVIDIA)
            summary:
              C: ALT Atomic Onyx with NVIDIA drivers
        latest-release:
          version: "0.5"
          date: "2026-10-10"
          description:
            ru: |
              <p>Обновлены компоненты системы.</p>
```

`C` задаёт исходный текст, остальные ключи — переводы. `description` содержит
XML-разметку AppStream (`p`, `ul`, `ol`, `li`); некорректный XML приводит к ошибке.
`variants` сохраняется в XML как `x-variants` и содержит список образов с переводимыми `name`, `summary`, `description`.
`variants[].conditions` — необязательный список ограничений с полями `type`,
`value` и необязательным `op`. Каждое ограничение создаёт XML-элемент
`<condition type="archs">x86_64,aarch64</condition>` внутри `<conditions>`.
Если список отсутствует или пуст, `<conditions>` не создаётся. Типы ограничений
передаются как есть; генератор не проверяет их семантику.

После установки пакетов брендинга и изменения os-release модуль определяет ID
компонента из его `HOME_URL` и `ID` из os-release. Примеры:

```
HOME_URL=www.alt-atomic.org
ID=Diamond
component-id: org.alt_atomic.Diamond
```

```
HOME_URL=www.alt-atomic.org
ID=org.alt_atomic.Diamond
component-id: org.alt_atomic.Diamond
```

Если компонент `operating-system` с этим ID найден в локальных metainfo или
каталогах AppStream, без поля `metainfo` генерация XML пропускается, а с ним
(включая `{}`) возвращается ошибка конфликта. Иначе создаётся
`/usr/share/metainfo/<id>.metainfo.xml` с настройками из YAML и значениями
по умолчанию из os-release. Установка брендинга и изменения os-release
выполняются до этой проверки.

| Поле XML               | Источник                                   |
|------------------------|--------------------------------------------|
| `id`                   | Алгоритм выше                              |
| `name` без перевода    | `NAME`, если ключ `C` отсутствует в YAML   |
| `summary` без перевода | `NAME`, если ключ `C` отсутствует в YAML   |
| `url type="homepage"`  | `HOME_URL`                                 |
| `developer/@id`        | Обратный домен `VENDOR_URL`                |
| `developer/name`       | `VENDOR_NAME`                              |
| `release/@version`     | `VERSION_ID`, если версия не задана в YAML |
| `release/@type`        | `RELEASE_TYPE` из `os-release`             |
| `metadata_license`     | `FSFAP`                                    |

Для `release/@type` значения `stable` и `lts` преобразуются в `stable`,
`development` и `experiment` — в `development`. Если `RELEASE_TYPE` отсутствует
или неизвестен, используется `stable`, как предусмотрено `os-release`.
Тип релиза задаётся автоматически, без дополнительного поля в YAML.

Отсутствующие описания, варианты и дата выпуска не генерируются. Раздел developer
выводится при наличии `VENDOR_NAME`, раздел releases — при наличии версии.
Явно заданные переводы не заменяются значениями из os-release.
