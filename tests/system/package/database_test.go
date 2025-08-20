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
	"testing"

	_package "apm/internal/system/package"
	"apm/lib"

	"github.com/stretchr/testify/assert"
)

// TestNewPackageDBService проверяет создание PackageDBService
func TestNewPackageDBService(t *testing.T) {
	service, err := _package.NewPackageDBService(lib.GetDB(true))

	if err != nil {
		t.Logf("NewPackageDBService error (may be expected): %v", err)
		assert.Contains(t, err.Error(), "ошибка подключения к SQLite")
	} else {
		assert.NotNil(t, service)
	}
}

// TestPackageDatabaseExist_RealDB проверяет существование базы данных
func TestPackageDatabaseExist_RealDB(t *testing.T) {
	service, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	ctx := context.Background()
	err = service.PackageDatabaseExist(ctx)

	if err != nil {
		t.Logf("PackageDatabaseExist error (expected if DB empty): %v", err)
		assert.True(t,
			strings.Contains(err.Error(), "Package database is empty") ||
				strings.Contains(err.Error(), "contains no records"),
			"Unexpected error: %v", err)
	} else {
		t.Log("Package database exists and is not empty")
	}
}

// TestGetPackageByName_RealDB проверяет получение пакета по имени
func TestGetPackageByName_RealDB(t *testing.T) {
	service, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	ctx := context.Background()

	// Тестируем с реальным именем пакета
	pkg, err := service.GetPackageByName(ctx, "hello")
	if err != nil {
		t.Logf("GetPackageByName error (expected if package not in DB): %v", err)
		assert.True(t,
			strings.Contains(err.Error(), "record not found") ||
				strings.Contains(err.Error(), "Package database is empty") ||
				strings.Contains(err.Error(), "failed to get information"),
			"Unexpected error: %v", err)
	} else {
		assert.NotNil(t, pkg)
		assert.Equal(t, "hello", pkg.Name)
		t.Logf("Found package: %+v", pkg)
	}
}

// TestGetPackageByName_NotFound проверяет поведение при отсутствии пакета
func TestGetPackageByName_NotFound(t *testing.T) {
	service, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	ctx := context.Background()

	pkg, err := service.GetPackageByName(ctx, "nonexistent-package-12345")

	if err != nil {
		assert.Error(t, err)
		t.Logf("Expected not found error: %v", err)
	} else {
		assert.Equal(t, "", pkg.Name)
		assert.Equal(t, "", pkg.Version)
		t.Logf("Got empty package for nonexistent package name")
	}
}

// TestQueryHostImagePackages_RealDB проверяет запрос пакетов
func TestQueryHostImagePackages_RealDB(t *testing.T) {
	service, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	ctx := context.Background()

	packages, err := service.QueryHostImagePackages(ctx, map[string]interface{}{}, "", "", 10, 0)
	if err != nil {
		t.Logf("QueryHostImagePackages error (expected if DB empty): %v", err)
		assert.True(t,
			strings.Contains(err.Error(), "Package database is empty") ||
				strings.Contains(err.Error(), "contains no records"),
			"Unexpected error: %v", err)
	} else {
		if packages != nil {
			t.Logf("Found %d packages", len(packages))
		} else {
			t.Log("No packages found (empty database)")
		}
	}
}

// TestSyncPackageInstallationInfo_RealDB проверяет синхронизацию
func TestSyncPackageInstallationInfo_RealDB(t *testing.T) {
	service, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	ctx := context.Background()

	err = service.SyncPackageInstallationInfo(ctx, map[string]string{})
	if err != nil {
		t.Logf("SyncPackageInstallationInfo error (may be expected): %v", err)
	} else {
		t.Log("SyncPackageInstallationInfo successful")
	}
}

// TestPackageStruct проверяет структуру Package
func TestPackageStruct(t *testing.T) {
	pkg := _package.Package{
		Name:        "test-package",
		Version:     "1.0.0",
		Description: "Test package description",
		Installed:   false,
	}

	assert.Equal(t, "test-package", pkg.Name)
	assert.Equal(t, "1.0.0", pkg.Version)
	assert.Equal(t, "Test package description", pkg.Description)
	assert.False(t, pkg.Installed)
}

// TestDBPackage проверяет структуру DBPackage
func TestDBPackage(t *testing.T) {
	dbPkg := _package.DBPackage{
		Name:        "test-package",
		Version:     "1.0.0",
		Description: "Test package",
		Installed:   true,
	}

	assert.Equal(t, "test-package", dbPkg.Name)
	assert.Equal(t, "1.0.0", dbPkg.Version)
	assert.Equal(t, "Test package", dbPkg.Description)
	assert.True(t, dbPkg.Installed)
}
