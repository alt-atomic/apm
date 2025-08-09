package apt

import (
	"apm/internal/common/apt"
	"apm/internal/common/binding/apt/lib"
	"sync"
)

type Actions struct {
	system     *lib.System
	systemOnce sync.Once
	systemErr  error
}

func NewActions() *Actions {
	return &Actions{}
}

func (a *Actions) getSystem() (*lib.System, error) {
	a.systemOnce.Do(func() {
		a.system, a.systemErr = lib.NewSystem()
	})
	return a.system, a.systemErr
}

func (a *Actions) Close() {
	if a.system != nil {
		a.system.Close()
		a.system = nil
	}
	a.systemErr = nil
	a.systemOnce = sync.Once{}
}

// InstallPackages installs the given packages with optional progress handler (instance method)
func (a *Actions) InstallPackages(packageNames []string, handler lib.ProgressHandler) (err error) {
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()

	system, err := a.getSystem()
	if err != nil {
		return
	}
	if len(packageNames) == 0 {
		err = lib.CustomError(int(lib.APT_ERROR_INVALID_PARAMETERS), "no packages specified")
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

// RemovePackages removes packages (optionally purge) with optional progress handler (instance method)
func (a *Actions) RemovePackages(packageNames []string, purge bool, handler lib.ProgressHandler) (err error) {
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()

	system, err := a.getSystem()
	if err != nil {
		return
	}
	if len(packageNames) == 0 {
		err = lib.CustomError(int(lib.APT_ERROR_INVALID_PARAMETERS), "no packages specified")
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

// DistUpgrade performs full system upgrade (instance method)
func (a *Actions) DistUpgrade(handler lib.ProgressHandler) (err error) {
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()

	system, err := a.getSystem()
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

// Search opens RO cache and searches packages (instance method)
func (a *Actions) Search(pattern string) (packages []lib.PackageInfo, err error) {
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()

	system, err := a.getSystem()
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

// GetInfo returns package info (instance method)
func (a *Actions) GetInfo(packageName string) (packageInfo *lib.PackageInfo, err error) {
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := a.getSystem()
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

// SimulateInstall simulates install (instance method)
func (a *Actions) SimulateInstall(packageNames []string) (packageInfo *lib.PackageChanges, err error) {
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := a.getSystem()
	if err != nil {
		return
	}
	if len(packageNames) == 0 {
		err = lib.CustomError(int(lib.APT_ERROR_INVALID_PARAMETERS), "no packages specified")
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

// SimulateRemove simulates removal (instance method)
func (a *Actions) SimulateRemove(packageNames []string) (packageInfo *lib.PackageChanges, err error) {
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := a.getSystem()
	if err != nil {
		return
	}
	if len(packageNames) == 0 {
		err = lib.CustomError(int(lib.APT_ERROR_INVALID_PARAMETERS), "no packages specified")
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

// SimulateDistUpgrade simulates dist-upgrade (instance method)
func (a *Actions) SimulateDistUpgrade() (packageChanges *lib.PackageChanges, err error) {
	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)
	defer func() {
		lib.CaptureStdIO(false)
		lib.SetLogHandler(nil)
		err = a.checkAnyError(logs, err)
	}()
	system, err := a.getSystem()
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

// checkAnyError analyzes captured logs together with the error from bindings
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
