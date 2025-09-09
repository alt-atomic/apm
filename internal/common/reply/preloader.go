// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package reply

import (
	"apm/internal/common/app"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	p             *tea.Program
	doneChan      chan struct{}
	tasksDoneChan chan struct{}
	mu            sync.Mutex
	lastLines     int
	lastRender    string
)

// TaskUpdateMsg TASK" или "PROGRESS"
type TaskUpdateMsg struct {
	eventType        string
	taskName         string
	viewName         string
	state            string
	progressValue    float64
	progressDoneText string
}

type task struct {
	eventType string
	name      string
	viewName  string
	state     string

	progressModel    *progress.Model
	progressDoneText string
}

type model struct {
	spinner      spinner.Model
	tasksSpinner spinner.Model
	tasks        []task
}

// CreateSpinner Создание и запуск Bubble Tea
func CreateSpinner() {
	if lib.Env.Format != "text" || !IsTTY() {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if p != nil {
		return
	}
	doneChan = make(chan struct{})
	tasksDoneChan = make(chan struct{})

	p = tea.NewProgram(
		newModel(),
		tea.WithOutput(os.Stdout),
		tea.WithInput(nil),
	)

	go func() {
		_, err := p.Run()
		if err != nil {
			app.Log.Error(err.Error())
		}
		close(doneChan)
	}()
}

// StopSpinner Остановка и очистка вывода
func StopSpinner() {
	StopSpinnerWithKeepTasks(true)
}

// StopSpinnerWithKeepTasks Остановка с возможностью сохранения задач
func StopSpinnerWithKeepTasks(keepTasks bool) {
	if lib.Env.Format != "text" || !IsTTY() {
		return
	}

	// Ждём, пока все задачи не завершены, но не более 100мс
	select {
	case <-tasksDoneChan:
	case <-time.After(100 * time.Millisecond):
	}

	// Небольшая пауза
	time.Sleep(60 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if p != nil {
		p.Quit()
		<-doneChan
		p = nil

		// Безопасно перерисуем блок: очистим всё и выведем заново без первой строки
		if lastLines > 0 {
			// Подняться к первой строке блока
			for i := 0; i < lastLines-1; i++ {
				fmt.Print("\033[F")
			}
			// Очистить все строки блока
			for i := 0; i < lastLines; i++ {
				fmt.Print("\r\033[2K")
				if i < lastLines-1 {
					fmt.Print("\033[E")
				}
			}
			// Вернуться к началу блока
			for i := 0; i < lastLines-1; i++ {
				fmt.Print("\033[F")
			}

			// Переотрисовать без первой строки (удаляем строку со спиннером "Executing tasks")
			if keepTasks && lastRender != "" {
				lines := strings.Split(lastRender, "\n")
				if len(lines) > 1 {
					fmt.Print(strings.Join(lines[1:], "\n"))
					// гарантируем перевод строки после переотрисовки, чтобы следующий вывод не склеивался
					fmt.Print("\n")
				}
			}
		}
	}
}

// StopSpinnerForDialog останавливает спиннер и полностью очищает экран перед диалогом
func StopSpinnerForDialog() {
	StopSpinnerWithKeepTasks(false)
}

// UpdateTask  Функция для внешнего вызова: отправить задачу/прогресс в модель ===
// Пример:
//
//	// Начать прогресс (0%)
//	UpdateTask("PROGRESS", "downloadTask", "Загрузка...", "BEFORE", "0")
//
//	// Обновить до 10%
//	UpdateTask("PROGRESS", "downloadTask", "Загрузка...", "BEFORE", "10")
//
//	// Завершить (100%)
//	UpdateTask("PROGRESS", "downloadTask", "Загрузка...", "AFTER", "100")
//
//	// Обычная задача
//	UpdateTask("TASK", "install", "Установка пакетов", "BEFORE", "")
//	UpdateTask("TASK", "install", "Установка пакетов", "AFTER", "")
func UpdateTask(eventType string, taskName string, viewName string, state string, progressValue float64, progressDone string) {
	if lib.Env.Format != "text" || !IsTTY() {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if p != nil {
		p.Send(TaskUpdateMsg{
			eventType:        eventType,
			taskName:         taskName,
			viewName:         viewName,
			state:            state,
			progressValue:    progressValue,
			progressDoneText: progressDone,
		})
	}
}

// === Инициализация модели ===
func newModel() model {
	// «Общий» спиннер
	s := spinner.New()
	s.Spinner = spinner.Points

	// Спиннер для задач
	ts := spinner.New()
	ts.Spinner = spinner.Jump

	return model{
		spinner:      s,
		tasksSpinner: ts,
		tasks:        []task{},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.tasksSpinner.Tick)
}

// Update === Bubble Tea: Update() ===
//
// Здесь мы:
//   - Обрабатываем spinner.TickMsg
//   - Перехватываем progress.FrameMsg для анимации прогресса
//   - Обновляем задачу при TaskUpdateMsg
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case spinner.TickMsg:
		var cmd1, cmd2 tea.Cmd
		m.spinner, cmd1 = m.spinner.Update(msg)
		m.tasksSpinner, cmd2 = m.tasksSpinner.Update(msg)
		return m, tea.Batch(cmd1, cmd2)

	case progress.FrameMsg:
		var cmds []tea.Cmd
		for i, t := range m.tasks {
			if t.eventType == "PROGRESS" && t.state != "AFTER" && t.progressModel != nil {
				updatedProg, cmd := t.progressModel.Update(msg)
				// Разыменовываем, чтобы переписать содержимое
				*(m.tasks[i].progressModel) = updatedProg.(progress.Model)

				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		return m, tea.Batch(cmds...)

	case TaskUpdateMsg:
		return m.updateTask(msg)

	default:
		return m, nil
	}
}

// TaskUpdateMsg обновление task
func (m model) updateTask(msg TaskUpdateMsg) (tea.Model, tea.Cmd) {
	updated := false
	var batchCmds []tea.Cmd

	var newPercent float64
	if msg.eventType == "PROGRESS" {
		val := msg.progressValue
		if val < 0 {
			val = 0
		} else if val > 100 {
			val = 100
		}

		newPercent = val / 100
	}

	for i, t := range m.tasks {
		if t.name == msg.taskName {
			// Задача уже существует – обновим поля
			m.tasks[i].eventType = msg.eventType
			m.tasks[i].viewName = msg.viewName
			m.tasks[i].state = msg.state

			// Если это ПРОГРЕСС
			if msg.eventType == "PROGRESS" {
				m.tasks[i].progressDoneText = msg.progressDoneText
				// Инициализируем progressModel, если впервые
				if m.tasks[i].progressModel == nil {
					pm := progress.New(progress.WithGradient(lib.Env.Colors.ProgressStart, lib.Env.Colors.ProgressEnd))
					pm.Width = 40
					m.tasks[i].progressModel = &pm
				}

				if msg.state != "AFTER" {
					cmd := m.tasks[i].progressModel.SetPercent(newPercent)
					batchCmds = append(batchCmds, cmd)
				}
			}
			updated = true
			break
		}
	}

	// Если мы не нашли задачу – значит это первая посылка "BEFORE"
	if !updated && msg.state == "BEFORE" {
		newT := task{
			eventType: msg.eventType,
			name:      msg.taskName,
			viewName:  msg.viewName,
			state:     msg.state,
		}

		if msg.eventType == "PROGRESS" {
			// Создаём прогресс-бар
			pm := progress.New(progress.WithGradient(lib.Env.Colors.ProgressStart, lib.Env.Colors.ProgressEnd))
			pm.Width = 40
			newT.progressModel = &pm

			// Устанавливаем начальный процент
			cmd := newT.progressModel.SetPercent(newPercent)
			batchCmds = append(batchCmds, cmd)
		}
		m.tasks = append(m.tasks, newT)
	}

	// Проверяем, все ли задачи (TASK и PROGRESS) завершены
	allFinished := true
	for _, t := range m.tasks {
		if t.state != "AFTER" {
			allFinished = false
			break
		}
	}
	if allFinished {
		select {
		case <-tasksDoneChan:
		default:
			close(tasksDoneChan)
		}
	}

	return m, tea.Batch(batchCmds...)
}

// View общее отображение
func (m model) View() string {
	// Общий спиннер + фраза
	s := fmt.Sprintf("\r\033[K%s \033[33m%s\033[0m", m.spinner.View(), app.T_("Executing tasks"))

	// Перебираем все задачи
	for _, t := range m.tasks {
		switch t.eventType {

		// Обычная задача
		case "TASK":
			if t.state == "AFTER" {
				s += fmt.Sprintf("\n[✓] %s", t.viewName)
			} else {
				s += fmt.Sprintf("\n[%s] %s", m.tasksSpinner.View(), t.viewName)
			}

		// Прогресс-бар
		case "PROGRESS":
			if t.state == "AFTER" {
				text := fmt.Sprintf("\n[✓] %s", app.T_("Progress completed"))
				if len(t.progressDoneText) > 0 {
					text = fmt.Sprintf("\n[✓] %s", fmt.Sprintf(app.T_("Progress: %s completed"), t.progressDoneText))
				}
				s += text
			} else {
				if t.progressModel != nil {
					bar := t.progressModel.View()
					s += fmt.Sprintf("\n%s %s", bar, t.viewName)
				} else {
					s += fmt.Sprintf("\n[....] %s", t.viewName)
				}
			}

		default:
			if t.state == "AFTER" {
				s += fmt.Sprintf("\n[✓] %s", t.viewName)
			} else {
				s += fmt.Sprintf("\n[%s] %s", m.tasksSpinner.View(), t.viewName)
			}
		}
	}

	lastRender = s
	lastLines = strings.Count(s, "\n") + 1
	return s
}
