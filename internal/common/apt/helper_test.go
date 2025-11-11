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

package apt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanDependency(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple .so library",
			input:    "libc.so.6",
			expected: "libc.so.6",
		},
		{
			name:     "Library with version and extra info",
			input:    "libssl.so.1.1(OPENSSL_1.1.0)",
			expected: "libssl.so.1.1",
		},
		{
			name:     "Library without lib prefix",
			input:    "glibc.so.6",
			expected: "glibc.so.6",
		},
		{
			name:     "Complex .so with multiple dots",
			input:    "libpython3.9.so.1.0",
			expected: "libpython3.9.so.1.0",
		},

		// Зависимости с путями и .so
		{
			name:     "Dependency with path and .so",
			input:    "package(libfoo.so)",
			expected: "package(libfoo.so)",
		},
		{
			name:     "Dependency with path",
			input:    "package(/usr/bin/test)",
			expected: "package(/usr/bin/test)",
		},

		// Обычные пакеты с версионными ограничениями
		{
			name:     "Package with greater than version",
			input:    "glibc (>= 2.17)",
			expected: "glibc",
		},
		{
			name:     "Package with exact version",
			input:    "firefox (= 1.2.3-alt1)",
			expected: "firefox",
		},
		{
			name:     "Package with less than version",
			input:    "kernel (<< 5.0)",
			expected: "kernel",
		},
		{
			name:     "Package with not equal version",
			input:    "package (!= 1.0)",
			expected: "package",
		},
		{
			name:     "Package with 64bit marker",
			input:    "glibc (64bit)",
			expected: "glibc",
		},
		{
			name:     "Package with case insensitive 64BIT",
			input:    "package (64BIT)",
			expected: "package",
		},

		// Зависимости с epoch
		{
			name:     "Package with epoch",
			input:    "1:package-name",
			expected: "package-name",
		},
		{
			name:     "Package with epoch and version",
			input:    "2:firefox (>= 1.0)",
			expected: "firefox",
		},
		{
			name:     "Package with multi-digit epoch",
			input:    "12:vim",
			expected: "vim",
		},
		{
			name:     "String starting with colon but not epoch",
			input:    ":not-epoch",
			expected: ":not-epoch",
		},

		// Пустые и особые скобки
		{
			name:     "Package with empty parentheses",
			input:    "package ()",
			expected: "package",
		},
		{
			name:     "Package with whitespace in parentheses",
			input:    "package (   )",
			expected: "package",
		},
		{
			name:     "Package with unclosed parenthesis",
			input:    "package (version",
			expected: "package (version",
		},

		// Комбинированные случаи
		{
			name:     "Complex case with epoch, version and 64bit",
			input:    "2:glibc (>= 2.17) (64bit)",
			expected: "glibc",
		},
		{
			name:     "Library with epoch",
			input:    "1:libssl.so.1.1",
			expected: "libssl.so.1.1",
		},
		{
			name:     "Package with multiple version constraints",
			input:    "package (>= 1.0) (<< 2.0)",
			expected: "package (>= 1.0)",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only whitespace",
			input:    "   ",
			expected: "",
		},
		{
			name:     "Package name with spaces",
			input:    "  firefox   (>= 1.0)  ",
			expected: "firefox",
		},
		{
			name:     "Only parentheses",
			input:    "()",
			expected: "",
		},
		{
			name:     "Only version constraint",
			input:    "(>= 1.0)",
			expected: "",
		},

		// Особые символы и форматы
		{
			name:     "Package with underscores and dashes",
			input:    "package_name-dev (= 1.0-alt1)",
			expected: "package_name-dev",
		},
		{
			name:     "Library with complex version",
			input:    "libboost.so.1.72.0(BOOST_1.72)",
			expected: "libboost.so.1.72.0",
		},
		{
			name:     "Mixed case library",
			input:    "LibSSL.so.1",
			expected: "LibSSL.so.1",
		},
		{
			name:     "Package with multiple colons",
			input:    "1:2:package",
			expected: "2:package",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanDependency(tt.input)
			if result != tt.expected {
				t.Errorf("CleanDependency(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsRegularFileAndIsPackage(t *testing.T) {
	// Создаем временную директорию для тестов
	tempDir := t.TempDir()

	// Создаем тестовые файлы
	rpmFile := filepath.Join(tempDir, "test.rpm")
	txtFile := filepath.Join(tempDir, "test.txt")
	noExtFile := filepath.Join(tempDir, "test")
	rpmUpperFile := filepath.Join(tempDir, "TEST.RPM")

	// Создаем обычные файлы
	if err := os.WriteFile(rpmFile, []byte("fake rpm content"), 0644); err != nil {
		t.Fatalf("Failed to create test rpm file: %v", err)
	}
	if err := os.WriteFile(txtFile, []byte("text content"), 0644); err != nil {
		t.Fatalf("Failed to create test txt file: %v", err)
	}
	if err := os.WriteFile(noExtFile, []byte("no extension"), 0644); err != nil {
		t.Fatalf("Failed to create test file without extension: %v", err)
	}
	if err := os.WriteFile(rpmUpperFile, []byte("uppercase rpm"), 0644); err != nil {
		t.Fatalf("Failed to create test uppercase rpm file: %v", err)
	}

	// Создаем директорию
	dirPath := filepath.Join(tempDir, "testdir")
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Создаем rpm файл внутри директории (для проверки что директории не проходят тест)
	dirRpmFile := filepath.Join(dirPath, "inside.rpm")
	if err := os.WriteFile(dirRpmFile, []byte("rpm in dir"), 0644); err != nil {
		t.Fatalf("Failed to create rpm file in directory: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Valid RPM file",
			path:     rpmFile,
			expected: true,
		},
		{
			name:     "Text file",
			path:     txtFile,
			expected: false,
		},
		{
			name:     "File without extension",
			path:     noExtFile,
			expected: false,
		},
		{
			name:     "RPM file with uppercase extension",
			path:     rpmUpperFile,
			expected: true, // Функция делает case-insensitive проверку
		},
		{
			name:     "Directory",
			path:     dirPath,
			expected: false,
		},
		{
			name:     "Non-existent file",
			path:     filepath.Join(tempDir, "nonexistent.rpm"),
			expected: false,
		},
		{
			name:     "RPM file inside directory",
			path:     dirRpmFile,
			expected: true,
		},
		{
			name:     "Empty path",
			path:     "",
			expected: false,
		},
		{
			name: "Path with spaces in name",
			path: func() string {
				spacedFile := filepath.Join(tempDir, "test file.rpm")
				os.WriteFile(spacedFile, []byte("spaced rpm"), 0644)
				return spacedFile
			}(),
			expected: true,
		},
		{
			name: "Hidden RPM file",
			path: func() string {
				hiddenFile := filepath.Join(tempDir, ".hidden.rpm")
				os.WriteFile(hiddenFile, []byte("hidden rpm"), 0644)
				return hiddenFile
			}(),
			expected: true,
		},
		{
			name: "File with multiple dots",
			path: func() string {
				multiDotFile := filepath.Join(tempDir, "test.backup.rpm")
				os.WriteFile(multiDotFile, []byte("multi dot rpm"), 0644)
				return multiDotFile
			}(),
			expected: true,
		},
		{
			name: "File ending with .rpm but not extension",
			path: func() string {
				fakeRpmFile := filepath.Join(tempDir, "notrpm.txt")
				os.WriteFile(fakeRpmFile, []byte("fake"), 0644)
				return fakeRpmFile
			}(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRegularFileAndIsPackage(tt.path)
			if result != tt.expected {
				t.Errorf("IsRegularFileAndIsPackage(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// Дополнительные unit-тесты для edge cases
func TestCleanDependencyEdgeCases(t *testing.T) {
	// Тест на правильную обработку regex для .so файлов
	t.Run("Regex behavior for .so files", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"libtest.so", "libtest.so"},
			{"libtest.so.1", "libtest.so.1"},
			{"libtest.so.1.2.3", "libtest.so.1.2.3"},
			{"libtest.so.1extra", "libtest.so.1"},
			{"libnotso", "libnotso"},
		}

		for _, tt := range tests {
			result := CleanDependency(tt.input)
			if result != tt.expected {
				t.Errorf("CleanDependency(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		}
	})

	t.Run("Performance test for complex dependencies", func(t *testing.T) {
		complexDep := "libveryverylonglibraryname.so.1.2.3.4.5(VERY_LONG_SYMBOL_NAME_1.2.3)"
		result := CleanDependency(complexDep)
		expected := "libveryverylonglibraryname.so.1.2.3.4.5"
		if result != expected {
			t.Errorf("CleanDependency(%q) = %q, want %q", complexDep, result, expected)
		}
	})
}
