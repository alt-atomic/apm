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
	"sync"
	"sync/atomic"
)

var (
	aptWG   sync.WaitGroup
	aptBusy int32
)

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
