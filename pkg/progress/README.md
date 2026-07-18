# Progress

Анимированный спиннер задач и прогресс-баров для терминала.
Своя перерисовка на ANSI-кодах без bubbletea. Спиннер печатает активные задачи со сменой кадра, а завершённые сдвигает выше со значком `[✓]`


## Использование
```go
progress.SetTranslator(myT)
```

```go
sp := progress.New(progress.Colors{Filled: "#26a269", Empty: "#c4c8c6"})
sp.Start()

sp.Update(progress.TaskUpdate{Name: "update", View: "Обновление списка пакетов"})
// ... позже, по тому же Name:
sp.Update(progress.TaskUpdate{Name: "update", View: "Обновление списка пакетов", Done: true})

sp.Stop() 
```

Задача адресуется по `Name`: первый `Update` добавляет её, последующие с тем же именем обновляют. `Done: true` переводит задачу в завершённые.

## Прогресс-бар

```go
sp.Update(progress.TaskUpdate{
    Name:       "download",
    View:       "ftp.altlinux.org release",
    IsProgress: true,
    Percent:    42,
})
```

`IsProgress` рисует полосу с процентами вместо простой строки со спиннером. При завершении, если задан `DoneText`, печатается строка `Progress: <DoneText> completed`.
