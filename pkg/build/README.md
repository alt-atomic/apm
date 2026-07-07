# pkg/build

Встраиваемый простой движок для DSL сборочных рецептов на основе модулей (YAML/JSON).

Движок выполняет список *модулей* над файловой системой: копирует файлы, запускает
shell-команды, создаёт симлинки, подключает другие рецепты и т.д. Он ничего не знает
о конкретной предметной области — вызывающий сам предоставляет способности времени
выполнения и может регистрировать собственные типы модулей.

## Быстрый старт

```go
package main

import (
	"context"

	"altlinux.space/alt-atomic/apm/pkg/build"
	_ "altlinux.space/alt-atomic/apm/pkg/build/modules
	"altlinux.space/alt-atomic/apm/pkg/command"
)

// host реализует build.RuntimeContext.
type host struct {
	engine *build.Engine
	runner command.Runner
}

func (h *host) Runner() command.Runner    { return h.runner }
func (h *host) CollectOutput(text string) { h.engine.CollectOutput(text) }

func (h *host) EmitEvent(ctx context.Context, state, name, view string) {
	// колбэк прогресса; state — это build.StateBefore или build.StateAfter
}

// ExecuteInclude делегирует движку, передавая контекст обратно (или своя логика)
func (h *host) ExecuteInclude(ctx context.Context, target string) (map[string]*build.MapModule, error) {
	return h.engine.ExecuteInclude(ctx, h, target)
}

func main() {
	// build.SetLogger(myLogger)
	// build.SetTranslator(func(s string) string { return s })

	eng := build.NewEngine(build.Version{}, "myapp.module")
	h := &host{engine: eng, runner: command.NewRunner("", false)}

	cfg, err := build.ReadAndParseConfigYamlFile("recipe.yml")
	if err != nil {
		panic(err)
	}

	if err := eng.Build(context.Background(), h, cfg.Modules); err != nil {
		panic(err)
	}
}
```

- `ver` доступен в выражениях рецепта как `${{ Version.* }}` (передай `build.Version{}`, если не нужен).
- `eventPrefix` — префикс имён событий, напр. `myapp.module.1`, `myapp.module.2`.

## Формат рецепта

```yaml
image: "alt:sisyphus"
modules:
  - name: create dir
    type: mkdir
    body:
      targets:
        - /opt/app
      perm: "rwxr-xr-x"

  - name: copy config
    type: copy
    body:
      source: /src/app.conf
      destination: /etc/app.conf

  - name: include another config
    type: include
    body:
      targets:
        - ./extra.yml
```

Ключи уровня модуля, которые обрабатывает движок:

- `type` — тип модуля (обязателен).
- `name` — метка для логов/событий.
- `id` + `output` — прокинуть значения в следующие модули через `${{ Modules.<id>.<key> }}`.
- `if` — условие [expr](https://expr-lang.org/); модуль пропускается, если false.
- `env` — переменные окружения, выставляемые для этого модуля.

Выражения используют `${{ ... }}` и читают `Version`, `Env`, `Modules`.

## Свой модуль

Модуль — любая структура, реализующая `Body`, зарегистрированная под именем `type`.

```go
package mymodules

import (
	"context"

	build "altlinux.space/alt-atomic/apm/pkg/build"
)

type EchoBody struct {
	Message string `yaml:"message" json:"message" required:""`
}

func (b *EchoBody) Execute(ctx context.Context, rt build.RuntimeContext) (any, error) {
	build.Log().Info(b.Message)
	return nil, nil
}

func init() {
	build.Register("echo", func() build.Body { return &EchoBody{} })
}
```

## Хуки

```go
eng.Hook(build.PreModules, func (ctx context.Context) error { return nil }) // до модулей
eng.Hook(build.PostModules, func (ctx context.Context) error { return nil }) // после модулей, даже при ошибке
eng.Hook(build.PostBuild, func (ctx context.Context) error { return nil }) // после успешного прохода
```

## Логирование и перевод

```go
build.SetLogger(myLogger) // реализует build.Logger
build.SetTranslator(func (s string) string { … }) // напр. gettext
```

## Хелперы
- `ReadAndParseConfigYamlFile(path)` / `ParseYamlConfigData(bytes)` / `ParseJsonConfigData(bytes)` — разобрать полный
  `Config`.
- `ReadAndParseModules(target)` — разобрать список модулей из файла, директории или URL.
- `ValidateConfigRecursive(cfg, basePath)` — провалидировать конфиг и все его `include`-цели.
