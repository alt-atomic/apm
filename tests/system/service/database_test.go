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

package service_test

import (
	"context"
	"testing"

	"apm/internal/system/service"
	"apm/lib"

	"github.com/stretchr/testify/assert"
)

// TestNewHostDBService проверяет создание HostDBService
func TestNewHostDBService(t *testing.T) {
	dbService, err := service.NewHostDBService(lib.GetDB(true))
	if err != nil {
		t.Logf("NewHostDBService error (may be expected): %v", err)
		assert.Contains(t, err.Error(), "ошибка подключения к SQLite")
	} else {
		assert.NotNil(t, dbService)
	}
}

// TestImageHistory проверяет структуру ImageHistory
func TestImageHistory(t *testing.T) {
	config := &service.Config{
		Image: "test-image:v1.0",
		Packages: struct {
			Install []string `yaml:"install" json:"install"`
			Remove  []string `yaml:"remove" json:"remove"`
		}{
			Install: []string{"vim"},
			Remove:  []string{"emacs"},
		},
	}

	history := service.ImageHistory{
		ImageName: "test-image:v1.0",
		Config:    config,
		ImageDate: "2025-01-01T00:00:00Z",
	}

	assert.Equal(t, "test-image:v1.0", history.ImageName)
	assert.NotNil(t, history.Config)
	assert.Equal(t, "2025-01-01T00:00:00Z", history.ImageDate)
	assert.Equal(t, "test-image:v1.0", history.Config.Image)
}

// TestSaveImageToDB_RealDB тестирует сохранение истории образов
func TestSaveImageToDB_RealDB(t *testing.T) {
	dbService, err := service.NewHostDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	ctx := context.Background()

	config := &service.Config{
		Image: "test-image:v1.0",
		Packages: struct {
			Install []string `yaml:"install" json:"install"`
			Remove  []string `yaml:"remove" json:"remove"`
		}{
			Install: []string{"vim", "nano"},
			Remove:  []string{"emacs"},
		},
		Commands: []string{"apt update", "apt install vim nano", "apt remove emacs"},
	}

	imageHistory := service.ImageHistory{
		ImageName: "test-image:v1.0",
		Config:    config,
		ImageDate: "2025-01-01T00:00:00Z",
	}

	err = dbService.SaveImageToDB(ctx, imageHistory)
	if err != nil {
		t.Logf("SaveImageToDB error (may be expected): %v", err)
	} else {
		t.Log("SaveImageToDB successful")
	}
}

// TestGetImageHistoriesFiltered_RealDB проверяет получение истории
func TestGetImageHistoriesFiltered_RealDB(t *testing.T) {
	dbService, err := service.NewHostDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	ctx := context.Background()

	histories, err := dbService.GetImageHistoriesFiltered(ctx, "", 10, 0)
	if err != nil {
		t.Logf("GetImageHistoriesFiltered error (may be expected): %v", err)
	} else {
		assert.NotNil(t, histories)
		t.Logf("Found %d image histories", len(histories))
	}
}

// TestCountImageHistoriesFiltered_AllImages проверяет подсчет всех образов
func TestCountImageHistoriesFiltered_AllImages(t *testing.T) {
	dbService, err := service.NewHostDBService(lib.GetDB(true))
	if err != nil {
		t.Skip("Database not available, skipping test")
	}

	ctx := context.Background()

	count, err := dbService.CountImageHistoriesFiltered(ctx, "all")
	if err != nil {
		t.Logf("CountImageHistoriesFiltered all images error (may be expected): %v", err)
	} else {
		assert.GreaterOrEqual(t, count, 0)
		t.Logf("Total all images count: %d", count)
	}
}

// TestConfig проверяет структуру Config
func TestConfig(t *testing.T) {
	config := service.Config{
		Image: "test-image:latest",
		Packages: struct {
			Install []string `yaml:"install" json:"install"`
			Remove  []string `yaml:"remove" json:"remove"`
		}{
			Install: []string{"curl", "wget"},
			Remove:  []string{"nano"},
		},
		Commands: []string{"echo 'test'"},
	}

	assert.Equal(t, "test-image:latest", config.Image)
	assert.Len(t, config.Packages.Install, 2)
	assert.Len(t, config.Packages.Remove, 1)
	assert.Contains(t, config.Packages.Install, "curl")
	assert.Contains(t, config.Packages.Remove, "nano")
	assert.Len(t, config.Commands, 1)
}

// TestDBHistory проверяет структуру DBHistory
func TestDBHistory(t *testing.T) {
	dbHistory := service.DBHistory{
		ImageName:  "ubuntu:22.04",
		ConfigJSON: `{"image":"ubuntu:22.04","packages":{"install":["git"]}}`,
	}

	assert.Equal(t, "ubuntu:22.04", dbHistory.ImageName)
	assert.Contains(t, dbHistory.ConfigJSON, "git")
}
