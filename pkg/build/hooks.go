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

package build

// Logger is the minimal logging interface.
type Logger interface {
	Debug(args ...any)
	Debugf(format string, args ...any)
	Info(args ...any)
	Warn(args ...any)
	Warning(args ...any)
	Error(args ...any)
}

type nopLogger struct{}

func (nopLogger) Debug(...any)          {}
func (nopLogger) Debugf(string, ...any) {}
func (nopLogger) Info(...any)           {}
func (nopLogger) Warn(...any)           {}
func (nopLogger) Warning(...any)        {}
func (nopLogger) Error(...any)          {}

// logger is the active logger hook; no-op by default.
var logger Logger = nopLogger{}

// Log returns the active logger.
func Log() Logger {
	return logger
}

// SetLogger installs a logger; nil is ignored.
func SetLogger(l Logger) {
	if l != nil {
		logger = l
	}
}

// T_ is the translation hook; identity by default.
var T_ = func(msgID string) string { return msgID }

// SetTranslator installs a translation function; nil is ignored.
func SetTranslator(fn func(string) string) {
	if fn != nil {
		T_ = fn
	}
}
