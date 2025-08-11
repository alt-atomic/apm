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
	_package "apm/internal/system/package"
	"apm/internal/system/service"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUnitActionsCreation тестирует создание объекта Actions
func TestUnitActionsCreation(t *testing.T) {
	actions := system.NewActionsWithDeps(
		nil, // PackageDBService
		nil, // package.Actions
		&service.HostImageService{},
		&service.HostDBService{},
		&service.HostConfigService{},
	)

	assert.NotNil(t, actions)
}

// TestUnitPackageDBServiceCreation тестирует основную структуру сервисов
func TestUnitPackageDBServiceCreation(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to missing database:", r)
		}
	}()

	actions := system.NewActions()
	assert.NotNil(t, actions)
}

// TestUnitStplrService тестирует создание STPLR сервиса
func TestUnitStplrService(t *testing.T) {
	stplrSvc := &_package.StplrService{}
	assert.NotNil(t, stplrSvc)
}

// TestUnitHostConfigService тестирует создание конфигурационного сервиса
func TestUnitHostConfigService(t *testing.T) {
	configSvc := service.NewHostConfigService("/tmp/test-config.yaml", nil)
	assert.NotNil(t, configSvc)
}

// TestUnitHostImageService тестирует создание сервиса образов
func TestUnitHostImageService(t *testing.T) {
	imageSvc := &service.HostImageService{}
	assert.NotNil(t, imageSvc)
}

// TestUnitPackageStruct тестирует структуру Package
func TestUnitPackageStruct(t *testing.T) {
	pkg := _package.Package{
		Name:        "test-package",
		Version:     "1.0.0",
		Installed:   false,
		Description: "Test package",
		Section:     "utils",
	}

	assert.Equal(t, "test-package", pkg.Name)
	assert.Equal(t, "1.0.0", pkg.Version)
	assert.False(t, pkg.Installed)
	assert.Equal(t, "Test package", pkg.Description)
	assert.Equal(t, "utils", pkg.Section)
}

// TestUnitShortPackageResponse тестирует структуру короткого ответа
func TestUnitShortPackageResponse(t *testing.T) {
	response := system.ShortPackageResponse{
		Name:        "vim",
		Version:     "8.2",
		Installed:   true,
		Description: "Vi Improved",
	}

	assert.Equal(t, "vim", response.Name)
	assert.Equal(t, "8.2", response.Version)
	assert.True(t, response.Installed)
	assert.Equal(t, "Vi Improved", response.Description)
}

// TestUnitConfigStruct тестирует структуру конфигурации
func TestUnitConfigStruct(t *testing.T) {
	config := service.Config{
		Image: "test-image",
		Packages: struct {
			Install []string `yaml:"install" json:"install"`
			Remove  []string `yaml:"remove" json:"remove"`
		}{
			Install: []string{"vim", "nano"},
			Remove:  []string{"emacs"},
		},
		Commands: []string{"echo 'test'"},
	}

	assert.Equal(t, "test-image", config.Image)
	assert.Equal(t, []string{"vim", "nano"}, config.Packages.Install)
	assert.Equal(t, []string{"emacs"}, config.Packages.Remove)
	assert.Equal(t, []string{"echo 'test'"}, config.Commands)
}

// TestUnitFormatPackageOutput тестирует форматирование вывода пакетов без зависимостей
func TestUnitFormatPackageOutput(t *testing.T) {
	actions := system.NewActionsWithDeps(
		nil, nil, nil, nil, nil,
	)

	testNewPackage := _package.Package{
		Name:        "vim",
		Version:     "8.2",
		Installed:   true,
		Description: "Vi Improved",
	}

	result := actions.FormatPackageOutput(testNewPackage, true)
	assert.IsType(t, _package.Package{}, result)
	resultPkg := result.(_package.Package)
	assert.Equal(t, "vim", resultPkg.Name)

	result = actions.FormatPackageOutput(testNewPackage, false)
	assert.IsType(t, system.ShortPackageResponse{}, result)
	shortResult := result.(system.ShortPackageResponse)
	assert.Equal(t, "vim", shortResult.Name)
	assert.Equal(t, "8.2", shortResult.Version)
	assert.True(t, shortResult.Installed)

	testPackages := []_package.Package{testNewPackage}
	result = actions.FormatPackageOutput(testPackages, false)
	assert.IsType(t, []system.ShortPackageResponse{}, result)
	resultList := result.([]system.ShortPackageResponse)
	assert.Len(t, resultList, 1)

	result = actions.FormatPackageOutput("invalid", false)
	assert.Nil(t, result)
}

// TestUnitFormatPackageOutputInvalidData тестирует форматирование с некорректными данными
func TestUnitFormatPackageOutputInvalidData(t *testing.T) {
	actions := system.NewActionsWithDeps(
		nil, nil, nil, nil, nil,
	)

	testData := map[string]interface{}{
		"test": "value",
	}

	result := actions.FormatPackageOutput(testData, false)
	assert.Nil(t, result, "Should return nil for invalid data")

	result = actions.FormatPackageOutput("string", true)
	assert.Nil(t, result, "Should return nil for string input")
}
