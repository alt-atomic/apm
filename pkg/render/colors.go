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

// Colors is the color scheme configuration
type Colors struct {
	Accent string `yaml:"accent"`

	TreeBranch  string `yaml:"treeBranch"`
	ResultError string `yaml:"resultError"`

	DialogAction     string `yaml:"dialogAction"`
	DialogDanger     string `yaml:"dialogDanger"`
	DialogHint       string `yaml:"dialogHint"`
	DialogScroll     string `yaml:"dialogScroll"`
	DialogLabelLight string `yaml:"dialogLabelLight"`
	DialogLabelDark  string `yaml:"dialogLabelDark"`

	ProgressEmpty  string `yaml:"progressEmpty"`
	ProgressFilled string `yaml:"progressFilled"`
}

// DefaultColors returns the default color scheme
func DefaultColors() Colors {
	return Colors{
		Accent:      "#a2734c",
		TreeBranch:  "#c4c8c6",
		ResultError: "9",

		DialogAction:     "#26a269",
		DialogDanger:     "#a81c1f",
		DialogHint:       "#888888",
		DialogScroll:     "#ff0000",
		DialogLabelLight: "#234f55",
		DialogLabelDark:  "#82a0a3",

		ProgressEmpty:  "#c4c8c6",
		ProgressFilled: "#26a269",
	}
}
