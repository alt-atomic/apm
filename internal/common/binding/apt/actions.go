package apt

import (
	"apm/internal/common/binding/apt/lib"
	"fmt"
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
func (a *Actions) InstallPackages(packageNames []string, handler lib.ProgressHandler) error {
	system, err := a.getSystem()
	if err != nil {
		return err
	}
	if len(packageNames) == 0 {
		return &lib.AptError{Code: lib.APT_ERROR_INVALID_PARAMETERS, Message: "no packages specified"}
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return err
	}
	defer cache.Close()

	for _, name := range packageNames {
		if err = cache.MarkInstall(name, true); err != nil {
			return fmt.Errorf("mark install '%s': %w", name, err)
		}
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
}

// RemovePackages removes packages (optionally purge) with optional progress handler (instance method)
func (a *Actions) RemovePackages(packageNames []string, purge bool, handler lib.ProgressHandler) error {
	system, err := a.getSystem()
	if err != nil {
		return err
	}
	if len(packageNames) == 0 {
		return &lib.AptError{Code: lib.APT_ERROR_INVALID_PARAMETERS, Message: "no packages specified"}
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return err
	}
	defer cache.Close()

	for _, name := range packageNames {
		if err = cache.MarkRemove(name, purge); err != nil {
			return fmt.Errorf("mark remove '%s': %w", name, err)
		}
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
}

// DistUpgrade performs full system upgrade (instance method)
func (a *Actions) DistUpgrade(handler lib.ProgressHandler) error {
	system, err := a.getSystem()
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
}

// Search opens RO cache and searches packages (instance method)
func (a *Actions) Search(pattern string) ([]lib.PackageInfo, error) {
	system, err := a.getSystem()
	if err != nil {
		return nil, err
	}
	cache, err := lib.OpenCache(system, true)
	if err != nil {
		return nil, err
	}
	defer cache.Close()
	return cache.SearchPackages(pattern)
}

// GetInfo returns package info (instance method)
func (a *Actions) GetInfo(packageName string) (*lib.PackageInfo, error) {
	system, err := a.getSystem()
	if err != nil {
		return nil, err
	}
	cache, err := lib.OpenCache(system, true)
	if err != nil {
		return nil, err
	}
	defer cache.Close()
	return cache.GetPackageInfo(packageName)
}

// SimulateInstall simulates install (instance method)
func (a *Actions) SimulateInstall(packageNames []string) (*lib.PackageChanges, error) {
	system, err := a.getSystem()
	if err != nil {
		return nil, err
	}
	if len(packageNames) == 0 {
		return nil, &lib.AptError{Code: lib.APT_ERROR_INVALID_PARAMETERS, Message: "no packages specified"}
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return nil, err
	}
	defer cache.Close()
	return cache.SimulateInstall(packageNames)
}

// SimulateRemove simulates removal (instance method)
func (a *Actions) SimulateRemove(packageNames []string) (*lib.PackageChanges, error) {
	system, err := a.getSystem()
	if err != nil {
		return nil, err
	}
	if len(packageNames) == 0 {
		return nil, &lib.AptError{Code: lib.APT_ERROR_INVALID_PARAMETERS, Message: "no packages specified"}
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return nil, err
	}
	defer cache.Close()
	return cache.SimulateRemove(packageNames)
}

// SimulateDistUpgrade simulates dist-upgrade (instance method)
func (a *Actions) SimulateDistUpgrade() (*lib.PackageChanges, error) {
	system, err := a.getSystem()
	if err != nil {
		return nil, err
	}
	cache, err := lib.OpenCache(system, false)
	if err != nil {
		return nil, err
	}
	defer cache.Close()
	return cache.SimulateDistUpgrade()
}
