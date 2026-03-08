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

package _package

import (
	aptParser "apm/internal/common/apt"
	aptLib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/helper"
	"strings"
)

// convertAptPackage преобразует aptLib.PackageInfo в Package
func convertAptPackage(ap *aptLib.PackageInfo) Package {
	cleanList := func(csv string) []string {
		if csv == "" {
			return nil
		}
		var result []string
		seen := make(map[string]bool)
		for _, item := range strings.Split(csv, ",") {
			clean := strings.TrimSpace(item)
			if clean == "" {
				continue
			}
			clean = aptParser.CleanDependency(clean)
			if !seen[clean] {
				seen[clean] = true
				result = append(result, clean)
			}
		}
		return result
	}

	formattedVersion := ap.Version
	if v, err := helper.GetVersionFromAptCache(ap.Version); err == nil && v != "" {
		formattedVersion = v
	}

	description := strings.TrimSpace(ap.Description)
	summary := strings.TrimSpace(ap.ShortDescription)
	if description == "" && summary != "" {
		description = summary
	}

	return Package{
		Name:             ap.Name,
		Architecture:     ap.Architecture,
		Section:          ap.Section,
		InstalledSize:    int(ap.InstalledSize),
		Maintainer:       ap.Maintainer,
		Version:          formattedVersion,
		VersionRaw:       ap.Version,
		VersionInstalled: "",
		Depends:          cleanList(ap.Depends),
		Aliases:          ap.Aliases,
		Provides:         cleanList(ap.Provides),
		Size:             int(ap.DownloadSize),
		Filename:         ap.Filename,
		Summary:          summary,
		Description:      description,
		Changelog:        ap.Changelog,
		Installed:        false,
		TypePackage:      int(PackageTypeSystem),
		Files:            ap.Files,
	}
}

func extractLastMessage(changelog string) string {
	lines := strings.Split(changelog, "\n")
	var result []string
	found := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if isChangelogHeader(trimmed) {
			if !found {
				result = append(result, trimmed)
				found = true
			} else {
				break
			}
		} else if found {
			result = append(result, trimmed)
		}
	}

	return strings.Join(result, "\n")
}

// isChangelogHeader проверяет, является ли строка заголовком записи changelog.
func isChangelogHeader(line string) bool {
	if !strings.HasPrefix(line, "* ") {
		return false
	}
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return false
	}
	year := fields[4]
	return len(year) == 4 && year[0] >= '0' && year[0] <= '9'
}
