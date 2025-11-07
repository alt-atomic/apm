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

package build

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"apm/internal/common/app"
	"apm/internal/common/build"
	"apm/tests/integration/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// BuildTestSuite для интеграционных тестов сборки
type BuildTestSuite struct {
	suite.Suite
	configService *build.ConfigService
	hostConfig    *build.HostConfigService
	ctx           context.Context
	appConfig     *app.Config
	testDir       string
	resourcesDir  string
	testImageFile string
}

// SetupSuite создает окружение
func (s *BuildTestSuite) SetupSuite() {
	if syscall.Geteuid() != 0 {
		s.T().Skip("This test suite requires root privileges. Run with sudo.")
	}

	var err error
	s.appConfig, s.ctx = common.GetTestAppConfig(s.T())

	// Создаем временную директорию для тестов
	s.testDir = filepath.Join(os.TempDir(), "apm-build-test")
	err = os.MkdirAll(s.testDir, 0755)
	assert.NoError(s.T(), err, "Failed to create test directory")

	s.resourcesDir = filepath.Join(s.testDir, "resources")
	err = os.MkdirAll(s.resourcesDir, 0755)
	assert.NoError(s.T(), err, "Failed to create resources directory")

	s.testImageFile = filepath.Join(s.testDir, "image.yaml")

	// Инициализируем ConfigService для тестов
	s.configService = build.NewConfigService(
		s.appConfig,
		nil,
		nil,
		nil,
		nil,
	)

	s.T().Log("Build test suite initialized")
}

// TearDownSuite очищает окружение после всех тестов
func (s *BuildTestSuite) TearDownSuite() {
	if s.testDir != "" {
		os.RemoveAll(s.testDir)
		s.T().Logf("Cleaned up test directory: %s", s.testDir)
	}
}

// SetupTest запускается перед каждым тестом
func (s *BuildTestSuite) SetupTest() {
	// Очищаем resources директорию перед каждым тестом
	os.RemoveAll(s.resourcesDir)
	os.MkdirAll(s.resourcesDir, 0755)
}

// TestCopyModule тестирует модуль copy
func (s *BuildTestSuite) TestCopyModule() {
	s.T().Log("Testing copy module")

	// Создаем исходный файл
	sourceFile := filepath.Join(s.resourcesDir, "source.txt")
	sourceContent := "test content for copy"
	err := os.WriteFile(sourceFile, []byte(sourceContent), 0644)
	assert.NoError(s.T(), err)

	destFile := filepath.Join(s.testDir, "dest.txt")

	// Создаем конфигурацию с copy модулем
	yamlConfig := `
image: "alt:sisyphus"
name: "test-copy"
repos:
  branch: "sisyphus"
modules:
  - name: "Test copy"
    type: copy
    body:
      target: "` + sourceFile + `"
      destination: "` + destFile + `"
      replace: false
`
	err = os.WriteFile(s.testImageFile, []byte(yamlConfig), 0644)
	assert.NoError(s.T(), err)

	// Загружаем и выполняем конфигурацию
	cfg, err := build.ReadAndParseYamlFile(s.testImageFile)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(cfg.Modules))
	assert.Equal(s.T(), build.TypeCopy, cfg.Modules[0].Type)

	// Выполняем модуль через реальный ConfigService
	module := cfg.Modules[0]
	err = s.configService.ExecuteModule(s.ctx, module)
	assert.NoError(s.T(), err)

	// Проверяем что файл скопирован
	content, err := os.ReadFile(destFile)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), sourceContent, string(content))

	s.T().Log("Copy module test passed")
}

// TestMoveModule тестирует модуль move
func (s *BuildTestSuite) TestMoveModule() {
	s.T().Log("Testing move module")

	sourceFile := filepath.Join(s.resourcesDir, "source_move.txt")
	sourceContent := "test content for move"
	err := os.WriteFile(sourceFile, []byte(sourceContent), 0644)
	assert.NoError(s.T(), err)

	destFile := filepath.Join(s.testDir, "dest_move.txt")

	yamlConfig := `
image: "alt:sisyphus"
name: "test-move"
repos:
  branch: "sisyphus"
modules:
  - name: "Test move"
    type: move
    body:
      target: "` + sourceFile + `"
      destination: "` + destFile + `"
      replace: false
      create-link: false
`
	err = os.WriteFile(s.testImageFile, []byte(yamlConfig), 0644)
	assert.NoError(s.T(), err)

	cfg, err := build.ReadAndParseYamlFile(s.testImageFile)
	assert.NoError(s.T(), err)

	module := cfg.Modules[0]
	err = s.configService.ExecuteModule(s.ctx, module)
	assert.NoError(s.T(), err)

	// Проверяем что файл перемещен
	content, err := os.ReadFile(destFile)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), sourceContent, string(content))

	// Проверяем что исходный файл удален
	_, err = os.Stat(sourceFile)
	assert.True(s.T(), os.IsNotExist(err), "Source file should be removed after move")

	s.T().Log("Move module test passed")
}

// TestRemoveModule тестирует модуль remove
func (s *BuildTestSuite) TestRemoveModule() {
	s.T().Log("Testing remove module")

	// Создаем файл для удаления
	fileToRemove := filepath.Join(s.testDir, "to_remove.txt")
	err := os.WriteFile(fileToRemove, []byte("remove me"), 0644)
	assert.NoError(s.T(), err)

	yamlConfig := `
image: "alt:sisyphus"
name: "test-remove"
repos:
  branch: "sisyphus"
modules:
  - name: "Test remove"
    type: remove
    body:
      target: "` + fileToRemove + `"
`
	err = os.WriteFile(s.testImageFile, []byte(yamlConfig), 0644)
	assert.NoError(s.T(), err)

	cfg, err := build.ReadAndParseYamlFile(s.testImageFile)
	assert.NoError(s.T(), err)

	module := cfg.Modules[0]
	err = s.configService.ExecuteModule(s.ctx, module)
	assert.NoError(s.T(), err)

	// Проверяем что файл удален
	_, err = os.Stat(fileToRemove)
	assert.True(s.T(), os.IsNotExist(err), "File should be removed")

	s.T().Log("Remove module test passed")
}

// TestMkdirModule тестирует модуль mkdir
func (s *BuildTestSuite) TestMkdirModule() {
	s.T().Log("Testing mkdir module")

	dirToCreate := filepath.Join(s.testDir, "new_dir", "nested", "deep")

	yamlConfig := `
image: "alt:sisyphus"
name: "test-mkdir"
repos:
  branch: "sisyphus"
modules:
  - name: "Test mkdir"
    type: mkdir
    body:
      target: "` + dirToCreate + `"
`
	err := os.WriteFile(s.testImageFile, []byte(yamlConfig), 0644)
	assert.NoError(s.T(), err)

	cfg, err := build.ReadAndParseYamlFile(s.testImageFile)
	assert.NoError(s.T(), err)

	module := cfg.Modules[0]
	err = s.configService.ExecuteModule(s.ctx, module)
	assert.NoError(s.T(), err)

	// Проверяем что директория создана
	stat, err := os.Stat(dirToCreate)
	assert.NoError(s.T(), err)
	assert.True(s.T(), stat.IsDir(), "Should create a directory")

	s.T().Log("Mkdir module test passed")
}

// TestLinkModule тестирует модуль link
func (s *BuildTestSuite) TestLinkModule() {
	s.T().Log("Testing link module")

	targetFile := filepath.Join(s.resourcesDir, "link_target.txt")
	err := os.WriteFile(targetFile, []byte("link target"), 0644)
	assert.NoError(s.T(), err)

	linkFile := filepath.Join(s.testDir, "symlink.txt")

	yamlConfig := `
image: "alt:sisyphus"
name: "test-link"
repos:
  branch: "sisyphus"
modules:
  - name: "Test link"
    type: link
    body:
      target: "` + targetFile + `"
      destination: "` + linkFile + `"
`
	err = os.WriteFile(s.testImageFile, []byte(yamlConfig), 0644)
	assert.NoError(s.T(), err)

	cfg, err := build.ReadAndParseYamlFile(s.testImageFile)
	assert.NoError(s.T(), err)

	module := cfg.Modules[0]
	err = s.configService.ExecuteModule(s.ctx, module)
	assert.NoError(s.T(), err)

	// Проверяем что ссылка создана
	stat, err := os.Lstat(linkFile)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), os.ModeSymlink, stat.Mode()&os.ModeSymlink, "Should be a symlink")

	// Проверяем что можем прочитать содержимое через ссылку
	content, err := os.ReadFile(linkFile)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "link target", string(content))

	s.T().Log("Link module test passed")
}

// TestMergeModule тестирует модуль merge
func (s *BuildTestSuite) TestMergeModule() {
	s.T().Log("Testing merge module")

	// Создаем файл назначения
	destFile := filepath.Join(s.testDir, "merge_dest.txt")
	err := os.WriteFile(destFile, []byte("existing content\n"), 0644)
	assert.NoError(s.T(), err)

	// Создаем файл-источник
	sourceFile := filepath.Join(s.resourcesDir, "merge_source.txt")
	err = os.WriteFile(sourceFile, []byte("new content\n"), 0644)
	assert.NoError(s.T(), err)

	yamlConfig := `
image: "alt:sisyphus"
name: "test-merge"
repos:
  branch: "sisyphus"
modules:
  - name: "Test merge"
    type: merge
    body:
      target: "` + sourceFile + `"
      destination: "` + destFile + `"
`
	err = os.WriteFile(s.testImageFile, []byte(yamlConfig), 0644)
	assert.NoError(s.T(), err)

	cfg, err := build.ReadAndParseYamlFile(s.testImageFile)
	assert.NoError(s.T(), err)

	module := cfg.Modules[0]
	err = s.configService.ExecuteModule(s.ctx, module)
	assert.NoError(s.T(), err)

	// Проверяем что содержимое объединено
	content, err := os.ReadFile(destFile)
	assert.NoError(s.T(), err)
	assert.Contains(s.T(), string(content), "existing content")
	assert.Contains(s.T(), string(content), "new content")

	s.T().Log("Merge module test passed")
}

// TestComplexMultiModuleBuild тестирует комплексную сборку с несколькими модулями
func (s *BuildTestSuite) TestComplexMultiModuleBuild() {
	s.T().Log("Testing complex multi-module build")

	// Создаем файлы для теста
	sourceFile1 := filepath.Join(s.resourcesDir, "file1.txt")
	sourceFile2 := filepath.Join(s.resourcesDir, "file2.txt")
	err := os.WriteFile(sourceFile1, []byte("content 1"), 0644)
	assert.NoError(s.T(), err)
	err = os.WriteFile(sourceFile2, []byte("content 2"), 0644)
	assert.NoError(s.T(), err)

	destDir := filepath.Join(s.testDir, "output")
	destFile1 := filepath.Join(destDir, "file1.txt")
	destFile2 := filepath.Join(destDir, "file2.txt")

	yamlConfig := `
image: "alt:sisyphus"
name: "test-multi-module"
repos:
  branch: "sisyphus"
modules:
  - name: "Create directory"
    type: mkdir
    body:
      target: "` + destDir + `"

  - name: "Copy file 1"
    type: copy
    body:
      target: "` + sourceFile1 + `"
      destination: "` + destFile1 + `"

  - name: "Copy file 2"
    type: copy
    body:
      target: "` + sourceFile2 + `"
      destination: "` + destFile2 + `"
`
	err = os.WriteFile(s.testImageFile, []byte(yamlConfig), 0644)
	assert.NoError(s.T(), err)

	cfg, err := build.ReadAndParseYamlFile(s.testImageFile)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 3, len(cfg.Modules), "Should have 3 modules")

	// Выполняем все модули последовательно через реальный ConfigService
	for i, module := range cfg.Modules {
		s.T().Logf("Executing module %d: %s", i+1, module.Name)
		err = s.configService.ExecuteModule(s.ctx, module)
		assert.NoError(s.T(), err, "Module %d failed", i+1)
	}

	// Проверяем результаты
	_, err = os.Stat(destDir)
	assert.NoError(s.T(), err, "Directory should exist")

	content1, err := os.ReadFile(destFile1)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "content 1", string(content1))

	content2, err := os.ReadFile(destFile2)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "content 2", string(content2))

	s.T().Log("Complex multi-module build test passed")
}

// TestConfigValidation тестирует валидацию конфигурации
func (s *BuildTestSuite) TestConfigValidation() {
	s.T().Log("Testing config validation")

	tests := []struct {
		name      string
		yaml      string
		shouldErr bool
		errMsg    string
	}{
		{
			name: "Empty image",
			yaml: `
name: "test"
modules: []
`,
			shouldErr: true,
			errMsg:    "Image can not be empty",
		},
		{
			name: "Empty name",
			yaml: `
image: "alt:sisyphus"
modules: []
`,
			shouldErr: true,
			errMsg:    "Name can not be empty",
		},
		{
			name: "Valid minimal config",
			yaml: `
image: "alt:sisyphus"
name: "test"
repos:
  branch: "sisyphus"
modules: []
`,
			shouldErr: false,
		},
		{
			name: "Copy without target",
			yaml: `
image: "alt:sisyphus"
name: "test"
repos:
  branch: "sisyphus"
modules:
  - type: copy
    body:
      destination: "/tmp/dest"
`,
			shouldErr: true,
			errMsg:    "target",
		},
		{
			name: "Copy without destination",
			yaml: `
image: "alt:sisyphus"
name: "test"
repos:
  branch: "sisyphus"
modules:
  - type: copy
    body:
      target: "/tmp/src"
`,
			shouldErr: true,
			errMsg:    "destination",
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(s.testDir, "test_validation.yaml")
			err := os.WriteFile(testFile, []byte(tt.yaml), 0644)
			assert.NoError(t, err)

			_, err = build.ReadAndParseYamlFile(testFile)
			if tt.shouldErr {
				assert.Error(t, err, "Should fail validation")
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err, "Should pass validation")
			}
		})
	}

	s.T().Log("Config validation test passed")
}

// TestRunner запускает тест-сьют
func TestBuildIntegration(t *testing.T) {
	suite.Run(t, new(BuildTestSuite))
}
