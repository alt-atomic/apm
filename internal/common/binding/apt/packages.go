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

type Actions struct{}

func NewActions() *Actions {
	return &Actions{}
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

// operationWrapper обёртка для всех операций с APT
func (a *Actions) operationWrapper(fn func() error) error {
	aptMutex.Lock()
	defer aptMutex.Unlock()

	// Проверяем блокировку перед началом операции
	if err := lib.CheckLockOrError(); err != nil {
		return err
	}

	lib.StartOperation()
	defer lib.EndOperation()

	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)

	// Выполняем основную функцию
	err := fn()

	// Очищаем и анализируем ошибки
	lib.CaptureStdIO(false)
	lib.SetLogHandler(nil)

	return a.checkAnyError(logs, err)
}

// CombineInstallRemovePackages комбинированный метод установки и удаления
func (a *Actions) CombineInstallRemovePackages(packagesInstall []string, packagesRemove []string,
	handler lib.ProgressHandler, purge bool, depends bool) error {
	return a.operationWrapper(func() error {
		system, err := getSystem()
		if err != nil {
			return err
		}

		cache, err := lib.OpenCache(system, false)
		if err != nil {
			return err
		}
		defer cache.Close()

		// ApplyChanges to apply all changes in ONE transaction
		if e := cache.ApplyChanges(packagesInstall, packagesRemove, purge, depends); e != nil {
			return e
		}

		pm, err := lib.NewPackageManager(cache)
		if err != nil {
			return err
		}
		defer pm.Close()

		if handler != nil {
			return pm.InstallPackagesWithProgress(handler)
		}
		return pm.InstallPackages()
	})
}

// InstallPackages установка пакетов
func (a *Actions) InstallPackages(packageNames []string, handler lib.ProgressHandler) error {
	if len(packageNames) == 0 {
		return lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}

	return a.operationWrapper(func() error {
		system, err := getSystem()
		if err != nil {
			return err
		}

		cache, err := lib.OpenCache(system, false)
		if err != nil {
			return err
		}
		defer cache.Close()

		// Use ApplyChanges to apply all packages in ONE transaction
		if e := cache.ApplyChanges(packageNames, nil, false, false); e != nil {
			return e
		}

		pm, err := lib.NewPackageManager(cache)
		if err != nil {
			return err
		}
		defer pm.Close()

		if handler != nil {
			return pm.InstallPackagesWithProgress(handler)
		}
		return pm.InstallPackages()
	})
}

// RemovePackages удаление пакетов
func (a *Actions) RemovePackages(packageNames []string, purge bool, depends bool, handler lib.ProgressHandler) error {
	if len(packageNames) == 0 {
		return lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}

	return a.operationWrapper(func() error {
		system, err := getSystem()
		if err != nil {
			return err
		}

		cache, err := lib.OpenCache(system, false)
		if err != nil {
			return err
		}
		defer cache.Close()

		// Use ApplyChanges to apply all packages in ONE transaction
		if e := cache.ApplyChanges(nil, packageNames, purge, depends); e != nil {
			return e
		}

		pm, err := lib.NewPackageManager(cache)
		if err != nil {
			return err
		}
		defer pm.Close()

		if handler != nil {
			return pm.InstallPackagesWithProgress(handler)
		}
		return pm.InstallPackages()
	})
}

// DistUpgrade обновление системы
func (a *Actions) DistUpgrade(handler lib.ProgressHandler) error {
	return a.operationWrapper(func() error {
		system, err := getSystem()
		if err != nil {
			return err
		}

		cache, err := lib.OpenCache(system, false)
		if err != nil {
			return err
		}
		defer cache.Close()

		if handler != nil {
			return cache.DistUpgradeWithProgress(handler)
		}
		return cache.DistUpgradeWithProgress(nil)
	})
}

// Update обновление локальной базы пакетов
func (a *Actions) Update() error {
	return a.operationWrapper(func() error {
		system, err := getSystem()
		if err != nil {
			return err
		}

		cache, err := lib.OpenCache(system, false)
		if err != nil {
			return err
		}
		defer cache.Close()

		return cache.Update()
	})
}

// Search поиск по пакетам
func (a *Actions) Search(pattern string) (packages []lib.PackageInfo, err error) {
	err = a.operationWrapper(func() error {
		system, e := getSystem()
		if e != nil {
			return e
		}

		cache, e := lib.OpenCache(system, true)
		if e != nil {
			return e
		}
		defer cache.Close()

		packages, e = cache.SearchPackages(pattern)
		return e
	})
	return
}

// GetInfo поиск одного пакета
func (a *Actions) GetInfo(packageName string) (packageInfo *lib.PackageInfo, err error) {
	err = a.operationWrapper(func() error {
		system, e := getSystem()
		if e != nil {
			return e
		}

		cache, e := lib.OpenCache(system, true)
		if e != nil {
			return e
		}
		defer cache.Close()

		packageInfo, e = cache.GetPackageInfo(packageName)
		return e
	})
	return
}

// SimulateInstall симуляция установки
func (a *Actions) SimulateInstall(packageNames []string) (packageInfo *lib.PackageChanges, err error) {
	if len(packageNames) == 0 {
		return nil, lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}

	err = a.operationWrapper(func() error {
		system, e := getSystem()
		if e != nil {
			return e
		}

		cache, e := lib.OpenCache(system, false)
		if e != nil {
			return e
		}
		defer cache.Close()

		packageInfo, e = cache.SimulateInstall(packageNames)
		return e
	})
	return
}

// SimulateRemove симуляция удаления
func (a *Actions) SimulateRemove(packageNames []string, purge bool, depends bool) (packageInfo *lib.PackageChanges, err error) {
	if len(packageNames) == 0 {
		return nil, lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}

	err = a.operationWrapper(func() error {
		system, e := getSystem()
		if e != nil {
			return e
		}

		cache, e := lib.OpenCache(system, false)
		if e != nil {
			return e
		}
		defer cache.Close()

		packageInfo, e = cache.SimulateRemove(packageNames, purge, depends)
		return e
	})
	return
}

// SimulateDistUpgrade симуляция обновления системы
func (a *Actions) SimulateDistUpgrade() (packageChanges *lib.PackageChanges, err error) {
	err = a.operationWrapper(func() error {
		system, e := getSystem()
		if e != nil {
			return e
		}

		cache, e := lib.OpenCache(system, false)
		if e != nil {
			return e
		}
		defer cache.Close()

		packageChanges, e = cache.SimulateDistUpgrade()
		return e
	})
	return
}

// SimulateAutoRemove симуляция автоматического удаления неиспользуемых пакетов
func (a *Actions) SimulateAutoRemove() (packageChanges *lib.PackageChanges, err error) {
	err = a.operationWrapper(func() error {
		system, e := getSystem()
		if e != nil {
			return e
		}

		cache, e := lib.OpenCache(system, false)
		if e != nil {
			return e
		}
		defer cache.Close()

		packageChanges, e = cache.SimulateAutoRemove()
		return e
	})
	return
}

// SimulateChange комбинированная симуляция установки и удаления
func (a *Actions) SimulateChange(installNames []string, removeNames []string, purge bool, depends bool) (packageChanges *lib.PackageChanges, err error) {
	if len(installNames) == 0 && len(removeNames) == 0 {
		return nil, lib.CustomError(lib.AptErrorInvalidParameters, "Invalid parameters")
	}

	err = a.operationWrapper(func() error {
		system, e := getSystem()
		if e != nil {
			return e
		}

		cache, e := lib.OpenCache(system, false)
		if e != nil {
			return e
		}
		defer cache.Close()

		packageChanges, e = cache.SimulateChange(installNames, removeNames, purge, depends)
		return e
	})
	return
}

// SimulateReinstall симуляция переустановки пакетов
func (a *Actions) SimulateReinstall(packageNames []string) (packageInfo *lib.PackageChanges, err error) {
	if len(packageNames) == 0 {
		return nil, lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}

	err = a.operationWrapper(func() error {
		system, e := getSystem()
		if e != nil {
			return e
		}

		cache, e := lib.OpenCache(system, false)
		if e != nil {
			return e
		}
		defer cache.Close()

		packageInfo, e = cache.SimulateReinstall(packageNames)
		return e
	})
	return
}

// ReinstallPackages переустановка пакетов
func (a *Actions) ReinstallPackages(packageNames []string, handler lib.ProgressHandler) error {
	if len(packageNames) == 0 {
		return lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}

	return a.operationWrapper(func() error {
		system, err := getSystem()
		if err != nil {
			return err
		}

		cache, err := lib.OpenCache(system, false)
		if err != nil {
			return err
		}
		defer cache.Close()

		// Apply reinstall changes to cache
		if e := cache.ApplyReinstall(packageNames); e != nil {
			return e
		}

		pm, err := lib.NewPackageManager(cache)
		if err != nil {
			return err
		}
		defer pm.Close()

		if handler != nil {
			return pm.InstallPackagesWithProgress(handler)
		}
		return pm.InstallPackages()
	})
}

// checkAnyError анализ всех ошибок, включает в себя stdout из apt-lib
func (a *Actions) checkAnyError(logs []string, err error) error {
	aptErrors := apt.ErrorLinesAnalyseAll(logs)
	for _, errApr := range aptErrors {
		return errApr
	}

	if err == nil {
		return nil
	}

	if msg := strings.TrimSpace(err.Error()); msg != "" {
		lines := strings.Split(msg, "\n")
		if m := apt.ErrorLinesAnalise(lines); m != nil {
			// Если это ошибка с провайдерами, захватываем весь список
			if m.Entry.Code == apt.ErrMultiInstallProvidersSelect && len(lines) > 1 {
				var providers []string
				for i := 1; i < len(lines); i++ {
					line := strings.TrimSpace(lines[i])
					if line != "" && !strings.HasPrefix(line, "You should") {
						providers = append(providers, line)
					}
				}
				if len(providers) > 0 {
					m.Params = append(m.Params, strings.Join(providers, "\n"))
				}
			}
			return m
		}
		if m := apt.CheckError(msg); m != nil {
			return m
		}
	}

	return err
}
