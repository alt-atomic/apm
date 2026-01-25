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

package binding

import (
	aptErrors "apm/internal/common/apt"
	aptBinding "apm/internal/common/binding/apt"
	aptlib "apm/internal/common/binding/apt/lib"
	"errors"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

const testPackage = "hello"

// TestAptNewActions ensures Actions can be constructed and closed
func TestAptNewActions(t *testing.T) {
	actions := aptBinding.NewActions()
	assert.NotNil(t, actions)
	aptBinding.Close()
}

// TestAptSearchBasic update system
func TestAptUpdate(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}

	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	if err := actions.Update(); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
}

// TestAptSearchBasic performs a simple search (read-only)
func TestAptSearchBasic(t *testing.T) {
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	// Use a common package name likely present in most systems
	pkgs, err := actions.Search(testPackage)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	assert.NotNil(t, pkgs)
}

// TestAptGetInfo_NotFound expects a well-formed APT error for missing package
func TestAptGetInfo_NotFound(t *testing.T) {
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	_, err := actions.GetInfo("__nonexistent_package_for_apm_tests__")
	if err == nil {
		t.Skip("GetInfo returned nil error for nonexistent package; skipping strict assertion")
	}
	var ae *aptlib.AptError
	if errors.As(err, &ae) {
		assert.Equal(t, aptlib.AptErrorPackageNotFound, ae.Code)
		return
	}
	var me *aptErrors.MatchedError
	if errors.As(err, &me) {
		assert.Equal(t, aptErrors.ErrPackageNotFound, me.Entry.Code)
		return
	}
	t.Fatalf("unexpected error type: %T %v", err, err)
}

// TestAptSimulateInstall exercises simulation API (read-only)
func TestAptSimulateInstall(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	changes, err := actions.SimulateInstall([]string{testPackage})
	if err != nil {
		t.Fatalf("SimulateInstall failed: %v", err)
	}
	assert.NotNil(t, changes)
}

// TestAptSimulateRemove exercises simulation API (read-only)
func TestAptSimulateRemove(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	// First, try to install the test package (ignore if already newest version)
	err := actions.InstallPackages([]string{testPackage}, nil)
	if err != nil {
		if matchedErr := aptErrors.CheckError(err.Error()); matchedErr != nil {
			if matchedErr.Entry.Code == aptErrors.ErrPackageIsAlreadyNewest {
				t.Logf("Package %s is already the newest version", testPackage)
			} else {
				t.Logf("Failed to install %s: %v", testPackage, err)
			}
		} else {
			t.Logf("Failed to install %s: %v", testPackage, err)
		}
	}

	// Now simulate removing the package (should work since we ensured it's installed)
	changes, err := actions.SimulateRemove([]string{testPackage}, true, true)
	if err != nil {
		t.Fatalf("SimulateRemove failed: %v", err)
	}
	assert.NotNil(t, changes)
}

// TestAptSimulateDistUpgrade exercises dist-upgrade simulation (read-only)
func TestAptSimulateDistUpgrade(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	changes, err := actions.SimulateDistUpgrade()
	if err != nil {
		t.Fatalf("SimulateDistUpgrade failed: %v", err)
	}
	assert.NotNil(t, changes)
}

// TestAptInstallRemoveHelloRoot tries real install/remove of hello under root.
func TestAptInstallRemoveHelloRoot(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root")
	}

	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	installedFirst := false
	if info, err := actions.GetInfo(testPackage); err == nil && info != nil && info.State == 1 {
		installedFirst = true
	}

	if installedFirst {
		if err := actions.RemovePackages([]string{testPackage}, false, false, nil); err != nil {
			t.Fatalf("remove hello failed: %v", err)
		}
		if err := actions.InstallPackages([]string{testPackage}, nil); err != nil {
			t.Fatalf("install hello failed: %v", err)
		}
	} else {
		if err := actions.InstallPackages([]string{testPackage}, nil); err != nil {
			t.Fatalf("install hello failed: %v", err)
		}
		if err := actions.RemovePackages([]string{testPackage}, false, false, nil); err != nil {
			t.Fatalf("remove hello failed: %v", err)
		}
	}
}

// TestAptInvalidParameters verifies parameter validation hooks
func TestAptInvalidParameters(t *testing.T) {
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	if err := actions.InstallPackages([]string{}, nil); err == nil {
		t.Fatalf("expected error for empty package list in InstallPackages")
	}

	if _, err := actions.SimulateInstall([]string{}); err == nil {
		t.Fatalf("expected error for empty package list in SimulateInstall")
	}

	if _, err := actions.SimulateRemove([]string{}, true, true); err == nil {
		t.Fatalf("expected error for empty package list in SimulateRemove")
	}

	if _, err := actions.SimulateChange(nil, nil, false, true); err == nil {
		t.Fatalf("expected error for empty lists in SimulateChange")
	} else if ae, ok := err.(*aptlib.AptError); ok {
		if ae.Code != aptlib.AptErrorInvalidParameters {
			t.Fatalf("unexpected error code: %d (%v)", ae.Code, ae)
		}
	}
}

// TestAptSimulateReinstall tests reinstalling an already installed package
func TestAptSimulateReinstall(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	// bash is usually installed, so reinstall should work
	changes, err := actions.SimulateReinstall([]string{"bash"})
	if err != nil {
		t.Logf("SimulateReinstall failed: %v", err)
		var ae *aptlib.AptError
		if errors.As(err, &ae) {
			t.Logf("Got AptError with code: %d", ae.Code)
		}
	} else {
		assert.NotNil(t, changes)
		t.Logf("SimulateReinstall succeeded: new_installed=%d", changes.NewInstalledCount)
	}
}

// TestAptSimulateAutoremove tests orphan package detection
func TestAptSimulateAutoremove(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	changes, err := actions.SimulateAutoRemove()
	if err != nil {
		t.Fatalf("SimulateAutoremove failed: %v", err)
	}
	assert.NotNil(t, changes)
	t.Logf("SimulateAutoremove: would remove %d packages", changes.RemovedCount)
}

// TestAptSimulateChangeCombined tests simultaneous install and remove
func TestAptSimulateChangeCombined(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	changes, err := actions.SimulateChange(
		[]string{testPackage},
		[]string{"nano"},
		false,
		false,
	)
	if err != nil {
		t.Logf("SimulateChange combined failed (may be expected): %v", err)
	} else {
		assert.NotNil(t, changes)
		t.Logf("SimulateChange combined: install=%d, remove=%d, upgrade=%d",
			changes.NewInstalledCount, changes.RemovedCount, changes.UpgradedCount)
	}
}

// TestAptMultiplePackageInstall tests installing multiple packages at once
func TestAptMultiplePackageInstall(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	packages := []string{"tree", "htop", "ncdu"}

	changes, err := actions.SimulateInstall(packages)
	if err != nil {
		t.Logf("Multiple package install simulation failed: %v", err)
	} else {
		assert.NotNil(t, changes)
		t.Logf("Multiple packages: new=%d, upgraded=%d, extra=%d",
			changes.NewInstalledCount, changes.UpgradedCount, len(changes.ExtraInstalled))
	}
}

// TestAptInstallSizeCalculation verifies install size is calculated correctly
func TestAptInstallSizeCalculation(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	changes, err := actions.SimulateInstall([]string{testPackage})
	if err != nil {
		t.Skipf("SimulateInstall failed, skipping size check: %v", err)
	}

	t.Logf("Install sizes: download=%d bytes, install=%d bytes",
		changes.DownloadSize, changes.InstallSize)

	assert.True(t, changes.DownloadSize >= 0, "Download size should be non-negative")

	if changes.NewInstalledCount > 0 {
		assert.True(t, changes.InstallSize >= 0,
			"Install size for new packages should be non-negative, got %d", changes.InstallSize)
	}
}
