package system

import (
	_package "apm/internal/common/apt/package"
	"fmt"
	"testing"
)

func TestActions_FormatPackageOutput_SinglePackage(t *testing.T) {
	actions := &Actions{}

	pkg := _package.Package{
		Name:        "test-package",
		Version:     "1.0.0",
		Installed:   true,
		Maintainer:  "Test Maintainer",
		Description: "Test package description",
		Size:        1024,
	}

	// Тестируем полный формат
	fullResult := actions.FormatPackageOutput(pkg, true)
	fullPkg, ok := fullResult.(_package.Package)
	if !ok {
		t.Error("FormatPackageOutput should return Package for full format")
	}

	if fullPkg.Name != "test-package" {
		t.Errorf("Expected name 'test-package', got %s", fullPkg.Name)
	}

	if fullPkg.Description != "Test package description" {
		t.Errorf("Expected description 'Test package description', got %s", fullPkg.Description)
	}

	// Тестируем краткий формат
	shortResult := actions.FormatPackageOutput(pkg, false)
	shortPkg, ok := shortResult.(ShortPackageResponse)
	if !ok {
		t.Error("FormatPackageOutput should return ShortPackageResponse for short format")
	}

	if shortPkg.Name != "test-package" {
		t.Errorf("Expected name 'test-package', got %s", shortPkg.Name)
	}

	if shortPkg.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", shortPkg.Version)
	}

	if shortPkg.Installed != true {
		t.Errorf("Expected installed true, got %t", shortPkg.Installed)
	}

	if shortPkg.Maintainer != "Test Maintainer" {
		t.Errorf("Expected maintainer 'Test Maintainer', got %s", shortPkg.Maintainer)
	}
}

func TestActions_FormatPackageOutput_PackageSlice(t *testing.T) {
	actions := &Actions{}

	packages := []_package.Package{
		{
			Name:        "package1",
			Version:     "1.0.0",
			Installed:   true,
			Maintainer:  "Maintainer1",
			Description: "Package 1 description",
		},
		{
			Name:        "package2",
			Version:     "2.0.0",
			Installed:   false,
			Maintainer:  "Maintainer2",
			Description: "Package 2 description",
		},
	}

	// Тестируем полный формат
	fullResult := actions.FormatPackageOutput(packages, true)
	fullPkgs, ok := fullResult.([]_package.Package)
	if !ok {
		t.Error("FormatPackageOutput should return []Package for full format")
	}

	if len(fullPkgs) != 2 {
		t.Errorf("Expected 2 packages, got %d", len(fullPkgs))
	}

	if fullPkgs[0].Description != "Package 1 description" {
		t.Error("Full format should preserve all fields")
	}

	// Тестируем краткий формат
	shortResult := actions.FormatPackageOutput(packages, false)
	shortPkgs, ok := shortResult.([]ShortPackageResponse)
	if !ok {
		t.Error("FormatPackageOutput should return []ShortPackageResponse for short format")
	}

	if len(shortPkgs) != 2 {
		t.Errorf("Expected 2 packages, got %d", len(shortPkgs))
	}

	if shortPkgs[0].Name != "package1" {
		t.Errorf("Expected name 'package1', got %s", shortPkgs[0].Name)
	}

	if shortPkgs[1].Installed != false {
		t.Errorf("Expected installed false for package2, got %t", shortPkgs[1].Installed)
	}
}

func TestActions_FormatPackageOutput_UnsupportedType(t *testing.T) {
	actions := &Actions{}

	result := actions.FormatPackageOutput("invalid-type", true)
	if result != nil {
		t.Error("FormatPackageOutput should return nil for unsupported types")
	}

	result = actions.FormatPackageOutput(123, false)
	if result != nil {
		t.Error("FormatPackageOutput should return nil for unsupported types")
	}
}

func TestActions_FormatPackageOutput_EmptySlice(t *testing.T) {
	actions := &Actions{}

	var packages []_package.Package

	// Тестируем полный формат
	fullResult := actions.FormatPackageOutput(packages, true)
	fullPkgs, ok := fullResult.([]_package.Package)
	if !ok {
		t.Error("FormatPackageOutput should return []Package for full format")
	}

	if len(fullPkgs) != 0 {
		t.Errorf("Expected 0 packages, got %d", len(fullPkgs))
	}

	// Тестируем краткий формат
	shortResult := actions.FormatPackageOutput(packages, false)
	shortPkgs, ok := shortResult.([]ShortPackageResponse)
	if !ok {
		t.Error("FormatPackageOutput should return []ShortPackageResponse for short format")
	}

	if len(shortPkgs) != 0 {
		t.Errorf("Expected 0 packages, got %d", len(shortPkgs))
	}
}

func TestShortPackageResponse_Structure(t *testing.T) {
	response := ShortPackageResponse{
		Name:       "test-package",
		Installed:  true,
		Version:    "1.0.0",
		Maintainer: "Test Maintainer",
	}

	if response.Name != "test-package" {
		t.Errorf("Expected name 'test-package', got %s", response.Name)
	}

	if !response.Installed {
		t.Error("Expected installed true")
	}

	if response.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", response.Version)
	}

	if response.Maintainer != "Test Maintainer" {
		t.Errorf("Expected maintainer 'Test Maintainer', got %s", response.Maintainer)
	}
}

func TestActions_FormatPackageOutput_EdgeCases(t *testing.T) {
	actions := &Actions{}

	result := actions.FormatPackageOutput(nil, true)
	if result != nil {
		t.Error("FormatPackageOutput should return nil for nil input")
	}

	// Тестируем пакет с пустыми полями
	emptyPkg := _package.Package{}
	result = actions.FormatPackageOutput(emptyPkg, false)
	shortPkg, ok := result.(ShortPackageResponse)
	if !ok {
		t.Error("Should return ShortPackageResponse for empty package")
		return
	}

	if shortPkg.Name != "" {
		t.Errorf("Expected empty name, got '%s'", shortPkg.Name)
	}

	if shortPkg.Installed != false {
		t.Error("Expected installed false for empty package")
	}
}

func TestActions_FormatPackageOutput_LargeSlice(t *testing.T) {
	actions := &Actions{}

	// Создаем большой срез пакетов
	packages := make([]_package.Package, 1000)
	for i := 0; i < 1000; i++ {
		packages[i] = _package.Package{
			Name:      fmt.Sprintf("package-%d", i),
			Version:   "1.0.0",
			Installed: i%2 == 0,
		}
	}

	// Тестируем краткий формат
	result := actions.FormatPackageOutput(packages, false)
	shortPkgs, ok := result.([]ShortPackageResponse)
	if !ok {
		t.Error("Should return []ShortPackageResponse for large slice")
		return
	}

	if len(shortPkgs) != 1000 {
		t.Errorf("Expected 1000 packages, got %d", len(shortPkgs))
	}

	// Проверяем первый и последний элемент
	if shortPkgs[0].Name != "package-0" {
		t.Errorf("Expected first package 'package-0', got %s", shortPkgs[0].Name)
	}

	if shortPkgs[999].Name != "package-999" {
		t.Errorf("Expected last package 'package-999', got %s", shortPkgs[999].Name)
	}

	// Проверяем паттерн installed
	if !shortPkgs[0].Installed || shortPkgs[1].Installed {
		t.Error("Installed pattern should be even=true, odd=false")
	}
}
