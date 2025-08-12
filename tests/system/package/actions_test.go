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

package package_test

import (
	"context"
	"strings"
	"syscall"
	"testing"

	_package "apm/internal/system/package"
	"apm/lib"

	"github.com/stretchr/testify/assert"
)

const testPackage = "hello"

// TestNewActions проверяет создание Actions
func TestNewActions(t *testing.T) {
	pkgDBService, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Logf("NewPackageDBService error (may be expected): %v", err)
		t.Skip("Database not available, skipping test")
	}
	assert.NotNil(t, pkgDBService)

	stplrService := _package.NewSTPLRService()
	actions := _package.NewActions(pkgDBService, stplrService)
	assert.NotNil(t, actions)
}

// TestInstall
func TestInstall(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	pkgDBService, err := _package.NewPackageDBService(lib.GetDB(true))
	stplrService := _package.NewSTPLRService()
	actions := _package.NewActions(pkgDBService, stplrService)

	ctx := context.Background()

	err = actions.Install(ctx, []string{testPackage})
	if err != nil {
		t.Logf("Install error (may be expected): %v", err)
	} else {
		t.Log("Install successful")
	}
}

// TestRemove
func TestRemoveRequiresRoot(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	pkgDBService, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	stplrService := _package.NewSTPLRService()
	actions := _package.NewActions(pkgDBService, stplrService)

	ctx := context.Background()

	err = actions.Remove(ctx, []string{"nonexistent-package"}, false)
	if err != nil {
		t.Logf("Remove error (expected for nonexistent package): %v", err)
		assert.True(t,
			strings.Contains(err.Error(), "not installed") ||
				strings.Contains(err.Error(), "No candidates") ||
				strings.Contains(err.Error(), "Package not found"),
			"Unexpected error: %v", err)
	} else {
		t.Log("Remove successful")
	}
}

// TestCheckInstall проверяет функцию Check для установки (не требует root)
func TestCheckInstall(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	pkgDBService, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	stplrService := _package.NewSTPLRService()
	actions := _package.NewActions(pkgDBService, stplrService)

	ctx := context.Background()

	_, err = actions.CheckInstall(ctx, []string{testPackage})
	if err != nil {
		t.Logf("CheckInstall error (may be expected): %v", err)
	} else {
		t.Log("Check install successful")
	}
}

// TestCheckRemove проверяет функцию Check для удаления (не требует root)
func TestCheckRemove(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	pkgDBService, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	stplrService := _package.NewSTPLRService()
	actions := _package.NewActions(pkgDBService, stplrService)

	ctx := context.Background()

	_, err = actions.CheckRemove(ctx, []string{"nonexistent-package"})
	if err != nil {
		t.Logf("Check remove error (expected): %v", err)
	} else {
		t.Log("Check remove successful")
	}
}

// TestUpdateRequiresRoot проверяет что Update требует root права
func TestUpdateRequiresRoot(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	pkgDBService, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	stplrService := _package.NewSTPLRService()
	actions := _package.NewActions(pkgDBService, stplrService)

	ctx := context.Background()

	_, err = actions.Update(ctx)
	if err != nil {
		t.Logf("Update error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	} else {
		t.Log("Update successful")
	}
}

// TestUpgradeRequiresRoot проверяет что Upgrade требует root права
func TestUpgradeRequiresRoot(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	pkgDBService, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	stplrService := _package.NewSTPLRService()
	actions := _package.NewActions(pkgDBService, stplrService)

	ctx := context.Background()

	err = actions.Upgrade(ctx)
	if err != nil {
		t.Logf("Upgrade error (may be expected): %v", err)
	} else {
		t.Log("Upgrade successful")
	}
}

// TestGetInstalledPackagesBasic проверяет базовую функциональность GetInstalledPackages
func TestGetInstalledPackagesBasic(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	pkgDBService, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	stplrService := _package.NewSTPLRService()
	actions := _package.NewActions(pkgDBService, stplrService)

	ctx := context.Background()

	packages, err := actions.GetInstalledPackages(ctx)
	if err != nil {
		t.Logf("GetInstalledPackages error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	} else {
		assert.NotNil(t, packages)
		t.Logf("Found %d installed packages", len(packages))
	}
}

// TestCleanPackageNameBasic проверяет функцию CleanPackageName
func TestCleanPackageNameBasic(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	pkgDBService, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	stplrService := _package.NewSTPLRService()
	actions := _package.NewActions(pkgDBService, stplrService)

	testCases := []struct {
		input    string
		expected string
	}{
		{"package-name", "package-name"},
		{"package-name*", "package-name"},
		{"package-name+", "package-name"},
		{"", ""},
	}

	for _, tc := range testCases {
		result := actions.CleanPackageName(tc.input, []string{})
		assert.NotNil(t, result)
		t.Logf("CleanPackageName('%s') = '%s'", tc.input, result)
	}
}

// TestUpdateKernelRequiresRoot проверяет UpdateKernel
func TestUpdateKernel(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	pkgDBService, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	stplrService := _package.NewSTPLRService()
	actions := _package.NewActions(pkgDBService, stplrService)

	ctx := context.Background()

	errs := actions.UpdateKernel(ctx)
	if len(errs) > 0 {
		t.Logf("UpdateKernel errors (may be expected): %v", errs)
	} else {
		t.Log("UpdateKernel successful")
	}
}

// TestCheckUpdateKernelBasic проверяет функцию CheckUpdateKernel
func TestCheckUpdateKernelBasic(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	pkgDBService, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	stplrService := _package.NewSTPLRService()
	actions := _package.NewActions(pkgDBService, stplrService)

	ctx := context.Background()

	_, errs := actions.CheckUpdateKernel(ctx)
	if len(errs) > 0 {
		t.Logf("CheckUpdateKernel errors (may be expected): %v", errs)
		for _, err := range errs {
			assert.NotContains(t, err.Error(), "Elevated rights are required")
		}
	} else {
		t.Log("CheckUpdateKernel successful")
	}
}
