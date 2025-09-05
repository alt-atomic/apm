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

package mocks_test

import (
	"context"
	"errors"
	"testing"

	"apm/internal/common/apt/package"
	"github.com/stretchr/testify/assert"
)

// MockPackageDBService простой мок для тестирования
type MockPackageDBService struct {
	packages map[string]_package.Package
	failNext bool
}

// NewMockPackageDBService создает новый простой мок
func NewMockPackageDBService() *MockPackageDBService {
	return &MockPackageDBService{
		packages: make(map[string]_package.Package),
		failNext: false,
	}
}

// SetPackage добавляет пакет в мок
func (m *MockPackageDBService) SetPackage(name string, pkg _package.Package) {
	m.packages[name] = pkg
}

// SetFailNext заставляет следующий вызов провалиться
func (m *MockPackageDBService) SetFailNext() {
	m.failNext = true
}

// GetPackageByName реализация для мока
func (m *MockPackageDBService) GetPackageByName(ctx context.Context, name string) (_package.Package, error) {
	if m.failNext {
		m.failNext = false
		return _package.Package{}, errors.New("mock database error")
	}

	if pkg, exists := m.packages[name]; exists {
		return pkg, nil
	}

	return _package.Package{}, errors.New("record not found")
}

// PackageDatabaseExist реализация для мока
func (m *MockPackageDBService) PackageDatabaseExist(ctx context.Context) error {
	if m.failNext {
		m.failNext = false
		return errors.New("Package database is empty")
	}

	if len(m.packages) == 0 {
		return errors.New("Package database is empty")
	}

	return nil
}

// QueryHostImagePackages реализация для мока
func (m *MockPackageDBService) QueryHostImagePackages(ctx context.Context,
	filters map[string]interface{}, orderBy, groupBy string, limit, offset int) ([]_package.Package, error) {

	if m.failNext {
		m.failNext = false
		return nil, errors.New("mock database error")
	}

	var result []_package.Package
	for _, pkg := range m.packages {
		result = append(result, pkg)
		if len(result) >= limit && limit > 0 {
			break
		}
	}

	return result, nil
}

// SyncPackageInstallationInfo реализация для мока
func (m *MockPackageDBService) SyncPackageInstallationInfo(ctx context.Context,
	installedPackages map[string]string) error {

	if m.failNext {
		m.failNext = false
		return errors.New("mock database error")
	}

	// Обновляем статус установки в моке
	for name, version := range installedPackages {
		if pkg, exists := m.packages[name]; exists {
			pkg.Installed = true
			pkg.Version = version
			m.packages[name] = pkg
		}
	}

	return nil
}

// TestMockPackageDBService_GetPackageByName тестирует получение пакета
func TestMockPackageDBService_GetPackageByName(t *testing.T) {
	mock := NewMockPackageDBService()

	// Добавляем тестовый пакет
	testPkg := _package.Package{
		Name:        "vim",
		Version:     "8.2.0",
		Description: "Vi Improved text editor",
		Installed:   true,
	}
	mock.SetPackage("vim", testPkg)

	ctx := context.Background()
	pkg, err := mock.GetPackageByName(ctx, "vim")

	assert.NoError(t, err)
	assert.Equal(t, "vim", pkg.Name)
	assert.Equal(t, "8.2.0", pkg.Version)
	assert.True(t, pkg.Installed)
}

// TestMockPackageDBService_GetPackageByName_NotFound тестирует случай когда пакет не найден
func TestMockPackageDBService_GetPackageByName_NotFound(t *testing.T) {
	mock := NewMockPackageDBService()

	ctx := context.Background()
	pkg, err := mock.GetPackageByName(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "record not found")
	assert.Equal(t, "", pkg.Name)
}

// TestMockPackageDBService_PackageDatabaseExist тестирует проверку существования БД
func TestMockPackageDBService_PackageDatabaseExist(t *testing.T) {
	t.Run("Empty database", func(t *testing.T) {
		mock := NewMockPackageDBService()

		ctx := context.Background()
		err := mock.PackageDatabaseExist(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Package database is empty")
	})

	t.Run("Non-empty database", func(t *testing.T) {
		mock := NewMockPackageDBService()

		// Добавляем пакет
		testPkg := _package.Package{Name: "test", Version: "1.0.0"}
		mock.SetPackage("test", testPkg)

		ctx := context.Background()
		err := mock.PackageDatabaseExist(ctx)

		assert.NoError(t, err)
	})
}

// TestMockPackageDBService_QueryHostImagePackages тестирует запрос пакетов
func TestMockPackageDBService_QueryHostImagePackages(t *testing.T) {
	mock := NewMockPackageDBService()

	// Добавляем тестовые пакеты
	pkg1 := _package.Package{Name: "vim", Version: "8.2.0", Description: "Vi Improved", Installed: true, Section: "editors"}
	pkg2 := _package.Package{Name: "nano", Version: "5.0", Description: "Simple text editor", Installed: false, Section: "editors"}

	mock.SetPackage("vim", pkg1)
	mock.SetPackage("nano", pkg2)

	ctx := context.Background()
	packages, err := mock.QueryHostImagePackages(ctx, map[string]interface{}{}, "", "", 10, 0)

	assert.NoError(t, err)
	assert.Len(t, packages, 2)

	// Проверяем что есть оба пакета (порядок может быть любой)
	names := []string{packages[0].Name, packages[1].Name}
	assert.Contains(t, names, "vim")
	assert.Contains(t, names, "nano")
}

// TestMockPackageDBService_SyncPackageInstallationInfo тестирует синхронизацию
func TestMockPackageDBService_SyncPackageInstallationInfo(t *testing.T) {
	mock := NewMockPackageDBService()

	// Добавляем пакеты
	pkg1 := _package.Package{Name: "vim", Version: "8.1.0", Installed: false}
	pkg2 := _package.Package{Name: "nano", Version: "4.0", Installed: false}

	mock.SetPackage("vim", pkg1)
	mock.SetPackage("nano", pkg2)

	ctx := context.Background()
	installedPackages := map[string]string{
		"vim":  "8.2.0",
		"nano": "5.0",
	}

	err := mock.SyncPackageInstallationInfo(ctx, installedPackages)

	assert.NoError(t, err)

	// Проверяем что статус обновился
	updatedVim, _ := mock.GetPackageByName(ctx, "vim")
	updatedNano, _ := mock.GetPackageByName(ctx, "nano")

	assert.True(t, updatedVim.Installed)
	assert.Equal(t, "8.2.0", updatedVim.Version)
	assert.True(t, updatedNano.Installed)
	assert.Equal(t, "5.0", updatedNano.Version)
}

// TestMockPackageDBService_DatabaseError тестирует обработку ошибок БД
func TestMockPackageDBService_DatabaseError(t *testing.T) {
	mock := NewMockPackageDBService()
	mock.SetFailNext()

	ctx := context.Background()
	_, err := mock.GetPackageByName(ctx, "test")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mock database error")
}

// TestMockPackageDBService_Performance тестирует производительность
func TestMockPackageDBService_Performance(t *testing.T) {
	mock := NewMockPackageDBService()

	// Добавляем много пакетов
	for i := 0; i < 1000; i++ {
		pkg := _package.Package{
			Name:    "package" + string(rune('0'+i%10)),
			Version: "1.0.0",
		}
		mock.SetPackage(pkg.Name, pkg)
	}

	ctx := context.Background()

	// Тестируем быстродействие
	for i := 0; i < 100; i++ {
		_, _ = mock.GetPackageByName(ctx, "package0")
	}
}
