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

package lib

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	aptWG sync.WaitGroup

	signalChan chan os.Signal
	signalMu   sync.Mutex
)

// RegisterSignalChannel registers the signal channel to restore after APT operations
func RegisterSignalChannel(ch chan os.Signal) {
	signalMu.Lock()
	signalChan = ch
	signalMu.Unlock()
}

// blockSignals blocks interrupt signals during critical operations
func blockSignals() {
	signal.Ignore(syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}

// restoreSignals restores signal handling after critical operations
func restoreSignals() {
	signalMu.Lock()
	ch := signalChan
	signalMu.Unlock()

	if ch != nil {
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	}
}

// BeginOperation blocks signals and marks an active APT operation
func BeginOperation() {
	blockSignals()
	aptWG.Add(1)
}

// EndOperation unmarks the operation and restores signals
func EndOperation() {
	aptWG.Done()
	restoreSignals()
}

// WaitIdle waits for all APT operations to finish
func WaitIdle() { aptWG.Wait() }
