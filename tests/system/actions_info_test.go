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

// actions_info_sqlmock_test.go
package system

import (
	"apm/cmd/system"
	"apm/cmd/system/service"
	"context"
	"fmt"
	"regexp"
	"testing"

	_package "apm/cmd/system/package"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

// TestInfo_Success_sqlmock проверяет случай, когда пакет найден в базе
func TestInfo_Success_sqlmock(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	packageDBSvc, _ := _package.NewPackageDBService(db)

	expectedQuery := "SELECT COUNT(*) FROM host_image_packages"
	mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Создаем фиктивный пакет
	fakePkg := _package.Package{
		Name:             "vim",
		Section:          "editors",
		InstalledSize:    1024,
		Maintainer:       "vim maintainer",
		Version:          "8.2",
		VersionInstalled: "8.2",
		Depends:          []string{"lib1", "lib2"},
		Provides:         []string{"vim"},
		Size:             2048,
		Filename:         "/usr/bin/vim",
		Description:      "Vi Improved",
		Changelog:        "changelog",
		Installed:        true,
	}

	// Ожидаем выполнения SQL-запроса для получения информации о пакете.
	query := regexp.QuoteMeta(fmt.Sprintf(`
		SELECT name, section, installed_size, maintainer, version, versionInstalled, depends, provides, size, filename, description, changelog, installed 
		FROM %s 
		WHERE name = ?`, "host_image_packages"))
	rows := sqlmock.NewRows([]string{
		"name", "section", "installed_size", "maintainer", "version",
		"versionInstalled", "depends", "provides", "size", "filename", "description", "changelog", "installed",
	}).AddRow(
		fakePkg.Name,
		fakePkg.Section,
		fakePkg.InstalledSize,
		fakePkg.Maintainer,
		fakePkg.Version,
		fakePkg.VersionInstalled,
		"lib1,lib2",
		"vim",
		fakePkg.Size,
		fakePkg.Filename,
		fakePkg.Description,
		fakePkg.Changelog,
		1, // installed
	)
	mock.ExpectQuery(query).WithArgs("vim").WillReturnRows(rows)

	actions := system.NewActionsWithDeps(
		packageDBSvc,
		_package.NewActions(packageDBSvc, &_package.StplrService{}),
		&service.HostImageService{},  // фиктивный объект
		&service.HostDBService{},     // фиктивный объект
		&service.HostConfigService{}, // фиктивный объект
	)

	ctx := context.Background()
	resp, err := actions.Info(ctx, "vim", true)
	assert.NoError(t, err)
	assert.False(t, resp.Error)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Найден пакет", data["message"])

	pkgInfo, ok := data["packageInfo"].(_package.Package)
	assert.True(t, ok)
	assert.Equal(t, fakePkg.Name, pkgInfo.Name)
	assert.Equal(t, fakePkg.Version, pkgInfo.Version)
	assert.Equal(t, fakePkg.Installed, pkgInfo.Installed)
	assert.Equal(t, fakePkg.Description, pkgInfo.Description)

	// Проверяем, что все ожидания sqlmock выполнены.
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}
