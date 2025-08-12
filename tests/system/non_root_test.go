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
	"context"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNonRootInfo тестирует функцию Info
func TestNonRootInfo(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Info(ctx, testPackage, false)
	if err != nil {
		t.Logf("Info error (may be expected if package not in DB): %v", err)
		// Проверяем что это не критическая ошибка
		assert.True(t,
			strings.Contains(err.Error(), "Package database is empty") ||
				strings.Contains(err.Error(), "Failed to retrieve information") ||
				strings.Contains(err.Error(), "Elevated rights are required"),
			"Unexpected error: %v", err)
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Info successful: %+v", resp.Data)
	}
}

// TestNonRootInfoEmptyPackageName проверяет поведение с пустым именем пакета
func TestNonRootInfoEmptyPackageName(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	_, err := actions.Info(ctx, "", false)

	assert.Error(t, err)
	t.Logf("Expected validation error: %v", err)
}

// TestNonRootSearch тестирует функцию Search (не требует root)
func TestNonRootSearch(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Search(ctx, testPackage, false, false)
	if err != nil {
		t.Logf("Search error (may be expected): %v", err)
		assert.True(t,
			strings.Contains(err.Error(), "Package database is empty") ||
				strings.Contains(err.Error(), "Search query too short") ||
				strings.Contains(err.Error(), "Elevated rights are required"),
			"Unexpected error: %v", err)
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Search successful")
	}
}

// TestNonRootCheckInstall тестирует CheckInstall
func TestNonRootCheckInstall(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	_, err := actions.CheckInstall(ctx, []string{testPackage})
	if err != nil {
		t.Logf("CheckInstall error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	}
}

// TestNonRootCheckRemove тестирует CheckRemove
func TestNonRootCheckRemove(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	_, err := actions.CheckRemove(ctx, []string{"nonexistent-package"})
	if err != nil {
		t.Logf("CheckRemove error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	}
}

// TestNonRootGetFilterFields тестирует получение полей фильтрации
func TestNonRootGetFilterFields(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.GetFilterFields(ctx)
	if err != nil {
		t.Logf("GetFilterFields error (may be expected): %v", err)
		assert.True(t,
			strings.Contains(err.Error(), "Elevated rights are required") ||
				strings.Contains(err.Error(), "Package database is empty"),
			"Unexpected error: %v", err)
		return
	}

	assert.NotNil(t, resp)
	assert.False(t, resp.Error)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, data, "filterFields")

	t.Logf("GetFilterFields successful: %+v", data["filterFields"])
}

// TestNonRootFormatOutput тестирует форматирование без внешних зависимостей
func TestNonRootFormatOutput(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	testData := map[string]interface{}{
		"test": "value",
	}

	result := actions.FormatPackageOutput(testData, false)
	assert.Nil(t, result, "Should return nil for invalid data")

	result = actions.FormatPackageOutput("string", true)
	assert.Nil(t, result, "Should return nil for string input")
}
