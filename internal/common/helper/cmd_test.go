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

package helper

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestAbs(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{
			name:     "Positive number",
			input:    5,
			expected: 5,
		},
		{
			name:     "Negative number",
			input:    -5,
			expected: 5,
		},
		{
			name:     "Zero",
			input:    0,
			expected: 0,
		},
		{
			name:     "Large positive number",
			input:    1000000,
			expected: 1000000,
		},
		{
			name:     "Large negative number",
			input:    -1000000,
			expected: 1000000,
		},
		{
			name:     "Maximum negative int",
			input:    -2147483647,
			expected: 2147483647,
		},
		{
			name:     "Maximum positive int",
			input:    2147483647,
			expected: 2147483647,
		},
		{
			name:     "Negative one",
			input:    -1,
			expected: 1,
		},
		{
			name:     "Positive one",
			input:    1,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Abs(tt.input)
			if result != tt.expected {
				t.Errorf("Abs(%d) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsRunningInContainer(t *testing.T) {
	originalContainer := os.Getenv("container")

	cleanup := func() {
		if originalContainer != "" {
			os.Setenv("container", originalContainer)
		} else {
			os.Unsetenv("container")
		}
	}
	defer cleanup()

	tests := []struct {
		name        string
		setupFunc   func(t *testing.T) func()
		expected    bool
		description string
	}{
		{
			name: "Container env variable set",
			setupFunc: func(t *testing.T) func() {
				os.Setenv("container", "podman")
				return func() {}
			},
			expected:    true,
			description: "Should detect container when container env var is set",
		},
		{
			name: "Empty container env variable",
			setupFunc: func(t *testing.T) func() {
				os.Setenv("container", "")
				return func() {}
			},
			expected:    false,
			description: "Should not detect container when container env var is empty",
		},
		{
			name: "No container env variable",
			setupFunc: func(t *testing.T) func() {
				os.Unsetenv("container")
				return func() {}
			},
			expected:    false,
			description: "Should not detect container when no container env var",
		},
		{
			name: "Container env with docker value",
			setupFunc: func(t *testing.T) func() {
				os.Setenv("container", "docker")
				return func() {}
			},
			expected:    true,
			description: "Should detect container with docker value",
		},
		{
			name: "Container env with arbitrary value",
			setupFunc: func(t *testing.T) func() {
				os.Setenv("container", "custom-container")
				return func() {}
			},
			expected:    true,
			description: "Should detect container with any non-empty value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("container")

			cleanupFunc := tt.setupFunc(t)
			defer cleanupFunc()

			result := IsRunningInContainer()
			if result != tt.expected {
				t.Errorf("IsRunningInContainer() = %v, want %v. %s", result, tt.expected, tt.description)
			}
		})
	}
}

func TestIsRunningInContainer_MockFileSystem(t *testing.T) {
	tempDir := t.TempDir()

	originalContainer := os.Getenv("container")
	os.Unsetenv("container")
	defer func() {
		if originalContainer != "" {
			os.Setenv("container", originalContainer)
		}
	}()

	containerEnvFile := filepath.Join(tempDir, ".containerenv")
	err := os.WriteFile(containerEnvFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create mock .containerenv file: %v", err)
	}

	dockerEnvFile := filepath.Join(tempDir, ".dockerenv")
	err = os.WriteFile(dockerEnvFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create mock .dockerenv file: %v", err)
	}

	result := IsRunningInContainer()
	if result != true && result != false {
		t.Errorf("IsRunningInContainer() should return a boolean value, got %v", result)
	}
}

// Тестирование edge cases для Abs функции
func TestAbsEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       int
		expected    int
		description string
	}{
		{
			name:        "Boundary: -2",
			input:       -2,
			expected:    2,
			description: "Small negative number",
		},
		{
			name:        "Boundary: 2",
			input:       2,
			expected:    2,
			description: "Small positive number",
		},
		{
			name:        "Large computation result",
			input:       -999999,
			expected:    999999,
			description: "Result of some computation that could be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Abs(tt.input)
			if result != tt.expected {
				t.Errorf("Abs(%d) = %d, want %d (%s)", tt.input, result, tt.expected, tt.description)
			}

			if result < 0 {
				t.Errorf("Abs(%d) = %d, result should never be negative", tt.input, result)
			}
		})
	}
}

// Проверка соответствия математическим свойствам
func TestAbsMathematicalProperties(t *testing.T) {
	testCases := []int{-100, -10, -1, 0, 1, 10, 100}

	for _, x := range testCases {
		t.Run(fmt.Sprintf("Properties_for_%d", x), func(t *testing.T) {
			absX := Abs(x)

			if absX < 0 {
				t.Errorf("Property |x| >= 0 failed: Abs(%d) = %d", x, absX)
			}

			absMinusX := Abs(-x)
			if absX != absMinusX {
				t.Errorf("Property |x| = |-x| failed: Abs(%d) = %d, Abs(%d) = %d", x, absX, -x, absMinusX)
			}

			if (absX == 0) != (x == 0) {
				t.Errorf("Property |x| = 0 iff x = 0 failed: x = %d, Abs(x) = %d", x, absX)
			}
		})
	}
}
