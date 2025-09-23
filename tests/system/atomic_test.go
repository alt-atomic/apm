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

package system

import (
	"apm/internal/system"
	"apm/lib"
	"context"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAtomicImageStatus тестирует получение статуса образа (только для атомарной системы)
func TestAtomicImageStatus(t *testing.T) {
	if !lib.Env.IsAtomic {
		t.Skip("This test is available only for atomic systems")
	}

	// Пропускаем если запущено не от root
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.ImageStatus(ctx)
	if err != nil {
		t.Logf("ImageStatus error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("ImageStatus successful")
	}
}

// TestAtomicImageUpdate тестирует обновление образа (только для атомарной системы)
func TestAtomicImageUpdate(t *testing.T) {
	if !lib.Env.IsAtomic {
		t.Skip("This test is available only for atomic systems")
	}

	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.ImageUpdate(ctx)
	if err != nil {
		t.Logf("ImageUpdate error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("ImageUpdate successful")
	}
}

// TestAtomicImageApply тестирует применение изменений к образу (только для атомарной системы)
func TestAtomicImageApply(t *testing.T) {
	if !lib.Env.IsAtomic {
		t.Skip("This test is available only for atomic systems")
	}

	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.ImageApply(ctx)
	if err != nil {
		t.Logf("ImageApply error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("ImageApply successful")
	}
}

// TestAtomicImageHistory тестирует получение истории образов (только для атомарной системы)
func TestAtomicImageHistory(t *testing.T) {
	if !lib.Env.IsAtomic {
		t.Skip("This test is available only for atomic systems")
	}

	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.ImageHistory(ctx, "", 10, 0)
	if err != nil {
		t.Logf("ImageHistory error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("ImageHistory successful")
	}
}

// TestAtomicInstall тестирует установку пакетов в атомарной системе
func TestAtomicInstall(t *testing.T) {
	if !lib.Env.IsAtomic {
		t.Skip("This test is available only for atomic systems")
	}

	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Install(ctx, []string{"hello"})
	if err != nil {
		t.Logf("Install error (may be expected if already installed): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Install successful: %+v", resp.Data)
	}
}

// TestAtomicRemove тестирует удаление пакетов в атомарной системе
func TestAtomicRemove(t *testing.T) {
	if !lib.Env.IsAtomic {
		t.Skip("This test is available only for atomic systems")
	}

	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	// Тестируем удаление пакета в атомарной системе
	resp, err := actions.Remove(ctx, []string{"nonexistent-package"}, false)
	if err != nil {
		t.Logf("Remove error (expected for nonexistent package): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Remove successful")
	}
}
