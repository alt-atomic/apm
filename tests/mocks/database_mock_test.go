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
	"database/sql"
	"testing"

	_package "apm/internal/system/package"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockPackageDBService представляет мок для тестирования
type MockPackageDBService struct {
	db   *gorm.DB
	mock sqlmock.Sqlmock
}

// NewMockPackageDBService создает новый мок сервис
func NewMockPackageDBService() (*MockPackageDBService, error) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		return nil, err
	}

	gormDB, err := gorm.Open(sqlite.Dialector{
		Conn: sqlDB,
	}, &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return &MockPackageDBService{
		db:   gormDB,
		mock: mock,
	}, nil
}

// Close закрывает соединение
func (m *MockPackageDBService) Close() error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// TestMockPackageDBService_GetPackageByName тестирует получение пакета с помощью мока
func TestMockPackageDBService_GetPackageByName(t *testing.T) {
	mockSvc, err := NewMockPackageDBService()
	require.NoError(t, err)
	defer mockSvc.Close()

	// Настраиваем ожидания мока
	rows := sqlmock.NewRows([]string{"name", "version", "description", "installed"}).
		AddRow("vim", "8.2.0", "Vi Improved text editor", true)

	mockSvc.mock.ExpectQuery("SELECT (.+) FROM (.+)packages(.+) WHERE (.+)name(.+)").
		WithArgs("vim").
		WillReturnRows(rows)

	// Создаем PackageDBService с моком (используем конструктор)
	sqlDB, err := mockSvc.db.DB()
	require.NoError(t, err)
	service, err := _package.NewPackageDBService(sqlDB)
	require.NoError(t, err)

	ctx := context.Background()
	pkg, err := service.GetPackageByName(ctx, "vim")

	assert.NoError(t, err)
	assert.Equal(t, "vim", pkg.Name)
	assert.Equal(t, "8.2.0", pkg.Version)
	assert.True(t, pkg.Installed)

	// Проверяем что все ожидания выполнены
	assert.NoError(t, mockSvc.mock.ExpectationsWereMet())
}

// TestMockPackageDBService_GetPackageByName_NotFound тестирует случай когда пакет не найден
func TestMockPackageDBService_GetPackageByName_NotFound(t *testing.T) {
	mockSvc, err := NewMockPackageDBService()
	require.NoError(t, err)
	defer mockSvc.Close()

	// Настраиваем мок для возврата ошибки "record not found"
	mockSvc.mock.ExpectQuery("SELECT (.+) FROM (.+)packages(.+) WHERE (.+)name(.+)").
		WithArgs("nonexistent").
		WillReturnError(gorm.ErrRecordNotFound)

	sqlDB, err := mockSvc.db.DB()
	require.NoError(t, err)
	service, err := _package.NewPackageDBService(sqlDB)
	require.NoError(t, err)

	ctx := context.Background()
	pkg, err := service.GetPackageByName(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "record not found")
	assert.Equal(t, "", pkg.Name)

	assert.NoError(t, mockSvc.mock.ExpectationsWereMet())
}

// TestMockPackageDBService_PackageDatabaseExist тестирует проверку существования БД
func TestMockPackageDBService_PackageDatabaseExist(t *testing.T) {
	mockSvc, err := NewMockPackageDBService()
	require.NoError(t, err)
	defer mockSvc.Close()

	// Тест с пустой БД
	t.Run("Empty database", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"count"}).AddRow(0)
		mockSvc.mock.ExpectQuery("SELECT count(.+) FROM (.+)packages(.+)").
			WillReturnRows(rows)

		sqlDB, err := mockSvc.db.DB()
		require.NoError(t, err)
		service, err := _package.NewPackageDBService(sqlDB)
		require.NoError(t, err)

		ctx := context.Background()
		err = service.PackageDatabaseExist(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Package database is empty")
	})

	// Тест с непустой БД
	t.Run("Non-empty database", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"count"}).AddRow(100)
		mockSvc.mock.ExpectQuery("SELECT count(.+) FROM (.+)packages(.+)").
			WillReturnRows(rows)

		sqlDB, err := mockSvc.db.DB()
		require.NoError(t, err)
		service, err := _package.NewPackageDBService(sqlDB)
		require.NoError(t, err)

		ctx := context.Background()
		err = service.PackageDatabaseExist(ctx)

		assert.NoError(t, err)
	})

	assert.NoError(t, mockSvc.mock.ExpectationsWereMet())
}

// TestMockPackageDBService_QueryHostImagePackages тестирует запрос пакетов
func TestMockPackageDBService_QueryHostImagePackages(t *testing.T) {
	mockSvc, err := NewMockPackageDBService()
	require.NoError(t, err)
	defer mockSvc.Close()

	// Настраиваем ожидания для count запроса
	countRows := sqlmock.NewRows([]string{"count"}).AddRow(2)
	mockSvc.mock.ExpectQuery("SELECT count(.+) FROM (.+)packages(.+)").
		WillReturnRows(countRows)

	// Настраиваем ожидания для основного запроса
	rows := sqlmock.NewRows([]string{"name", "version", "description", "installed", "section"}).
		AddRow("vim", "8.2.0", "Vi Improved", true, "editors").
		AddRow("nano", "5.0", "Simple text editor", false, "editors")

	mockSvc.mock.ExpectQuery("SELECT (.+) FROM (.+)packages(.+)").
		WillReturnRows(rows)

	sqlDB, err := mockSvc.db.DB()
	require.NoError(t, err)
	service, err := _package.NewPackageDBService(sqlDB)
	require.NoError(t, err)

	ctx := context.Background()
	packages, err := service.QueryHostImagePackages(ctx, map[string]interface{}{}, "", "", 10, 0)

	assert.NoError(t, err)
	assert.Len(t, packages, 2)
	assert.Equal(t, "vim", packages[0].Name)
	assert.Equal(t, "nano", packages[1].Name)

	assert.NoError(t, mockSvc.mock.ExpectationsWereMet())
}

// TestMockPackageDBService_SyncPackageInstallationInfo тестирует синхронизацию
func TestMockPackageDBService_SyncPackageInstallationInfo(t *testing.T) {
	mockSvc, err := NewMockPackageDBService()
	require.NoError(t, err)
	defer mockSvc.Close()

	// Настраиваем ожидания для обновления статуса установки
	mockSvc.mock.ExpectBegin()
	mockSvc.mock.ExpectExec("UPDATE (.+)packages(.+) SET (.+)installed(.+)").
		WillReturnResult(sqlmock.NewResult(1, 2))
	mockSvc.mock.ExpectCommit()

	sqlDB, err := mockSvc.db.DB()
	require.NoError(t, err)
	service, err := _package.NewPackageDBService(sqlDB)
	require.NoError(t, err)

	ctx := context.Background()
	installedPackages := map[string]string{
		"vim":  "8.2.0",
		"nano": "5.0",
	}

	err = service.SyncPackageInstallationInfo(ctx, installedPackages)

	assert.NoError(t, err)
	assert.NoError(t, mockSvc.mock.ExpectationsWereMet())
}

// TestMockPackageDBService_DatabaseError тестирует обработку ошибок БД
func TestMockPackageDBService_DatabaseError(t *testing.T) {
	mockSvc, err := NewMockPackageDBService()
	require.NoError(t, err)
	defer mockSvc.Close()

	// Симулируем ошибку БД
	mockSvc.mock.ExpectQuery("SELECT (.+) FROM (.+)packages(.+)").
		WillReturnError(sql.ErrConnDone)

	sqlDB, err := mockSvc.db.DB()
	require.NoError(t, err)
	service, err := _package.NewPackageDBService(sqlDB)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = service.GetPackageByName(ctx, "test")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sql: connection is already closed")

	assert.NoError(t, mockSvc.mock.ExpectationsWereMet())
}

// BenchmarkMockPackageOperations тестирует производительность операций с пакетами
func BenchmarkMockPackageOperations(b *testing.B) {
	mockSvc, err := NewMockPackageDBService()
	require.NoError(b, err)
	defer mockSvc.Close()

	sqlDB, err := mockSvc.db.DB()
	require.NoError(b, err)
	service, err := _package.NewPackageDBService(sqlDB)
	require.NoError(b, err)

	ctx := context.Background()

	b.Run("GetPackageByName", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Настраиваем мок для каждой итерации
			rows := sqlmock.NewRows([]string{"name", "version", "description", "installed"}).
				AddRow("test-pkg", "1.0.0", "Test package", true)
			mockSvc.mock.ExpectQuery("SELECT (.+) FROM (.+)packages(.+)").
				WillReturnRows(rows)

			_, _ = service.GetPackageByName(ctx, "test-pkg")
		}
	})
}