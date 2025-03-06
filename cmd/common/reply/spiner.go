package reply

import (
	"apm/lib"
	"fmt"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	p             *tea.Program
	doneChan      chan struct{}
	tasksDoneChan chan struct{} // Канал, сигнализирующий о завершении всех задач
	mu            sync.Mutex
	lastLines     int // Количество строк, выведенных спинером
)

type TaskUpdateMsg struct {
	taskName string
	viewName string // Отображаемое имя задачи
	state    string // "BEFORE" или "AFTER"
}

type task struct {
	name     string // Внутреннее имя задачи
	viewName string // Отображаемое имя
	state    string
}

type model struct {
	spinner spinner.Model
	tasks   []task
}

// CreateSpinner — запуск спинера в отдельной горутине.
func CreateSpinner() {
	if lib.Env.Format != "text" && IsTTY() {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if p != nil {
		return
	}
	doneChan = make(chan struct{})
	tasksDoneChan = make(chan struct{}) // Инициализируем канал для отслеживания завершения задач

	p = tea.NewProgram(
		newSpinner(),
		tea.WithOutput(os.Stdout),
		//tea.WithAltScreen(),
	)

	go func() {
		if err := p.Start(); err != nil {
			log.Println("Ошибка запуска спинера:", err)
		}
		close(doneChan)
	}()
}

// StopSpinner — останавливает спинер, ожидая, пока все задачи завершатся и галочки успеют отрендериться.
func StopSpinner() {
	if lib.Env.Format != "text" && IsTTY() {
		return
	}

	// Ждём, пока все задачи не перейдут в состояние "AFTER"
	<-tasksDoneChan

	// Небольшая задержка для финального обновления экрана
	time.Sleep(60 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if p != nil {
		// Посылаем сигнал на завершение спинера
		p.Quit()
		// Ждём, пока p.Start() действительно завершится
		<-doneChan
		p = nil

		// Очищаем вывод спинера
		for i := 0; i < lastLines-1; i++ {
			fmt.Print("\033[F\033[K")
		}
	}
}

// newSpinner инициализирует модель со спинером.
func newSpinner() model {
	s := spinner.New()
	s.Spinner = spinner.Points
	return model{spinner: s}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case TaskUpdateMsg:
		updated := false
		for i, t := range m.tasks {
			if t.name == msg.taskName {
				// Обновляем состояние задачи, можно при необходимости также обновить viewName
				m.tasks[i].state = msg.state
				m.tasks[i].viewName = msg.viewName
				updated = true
				break
			}
		}
		if !updated && msg.state == "BEFORE" {
			m.tasks = append(m.tasks, task{
				name:     msg.taskName,
				viewName: msg.viewName,
				state:    msg.state,
			})
		}

		// Проверяем, что все задачи завершены (имеют статус "AFTER")
		allFinished := true
		for _, t := range m.tasks {
			if t.state != "AFTER" {
				allFinished = false
				break
			}
		}
		if allFinished {
			// Закрываем канал один раз, если он ещё не закрыт
			select {
			case <-tasksDoneChan:
			default:
				close(tasksDoneChan)
			}
		}
		return m, nil
	}
	return m, nil
}

// UpdateTask отправляет сообщение об обновлении задачи в модель спинера.
func UpdateTask(taskName, viewName, state string) {
	mu.Lock()
	defer mu.Unlock()

	if p != nil {
		p.Send(TaskUpdateMsg{taskName: taskName, viewName: viewName, state: state})
	}
}

// View формирует строку для вывода спинера и списка задач.
func (m model) View() string {
	s := fmt.Sprintf("\r\033[K%s \033[33mВыполнение...\033[0m", m.spinner.View())

	// Вывод списка задач под спинером, отображаем viewName вместо внутреннего taskName.
	for _, t := range m.tasks {
		mark := "[ ]"
		if t.state == "AFTER" {
			mark = "[✓]"
		}
		s += fmt.Sprintf("\n%s %s", mark, t.viewName)
	}

	lastLines = strings.Count(s, "\n") + 1
	return s
}
