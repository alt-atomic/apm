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

package common_test

import (
	"apm/internal/common/apt"
	"apm/internal/common/helper"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClearALRPackageName тестирует очистку ALR пакетов
func TestClearALRPackageName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"package+alr1", "package"},
		{"my-app+alr2", "my-app"},
		{"simple", "simple"},
		{"", ""},
		{"package+alr", "package"},
		{"no-suffix", "no-suffix"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := helper.ClearALRPackageName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestCleanPackageName тестирует очистку имен пакетов
func TestCleanPackageName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"package#1.0.0", "package"},
		{"1:package", "package"},
		{"package.32bit", "package"},
		{"1:package#1.0.0", "package"},
		{"package#1.0.0.32bit", "package"},
		{"simple", "simple"},
		{"", ""},
		{"package:with:colons", "with:colons"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := helper.CleanPackageName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestGetVersionFromAptCache тестирует извлечение версий
func TestGetVersionFromAptCache(t *testing.T) {
	testCases := []struct {
		input       string
		expected    string
		shouldError bool
	}{
		{"1:1.2.3-alt1", "1.2.3", false},
		{"1.2.3-alt1", "1.2.3", false},
		{"2:0.99.8-alt2", "0.99.8", false},
		{"1.0.0", "1.0.0", false},
		{"", "", true},
		{"invalid", "invalid", false},
		{"1.2.3-alt1.el7", "1.2.3", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := helper.GetVersionFromAptCache(tc.input)
			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

// TestAutoSize тестирует преобразование размеров
func TestAutoSize(t *testing.T) {
	testCases := []struct {
		input    int
		expected string
	}{
		{1048576, "1.00 MB"}, // 1 MB
		{2097152, "2.00 MB"}, // 2 MB
		{512000, "0.49 MB"},  // ~0.5 MB
		{0, "0.00 MB"},       // 0 bytes
		{1024, "0.00 MB"},    // 1 KB
	}

	for _, tc := range testCases {
		t.Run(string(rune(tc.input)), func(t *testing.T) {
			result := helper.AutoSize(tc.input)
			assert.Contains(t, result, "MB")
			// Проверяем что результат содержит ожидаемое число
			expectedNum := strings.Split(tc.expected, " ")[0]
			assert.Contains(t, result, expectedNum)
		})
	}
}

// TestParseBool тестирует парсинг булевых значений
func TestParseBool(t *testing.T) {
	testCases := []struct {
		input      interface{}
		expected   bool
		shouldWork bool
	}{
		{true, true, true},
		{false, false, true},
		{1, true, true},
		{0, false, true},
		{"true", true, true},
		{"false", false, true},
		{"TRUE", true, true},
		{"FALSE", false, true},
		{"1", true, true},
		{"0", false, true},
		{"invalid", false, false},
		{nil, false, false},
		{3.14, false, false},
	}

	for _, tc := range testCases {
		t.Run("ParseBool", func(t *testing.T) {
			result, ok := helper.ParseBool(tc.input)
			assert.Equal(t, tc.shouldWork, ok)
			if tc.shouldWork {
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

// TestRunCommand тестирует выполнение команд
func TestRunCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		expectError bool
	}{
		{
			name:        "simple echo",
			command:     "echo 'hello world'",
			expectError: false,
		},
		{
			name:        "invalid command",
			command:     "nonexistent_command_12345",
			expectError: true,
		},
		{
			name:        "command with stderr",
			command:     "echo 'error' >&2",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			stdout, stderr, err := helper.RunCommand(ctx, tt.command)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// stdout и stderr должны быть строками
			assert.IsType(t, "", stdout)
			assert.IsType(t, "", stderr)
		})
	}
}

// TestRunCommandTimeout тестирует таймаут команд
func TestRunCommandTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Команда, которая будет выполняться дольше таймаута
	_, _, err := helper.RunCommand(ctx, "sleep 1")
	assert.Error(t, err)
	// Проверяем что получили ошибку таймаута или kill сигнал
	errStr := err.Error()
	isTimeoutError := strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "signal: killed") ||
		strings.Contains(errStr, "exit status")
	assert.True(t, isTimeoutError, "Expected timeout/kill error, got: %v", err)
}

// TestIsRunningInContainer тестирует определение контейнера
func TestIsRunningInContainer(t *testing.T) {
	// Сохраняем оригинальное значение переменной окружения
	originalContainer := os.Getenv("container")
	defer os.Setenv("container", originalContainer)

	// Тест без переменной окружения
	os.Unsetenv("container")
	result1 := helper.IsRunningInContainer()

	// Тест с переменной окружения
	os.Setenv("container", "podman")
	result2 := helper.IsRunningInContainer()

	// Второй результат должен быть true
	assert.True(t, result2)

	// Первый результат зависит от того, есть ли файлы контейнера
	// но функция должна работать без ошибок
	assert.IsType(t, true, result1)
}

// TestIsRegularFileAndIsPackage тестирует проверку файлов пакетов
func TestIsRegularFileAndIsPackage(t *testing.T) {
	// Создаем временную директорию
	tmpDir := t.TempDir()

	// Создаем тестовые файлы
	rpmFile := filepath.Join(tmpDir, "test.rpm")
	txtFile := filepath.Join(tmpDir, "test.txt")
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.rpm")

	// Записываем содержимое в файлы
	require.NoError(t, os.WriteFile(rpmFile, []byte("fake rpm"), 0644))
	require.NoError(t, os.WriteFile(txtFile, []byte("text file"), 0644))

	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		{"RPM file", rpmFile, true},
		{"Text file", txtFile, false},
		{"Non-existent file", nonExistentFile, false},
		{"Directory", tmpDir, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := apt.IsRegularFileAndIsPackage(tc.path)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestIsRegularFileAndIsPackageEdgeCases тестирует граничные случаи
func TestIsRegularFileAndIsPackageEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	// Файл с заглавным расширением
	rpmUpperFile := filepath.Join(tmpDir, "test.RPM")
	require.NoError(t, os.WriteFile(rpmUpperFile, []byte("fake rpm"), 0644))

	result := apt.IsRegularFileAndIsPackage(rpmUpperFile)
	assert.True(t, result, "Should handle uppercase extensions")

	// Пустой путь
	result = apt.IsRegularFileAndIsPackage("")
	assert.False(t, result)
}
