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

package common

import (
	"sync"
	"time"
)

var (
	// APTMutex - глобальный мьютекс для предотвращения одновременного использования APT
	APTMutex = &sync.Mutex{}
	
	// PackageOperationTimeout - таймаут для операций с пакетами
	PackageOperationTimeout = 30 * time.Second
)

// WithAPTLock выполняет функцию с блокировкой APT
func WithAPTLock(fn func() error) error {
	APTMutex.Lock()
	defer APTMutex.Unlock()
	return fn()
}

// WithAPTLockAndTimeout выполняет функцию с блокировкой APT и таймаутом
func WithAPTLockAndTimeout(fn func() error, timeout time.Duration) error {
	done := make(chan error, 1)
	
	go func() {
		APTMutex.Lock()
		defer APTMutex.Unlock()
		done <- fn()
	}()
	
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return &APTTimeoutError{Timeout: timeout}
	}
}

// APTTimeoutError представляет ошибку таймаута APT операции
type APTTimeoutError struct {
	Timeout time.Duration
}

func (e *APTTimeoutError) Error() string {
	return "APT operation timed out after " + e.Timeout.String()
}

// IsAPTTimeoutError проверяет является ли ошибка таймаутом APT
func IsAPTTimeoutError(err error) bool {
	_, ok := err.(*APTTimeoutError)
	return ok
}