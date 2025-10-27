package service

import (
	"strings"
	"testing"
)

func TestUniqueStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "no duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with duplicates",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "all same",
			input:    []string{"a", "a", "a"},
			expected: []string{"a"},
		},
		{
			name:     "single element",
			input:    []string{"test"},
			expected: []string{"test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uniqueStrings(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected length %d, got %d", len(tt.expected), len(result))
				return
			}

			for i, v := range tt.expected {
				if result[i] != v {
					t.Errorf("Expected %s at index %d, got %s", v, i, result[i])
				}
			}
		})
	}
}

func TestSplitCommand(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		cmd      string
		expected []string
	}{
		{
			name:     "empty command",
			prefix:   "RUN ",
			cmd:      "",
			expected: nil,
		},
		{
			name:     "short command",
			prefix:   "RUN ",
			cmd:      "echo hello",
			expected: []string{"RUN echo hello"},
		},
		{
			name:   "long command that needs splitting",
			prefix: "RUN ",
			cmd:    "apt update && apt install -y package-with-very-long-name another-package third-package fourth-package",
			expected: []string{
				"RUN apt update && apt install -y package-with-very-long-name another-package \\",
				"    third-package fourth-package",
			},
		},
		{
			name:     "command with single very long word",
			prefix:   "RUN ",
			cmd:      "command-with-extremely-long-name-that-exceeds-line-length-limit-by-itself",
			expected: []string{"RUN command-with-extremely-long-name-that-exceeds-line-length-limit-by-itself"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitCommand(tt.prefix, tt.cmd)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d lines, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Line %d: expected %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}

func TestSplitCommand_LineLength(t *testing.T) {
	const maxLineLength = 80
	prefix := "RUN "
	cmd := strings.Repeat("word ", 20) // создаем длинную команду

	result := splitCommand(prefix, cmd)

	for i, line := range result {
		// Последняя строка может не иметь символа продолжения
		if i < len(result)-1 && !strings.HasSuffix(line, " \\") {
			t.Errorf("Line %d should end with continuation character: %q", i, line)
		}

		// Проверяем длину строки (исключая символ продолжения)
		checkLine := strings.TrimSuffix(line, " \\")
		if len(checkLine) > maxLineLength {
			t.Errorf("Line %d exceeds max length (%d): %d characters: %q",
				i, maxLineLength, len(checkLine), line)
		}
	}
}

func TestHostImage_Structure(t *testing.T) {
	// Тестируем, что структуры корректно определены
	hostImage := HostImage{
		Spec: struct {
			Image ImageInfo `json:"image"`
		}{
			Image: ImageInfo{
				Image:     "test-image:latest",
				Transport: "docker",
			},
		},
		Status: struct {
			Staged *ImageStatus `json:"staged"`
			Booted ImageStatus  `json:"booted"`
		}{
			Booted: ImageStatus{
				Image: Image{
					Image: ImageInfo{
						Image:     "test-image:latest",
						Transport: "docker",
					},
					Version:   stringPtr("1.0.0"),
					Timestamp: "2023-01-01T00:00:00Z",
				},
				Pinned: false,
				Store:  "containers-storage",
			},
		},
	}

	if hostImage.Spec.Image.Image != "test-image:latest" {
		t.Errorf("Expected spec image 'test-image:latest', got %s", hostImage.Spec.Image.Image)
	}

	if hostImage.Status.Booted.Image.Image.Transport != "docker" {
		t.Errorf("Expected transport 'docker', got %s", hostImage.Status.Booted.Image.Image.Transport)
	}

	if hostImage.Status.Booted.Image.Version == nil || *hostImage.Status.Booted.Image.Version != "1.0.0" {
		t.Error("Expected version '1.0.0'")
	}
}

// Helper function for testing
func stringPtr(s string) *string {
	return &s
}
