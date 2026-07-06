# APT Repo

Управление списком APT-репозиториев ALT Linux

```go
aptrepo.SetTranslator(myT)
aptrepo.SetLogger(myLogger)
```

## Использование

```go
hasPackage := func(ctx context.Context, name string) bool {
    // установлен ли пакет; нужен только для apt-https (схема http/https)
    return false
}

s := aptrepo.NewRepoService(hasPackage, command.NewRunner("", false))

repos, err := s.GetRepositories(ctx, true)
added, err := s.AddRepository(ctx, []string{"p11"}, "")
removed, err := s.RemoveRepository(ctx, []string{"12345"}, "", false)
added, removed, err := s.SetBranch(ctx, "sisyphus", "20240315")
```

Источник — это имя ветки (`p11`, `sisyphus`, без учёта регистра; опционально с датой архива `YYYYMMDD` или `YYYY/MM/DD`), номер задачи git.altlinux.org, URL или строка в формате sources.list(5).

## API

- `NewRepoService(hasPackage, runner)` — конструктор
- `GetRepositories`, `AddRepository`, `RemoveRepository`, `SetBranch`, `CleanTemporary` — операции над источниками
- `SimulateAdd`, `SimulateRemove` — те же операции без записи в файлы
- `GetBranches`, `GetTaskPackages`, `GetArch`, `GetConfMain` — справочные
- `Repository` — описание источника с json-тегами, отдаётся наружу как есть

## Поведение

Пути конфигурации берутся из `apt-config` (лениво, при первом обращении), при его отсутствии — `/etc/apt/sources.list*`. Добавление раскомментирует существующую строку, удаление стирает из основного файла и комментирует в остальных — как apt-repo. Для задач при удалении строятся оба варианта URL (активный и архивный) без сетевых проверок, плюс обе схемы http/https. Мутирующие операции сериализуются внутренним мьютексом; чтение — без блокировки.
