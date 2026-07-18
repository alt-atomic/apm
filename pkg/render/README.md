# Render

Рендер произвольных Go-структур в красивое дерево или plain-текст для терминала.

Построение дерева здесь собственное, на `strings.Builder` — без промежуточных узлов-объектов. Изначально дерево строилось через `lipgloss/tree`, но на больших ответах (тысячи пакетов) это было значительно медленнее и жрало аллокации, поэтому осталась только стилизация из lipgloss (цвета, bold), а разметку веток пакет рисует сам. Ориентир по скорости: дерево из 3000 элементов рендерится за ~4-5 мс (`go test -bench=. ./pkg/render/`).

## Пример

```go
type Pkg struct {
    Name    string `json:"name"`
    Version string `json:"version"`
}

type Response struct {
    Message string `json:"message"`
    Package Pkg    `json:"package"`
}

r := render.New(render.DefaultColors(),
    render.WithAccentKeys("name"), // значения этих ключей — акцентным цветом
)

resp := Response{
    Message: "Package found",
    Package: Pkg{Name: "vim", Version: "9.1"},
}

// ToDataMap превращает структуру в map, ключи — из json-тегов
fmt.Println(r.RenderText(render.ToDataMap(resp), render.FormatTypeTree, false))
```

```
├── Package found
│
╰── package
    ├── name: vim
    ╰── version: 9.1
```

`FormatTypePlain` даёт плоский вывод, вложенность — через точку (`appStream.id: ...`) — удобно для grep. `isError=true` рисует `message` цветом ошибки.

Числа приходят в `WithValueFormatter` единым типом `json.Number` и печатаются точно (`render.AsInt` в помощь); типы со своим `String()` не трогаются. Ключ `message` — контракт пакета (как `msg` в slog): рендерится первым как заголовок, `FilterFields` его сохраняет; если у вас это обычное поле — переименуйте его до рендера.

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
