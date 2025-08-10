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
