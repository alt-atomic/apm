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

package lib1

import (
	"fmt"
	"os"

	"github.com/coreos/go-systemd/journal"
	"github.com/sirupsen/logrus"
)

var Log = logrus.New()

func InitLogger() {
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   false,
	})

	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	Log.SetOutput(devNull)

	if journal.Enabled() {
		hook := &JournalHook{}
		Log.AddHook(hook)
	}

	// Добавляем hook для вывода Fatal в stdout
	stdoutHook := &StdoutHook{}
	Log.AddHook(stdoutHook)

	if DevMode {
		Log.SetLevel(logrus.DebugLevel)
	} else {
		Log.SetLevel(logrus.InfoLevel)
	}
}

// JournalHook для записи в systemd journal
type JournalHook struct{}

func (hook *JournalHook) Fire(entry *logrus.Entry) error {
	var priority journal.Priority
	switch entry.Level {
	case logrus.PanicLevel:
		priority = journal.PriEmerg
	case logrus.FatalLevel:
		priority = journal.PriCrit
	case logrus.ErrorLevel:
		priority = journal.PriErr
	case logrus.WarnLevel:
		priority = journal.PriWarning
	case logrus.InfoLevel:
		priority = journal.PriInfo
	case logrus.DebugLevel:
		priority = journal.PriDebug
	default:
		priority = journal.PriInfo
	}

	// Отправляем в journal с правильными полями
	vars := map[string]string{
		"MESSAGE":           entry.Message,
		"PRIORITY":          fmt.Sprintf("%d", priority),
		"SYSLOG_IDENTIFIER": "apm",
	}

	return journal.Send(entry.Message, priority, vars)
}

func (hook *JournalHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// StdoutHook для вывода Fatal уровня в stdout
type StdoutHook struct{}

func (hook *StdoutHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		return err
	}

	fmt.Fprint(os.Stdout, line)
	return nil
}

func (hook *StdoutHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.FatalLevel, logrus.PanicLevel}
}
