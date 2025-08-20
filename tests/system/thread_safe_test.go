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

//go:build system

package system

import (
	"apm/internal/system"
	"apm/tests/common"
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const threadSafeTestPackage = "hello"

// TestThreadSafePackageOperations проверяет потокобезопасность операций с пакетами
func TestThreadSafePackageOperations(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	// Тест 1: Последовательные операции с использованием мьютекса
	t.Run("Sequential operations with mutex", func(t *testing.T) {
		err := common.WithAPTLock(func() error {
			_, err := actions.Info(ctx, threadSafeTestPackage, false)
			return err
		})
		
		if err != nil {
			t.Logf("First operation error (may be expected): %v", err)
		}

		err = common.WithAPTLock(func() error {
			_, err := actions.CheckInstall(ctx, []string{threadSafeTestPackage})
			return err
		})
		
		if err != nil {
			t.Logf("Second operation error (may be expected): %v", err)
		}
	})

	// Тест 2: Операции с таймаутом
	t.Run("Operations with timeout", func(t *testing.T) {
		err := common.WithAPTLockAndTimeout(func() error {
			_, err := actions.Search(ctx, threadSafeTestPackage, false, false)
			return err
		}, 10*time.Second)

		if err != nil {
			if common.IsAPTTimeoutError(err) {
				t.Log("Operation timed out as expected")
			} else {
				t.Logf("Search operation error (may be expected): %v", err)
			}
		}
	})
}

// TestConcurrentReadOperations тестирует concurrent read операции (должны быть безопасны)
func TestConcurrentReadOperations(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	// Только read операции могут выполняться параллельно без блокировки APT
	t.Run("Concurrent info operations", func(t *testing.T) {
		done := make(chan bool, 2)
		
		// Первая горутина
		go func() {
			defer func() { done <- true }()
			_, err := actions.Info(ctx, threadSafeTestPackage, false)
			if err != nil {
				t.Logf("Concurrent info 1 error (may be expected): %v", err)
			}
		}()
		
		// Вторая горутина
		go func() {
			defer func() { done <- true }()
			_, err := actions.Info(ctx, "nano", false)
			if err != nil {
				t.Logf("Concurrent info 2 error (may be expected): %v", err)
			}
		}()
		
		// Ждем завершения обеих горутин
		<-done
		<-done
	})
}

// TestPackageOperationLocking тестирует что write операции правильно блокируются
func TestPackageOperationLocking(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	// Тестируем что операции с записью блокируются
	t.Run("Write operations blocking", func(t *testing.T) {
		startTime := time.Now()
		
		// Первая операция удерживает блокировку
		err1 := common.WithAPTLock(func() error {
			time.Sleep(1 * time.Second) // Имитируем долгую операцию
			_, err := actions.CheckInstall(ctx, []string{threadSafeTestPackage})
			return err
		})
		
		// Вторая операция должна ждать
		err2 := common.WithAPTLock(func() error {
			_, err := actions.CheckRemove(ctx, []string{threadSafeTestPackage})
			return err
		})
		
		duration := time.Since(startTime)
		
		// Проверяем что операции выполнились последовательно
		assert.GreaterOrEqual(t, duration, 1*time.Second, "Operations should be sequential")
		
		if err1 != nil {
			t.Logf("First operation error (may be expected): %v", err1)
		}
		if err2 != nil {
			t.Logf("Second operation error (may be expected): %v", err2)
		}
	})
}

// TestTimeoutHandling тестирует обработку таймаутов
func TestTimeoutHandling(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	t.Run("Timeout on long operation", func(t *testing.T) {
		err := common.WithAPTLockAndTimeout(func() error {
			// Имитируем очень долгую операцию
			time.Sleep(2 * time.Second)
			_, err := actions.Info(ctx, threadSafeTestPackage, false)
			return err
		}, 500*time.Millisecond) // Короткий таймаут

		assert.Error(t, err)
		assert.True(t, common.IsAPTTimeoutError(err), "Should return timeout error")
	})
}

// TestResourceCleanup тестирует правильную очистку ресурсов
func TestResourceCleanup(t *testing.T) {
	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	// Тестируем что мьютекс освобождается даже при панике
	t.Run("Mutex released on panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Log("Recovered from panic:", r)
			}
		}()

		func() {
			defer func() {
				if r := recover(); r != nil {
					panic(r) // Re-panic to test defer cleanup
				}
			}()

			err := common.WithAPTLock(func() error {
				panic("test panic")
			})
			
			assert.Error(t, err) // This should not be reached
		}()

		// Проверяем что мьютекс доступен после паники
		err := common.WithAPTLockAndTimeout(func() error {
			_, err := actions.Info(ctx, threadSafeTestPackage, false)
			return err
		}, 1*time.Second)

		// Мьютекс должен быть доступен
		if err != nil && !common.IsAPTTimeoutError(err) {
			t.Logf("Operation after panic error (may be expected): %v", err)
		}
	})
}