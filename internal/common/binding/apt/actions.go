package apt

import (
	"apm/internal/common/apt"
	"apm/internal/common/binding/apt/lib"
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
		if e := cache.MarkInstall(name, true); e != nil {
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
		if e := cache.MarkInstall(name, true); e != nil {
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

// SimulateChange комбинированная симуляция установки и удаления
func (a *Actions) SimulateChange(installNames []string, removeNames []string, purge bool) (packageChanges *lib.PackageChanges, err error) {
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
	if err == nil {
		return nil
	}
	aptErrors := apt.ErrorLinesAnalyseAll(logs)
	for _, errApr := range aptErrors {
		if errApr.IsCritical() {
			return errApr
		}
	}
	return err
}
