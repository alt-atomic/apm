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

package apt

import (
	"apm/internal/common/app"
	"apm/internal/common/apt"
	"apm/internal/common/binding/apt/lib"
	"strings"
	"sync"
)

// Инициализация единственного экземпляра APT system, в конце его нужно ЗАКРЫТЬ
var (
	aptSystem     *lib.System
	aptSystemOnce sync.Once
	aptSystemErr  error
	aptMutex      sync.Mutex
)

type Actions struct {
	configOverrides map[string]string
}

func NewActions() *Actions {
	return &Actions{}
}

// SetConfigOverrides устанавливает переопределения конфигурации APT
func (a *Actions) SetConfigOverrides(overrides map[string]string) {
	a.configOverrides = overrides
}

// GetConfigOverrides возвращает текущие переопределения конфигурации APT
func (a *Actions) GetConfigOverrides() map[string]string {
	return a.configOverrides
}

func getSystem() (*lib.System, error) {
	aptSystemOnce.Do(func() {
		aptSystem, aptSystemErr = lib.NewSystem()
	})
	return aptSystem, aptSystemErr
}

func Close() {
	if aptSystem != nil {
		aptSystem.Close()
		aptSystem = nil
	}
	aptSystemErr = nil
	aptSystemOnce = sync.Once{}
}

// OperationOptions опции для операций с APT
type OperationOptions struct {
	SkipLock     bool
	RpmArguments []string
}

// runOperation единая обёртка для всех операций с APT
func (a *Actions) runOperation(opts OperationOptions, fn func(system *lib.System) error) error {
	aptMutex.Lock()
	defer aptMutex.Unlock()

	if opts.SkipLock {
		lib.SetNoLocking(true)
		defer lib.SetNoLocking(false)
	}

	// Проверяем блокировку перед началом операции
	if !opts.SkipLock {
		if err := lib.CheckLockOrError(); err != nil {
			return err
		}
	}

	lib.BlockSignals()
	defer lib.RestoreSignals()

	lib.StartOperation()
	defer lib.EndOperation()

	system, initErr := getSystem()
	if initErr != nil {
		return initErr
	}

	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)

	var err error
	if len(opts.RpmArguments) > 0 {
		err = lib.PreprocessInstallArguments(opts.RpmArguments)
	}

	if err == nil {
		err = lib.WithConfigOverrides(a.configOverrides, func() error {
			return fn(system)
		})
	}

	// Очищаем RPM аргументы
	lib.ClearInstallArguments()

	// Захватываем ошибки
	lib.CaptureStdIO(false)
	lib.SetLogHandler(nil)

	result := a.checkAnyError(logs, err)
	if result != nil && len(logs) > 0 {
		app.Log.Error("[APM DUMP ERROR] ", result.Error())
		for _, line := range logs {
			app.Log.Error("[APM DUMP TRACE] ", line)
		}
	}
	return result
}

// withCache открывает кеш APT и передаёт в fn
func withCache(system *lib.System, readOnly bool, fn func(*lib.Cache) error) error {
	cache, err := lib.OpenCache(system, readOnly)
	if err != nil {
		return err
	}
	defer cache.Close()
	return fn(cache)
}

// CombineInstallRemovePackages комбинированный метод установки и удаления
func (a *Actions) CombineInstallRemovePackages(packagesInstall []string, packagesRemove []string,
	handler lib.ProgressHandler, purge bool, depends bool, downloadOnly bool) error {
	return a.runOperation(OperationOptions{RpmArguments: packagesInstall}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			tx, err := cache.NewTransaction()
			if err != nil {
				return err
			}
			defer tx.Close()
			if len(packagesInstall) > 0 {
				if err := tx.Install(packagesInstall); err != nil {
					return err
				}
			}
			if len(packagesRemove) > 0 {
				if err := tx.Remove(packagesRemove, purge, depends); err != nil {
					return err
				}
			}
			return tx.Execute(handler, downloadOnly)
		})
	})
}

// InstallPackages установка пакетов
func (a *Actions) InstallPackages(packageNames []string, handler lib.ProgressHandler, downloadOnly bool) error {
	if len(packageNames) == 0 {
		return lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}
	return a.runOperation(OperationOptions{RpmArguments: packageNames}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			tx, err := cache.NewTransaction()
			if err != nil {
				return err
			}
			defer tx.Close()
			if err := tx.Install(packageNames); err != nil {
				return err
			}
			return tx.Execute(handler, downloadOnly)
		})
	})
}

// RemovePackages удаление пакетов
func (a *Actions) RemovePackages(packageNames []string, purge bool, depends bool, handler lib.ProgressHandler) error {
	if len(packageNames) == 0 {
		return lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}
	return a.runOperation(OperationOptions{}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			tx, err := cache.NewTransaction()
			if err != nil {
				return err
			}
			defer tx.Close()
			if err := tx.Remove(packageNames, purge, depends); err != nil {
				return err
			}
			return tx.Execute(handler, false)
		})
	})
}

// DistUpgrade обновление системы
func (a *Actions) DistUpgrade(handler lib.ProgressHandler, downloadOnly bool) error {
	return a.runOperation(OperationOptions{}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			tx, err := cache.NewTransaction()
			if err != nil {
				return err
			}
			defer tx.Close()
			if err := tx.DistUpgrade(); err != nil {
				return err
			}
			return tx.Execute(handler, downloadOnly)
		})
	})
}

// Update обновление локальной базы пакетов
func (a *Actions) Update(handler lib.ProgressHandler, noLock ...bool) error {
	skipLock := len(noLock) > 0 && noLock[0]
	return a.runOperation(OperationOptions{SkipLock: skipLock}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			return cache.Update(handler)
		})
	})
}

// Search поиск по пакетам
func (a *Actions) Search(pattern string, noLock ...bool) (packages []lib.PackageInfo, err error) {
	skipLock := len(noLock) > 0 && noLock[0]
	err = a.runOperation(OperationOptions{SkipLock: skipLock}, func(system *lib.System) error {
		return withCache(system, true, func(cache *lib.Cache) error {
			packages, err = cache.SearchPackages(pattern)
			return err
		})
	})
	return
}

// GetInfo поиск одного пакета
func (a *Actions) GetInfo(packageName string) (packageInfo *lib.PackageInfo, err error) {
	err = a.runOperation(OperationOptions{}, func(system *lib.System) error {
		return withCache(system, true, func(cache *lib.Cache) error {
			packageInfo, err = cache.GetPackageInfo(packageName)
			return err
		})
	})
	return
}

// SimulateInstall симуляция установки
func (a *Actions) SimulateInstall(packageNames []string) (packageInfo *lib.PackageChanges, err error) {
	if len(packageNames) == 0 {
		return nil, lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}
	err = a.runOperation(OperationOptions{RpmArguments: packageNames}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			packageInfo, err = cache.SimulateInstall(packageNames)
			return err
		})
	})
	return
}

// SimulateRemove симуляция удаления
func (a *Actions) SimulateRemove(packageNames []string, purge bool, depends bool) (packageInfo *lib.PackageChanges, err error) {
	if len(packageNames) == 0 {
		return nil, lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}
	err = a.runOperation(OperationOptions{}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			packageInfo, err = cache.SimulateRemove(packageNames, purge, depends)
			return err
		})
	})
	return
}

// SimulateDistUpgrade симуляция обновления системы
func (a *Actions) SimulateDistUpgrade() (packageChanges *lib.PackageChanges, err error) {
	err = a.runOperation(OperationOptions{}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			packageChanges, err = cache.SimulateDistUpgrade()
			return err
		})
	})
	return
}

// SimulateAutoRemove симуляция автоматического удаления неиспользуемых пакетов
func (a *Actions) SimulateAutoRemove() (packageChanges *lib.PackageChanges, err error) {
	err = a.runOperation(OperationOptions{}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			packageChanges, err = cache.SimulateAutoRemove()
			return err
		})
	})
	return
}

// SimulateChange комбинированная симуляция установки и удаления
func (a *Actions) SimulateChange(installNames []string, removeNames []string, purge bool, depends bool) (packageChanges *lib.PackageChanges, err error) {
	if len(installNames) == 0 && len(removeNames) == 0 {
		return nil, lib.CustomError(lib.AptErrorInvalidParameters, "Invalid parameters")
	}
	err = a.runOperation(OperationOptions{RpmArguments: installNames}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			packageChanges, err = cache.SimulateChange(installNames, removeNames, purge, depends)
			return err
		})
	})
	return
}

// SimulateChangeWithRpmInfo симуляция изменений с получением информации о RPM файлах за одну сессию кеша
func (a *Actions) SimulateChangeWithRpmInfo(installNames []string, removeNames []string, purge bool, depends bool, rpmFiles []string) (packageChanges *lib.PackageChanges, rpmInfos []*lib.PackageInfo, err error) {
	if len(installNames) == 0 && len(removeNames) == 0 {
		return nil, nil, lib.CustomError(lib.AptErrorInvalidParameters, "Invalid parameters")
	}
	err = a.runOperation(OperationOptions{RpmArguments: rpmFiles}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			packageChanges, rpmInfos, err = cache.SimulateChangeWithRpmInfo(installNames, removeNames, purge, depends, rpmFiles)
			return err
		})
	})
	return
}

// SimulateReinstall симуляция переустановки пакетов
func (a *Actions) SimulateReinstall(packageNames []string) (packageInfo *lib.PackageChanges, err error) {
	if len(packageNames) == 0 {
		return nil, lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}
	err = a.runOperation(OperationOptions{RpmArguments: packageNames}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			packageInfo, err = cache.SimulateReinstall(packageNames)
			return err
		})
	})
	return
}

// ReinstallPackages переустановка пакетов
func (a *Actions) ReinstallPackages(packageNames []string, handler lib.ProgressHandler) error {
	if len(packageNames) == 0 {
		return lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}
	return a.runOperation(OperationOptions{RpmArguments: packageNames}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			tx, err := cache.NewTransaction()
			if err != nil {
				return err
			}
			defer tx.Close()
			if err := tx.Reinstall(packageNames); err != nil {
				return err
			}
			return tx.Execute(handler, false)
		})
	})
}

// enrichErrorDetails добавляет детали к ошибке из логов и строк ошибки
func enrichErrorDetails(m *apt.MatchedError, logs []string, errLines []string) {
	if m.IsTransactionError() {
		if details := apt.CollectTransactionDetails(logs); details != "" {
			m.Details = details
		}
	}

	if m.Entry.Code == apt.ErrMultiInstallProvidersSelect && len(errLines) > 1 {
		var providers []string
		for i := 1; i < len(errLines); i++ {
			line := strings.TrimSpace(errLines[i])
			if line != "" && !strings.HasPrefix(line, "You should") {
				providers = append(providers, line)
			}
		}
		if len(providers) > 0 {
			m.Details = strings.Join(providers, "\n") + "\n" + app.T_("You should explicitly select one to install")
		}
	}
}

// checkAnyError анализ всех ошибок, включает в себя stdout из apt-lib
func (a *Actions) checkAnyError(logs []string, err error) error {
	aptErrors := apt.ErrorLinesAnalyseAll(logs)
	for _, errApr := range aptErrors {
		enrichErrorDetails(errApr, logs, nil)
		return errApr
	}

	if err == nil {
		return nil
	}

	if msg := strings.TrimSpace(err.Error()); msg != "" {
		lines := strings.Split(msg, "\n")
		if m := apt.ErrorLinesAnalise(lines); m != nil {
			enrichErrorDetails(m, logs, lines)
			return m
		}
		if m := apt.CheckError(msg); m != nil {
			enrichErrorDetails(m, logs, lines)
			return m
		}
	}

	return err
}
