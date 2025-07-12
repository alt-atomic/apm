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

package helper

import (
	"apm/lib"
	"fmt"
	"os"
	"strconv"

	"github.com/godbus/dbus/v5"
)

// callerPID возвращает PID процесса, пославшего D-Bus-сообщение.
func callerPID(conn *dbus.Conn, sender dbus.Sender) (uint32, error) {
	var pid uint32
	err := conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus").
		Call("org.freedesktop.DBus.GetConnectionUnixProcessID", 0, sender).
		Store(&pid)

	return pid, err
}

// getStartTime считывает поле 22 (/proc/PID/stat) – время запуска в тиках.
func getStartTime(pid uint32) (uint64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}

	fields := make([][]byte, 0, 24)
	inParen := false
	field := []byte{}
	for _, c := range data {
		switch {
		case c == '(':
			inParen = true
		case c == ')' && inParen:
			inParen = false
		case c == ' ' && !inParen:
			fields = append(fields, field)
			field = []byte{}
			continue
		}
		field = append(field, c)
	}

	startTime, err := strconv.ParseUint(string(fields[21]), 10, 64)

	return startTime, err
}

// PolkitCheck — универсальная проверка доступа.
func PolkitCheck(conn *dbus.Conn, sender dbus.Sender, actionID string) error {
	lib.Log.Infoln(actionID)
	lib.Log.Infoln(sender)
	pid, err := callerPID(conn, sender)
	if err != nil {
		return fmt.Errorf("cannot resolve sender pid: %w", err)
	}

	stime, err := getStartTime(pid)
	if err != nil {
		return fmt.Errorf("cannot read start-time: %w", err)
	}

	subject := map[string]map[string]dbus.Variant{
		"unix-process": {
			"pid":        dbus.MakeVariant(pid),
			"start-time": dbus.MakeVariant(stime),
		},
	}

	const allowInteraction uint32 = 1 // показать диалог, если rule не найден

	authority := conn.Object("org.freedesktop.PolicyKit1",
		"/org/freedesktop/PolicyKit1/Authority")

	var granted bool
	var details map[string]dbus.Variant

	call := authority.Call(
		"org.freedesktop.PolicyKit1.Authority.CheckAuthorization",
		0, subject, actionID,
		map[string]dbus.Variant{},
		allowInteraction,
		"",
	)

	if call.Err != nil {
		return fmt.Errorf("polkit dbus failure: %w", call.Err)
	}

	if err = call.Store(&granted, &details); err != nil {
		return fmt.Errorf("polkit unpack failure: %w", err)
	}

	if !granted {
		return fmt.Errorf("not authorized by polkit (action=%s)", actionID)
	}

	return nil
}
