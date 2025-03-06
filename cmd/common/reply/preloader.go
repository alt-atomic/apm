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
	spinner      spinner.Model // Общий спинер (стиль Points)
	tasksSpinner spinner.Model // Спинер для задач (стиль Hamburger)
	tasks        []task
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
	tasksDoneChan = make(chan struct{})

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

// StopSpinner — останавливает спинер
func StopSpinner() {
	if lib.Env.Format != "text" && IsTTY() {
		return
	}

	<-tasksDoneChan

	// Небольшая задержка для финального обновления экрана
	time.Sleep(60 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if p != nil {
		p.Quit()
		<-doneChan
		p = nil

		for i := 0; i < lastLines-1; i++ {
			fmt.Print("\033[F\033[K")
		}
	}
}

// newSpinner инициализирует модель со спинерами.
func newSpinner() model {
	// Общий спинер
	s := spinner.New()
	s.Spinner = spinner.Points

	// Спинер для задач
	ts := spinner.New()
	ts.Spinner = spinner.Jump

	return model{
		spinner:      s,
		tasksSpinner: ts,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.tasksSpinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd1, cmd2 tea.Cmd
		m.spinner, cmd1 = m.spinner.Update(msg)
		m.tasksSpinner, cmd2 = m.tasksSpinner.Update(msg)
		return m, tea.Batch(cmd1, cmd2)
	case TaskUpdateMsg:
		updated := false
		for i, t := range m.tasks {
			if t.name == msg.taskName {
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

// View формирует строку для вывода спинеров и списка задач.
func (m model) View() string {
	s := fmt.Sprintf("\r\033[K%s \033[33mВыполнение...\033[0m", m.spinner.View())

	for _, t := range m.tasks {
		var marker string
		if t.state == "AFTER" {
			marker = "[✓]"
		} else {
			marker = fmt.Sprintf("[%s]", m.tasksSpinner.View())
		}
		s += fmt.Sprintf("\n%s %s", marker, t.viewName)
	}

	lastLines = strings.Count(s, "\n") + 1
	return s
}
