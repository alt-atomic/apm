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
	"apm/internal/common/swcat"
	"database/sql/driver"
	"time"
)

// Package описывает структуру для хранения информации о пакете.
type Package struct {
	Name             string            `json:"name"`
	Architecture     string            `json:"architecture"`
	Section          string            `json:"section"`
	InstalledSize    int               `json:"installedSize"`
	Maintainer       string            `json:"maintainer"`
	Version          string            `json:"version"`
	VersionRaw       string            `json:"versionRaw"`
	VersionInstalled string            `json:"versionInstalled"`
	Depends          []string          `json:"depends"`
	Aliases          []string          `json:"aliases"`
	Provides         []string          `json:"provides"`
	Size             int               `json:"size"`
	Filename         string            `json:"filename"`
	Summary          string            `json:"summary"`
	Description      string            `json:"description"`
	AppStream        []swcat.Component `json:"appStream,omitempty"`
	HasAppStream     bool              `json:"-"`
	Changelog        string            `json:"lastChangelog"`
	Installed        bool              `json:"installed"`
	TypePackage      int               `json:"typePackage"`
	Files            []string          `json:"files"`
}

type PackageType uint8

const (
	PackageTypeSystem PackageType = iota
	PackageTypeStplr
)

func (t PackageType) String() string {
	switch t {
	case PackageTypeSystem:
		return "system"
	case PackageTypeStplr:
		return "stplr"
	default:
		return "unknown"
	}
}

func (t PackageType) Value() (driver.Value, error) { return int64(t), nil }

// packageProgress — вспомогательная структура для отслеживания прогресса пакета
type packageProgress struct {
	lastPercent int
	lastUpdate  time.Time
	id          int
}
