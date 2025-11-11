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
	"apm/tests/integration/common"
	"context"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const testPackage = "hello"

// SystemTestSuite для всех системных тестов (требуют root права)
type SystemTestSuite struct {
	suite.Suite
	actions *system.Actions
	ctx     context.Context
}

// SetupSuite создает actions один раз для всех тестов
func (s *SystemTestSuite) SetupSuite() {
	if syscall.Geteuid() != 0 {
		s.T().Skip("This test suite requires root privileges. Run with sudo.")
	}

	appConfig, ctx := common.GetTestAppConfig(s.T())
	s.actions = system.NewActions(appConfig)
	s.ctx = ctx
}

// TestInstall тестирует установку пакетов
func (s *SystemTestSuite) TestInstall() {
	resp, err := s.actions.Install(s.ctx, []string{testPackage}, true)
	if err != nil {
		s.T().Logf("Install error (may be expected if already installed): %v", err)

		assert.True(s.T(),
			contains(err.Error(), "already") ||
				contains(err.Error(), "nothing") ||
				contains(err.Error(), "not make any changes") ||
				contains(err.Error(), "Failed to retrieve information"),
			"Unexpected error type: %v", err)
	} else {
		assert.NotNil(s.T(), resp)
		assert.False(s.T(), resp.Error)
		s.T().Logf("Install successful: %+v", resp.Data)
	}
}

// TestRemove тестирует удаление пакетов
func (s *SystemTestSuite) TestRemove() {
	resp, err := s.actions.Remove(s.ctx, []string{testPackage}, false, false, true)
	if err != nil {
		s.T().Logf("Remove error (expected for nonexistent package): %v", err)
		assert.True(s.T(),
			contains(err.Error(), "not installed") ||
				contains(err.Error(), " Failed to retrieve information about the package") ||
				contains(err.Error(), "Couldn't find package"),
			"Unexpected error: %v", err)
	} else {
		assert.NotNil(s.T(), resp)
		assert.False(s.T(), resp.Error)
		s.T().Logf("Remove successful")
	}
}

// TestRemove тестирует удаление несуществующего пакета
func (s *SystemTestSuite) TestRemoveNotExistentPackage() {
	resp, err := s.actions.Remove(s.ctx, []string{"nonexistent-package"}, false, false, true)
	if err != nil {
		s.T().Logf("Remove error (expected for nonexistent package): %v", err)
		assert.True(s.T(),
			contains(err.Error(), "not installed") ||
				contains(err.Error(), " Failed to retrieve information about the package") ||
				contains(err.Error(), "Couldn't find package"),
			"Unexpected error: %v", err)
	} else {
		assert.NotNil(s.T(), resp)
		assert.False(s.T(), resp.Error)
		s.T().Logf("Remove successful")
	}
}

// TestUpdate тестирует обновление пакетов
func (s *SystemTestSuite) TestUpdate() {
	resp, err := s.actions.Update(s.ctx)
	if err != nil {
		s.T().Logf("Update error (may be expected): %v", err)
	} else {
		assert.NotNil(s.T(), resp)
		assert.False(s.T(), resp.Error)
		s.T().Logf("Update successful")
	}
}

// TestUpgrade тестирует обновление системы
func (s *SystemTestSuite) TestUpgrade() {
	resp, err := s.actions.Upgrade(s.ctx)
	if err != nil {
		s.T().Logf("Upgrade error (may be expected): %v", err)
	} else {
		assert.NotNil(s.T(), resp)
		assert.False(s.T(), resp.Error)
		s.T().Logf("Upgrade successful")
	}
}

// TestInfo тестирует функцию Info
func (s *SystemTestSuite) TestInfo() {
	resp, err := s.actions.Info(s.ctx, testPackage, false)
	if err != nil {
		s.T().Logf("Info error (may be expected if package not in DB): %v", err)
		// Проверяем что это не критическая ошибка
		assert.True(s.T(),
			strings.Contains(err.Error(), "Package database is empty") ||
				strings.Contains(err.Error(), "Failed to retrieve information"),
			"Unexpected error: %v", err)
	} else {
		assert.NotNil(s.T(), resp)
		assert.False(s.T(), resp.Error)
		s.T().Logf("Info successful: %+v", resp.Data)
	}
}

// TestInfoEmptyPackageName проверяет поведение с пустым именем пакета
func (s *SystemTestSuite) TestInfoEmptyPackageName() {
	_, err := s.actions.Info(s.ctx, "", false)

	assert.Error(s.T(), err)
	s.T().Logf("Expected validation error: %v", err)
}

// TestSearch тестирует функцию Search
func (s *SystemTestSuite) TestSearch() {
	resp, err := s.actions.Search(s.ctx, testPackage, false, false)
	if err != nil {
		s.T().Logf("Search error (may be expected): %v", err)
		assert.True(s.T(),
			strings.Contains(err.Error(), "Package database is empty") ||
				strings.Contains(err.Error(), "Search query too short"),
			"Unexpected error: %v", err)
	} else {
		assert.NotNil(s.T(), resp)
		assert.False(s.T(), resp.Error)
		s.T().Logf("Search successful")
	}
}

// TestCheckInstall тестирует CheckInstall
func (s *SystemTestSuite) TestCheckInstall() {
	_, err := s.actions.CheckInstall(s.ctx, []string{testPackage})
	if err != nil {
		s.T().Logf("CheckInstall error (may be expected): %v", err)
	}
}

// TestCheckRemove тестирует CheckRemove
func (s *SystemTestSuite) TestCheckRemove() {
	_, err := s.actions.CheckRemove(s.ctx, []string{"nonexistent-package"}, false, false)
	if err != nil {
		s.T().Logf("CheckRemove error (may be expected): %v", err)
	}
}

// TestGetFilterFields тестирует получение полей фильтрации
func (s *SystemTestSuite) TestGetFilterFields() {
	resp, err := s.actions.GetFilterFields(s.ctx)
	if err != nil {
		s.T().Logf("GetFilterFields error (may be expected): %v", err)
		assert.True(s.T(),
			strings.Contains(err.Error(), "Package database is empty"),
			"Unexpected error: %v", err)
		return
	}

	assert.NotNil(s.T(), resp)
	assert.False(s.T(), resp.Error)

	switch data := resp.Data.(type) {
	case []interface{}:
		s.T().Logf("Found interface{} slice with %d items", len(data))
		assert.NotEmpty(s.T(), data, "Expected non-empty filter fields")
		s.T().Logf("GetFilterFields successful: first item: %+v", data[0])
	default:
		if data == nil {
			s.T().Error("Data is nil")
		} else {
			s.T().Logf("Data type: %T, value: %+v", data, data)
			assert.NotNil(s.T(), data, "Expected non-nil data")
		}
	}
}

// TestFormatOutput тестирует форматирование без внешних зависимостей
func (s *SystemTestSuite) TestFormatOutput() {
	testData := map[string]interface{}{
		"test": "value",
	}

	result := s.actions.FormatPackageOutput(testData, false)
	assert.Nil(s.T(), result, "Should return nil for invalid data")

	result = s.actions.FormatPackageOutput("string", true)
	assert.Nil(s.T(), result, "Should return nil for string input")
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

// Запуск набора тестов
func TestSystemSuite(t *testing.T) {
	suite.Run(t, new(SystemTestSuite))
}
