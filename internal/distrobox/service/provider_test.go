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

package service

import (
	"strings"
	"testing"
)

// TestGetProvider проверяет логику выбора провайдера пакетов в зависимости от ОС контейнера
func TestGetProvider(t *testing.T) {
	// Создаем минимальный PackageService для тестирования
	packageService := &PackageService{
		commandPrefix: "sudo",
	}

	tests := []struct {
		name         string
		osName       string
		expectedType string
		expectError  bool
		description  string
	}{
		{
			name:         "Ubuntu detection",
			osName:       "Ubuntu",
			expectedType: "*service.UbuntuProvider",
			expectError:  false,
			description:  "Should detect Ubuntu and return Ubuntu provider",
		},
		{
			name:         "Ubuntu case insensitive",
			osName:       "ubuntu",
			expectedType: "*service.UbuntuProvider",
			expectError:  false,
			description:  "Should detect ubuntu in lowercase",
		},
		{
			name:         "Ubuntu with version",
			osName:       "Ubuntu 22.04",
			expectedType: "*service.UbuntuProvider",
			expectError:  false,
			description:  "Should detect Ubuntu even with version info",
		},
		{
			name:         "Debian detection",
			osName:       "Debian GNU/Linux",
			expectedType: "*service.UbuntuProvider",
			expectError:  false,
			description:  "Should detect Debian and return Ubuntu provider (same family)",
		},
		{
			name:         "Arch detection",
			osName:       "Arch Linux",
			expectedType: "*service.ArchProvider",
			expectError:  false,
			description:  "Should detect Arch Linux and return Arch provider",
		},
		{
			name:         "Arch case variations",
			osName:       "arch",
			expectedType: "*service.ArchProvider",
			expectError:  false,
			description:  "Should detect arch in any case",
		},
		{
			name:         "ALT Linux detection",
			osName:       "ALT Linux",
			expectedType: "*service.AltProvider",
			expectError:  false,
			description:  "Should detect ALT Linux and return Alt provider",
		},
		{
			name:         "ALT case variations",
			osName:       "alt",
			expectedType: "*service.AltProvider",
			expectError:  false,
			description:  "Should detect alt in lowercase",
		},
		{
			name:         "ALT with version",
			osName:       "ALT Regular Starterkit",
			expectedType: "*service.AltProvider",
			expectError:  false,
			description:  "Should detect ALT even with distro variant",
		},
		{
			name:         "Unsupported OS",
			osName:       "CentOS",
			expectedType: "",
			expectError:  true,
			description:  "Should return error for unsupported OS",
		},
		{
			name:         "Empty OS name",
			osName:       "",
			expectedType: "",
			expectError:  true,
			description:  "Should return error for empty OS name",
		},
		{
			name:         "Random string",
			osName:       "SomeRandomOS",
			expectedType: "",
			expectError:  true,
			description:  "Should return error for unknown OS",
		},
		{
			name:         "Mixed case Ubuntu in sentence",
			osName:       "This is Ubuntu based system",
			expectedType: "*service.UbuntuProvider",
			expectError:  false,
			description:  "Should detect Ubuntu even in longer description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := packageService.getProvider(tt.osName)

			if tt.expectError {
				if err == nil {
					t.Errorf("getProvider(%q) expected error but got none", tt.osName)
				}
				if provider != nil {
					t.Errorf("getProvider(%q) expected nil provider but got %T", tt.osName, provider)
				}
			} else {
				if err != nil {
					t.Errorf("getProvider(%q) unexpected error: %v", tt.osName, err)
				}
				if provider == nil {
					t.Errorf("getProvider(%q) expected provider but got nil", tt.osName)
				} else {
					// Проверяем тип провайдера
					providerType := getProviderTypeName(provider)
					if providerType != tt.expectedType {
						t.Errorf("getProvider(%q) = %s, want %s", tt.osName, providerType, tt.expectedType)
					}
				}
			}
		})
	}
}

// getProviderTypeName возвращает строковое представление типа провайдера
func getProviderTypeName(provider PackageProvider) string {
	switch provider.(type) {
	case *UbuntuProvider:
		return "*service.UbuntuProvider"
	case *ArchProvider:
		return "*service.ArchProvider"
	case *AltProvider:
		return "*service.AltProvider"
	default:
		return "unknown"
	}
}

// TestGetProviderCaseSensitivity проверяет работу с различными регистрами
func TestGetProviderCaseSensitivity(t *testing.T) {
	packageService := &PackageService{commandPrefix: "sudo"}

	osVariations := []struct {
		osName       string
		expectedType string
	}{
		{"UBUNTU", "*service.UbuntuProvider"},
		{"Ubuntu", "*service.UbuntuProvider"},
		{"ubuntu", "*service.UbuntuProvider"},
		{"UbUnTu", "*service.UbuntuProvider"},
		{"ARCH", "*service.ArchProvider"},
		{"Arch", "*service.ArchProvider"},
		{"arch", "*service.ArchProvider"},
		{"ArCh", "*service.ArchProvider"},
		{"ALT", "*service.AltProvider"},
		{"Alt", "*service.AltProvider"},
		{"alt", "*service.AltProvider"},
		{"AlT", "*service.AltProvider"},
	}

	for _, variation := range osVariations {
		t.Run("Case_"+variation.osName, func(t *testing.T) {
			provider, err := packageService.getProvider(variation.osName)
			if err != nil {
				t.Errorf("getProvider(%q) unexpected error: %v", variation.osName, err)
				return
			}

			providerType := getProviderTypeName(provider)
			if providerType != variation.expectedType {
				t.Errorf("getProvider(%q) = %s, want %s", variation.osName, providerType, variation.expectedType)
			}
		})
	}
}

// TestGetProviderSubstringMatching проверяет работу поиска подстрок в названиях ОС
func TestGetProviderSubstringMatching(t *testing.T) {
	packageService := &PackageService{commandPrefix: "sudo"}

	tests := []struct {
		name         string
		osName       string
		expectedType string
		description  string
	}{
		{
			name:         "Ubuntu in description",
			osName:       "Ubuntu 20.04.3 LTS",
			expectedType: "*service.UbuntuProvider",
			description:  "Should match Ubuntu in longer string",
		},
		{
			name:         "Debian in description",
			osName:       "Debian GNU/Linux 11 (bullseye)",
			expectedType: "*service.UbuntuProvider",
			description:  "Should match Debian and use Ubuntu provider",
		},
		{
			name:         "Arch in description",
			osName:       "Arch Linux ARM",
			expectedType: "*service.ArchProvider",
			description:  "Should match Arch in longer string",
		},
		{
			name:         "ALT in description",
			osName:       "ALT Linux Sisyphus",
			expectedType: "*service.AltProvider",
			description:  "Should match ALT in longer string",
		},
		{
			name:         "Multiple matches - first wins",
			osName:       "ubuntu-arch-alt-test",
			expectedType: "*service.UbuntuProvider",
			description:  "Should return first matching provider (ubuntu)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := packageService.getProvider(tt.osName)
			if err != nil {
				t.Errorf("getProvider(%q) unexpected error: %v", tt.osName, err)
				return
			}

			providerType := getProviderTypeName(provider)
			if providerType != tt.expectedType {
				t.Errorf("getProvider(%q) = %s, want %s. %s", tt.osName, providerType, tt.expectedType, tt.description)
			}
		})
	}
}

// TestGetProviderErrorMessages проверяет корректность сообщений об ошибках
func TestGetProviderErrorMessages(t *testing.T) {
	packageService := &PackageService{commandPrefix: "sudo"}

	unsupportedOSes := []string{
		"CentOS",
		"RedHat",
		"SUSE",
		"Fedora",
		"OpenSUSE",
		"FreeBSD",
		"Windows",
		"macOS",
		"Unknown OS",
	}

	for _, osName := range unsupportedOSes {
		t.Run("Error_"+osName, func(t *testing.T) {
			provider, err := packageService.getProvider(osName)

			if err == nil {
				t.Errorf("getProvider(%q) expected error but got none", osName)
			}
			if provider != nil {
				t.Errorf("getProvider(%q) expected nil provider but got %T", osName, provider)
			}

			// Проверяем, что сообщение об ошибке содержит название ОС
			if err != nil && !strings.Contains(err.Error(), osName) {
				t.Errorf("getProvider(%q) error message should contain OS name. Got: %v", osName, err)
			}
		})
	}
}
