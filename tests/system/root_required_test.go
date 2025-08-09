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

package system

import (
	"apm/internal/system"
	"context"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRootInstall тестирует установку пакетов
func TestRootInstall(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Install(ctx, []string{"hello"}, false)
	if err != nil {
		t.Logf("Install error (may be expected if already installed): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")

		assert.True(t,
			contains(err.Error(), "already") ||
				contains(err.Error(), "nothing") ||
				contains(err.Error(), "Failed to retrieve information"),
			"Unexpected error type: %v", err)
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Install successful: %+v", resp.Data)
	}
}

// TestRootRemove тестирует удаление пакетов
func TestRootRemove(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Remove(ctx, []string{"nonexistent-package"}, false)
	if err != nil {
		t.Logf("Remove error (expected for nonexistent package): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
		assert.True(t,
			contains(err.Error(), "not installed") ||
				contains(err.Error(), "No candidates") ||
				contains(err.Error(), "Couldn't find package"),
			"Unexpected error: %v", err)
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Remove successful")
	}
}

// TestRootUpdate тестирует обновление системы
func TestRootUpdate(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Update(ctx)
	if err != nil {
		t.Logf("Update error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Update successful")
	}
}

// TestRootUpgrade тестирует обновление системы
func TestRootUpgrade(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Upgrade(ctx)
	if err != nil {
		t.Logf("Upgrade error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Upgrade successful")
	}
}

// TestRootImageStatus тестирует получение статуса образа
func TestRootImageStatus(t *testing.T) {
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

// TestRootImageUpdate тестирует обновление образа
func TestRootImageUpdate(t *testing.T) {
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

// TestRootImageApply тестирует применение изменений к образу
func TestRootImageApply(t *testing.T) {
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

// TestRootImageHistory тестирует получение истории образов
func TestRootImageHistory(t *testing.T) {
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

// Helper function для проверки содержимого строки
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					strings.Contains(s, substr))))
}
