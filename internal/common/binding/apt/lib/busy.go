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
	"sync/atomic"
	"syscall"
)

var (
	aptWG   sync.WaitGroup
	aptBusy int32

	signalChan chan os.Signal
	signalMu   sync.Mutex
)

// RegisterSignalChannel регистрирует канал сигналов для восстановления после APT операций
func RegisterSignalChannel(ch chan os.Signal) {
	signalMu.Lock()
	signalChan = ch
	signalMu.Unlock()
}

// BlockSignals блокирует сигналы прерывания на время критических операций
func BlockSignals() {
	signal.Ignore(syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}

// RestoreSignals восстанавливает обработку сигналов после критических операций
func RestoreSignals() {
	signalMu.Lock()
	ch := signalChan
	signalMu.Unlock()

	if ch != nil {
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	}
}

// StartOperation начало маркировки APT операций
func StartOperation() {
	aptWG.Add(1)
	atomic.AddInt32(&aptBusy, 1)
}

// EndOperation окончание маркировки APT операций
func EndOperation() {
	atomic.AddInt32(&aptBusy, -1)
	aptWG.Done()
}

// WaitIdle Ожидание выполнения всех процессов внутри APT
func WaitIdle() { aptWG.Wait() }
