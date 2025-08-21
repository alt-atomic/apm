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
	var field []byte
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
	pid, _ := callerPID(conn, sender)
	stime, _ := getStartTime(pid)
	subject := struct {
		Kind    string
		Details map[string]dbus.Variant
	}{
		Kind: "unix-process",
		Details: map[string]dbus.Variant{
			"pid":        dbus.MakeVariant(pid),
			"start-time": dbus.MakeVariant(stime),
		},
	}

	authority := conn.Object("org.freedesktop.PolicyKit1",
		"/org/freedesktop/PolicyKit1/Authority")

	check := func(flags uint32) (granted bool, err error) {
		var reply struct {
			Granted   bool
			Challenge bool
			Details   map[string]string
		}
		c := authority.Call(
			"org.freedesktop.PolicyKit1.Authority.CheckAuthorization",
			0,
			subject, actionID,
			map[string]string{},
			flags,
			"",
		)
		if c.Err != nil {
			return false, fmt.Errorf(lib.T_("polkit dbus failure: %w"), c.Err)
		}
		if err := c.Store(&reply); err != nil {
			return false, fmt.Errorf(lib.T_("polkit unpack failure: %w"), err)
		}
		return reply.Granted, nil
	}

	granted, err := check(0)
	if err != nil {
		return err
	}

	if granted {
		return nil
	}

	granted, err = check(1)
	if err != nil {
		return err
	}

	if !granted {
		return fmt.Errorf(lib.T_("not authorized by polkit (action=%s)"), actionID)
	}

	return nil
}
