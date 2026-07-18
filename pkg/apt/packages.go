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
	"fmt"
	"os"
	"sync"

	"altlinux.space/alt-atomic/apm/pkg/apt/lib"
)

// Single APT system instance, must be CLOSED at the end
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

// SetConfigOverrides sets APT configuration overrides
func (a *Actions) SetConfigOverrides(overrides map[string]string) {
	a.configOverrides = overrides
}

// GetConfigOverrides returns current APT configuration overrides
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

// OperationOptions holds options for APT operations
type OperationOptions struct {
	SkipLock     bool
	RpmArguments []string
}

// runOperation is the single wrapper for all APT operations
func (a *Actions) runOperation(opts OperationOptions, fn func(system *lib.System) error) error {
	aptMutex.Lock()
	defer aptMutex.Unlock()

	if opts.SkipLock {
		lib.SetNoLocking(true)
		defer lib.SetNoLocking(false)
	} else if err := lib.CheckLockOrError(); err != nil {
		return err
	}

	lib.BeginOperation()
	defer lib.EndOperation()

	system, initErr := getSystem()
	if initErr != nil {
		return initErr
	}

	logs := make([]string, 0, 256)
	stopCapture := lib.BeginLogCapture(func(msg string) { logs = append(logs, msg) })

	var err error
	if len(opts.RpmArguments) > 0 {
		err = lib.PreprocessInstallArguments(opts.RpmArguments)
	}

	if err == nil {
		err = lib.WithConfigOverrides(a.configOverrides, func() error {
			return fn(system)
		})
	}

	// Clear RPM arguments and stop capture before analyzing logs
	lib.ClearInstallArguments()
	stopCapture()

	return errorAnalyzer(logs, err)
}

// withCache opens the APT cache and passes it to fn
func withCache(system *lib.System, readOnly bool, fn func(*lib.Cache) error) error {
	cache, err := lib.OpenCache(system, readOnly)
	if err != nil {
		return err
	}
	defer cache.Close()
	return fn(cache)
}

// CombineInstallRemovePackages installs and removes packages in one transaction
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

// InstallPackages installs packages
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

// RemovePackages removes packages
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

// DistUpgrade upgrades the system
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

// DownloadSourcePackages downloads .src.rpm files for the given names into destDir
func (a *Actions) DownloadSourcePackages(packageNames []string, destDir string,
	handler lib.ProgressHandler) (packages []lib.SourcePackage, err error) {
	if len(packageNames) == 0 {
		return nil, lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}
	if destDir == "" {
		if destDir, err = os.Getwd(); err != nil {
			return nil, err
		}
	}
	if info, statErr := os.Stat(destDir); statErr != nil || !info.IsDir() {
		return nil, lib.CustomError(lib.AptErrorInvalidParameters,
			fmt.Sprintf("destination directory is not accessible: %s", destDir))
	}
	err = a.runOperation(OperationOptions{SkipLock: true}, func(system *lib.System) error {
		return withCache(system, true, func(cache *lib.Cache) error {
			packages, err = cache.DownloadSource(packageNames, destDir, handler)
			return err
		})
	})
	return
}

// Update refreshes the local package database
func (a *Actions) Update(handler lib.ProgressHandler, noLock ...bool) error {
	skipLock := len(noLock) > 0 && noLock[0]
	return a.runOperation(OperationOptions{SkipLock: skipLock}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			return cache.Update(handler)
		})
	})
}

// Search searches packages
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

// GetInfo returns info for a single package
func (a *Actions) GetInfo(packageName string) (packageInfo *lib.PackageInfo, err error) {
	err = a.runOperation(OperationOptions{}, func(system *lib.System) error {
		return withCache(system, true, func(cache *lib.Cache) error {
			packageInfo, err = cache.GetPackageInfo(packageName)
			return err
		})
	})
	return
}

// SimulateInstall simulates installation
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

// SimulateRemove simulates removal
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

// SimulateDistUpgrade simulates a system upgrade
func (a *Actions) SimulateDistUpgrade() (packageChanges *lib.PackageChanges, err error) {
	err = a.runOperation(OperationOptions{}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			packageChanges, err = cache.SimulateDistUpgrade()
			return err
		})
	})
	return
}

// SimulateAutoRemove simulates removal of unused packages
func (a *Actions) SimulateAutoRemove() (packageChanges *lib.PackageChanges, err error) {
	err = a.runOperation(OperationOptions{}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			packageChanges, err = cache.SimulateAutoRemove()
			return err
		})
	})
	return
}

// SimulateChange simulates combined install and remove
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

// SimulateChangeWithRpmInfo simulates changes and fetches RPM file info in a single cache session
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

// SimulateReinstall simulates package reinstallation
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

// ReinstallPackages reinstalls packages
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
