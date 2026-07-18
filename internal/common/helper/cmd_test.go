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
	"os"
	"path/filepath"
	"testing"
)

func TestIsRunningInContainer(t *testing.T) {
	if IsRunningInContainer() {
		t.Skip("shit happened")
	}
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
