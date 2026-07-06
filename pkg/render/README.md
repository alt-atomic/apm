# Render

Рендер произвольных Go-структур в красивое дерево или plain-текст для терминала.

Построение дерева здесь собственное, на `strings.Builder` — без промежуточных узлов-объектов. Изначально дерево строилось через `lipgloss/tree`, но на больших ответах (тысячи пакетов) это было значительно медленнее и жрало аллокации, поэтому осталась только стилизация из lipgloss (цвета, bold), а разметку веток пакет рисует сам. Ориентир по скорости: дерево из 3000 элементов рендерится за ~8-9 мс (`go test -bench=. ./pkg/render/`).

## Пример

```go
r := render.New(render.DefaultColors(),
    render.WithAccentKeys("name"), // значения этих ключей — акцентным цветом
)

data := map[string]interface{}{
    "message": "Package found",
    "package": map[string]interface{}{
        "name":    "vim",
        "version": "9.1",
        "size":    3145728,
    },
}

fmt.Println(r.RenderText(data, render.FormatTypeTree, false))
```

```
├── Package found
│
╰── package
    ├── name: vim
    ├── size: 3145728
    ╰── version: 9.1
```

Сигнатура: `RenderText(data map[string]interface{}, formatType FormatType, isError bool)`. `FormatTypePlain` вместо `FormatTypeTree` даёт плоский вывод построчно (`name: vim`), вложенность — через точку (`appStream.id: ...`); единственная обёртка верхнего уровня разворачивается — удобно для grep. `isError=true` рисует `message` в цвете ошибки вместо жирного заголовка.

На вход идёт `map[string]interface{}`, но любую структуру можно скормить через `render.ToDataMap(v)` (JSON round-trip). Ключ `message` — контракт пакета (как `msg` в slog): всегда рендерится первым и стилизуется как заголовок (или как ошибка при `isError=true`), а `FilterFields` его сохраняет. Если в ваших данных `message` — обычное поле, переименуйте его до рендера.

## Зависимости задаются снаружи

Переводы и человеческие имена полей по умолчанию:

```go
render.SetTranslator(myGettext)
render.SetKeyTranslator(myFieldNames)    // "packageName" -> "Имя пакета"
```

Семантика конкретных ключей задаётся опциями конструктора:

```go
r := render.New(myColors,
    render.WithAccentKeys("name", "url"),       // какие значения подсвечивать
    render.WithValueFormatter(mySizeFormatter), // своё форматирование значения по ключу
)
```

`WithValueFormatter` получает `(key, value)` и возвращает `(string, bool)` — при `true` заменяет стандартный вывод (так apm форматирует поля размеров в «3.00 MB»). Цвета — структура `Colors` с yaml-тегами, её можно грузить прямо из конфига; `DefaultColors()` — рабочая палитра по умолчанию.

## Фильтрация полей

`FilterFields` оставляет только запрошенные поля (в apm это флаг `-o fields=...`), точка выбирает вложенные:

```go
data := map[string]interface{}{
    "message": "Found",
    "package": map[string]interface{}{
        "name":      "vim",
        "version":   "9.1",
        "appStream": map[string]interface{}{"id": "vim.desktop", "icon": "..."},
    },
}
filtered := render.FilterFields(data, []string{"name", "appStream.id"})
fmt.Println(r.RenderText(filtered, render.FormatTypeTree, false))
```
