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

// CombineInstallRemovePackages комбинированный метод установки и удаления
func (a *Actions) CombineInstallRemovePackages(packagesInstall []string, packagesRemove []string, handler lib.ProgressHandler) (err error) {
	lib.StartOperation()
	defer lib.EndOperation()
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := getSystem()
	if err != nil {
		return
	}

	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return
	}
	defer cache.Close()

	for _, name := range packagesRemove {
		if e := cache.MarkRemove(name, false); e != nil {
			err = e
			return
		}
	}

	for _, name := range packagesInstall {
		if e := cache.MarkInstall(name); e != nil {
			err = e
			return
		}
	}

	pm, err := lib.NewPackageManager(cache)
	if err != nil {
		return
	}
	defer pm.Close()

	if handler != nil {
		err = pm.InstallPackagesWithProgress(handler)
	} else {
		err = pm.InstallPackages()
	}
	return
}

// InstallPackages установка пакетов
func (a *Actions) InstallPackages(packageNames []string, handler lib.ProgressHandler) (err error) {
	lib.StartOperation()
	defer lib.EndOperation()
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := getSystem()
	if err != nil {
		return
	}
	if len(packageNames) == 0 {
		err = lib.CustomError(lib.APT_ERROR_INVALID_PARAMETERS, "no packages specified")
		return
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return
	}
	defer cache.Close()

	for _, name := range packageNames {
		if e := cache.MarkInstall(name); e != nil {
			err = e
			return
		}
	}

	pm, err := lib.NewPackageManager(cache)
	if err != nil {
		return
	}
	defer pm.Close()

	if handler != nil {
		err = pm.InstallPackagesWithProgress(handler)
	} else {
		err = pm.InstallPackages()
	}
	return
}

// RemovePackages удаление пакетов
func (a *Actions) RemovePackages(packageNames []string, purge bool, handler lib.ProgressHandler) (err error) {
	lib.StartOperation()
	defer lib.EndOperation()
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := getSystem()
	if err != nil {
		return
	}
	if len(packageNames) == 0 {
		err = lib.CustomError(lib.APT_ERROR_INVALID_PARAMETERS, "no packages specified")
		return
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return
	}
	defer cache.Close()

	for _, name := range packageNames {
		if e := cache.MarkRemove(name, purge); e != nil {
			err = e
			return
		}
	}

	pm, err := lib.NewPackageManager(cache)
	if err != nil {
		return
	}
	defer pm.Close()

	if handler != nil {
		err = pm.InstallPackagesWithProgress(handler)
	} else {
		err = pm.InstallPackages()
	}
	return
}

// DistUpgrade обновление системы
func (a *Actions) DistUpgrade(handler lib.ProgressHandler) (err error) {
	lib.StartOperation()
	defer lib.EndOperation()
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := getSystem()
	if err != nil {
		return
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return
	}
	defer cache.Close()

	if handler != nil {
		err = cache.DistUpgradeWithProgress(handler)
	} else {
		err = cache.DistUpgradeWithProgress(nil)
	}
	return
}

// Update обновление локальной базы пакетов
func (a *Actions) Update() (err error) {
	lib.StartOperation()
	defer lib.EndOperation()
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := getSystem()
	if err != nil {
		return
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return
	}
	defer cache.Close()
	err = cache.Update()

	return
}

// Search поиск по пакетам
func (a *Actions) Search(pattern string) (packages []lib.PackageInfo, err error) {
	lib.StartOperation()
	defer lib.EndOperation()
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := getSystem()
	if err != nil {
		return
	}
	cache, err := lib.OpenCache(system, true)
	if err != nil {
		return
	}
	defer cache.Close()

	packages, err = cache.SearchPackages(pattern)
	return
}

// GetInfo поиск одного пакета
func (a *Actions) GetInfo(packageName string) (packageInfo *lib.PackageInfo, err error) {
	lib.StartOperation()
	defer lib.EndOperation()
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := getSystem()
	if err != nil {
		return
	}
	cache, err := lib.OpenCache(system, true)
	if err != nil {
		return
	}
	defer cache.Close()

	packageInfo, err = cache.GetPackageInfo(packageName)
	return
}

// SimulateInstall симуляция установки
func (a *Actions) SimulateInstall(packageNames []string) (packageInfo *lib.PackageChanges, err error) {
	lib.StartOperation()
	defer lib.EndOperation()
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := getSystem()
	if err != nil {
		return
	}
	if len(packageNames) == 0 {
		err = lib.CustomError(lib.APT_ERROR_INVALID_PARAMETERS, "no packages specified")
		return
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return
	}
	defer cache.Close()

	packageInfo, err = cache.SimulateInstall(packageNames)
	return
}

// SimulateRemove симуляция удаления
func (a *Actions) SimulateRemove(packageNames []string) (packageInfo *lib.PackageChanges, err error) {
	lib.StartOperation()
	defer lib.EndOperation()
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := getSystem()
	if err != nil {
		return
	}
	if len(packageNames) == 0 {
		err = lib.CustomError(lib.APT_ERROR_INVALID_PARAMETERS, "no packages specified")
		return
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return
	}
	defer cache.Close()

	packageInfo, err = cache.SimulateRemove(packageNames)
	return
}

// SimulateDistUpgrade симуляция обновления системы
func (a *Actions) SimulateDistUpgrade() (packageChanges *lib.PackageChanges, err error) {
	lib.StartOperation()
	defer lib.EndOperation()
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := getSystem()
	if err != nil {
		return
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return
	}
	defer cache.Close()

	packageChanges, err = cache.SimulateDistUpgrade()
	return
}

// SimulateAutoRemove симуляция автоматического удаления неиспользуемых пакетов
func (a *Actions) SimulateAutoRemove() (packageChanges *lib.PackageChanges, err error) {
	lib.StartOperation()
	defer lib.EndOperation()
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := getSystem()
	if err != nil {
		return
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return
	}
	defer cache.Close()

	packageChanges, err = cache.SimulateAutoRemove()
	return
}

// SimulateChange комбинированная симуляция установки и удаления
func (a *Actions) SimulateChange(installNames []string, removeNames []string, purge bool) (packageChanges *lib.PackageChanges, err error) {
	lib.StartOperation()
	defer lib.EndOperation()
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := getSystem()
	if err != nil {
		return
	}
	if len(installNames) == 0 && len(removeNames) == 0 {
		err = lib.CustomError(lib.APT_ERROR_INVALID_PARAMETERS, "Invalid parameters")
		return
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return
	}
	defer cache.Close()

	packageChanges, err = cache.SimulateChange(installNames, removeNames, purge)
	return
}

// checkAnyError анализ всех ошибок, включает в себя stdout из apt-lib
func (a *Actions) checkAnyError(logs []string, err error) error {
	aptErrors := apt.ErrorLinesAnalyseAll(logs)
	for _, errApr := range aptErrors {
		if errApr.IsCritical() {
			return errApr
		}
	}

	if err == nil {
		return nil
	}

	if msg := strings.TrimSpace(err.Error()); msg != "" {
		if m := apt.CheckError(msg); m != nil {
			if m.IsCritical() {
				return m
			}
		}
	}

	return err
}
