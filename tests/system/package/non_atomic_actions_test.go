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

const nonAtomicActionsTestPackage = "hello"

// TestNonAtomicInstall проверяет Install
func TestNonAtomicInstall(t *testing.T) {
	if lib.Env.IsAtomic {
		t.Skip("This test is available only for non-atomic systems")
	}

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

	// Тестируем реальную установку в неатомарной системе
	err = actions.Install(ctx, []string{nonAtomicActionsTestPackage})
	if err != nil {
		t.Logf("Install error (may be expected): %v", err)
	} else {
		t.Log("Install successful")
	}
}

// TestNonAtomicRemove проверяет Remove
func TestNonAtomicRemove(t *testing.T) {
	// Пропускаем если система атомарная
	if lib.Env.IsAtomic {
		t.Skip("This test is available only for non-atomic systems")
	}

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

	err = actions.Remove(ctx, []string{nonAtomicActionsTestPackage}, false)
	if err != nil {
		t.Logf("Remove error (expected for nonexistent package): %v", err)
		assert.True(t,
			strings.Contains(err.Error(), "not installed") ||
				strings.Contains(err.Error(), "No candidates") ||
				strings.Contains(err.Error(), "Couldn't find package"),
			"Unexpected error: %v", err)
	} else {
		t.Log("Remove successful")
	}
}

// TestNonAtomicUpdate проверяет Update
func TestNonAtomicUpdate(t *testing.T) {
	if lib.Env.IsAtomic {
		t.Skip("This test is available only for non-atomic systems")
	}

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
	} else {
		t.Log("Update successful")
	}
}

// TestNonAtomicUpgrade проверяет Upgrade
func TestNonAtomicUpgrade(t *testing.T) {
	if lib.Env.IsAtomic {
		t.Skip("This test is available only for non-atomic systems")
	}

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

// TestNonAtomicUpdateKernel проверяет UpdateKernel
func TestNonAtomicUpdateKernel(t *testing.T) {
	if lib.Env.IsAtomic {
		t.Skip("This test is available only for non-atomic systems")
	}

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
