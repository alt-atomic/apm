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

package cli

import (
	aptLib "apm/internal/common/binding/apt/lib"
	"os"
	"os/signal"
	"syscall"
)

type SignalCallback func(sig os.Signal, graceful bool)

// InstallSignalHandler ловит SIGINT/SIGTERM/SIGQUIT и прокидывает в callback.
func InstallSignalHandler(cb SignalCallback) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	aptLib.RegisterSignalChannel(sigs)

	go func() {
		sig := <-sigs
		graceful := sig == syscall.SIGINT || sig == syscall.SIGTERM
		if cb != nil {
			cb(sig, graceful)
		}
		os.Exit(signalExitCode(sig))
	}()
}

func signalExitCode(sig os.Signal) int {
	s, ok := sig.(syscall.Signal)
	if !ok {
		return 1
	}
	switch s {
	case syscall.SIGINT:
		return 130
	case syscall.SIGTERM:
		return 143
	default:
		return 128 + int(s)
	}
}
