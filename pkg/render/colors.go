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

package render

// Colors is the renderer color scheme
type Colors struct {
	Accent      string `yaml:"accent"`
	TreeBranch  string `yaml:"treeBranch"`
	ResultError string `yaml:"resultError"`
}

// DefaultColors returns the default color scheme
func DefaultColors() Colors {
	return Colors{
		Accent:      "#a2734c",
		TreeBranch:  "#c4c8c6",
		ResultError: "9",
	}
}
