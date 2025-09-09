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

package app

// Build-time переменные (заполняются через -ldflags при сборке)
var (
	BuildCommandPrefix   string
	BuildEnvironment     string
	BuildPathLocales     string
	BuildPathDBSQLSystem string
	BuildPathImageFile   string
	BuildVersion         string
)

func GetBuildInfo() BuildInfo {
	return BuildInfo{
		CommandPrefix:   BuildCommandPrefix,
		Environment:     BuildEnvironment,
		PathLocales:     BuildPathLocales,
		PathDBSQLSystem: BuildPathDBSQLSystem,
		PathImageFile:   BuildPathImageFile,
		Version:         BuildVersion,
	}
}
