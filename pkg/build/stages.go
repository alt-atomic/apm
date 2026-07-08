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

package build

import "context"

// Stage is a build lifecycle point where the caller attaches hooks.
type Stage int

const (
	// PreModules runs before the modules.
	PreModules Stage = iota
	// PostModules runs after the modules, even on error.
	PostModules
	// PostBuild runs after the modules succeed.
	PostBuild
)

// HookFunc is a stage hook body.
type HookFunc func(ctx context.Context) error
