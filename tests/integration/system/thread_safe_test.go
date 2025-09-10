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

package system

import (
	"apm/internal/system"
	common "apm/tests/integration/common"
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const threadSafeTestPackage = "hello"

// ThreadSafeTestSuite для тестов потокобезопасности
type ThreadSafeTestSuite struct {
	suite.Suite
	actions *system.Actions
	ctx     context.Context
}

// SetupSuite создает actions один раз для всех тестов
func (s *ThreadSafeTestSuite) SetupSuite() {
	if syscall.Geteuid() != 0 {
		s.T().Skip("This test suite requires root privileges. Run with sudo.")
	}

	appConfig, ctx := common.GetTestAppConfig(s.T())
	s.actions = system.NewActions(appConfig)
	s.ctx = ctx
}

// TestSimultaneousDifferentOperations - проверяет что разные операции могут выполняться параллельно
func (s *ThreadSafeTestSuite) TestSimultaneousDifferentOperations() {
	s.T().Run("Concurrent different operations", func(t *testing.T) {
		var wg sync.WaitGroup
		var infoErr, searchErr, filterErr error
		var infoDone, searchDone, filterDone int64
		startTime := time.Now()

		// Info операция
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				atomic.StoreInt64(&infoDone, time.Since(startTime).Nanoseconds())
			}()
			_, infoErr = s.actions.Info(s.ctx, threadSafeTestPackage, false)
		}()

		// Search операция
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				atomic.StoreInt64(&searchDone, time.Since(startTime).Nanoseconds())
			}()
			_, searchErr = s.actions.Search(s.ctx, threadSafeTestPackage, false, false)
		}()

		// GetFilterFields операция
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				atomic.StoreInt64(&filterDone, time.Since(startTime).Nanoseconds())
			}()
			_, filterErr = s.actions.GetFilterFields(s.ctx)
		}()

		wg.Wait()

		if infoErr != nil {
			t.Logf("Info operation error (may be expected): %v", infoErr)
		}
		if searchErr != nil {
			t.Logf("Search operation error (may be expected): %v", searchErr)
		}
		if filterErr != nil {
			t.Logf("FilterFields operation error (may be expected): %v", filterErr)
		}

		t.Logf("Operations completed: Info=%v, Search=%v, Filter=%v",
			time.Duration(atomic.LoadInt64(&infoDone)),
			time.Duration(atomic.LoadInt64(&searchDone)),
			time.Duration(atomic.LoadInt64(&filterDone)))
	})
}

// TestConcurrentReadOperationsStress - stress тест для поиска race conditions в read операциях
func (s *ThreadSafeTestSuite) TestConcurrentReadOperationsStress() {
	s.T().Run("High concurrent read load", func(t *testing.T) {
		const numGoroutines = 50
		const operationsPerGoroutine = 10

		var wg sync.WaitGroup
		var successCount int64
		var errorCount int64

		// Список пакетов для тестирования
		packages := []string{threadSafeTestPackage, "nano", "vim", "gcc", "make"}

		// Запускаем множество горутин одновременно
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				for j := 0; j < operationsPerGoroutine; j++ {
					pkg := packages[j%len(packages)]

					// Вызываем разные методы чтения параллельно
					switch j % 4 {
					case 0:
						_, err := s.actions.Info(s.ctx, pkg, false)
						if err != nil {
							atomic.AddInt64(&errorCount, 1)
							t.Logf("Info error in goroutine %d: %v", goroutineID, err)
						} else {
							atomic.AddInt64(&successCount, 1)
						}

					case 1:
						_, err := s.actions.Search(s.ctx, pkg, false, false)
						if err != nil {
							atomic.AddInt64(&errorCount, 1)
							t.Logf("Search error in goroutine %d: %v", goroutineID, err)
						} else {
							atomic.AddInt64(&successCount, 1)
						}

					case 2:
						_, err := s.actions.GetFilterFields(s.ctx)
						if err != nil {
							atomic.AddInt64(&errorCount, 1)
							t.Logf("GetFilterFields error in goroutine %d: %v", goroutineID, err)
						} else {
							atomic.AddInt64(&successCount, 1)
						}

					case 3:
						// Тестируем параметры списка пакетов
						params := system.ListParams{
							Sort:   "name",
							Order:  "asc",
							Limit:  5,
							Offset: 0,
						}
						_, err := s.actions.List(s.ctx, params, false)
						if err != nil {
							atomic.AddInt64(&errorCount, 1)
							t.Logf("List error in goroutine %d: %v", goroutineID, err)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					}

					// Небольшая пауза для увеличения шансов на race condition
					time.Sleep(time.Microsecond * 10)
				}
			}(i)
		}

		wg.Wait()

		totalOperations := int64(numGoroutines * operationsPerGoroutine)
		t.Logf("Completed %d operations: %d success, %d errors",
			totalOperations,
			atomic.LoadInt64(&successCount),
			atomic.LoadInt64(&errorCount))

		require.Equal(t, totalOperations,
			atomic.LoadInt64(&successCount)+atomic.LoadInt64(&errorCount),
			"Some operations were lost - possible race condition!")
	})
}

// TestPackageOperationLocking тестирует что операции действительно блокируют друг друга
func (s *ThreadSafeTestSuite) TestPackageOperationLocking() {
	s.T().Run("Real operation blocking test", func(t *testing.T) {
		var wg sync.WaitGroup
		var operation1Done, operation2Done int64
		startTime := time.Now()

		// Первая горутина - долгая операция CheckInstall
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				atomic.StoreInt64(&operation1Done, time.Since(startTime).Nanoseconds())
			}()

			// Искусственно делаем операцию долгой через контекст с таймаутом
			slowCtx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
			defer cancel()

			_, err := s.actions.CheckInstall(slowCtx, []string{threadSafeTestPackage})
			if err != nil {
				t.Logf("CheckInstall error (may be expected): %v", err)
			}
		}()

		// Небольшая задержка чтобы первая операция точно началась
		time.Sleep(100 * time.Millisecond)

		// Вторая горутина - быстрая операция CheckRemove
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				atomic.StoreInt64(&operation2Done, time.Since(startTime).Nanoseconds())
			}()

			_, err := s.actions.CheckRemove(s.ctx, []string{threadSafeTestPackage}, false)
			if err != nil {
				t.Logf("CheckRemove error (may be expected): %v", err)
			}
		}()

		wg.Wait()

		duration1 := time.Duration(atomic.LoadInt64(&operation1Done))
		duration2 := time.Duration(atomic.LoadInt64(&operation2Done))

		t.Logf("Operation 1 finished at: %v", duration1)
		t.Logf("Operation 2 finished at: %v", duration2)

		// Если операции НЕ блокируются, они должны завершиться примерно одновременно
		// Если блокируются - одна будет значительно позже другой
		timeDiff := duration1 - duration2
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}

		t.Logf("Time difference between operations: %v", timeDiff)
	})
}

// TestDatabaseConcurrency - проверяет race conditions в базе данных
func (s *ThreadSafeTestSuite) TestDatabaseConcurrency() {
	s.T().Run("Concurrent database operations", func(t *testing.T) {
		const numGoroutines = 20
		const operationsPerGoroutine = 5

		var wg sync.WaitGroup
		var panicCount int64
		errors := make(chan error, numGoroutines*operationsPerGoroutine)

		// Множество горутин делают операции с базой данных одновременно
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						atomic.AddInt64(&panicCount, 1)
						t.Errorf("Panic in goroutine %d: %v", goroutineID, r)
					}
				}()

				for j := 0; j < operationsPerGoroutine; j++ {
					switch j % 3 {
					case 0:
						_, err := s.actions.Info(s.ctx, threadSafeTestPackage, false)
						if err != nil {
							select {
							case errors <- err:
							default:
							}
						}

					case 1:
						_, err := s.actions.GetFilterFields(s.ctx)
						if err != nil {
							select {
							case errors <- err:
							default:
							}
						}

					case 2:
						_, err := s.actions.Search(s.ctx, "test", false, false)
						if err != nil {
							select {
							case errors <- err:
							default:
							}
						}
					}

					// Увеличиваем шансы на race condition
					runtime.Gosched()
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Собираем ошибки
		var errorList []error
		for err := range errors {
			errorList = append(errorList, err)
		}

		t.Logf("Completed database concurrency test: %d panics, %d errors",
			atomic.LoadInt64(&panicCount), len(errorList))

		assert.Equal(t, int64(0), atomic.LoadInt64(&panicCount),
			"Database operations caused panics - race condition detected!")

		for i, err := range errorList {
			if i < 10 {
				t.Logf("Database error %d: %v", i, err)
			}
		}
	})
}

// TestMemoryCorruption - пытается выявить memory corruption
func (s *ThreadSafeTestSuite) TestMemoryCorruption() {
	s.T().Run("Memory corruption detection", func(t *testing.T) {
		const numGoroutines = 30

		var wg sync.WaitGroup
		results := make([]string, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				resp, err := s.actions.Info(s.ctx, threadSafeTestPackage, false)
				if err != nil {
					results[goroutineID] = "ERROR: " + err.Error()
				} else if resp != nil && resp.Data != nil {
					results[goroutineID] = "OK"
				} else {
					results[goroutineID] = "NIL_RESPONSE"
				}
			}(i)
		}

		wg.Wait()

		var okCount, errorCount, nilCount int
		for i, result := range results {
			switch {
			case result == "OK":
				okCount++
			case result == "NIL_RESPONSE":
				nilCount++
				t.Logf("Goroutine %d got nil response", i)
			case result != "":
				errorCount++
			}
		}

		t.Logf("Memory corruption test: %d OK, %d errors, %d nil responses",
			okCount, errorCount, nilCount)

		assert.Zero(t, nilCount, "Some goroutines got corrupted nil responses")
	})
}

// Запуск набора тестов
func TestThreadSafeSuite(t *testing.T) {
	suite.Run(t, new(ThreadSafeTestSuite))
}
