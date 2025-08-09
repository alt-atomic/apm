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
	errs := actions.Install(ctx, "hello")
	if len(errs) > 0 {
		t.Logf("Install errors (may be expected): %v", errs)
		// Проверяем что это не ошибка прав доступа
		for _, err := range errs {
			assert.NotContains(t, err.Error(), "Elevated rights are required")
		}
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

	errs := actions.Remove(ctx, "nonexistent-package")
	if len(errs) > 0 {
		t.Logf("Remove errors (expected for nonexistent package): %v", errs)
		for _, err = range errs {
			assert.NotContains(t, err.Error(), "Elevated rights are required")
			assert.True(t,
				strings.Contains(err.Error(), "not installed") ||
					strings.Contains(err.Error(), "No candidates") ||
					strings.Contains(err.Error(), "Couldn't find package"),
				"Unexpected error: %v", err)
		}
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
		assert.NotContains(t, err.Error(), "Elevated rights are required")
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

	errs := actions.Upgrade(ctx)
	if len(errs) > 0 {
		t.Logf("Upgrade errors (may be expected): %v", errs)
		for _, err = range errs {
			assert.NotContains(t, err.Error(), "Elevated rights are required")
		}
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
		for _, err = range errs {
			assert.NotContains(t, err.Error(), "Elevated rights are required")
		}
	} else {
		t.Log("UpdateKernel successful")
	}
}
