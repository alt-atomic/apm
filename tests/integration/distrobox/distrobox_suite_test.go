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
// along with this program.  If not, see <http://www.gnu.org/licenses/>. //go:build distrobox

package distrobox_test

import (
	"apm/internal/distrobox"
	"apm/tests/integration/common"
	"context"
	"fmt"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// DistroboxTestSuite управляет одним контейнером для всех тестов
type DistroboxTestSuite struct {
	suite.Suite
	actions       *distrobox.Actions
	ctx           context.Context
	containerName string
	image         string
}

// SetupSuite создает контейнер один раз для всех тестов
func (s *DistroboxTestSuite) SetupSuite() {
	if syscall.Geteuid() == 0 {
		s.T().Skip("Distrobox tests should be run without root privileges")
	}

	appConfig, ctx := common.GetTestAppConfig(s.T())

	s.actions = distrobox.NewActions(appConfig)
	s.ctx = ctx
	s.containerName = "apm-test-suite"
	s.image = "registry.altlinux.org/sisyphus/base:latest"

	s.T().Logf("Removing existing container %s if it exists...", s.containerName)
	_, err := s.actions.ContainerRemove(s.ctx, s.containerName)
	if err != nil {
		s.T().Logf("Container %s didn't exist or couldn't be removed: %v", s.containerName, err)
	} else {
		s.T().Logf("Existing container %s removed", s.containerName)
	}

	s.T().Logf("Creating new container %s...", s.containerName)
	resp, err := s.actions.ContainerAdd(s.ctx, s.image, s.containerName, "", "")
	if err != nil {
		s.T().Skipf("Failed to create test container: %v", err)
	}

	assert.NotNil(s.T(), resp)
	assert.False(s.T(), resp.Error, "Container creation should succeed")
	s.T().Logf("Test container created: %s", s.containerName)
}

// TearDownSuite удаляет контейнер после всех тестов
func (s *DistroboxTestSuite) TearDownSuite() {
	if s.actions != nil {
		resp, err := s.actions.ContainerRemove(s.ctx, s.containerName)
		if err != nil {
			s.T().Logf("Failed to cleanup test container: %v", err)
		} else {
			s.T().Logf("Test container cleaned up: %+v", resp.Data)
		}
	}
}

// TestActionsCreation тестирует базовое создание actions
func (s *DistroboxTestSuite) TestActionsCreation() {
	assert.NotNil(s.T(), s.actions)
}

// TestContainerList тестирует получение списка контейнеров
func (s *DistroboxTestSuite) TestContainerList() {
	resp, err := s.actions.ContainerList(s.ctx)

	if err != nil {
		s.T().Logf("ContainerList failed: %v", err)
		return
	}

	assert.NotNil(s.T(), resp)
	assert.False(s.T(), resp.Error)

	responseStr := fmt.Sprintf("%+v", resp.Data)
	s.T().Logf("ContainerList response: %s", responseStr)
	found := strings.Contains(responseStr, s.containerName)

	if found {
		s.T().Logf("Successfully found test container '%s' in response", s.containerName)
	} else {
		s.T().Logf("Test container '%s' not found in response", s.containerName)
	}

	assert.True(s.T(), found, "Test container should be found in container list")
}

// TestPackageInstall тестирует установку пакета
func (s *DistroboxTestSuite) TestPackageInstall() {
	resp, err := s.actions.Install(s.ctx, s.containerName, "hello", false)

	if err != nil {
		s.T().Logf("Install failed: %v", err)
		assert.NotContains(s.T(), err.Error(), "Elevated rights are not allowed")
		return
	}

	assert.NotNil(s.T(), resp)
	assert.False(s.T(), resp.Error)
	s.T().Logf("Package installed successfully")
}

// TestPackageSearch тестирует поиск пакетов
func (s *DistroboxTestSuite) TestPackageSearch() {
	resp, err := s.actions.Search(s.ctx, s.containerName, "vim")

	if err != nil {
		s.T().Logf("Search failed: %v", err)
		return
	}

	assert.NotNil(s.T(), resp)
	assert.False(s.T(), resp.Error)
	s.T().Logf("Search completed successfully")
}

// TestPackageInfo тестирует получение информации о пакете
func (s *DistroboxTestSuite) TestPackageInfo() {
	resp, err := s.actions.Info(s.ctx, s.containerName, "hello")

	if err != nil {
		s.T().Logf("Info failed: %v", err)
		return
	}

	assert.NotNil(s.T(), resp)
	assert.False(s.T(), resp.Error)
	s.T().Logf("Package info retrieved successfully")
}

// TestPackageList тестирует получение списка установленных пакетов
func (s *DistroboxTestSuite) TestPackageList() {
	params := distrobox.ListParams{
		Container: s.containerName,
		Limit:     10,
		Offset:    0,
	}
	resp, err := s.actions.List(s.ctx, params)

	if err != nil {
		s.T().Logf("List failed: %v", err)
		return
	}

	assert.NotNil(s.T(), resp)
	assert.False(s.T(), resp.Error)
	s.T().Logf("Package list retrieved successfully")
}

// TestPackageUpdate тестирует обновление пакетов
func (s *DistroboxTestSuite) TestPackageUpdate() {
	resp, err := s.actions.Update(s.ctx, s.containerName)

	if err != nil {
		s.T().Logf("Update failed: %v", err)
		return
	}

	assert.NotNil(s.T(), resp)
	assert.False(s.T(), resp.Error)
	s.T().Logf("Update completed successfully")
}

// TestPackageRemove тестирует удаление пакета
func (s *DistroboxTestSuite) TestPackageRemove() {
	resp, err := s.actions.Remove(s.ctx, s.containerName, "hello", false)

	if err != nil {
		s.T().Logf("Remove failed: %v", err)
		return
	}

	assert.NotNil(s.T(), resp)
	assert.False(s.T(), resp.Error)
	s.T().Logf("Package removed successfully")
}

// Запуск набора тестов
func TestDistroboxSuite(t *testing.T) {
	suite.Run(t, new(DistroboxTestSuite))
}
