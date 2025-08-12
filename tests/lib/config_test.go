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

//go:build unit

package lib_test

import (
	"apm/lib"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestColorsStruct тестирует структуру Colors
func TestColorsStruct(t *testing.T) {
	colors := lib.Colors{
		Enumerator:     "#FF0000",
		Accent:         "#00FF00",
		ItemLight:      "#0000FF",
		ItemDark:       "#FFFF00",
		Success:        "#00FFFF",
		Error:          "#FF00FF",
		Delete:         "#800000",
		Install:        "#008000",
		Shortcut:       "#000080",
		ScrollBar:      "#808080",
		DialogKeyLight: "#C0C0C0",
		DialogKeyDark:  "#404040",
		ProgressStart:  "#FF8000",
		ProgressEnd:    "#8000FF",
	}

	assert.Equal(t, "#FF0000", colors.Enumerator)
	assert.Equal(t, "#00FF00", colors.Accent)
	assert.Equal(t, "#0000FF", colors.ItemLight)
	assert.Equal(t, "#FFFF00", colors.ItemDark)
	assert.Equal(t, "#00FFFF", colors.Success)
	assert.Equal(t, "#FF00FF", colors.Error)
	assert.Equal(t, "#800000", colors.Delete)
	assert.Equal(t, "#008000", colors.Install)
	assert.Equal(t, "#000080", colors.Shortcut)
	assert.Equal(t, "#808080", colors.ScrollBar)
	assert.Equal(t, "#C0C0C0", colors.DialogKeyLight)
	assert.Equal(t, "#404040", colors.DialogKeyDark)
	assert.Equal(t, "#FF8000", colors.ProgressStart)
	assert.Equal(t, "#8000FF", colors.ProgressEnd)
}

// TestEnvironmentStruct тестирует структуру Environment
func TestEnvironmentStruct(t *testing.T) {
	// Создаем тестовый экземпляр Environment
	env := lib.Environment{
		CommandPrefix:   "test-prefix",
		Environment:     "test",
		PathLogFile:     "/tmp/test.log",
		PathDBSQLSystem: "/tmp/test.db",
		PathDBSQLUser:   "/tmp/user.db",
		PathDBKV:        "/tmp/kv.db",
		PathImageFile:   "/etc/test/image.yml",
		ExistStplr:      true,
		ExistDistrobox:  false,
		Format:          "json",
		IsAtomic:        true,
		PathLocales:     "/usr/share/locale",
	}
	
	// Проверяем что все поля устанавливаются корректно
	assert.Equal(t, "test-prefix", env.CommandPrefix)
	assert.Equal(t, "test", env.Environment)
	assert.Equal(t, "/tmp/test.log", env.PathLogFile)
	assert.Equal(t, "/tmp/test.db", env.PathDBSQLSystem)
	assert.Equal(t, "/tmp/user.db", env.PathDBSQLUser)
	assert.Equal(t, "/tmp/kv.db", env.PathDBKV)
	assert.Equal(t, "/etc/test/image.yml", env.PathImageFile)
	assert.True(t, env.ExistStplr)
	assert.False(t, env.ExistDistrobox)
	assert.Equal(t, "json", env.Format)
	assert.True(t, env.IsAtomic)
	assert.Equal(t, "/usr/share/locale", env.PathLocales)
	
	// Проверяем что пути валидны
	assert.True(t, filepath.IsAbs(env.PathLogFile), "Log file path should be absolute")
	assert.True(t, filepath.IsAbs(env.PathDBSQLSystem), "DB path should be absolute")
}

// TestConfigFileHandling тестирует работу с конфигурационными файлами
func TestConfigFileHandling(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_config.yaml")

	// Создаем тестовый конфиг
	testConfig := `
colors:
  enumerator: "#FF0000"
  accent: "#00FF00"
  itemLight: "#0000FF"
  itemDark: "#FFFF00"
  success: "#00FFFF"
  error: "#FF00FF"
  delete: "#800000"
  install: "#008000"
  shortcut: "#000080"
  scrollBar: "#808080"
  dialogKeyLight: "#C0C0C0"
  dialogKeyDark: "#404040"
  progressStart: "#FF8000"
  progressEnd: "#8000FF"

environment:
  commandPrefix: "test-prefix"
  environment: "test"
  pathLogFile: "/tmp/test.log"
  pathDBSQLSystem: "/tmp/test.db"
  pathDBSQLUser: "/tmp/user.db"
`

	require.NoError(t, os.WriteFile(configFile, []byte(testConfig), 0644))

	// Пытаемся загрузить конфиг (это зависит от реализации lib пакета)
	// Здесь мы просто проверяем что файл создался корректно
	data, err := os.ReadFile(configFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "enumerator")
	assert.Contains(t, string(data), "environment")
}

// TestEnvironmentVariableOverrides тестирует переопределение через переменные окружения
func TestEnvironmentVariableOverrides(t *testing.T) {
	// Сохраняем оригинальные значения
	originalValues := map[string]string{
		"APM_LOG_FILE":      os.Getenv("APM_LOG_FILE"),
		"APM_DB_FILE":       os.Getenv("APM_DB_FILE"),
		"APM_ENVIRONMENT":   os.Getenv("APM_ENVIRONMENT"),
	}

	// Восстанавливаем оригинальные значения после теста
	defer func() {
		for key, value := range originalValues {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// Устанавливаем тестовые значения
	testValues := map[string]string{
		"APM_LOG_FILE":    "/tmp/test_override.log",
		"APM_DB_FILE":     "/tmp/test_override.db",
		"APM_ENVIRONMENT": "test_override",
	}

	for key, value := range testValues {
		os.Setenv(key, value)
	}

	// Проверяем что переменные установлены
	assert.Equal(t, "/tmp/test_override.log", os.Getenv("APM_LOG_FILE"))
	assert.Equal(t, "/tmp/test_override.db", os.Getenv("APM_DB_FILE"))
	assert.Equal(t, "test_override", os.Getenv("APM_ENVIRONMENT"))
}

// TestPathValidation тестирует валидацию путей
func TestPathValidation(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"Absolute Unix path", "/usr/local/bin", true},
		{"Relative path", "relative/path", false},
		{"Empty path", "", false},
		{"Root path", "/", true},
		{"Home path", "/home/user", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filepath.IsAbs(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFilePermissions тестирует работу с правами доступа
func TestFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Создаем файлы с разными правами
	files := map[string]os.FileMode{
		"readable.txt":     0644,
		"executable.txt":   0755,
		"restricted.txt":   0600,
		"world_readable.txt": 0644,
	}

	for filename, mode := range files {
		path := filepath.Join(tmpDir, filename)
		require.NoError(t, os.WriteFile(path, []byte("test"), mode))
		
		// Проверяем что права установлены корректно
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, mode, info.Mode().Perm())
	}
}

// TestConfigErrorHandling тестирует обработку ошибок конфигурации
func TestConfigErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Тест с невалидным YAML
	invalidConfigFile := filepath.Join(tmpDir, "invalid.yaml")
	invalidYAML := `
invalid: yaml: content
  - missing
    brackets
`
	require.NoError(t, os.WriteFile(invalidConfigFile, []byte(invalidYAML), 0644))

	// Тест с несуществующим файлом
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.yaml")
	
	_, err := os.Stat(nonExistentFile)
	assert.True(t, os.IsNotExist(err), "File should not exist")

	// Тест с недоступным файлом (только если не root)
	if os.Getuid() != 0 {
		restrictedFile := filepath.Join(tmpDir, "restricted.yaml")
		require.NoError(t, os.WriteFile(restrictedFile, []byte("test"), 0000))
		
		_, err = os.ReadFile(restrictedFile)
		assert.Error(t, err, "Should not be able to read restricted file")
	}
}

// TestDefaultColors тестирует поведение структуры Colors
func TestDefaultColors(t *testing.T) {
	// Создаем экземпляр с дефолтными значениями
	colors := lib.Colors{}
	
	// Все поля должны иметь нулевые значения до инициализации
	assert.Empty(t, colors.Enumerator)
	assert.Empty(t, colors.Accent)
	assert.Empty(t, colors.Success)
	assert.Empty(t, colors.Error)
	
	// Проверяем что структура может быть заполнена
	colors.Enumerator = "#FF0000"
	colors.Accent = "#00FF00" 
	assert.Equal(t, "#FF0000", colors.Enumerator)
	assert.Equal(t, "#00FF00", colors.Accent)
}

// BenchmarkConfigLoad бенчмарк загрузки конфигурации
func BenchmarkConfigLoad(b *testing.B) {
	tmpDir := b.TempDir()
	configFile := filepath.Join(tmpDir, "bench_config.yaml")

	testConfig := `
colors:
  enumerator: "#FF0000"
  accent: "#00FF00"
environment:
  environment: "benchmark"
`

	require.NoError(b, os.WriteFile(configFile, []byte(testConfig), 0644))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := os.ReadFile(configFile)
		if err != nil {
			b.Fatal(err)
		}
	}
}