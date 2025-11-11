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
	"testing"
)

func TestClearALRPackageName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Package with +alr suffix",
			input:    "firefox+alr1.2.3",
			expected: "firefox",
		},
		{
			name:     "Package with +alr in the middle",
			input:    "some-package+alr-extra-stuff",
			expected: "some-package",
		},
		{
			name:     "Package without +alr",
			input:    "normal-package",
			expected: "normal-package",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Package with multiple +alr occurrences",
			input:    "test+alr+alr123",
			expected: "test",
		},
		{
			name:     "Package starting with +alr",
			input:    "+alr-package",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClearALRPackageName(tt.input)
			if result != tt.expected {
				t.Errorf("ClearALRPackageName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCleanPackageName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Package with EVR suffix",
			input:    "firefox#1.2.3-alt1",
			expected: "firefox",
		},
		{
			name:     "Package with epoch",
			input:    "1:package-name",
			expected: "package-name",
		},
		{
			name:     "Package with .32bit suffix",
			input:    "glibc.32bit",
			expected: "glibc",
		},
		{
			name:     "Package with all modifications",
			input:    "2:firefox#1.2.3-alt1.32bit",
			expected: "firefox",
		},
		{
			name:     "Clean package name",
			input:    "vim",
			expected: "vim",
		},
		{
			name:     "Package with multiple # symbols",
			input:    "test#version#extra",
			expected: "test",
		},
		{
			name:     "Package with multiple : symbols",
			input:    "1:2:package-name",
			expected: "2:package-name",
		},
		{
			name:     "Package with epoch and .32bit only",
			input:    "1:glibc.32bit",
			expected: "glibc",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only epoch",
			input:    "1:",
			expected: "",
		},
		{
			name:     "Only EVR",
			input:    "#1.2.3",
			expected: "",
		},
		{
			name:     "Only .32bit",
			input:    ".32bit",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanPackageName(tt.input)
			if result != tt.expected {
				t.Errorf("CleanPackageName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetVersionFromAptCache(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "Version with epoch",
			input:       "1:1.2.3-alt1",
			expected:    "1.2.3",
			expectError: false,
		},
		{
			name:        "Version without epoch",
			input:       "1.2.3-alt1",
			expected:    "1.2.3",
			expectError: false,
		},
		{
			name:        "Version with -alt suffix",
			input:       "2.5.1-alt2.1",
			expected:    "2.5.1",
			expectError: false,
		},
		{
			name:        "Simple version without dots",
			input:       "123-alt1",
			expected:    "123-alt1",
			expectError: false,
		},
		{
			name:        "Version without -alt",
			input:       "1.0.0",
			expected:    "1.0.0",
			expectError: false,
		},
		{
			name:        "Complex version with epoch and alt",
			input:       "2:3.14.159-alt1.2",
			expected:    "3.14.159",
			expectError: false,
		},
		{
			name:        "Empty string",
			input:       "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Only epoch",
			input:       "1:",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Version with non-numeric epoch",
			input:       "abc:1.2.3-alt1",
			expected:    "abc",
			expectError: false,
		},
		{
			name:        "Version with multiple colons",
			input:       "1:2:3.4.5-alt1",
			expected:    "2",
			expectError: false,
		},
		{
			name:        "Version ending with -alt but no numeric part",
			input:       "test-alt",
			expected:    "test-alt",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetVersionFromAptCache(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("GetVersionFromAptCache(%q) expected error, but got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("GetVersionFromAptCache(%q) unexpected error: %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("GetVersionFromAptCache(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAutoSize(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{
			name:     "Zero bytes",
			input:    0,
			expected: "0.00 MB",
		},
		{
			name:     "1 MB",
			input:    1024 * 1024,
			expected: "1.00 MB",
		},
		{
			name:     "2.5 MB",
			input:    1024 * 1024 * 2.5,
			expected: "2.50 MB",
		},
		{
			name:     "Small size",
			input:    1024,
			expected: "0.00 MB", // 1KB = ~0.001 MB rounded to 0.00
		},
		{
			name:     "Large size",
			input:    1024 * 1024 * 1000, // 1000 MB
			expected: "1000.00 MB",
		},
		{
			name:     "Fractional MB",
			input:    1536 * 1024, // 1.5 MB
			expected: "1.50 MB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AutoSize(tt.input)
			if result != tt.expected {
				t.Errorf("AutoSize(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		name          string
		input         interface{}
		expectedValue bool
		expectedOk    bool
	}{
		// bool values
		{
			name:          "bool true",
			input:         true,
			expectedValue: true,
			expectedOk:    true,
		},
		{
			name:          "bool false",
			input:         false,
			expectedValue: false,
			expectedOk:    true,
		},

		// int values
		{
			name:          "int zero",
			input:         0,
			expectedValue: false,
			expectedOk:    true,
		},
		{
			name:          "int positive",
			input:         1,
			expectedValue: true,
			expectedOk:    true,
		},
		{
			name:          "int negative",
			input:         -1,
			expectedValue: true,
			expectedOk:    true,
		},
		{
			name:          "int large",
			input:         999,
			expectedValue: true,
			expectedOk:    true,
		},

		// string values
		{
			name:          "string 'true'",
			input:         "true",
			expectedValue: true,
			expectedOk:    true,
		},
		{
			name:          "string 'True'",
			input:         "True",
			expectedValue: true,
			expectedOk:    true,
		},
		{
			name:          "string 'TRUE'",
			input:         "TRUE",
			expectedValue: true,
			expectedOk:    true,
		},
		{
			name:          "string 'false'",
			input:         "false",
			expectedValue: false,
			expectedOk:    true,
		},
		{
			name:          "string 'False'",
			input:         "False",
			expectedValue: false,
			expectedOk:    true,
		},
		{
			name:          "string 'FALSE'",
			input:         "FALSE",
			expectedValue: false,
			expectedOk:    true,
		},
		{
			name:          "string '0'",
			input:         "0",
			expectedValue: false,
			expectedOk:    true,
		},
		{
			name:          "string '1'",
			input:         "1",
			expectedValue: true,
			expectedOk:    true,
		},
		{
			name:          "string '-1'",
			input:         "-1",
			expectedValue: true,
			expectedOk:    true,
		},
		{
			name:          "string '123'",
			input:         "123",
			expectedValue: true,
			expectedOk:    true,
		},
		{
			name:          "string 'maybe'",
			input:         "maybe",
			expectedValue: false,
			expectedOk:    false,
		},
		{
			name:          "string 'yes'",
			input:         "yes",
			expectedValue: false,
			expectedOk:    false,
		},
		{
			name:          "empty string",
			input:         "",
			expectedValue: false,
			expectedOk:    false,
		},

		// other types
		{
			name:          "float64",
			input:         3.14,
			expectedValue: false,
			expectedOk:    false,
		},
		{
			name:          "nil",
			input:         nil,
			expectedValue: false,
			expectedOk:    false,
		},
		{
			name:          "slice",
			input:         []string{"true"},
			expectedValue: false,
			expectedOk:    false,
		},
		{
			name:          "map",
			input:         map[string]bool{"key": true},
			expectedValue: false,
			expectedOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok := ParseBool(tt.input)

			if ok != tt.expectedOk {
				t.Errorf("ParseBool(%v) ok = %v, want %v", tt.input, ok, tt.expectedOk)
			}

			if value != tt.expectedValue {
				t.Errorf("ParseBool(%v) value = %v, want %v", tt.input, value, tt.expectedValue)
			}
		})
	}
}
