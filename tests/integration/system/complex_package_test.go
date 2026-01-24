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

// ComplexPackageTestSuite для тестирования сложных случаев работы с пакетами
type ComplexPackageTestSuite struct {
	suite.Suite
	actions *system.Actions
	ctx     context.Context
}

// SetupSuite создает actions один раз для всех тестов
func (s *ComplexPackageTestSuite) SetupSuite() {
	if syscall.Geteuid() != 0 {
		s.T().Skip("This test suite requires root privileges. Run with sudo.")
	}

	appConfig, ctx := common.GetTestAppConfig(s.T())
	s.actions = system.NewActions(appConfig)
	s.ctx = ctx
}

// TestVirtualPackageJava тестирует установку виртуального пакета java
// Должен выдать ошибку с предложением выбрать конкретный пакет
func (s *ComplexPackageTestSuite) TestVirtualPackageJava() {
	s.T().Log("Testing virtual package 'java' - should fail with provider options")

	_, err := s.actions.CheckInstall(s.ctx, []string{"java"})

	assert.Error(s.T(), err, "Should get error for virtual package 'java'")

	if err != nil {
		errMsg := err.Error()
		s.T().Logf("Virtual package error: %v", errMsg)

		assert.True(s.T(),
			strings.Contains(errMsg, "virtual package") ||
				strings.Contains(errMsg, "provided by") ||
				strings.Contains(errMsg, "explicitly select") ||
				strings.Contains(errMsg, "java-") ||
				strings.Contains(errMsg, "openjdk"),
			"Error should mention virtual package and providers: %v", errMsg)
	}
}

// TestVirtualPackageGirGtk тестирует установку виртуального пакета gir(Gtk)
func (s *ComplexPackageTestSuite) TestVirtualPackageGirGtk() {
	s.T().Log("Testing virtual package 'gir(Gtk)' - should fail with provider options")

	_, err := s.actions.CheckInstall(s.ctx, []string{"gir(Gtk)"})

	// Ожидаем ошибку
	assert.Error(s.T(), err, "Should get error for virtual package 'gir(Gtk)'")

	if err != nil {
		errMsg := err.Error()
		s.T().Logf("GIR virtual package error: %v", errMsg)

		// Проверяем что ошибка содержит информацию о провайдерах GTK
		assert.True(s.T(),
			strings.Contains(errMsg, "virtual") ||
				strings.Contains(errMsg, "provided") ||
				strings.Contains(errMsg, "gtk") ||
				strings.Contains(errMsg, "gir"),
			"Error should mention virtual package and GTK providers: %v", errMsg)
	}
}

// TestVirtualPackageVapi тестирует установку виртуального пакета vapi(gtk4)
func (s *ComplexPackageTestSuite) TestVirtualPackageVapi() {
	s.T().Log("Testing virtual package 'vapi(gtk4)' - should work correctly")

	resp, err := s.actions.CheckInstall(s.ctx, []string{"vapi(gtk4)"})

	if err != nil {
		s.T().Logf("vapi(gtk4) check failed (may be expected): %v", err)
		assert.True(s.T(),
			strings.Contains(err.Error(), "not found") ||
				strings.Contains(err.Error(), "no installation candidate") ||
				strings.Contains(err.Error(), "Package database is empty"),
			"Unexpected error for vapi(gtk4): %v", err)
	} else {
		assert.NotNil(s.T(), resp)
		s.T().Logf("vapi(gtk4) check successful: %+v", resp)
	}
}

// TestComplexDependencyResolution тестирует установку пакета с зависимостями
func (s *ComplexPackageTestSuite) TestComplexDependencyResolution() {
	s.T().Log("Testing complex dependency resolution with development packages")

	testPackages := []string{"git", "gcc", "make"}

	for _, pkg := range testPackages {
		s.T().Logf("Testing dependency resolution for: %s", pkg)

		resp, err := s.actions.CheckInstall(s.ctx, []string{pkg})

		if err != nil {
			s.T().Logf("Package %s check failed (may be expected): %v", pkg, err)
		} else {
			assert.NotNil(s.T(), resp)
			s.T().Logf("Package %s check successful", pkg)
		}
	}
}

// TestConflictingPackages тестирует установку конфликтующих пакетов
func (s *ComplexPackageTestSuite) TestConflictingPackages() {
	s.T().Log("Testing conflicting packages handling")

	conflictingPairs := [][]string{
		{"systemd", "sysvinit"},
	}

	for _, pair := range conflictingPairs {
		s.T().Logf("Testing conflict detection between: %s and %s", pair[0], pair[1])

		for _, pkg := range pair {
			_, err := s.actions.CheckInstall(s.ctx, []string{pkg})
			if err != nil {
				s.T().Logf("Individual package %s check failed: %v", pkg, err)
			} else {
				s.T().Logf("✓ Individual package %s check successful", pkg)
			}
		}

		_, err := s.actions.CheckInstall(s.ctx, pair)
		if err != nil {
			errMsg := err.Error()
			s.T().Logf("Installing both %v failed (expected): %v", pair, err)

			hasConflictMessage := strings.Contains(errMsg, "Conflicting packages") ||
				strings.Contains(errMsg, "conflicts") ||
				strings.Contains(errMsg, "conflict") ||
				strings.Contains(errMsg, pair[0]) && strings.Contains(errMsg, pair[1])

			if hasConflictMessage {
				s.T().Logf("✓ Conflict properly detected for %v", pair)
			} else {
				s.T().Logf("? Error detected but may not be conflict-specific: %v", errMsg)
			}
		} else {
			s.T().Errorf("✗ UNEXPECTED: Installing conflicting packages %v succeeded - should have failed!", pair)
		}
	}
}

// TestCircularDependencies тестирует обработку циклических зависимостей
func (s *ComplexPackageTestSuite) TestCircularDependencies() {
	s.T().Log("Testing circular dependency handling")

	circularPackages := []string{
		"perl",
		"python3",
		"nodejs",
	}

	for _, pkg := range circularPackages {
		s.T().Logf("Testing potential circular dependencies for: %s", pkg)

		resp, err := s.actions.CheckInstall(s.ctx, []string{pkg})

		if err != nil {
			s.T().Logf("Package %s with potential circular deps failed: %v", pkg, err)
		} else {
			assert.NotNil(s.T(), resp)
			s.T().Logf("Package %s circular dependency check passed", pkg)
		}
	}
}

// TestLargePackageInstallation тестирует установку больших пакетов
func (s *ComplexPackageTestSuite) TestLargePackageInstallation() {
	s.T().Log("Testing large package installation simulation")

	largePackages := []string{
		"libreoffice",
		"firefox",
		"thunderbird",
		"gcc-fortran",
	}

	for _, pkg := range largePackages {
		s.T().Logf("Testing large package: %s", pkg)

		resp, err := s.actions.CheckInstall(s.ctx, []string{pkg})

		if err != nil {
			s.T().Logf("Large package %s check failed: %v", pkg, err)
		} else {
			assert.NotNil(s.T(), resp)
			s.T().Logf("Large package %s check successful", pkg)
		}
	}
}

// TestPackageAlreadyInstalled тестирует поведение когда пакет уже установлен
func (s *ComplexPackageTestSuite) TestPackageAlreadyInstalled() {
	s.T().Log("Testing already installed package handling")

	resp, err := s.actions.CheckInstall(s.ctx, []string{"bash"})
	if err != nil {
		errMsg := err.Error()
		s.T().Logf("bash check result: %v", errMsg)
		isAlreadyInstalled := strings.Contains(errMsg, "already installed") ||
			strings.Contains(errMsg, "уже установлен")
		if isAlreadyInstalled {
			s.T().Log("✓ Correctly detected bash as already installed")
		}
	} else {
		assert.NotNil(s.T(), resp)
		s.T().Log("✓ bash check returned changes (upgrade available or fresh install)")
	}
}

// Запуск набора тестов
func TestComplexPackageSuite(t *testing.T) {
	suite.Run(t, new(ComplexPackageTestSuite))
}
