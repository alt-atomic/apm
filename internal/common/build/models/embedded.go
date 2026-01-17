// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalов.online
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

package models

import (
	_ "embed"
)

// Встраиваем исходники моделей для парсинга комментариев при генерации схемы

//go:embed branding.go
var BrandingSource string

//go:embed copy.go
var CopySource string

//go:embed git.go
var GitSource string

//go:embed include.go
var IncludeSource string

//go:embed kernel.go
var KernelSource string

//go:embed link.go
var LinkSource string

//go:embed merge.go
var MergeSource string

//go:embed mkdir.go
var MkdirSource string

//go:embed move.go
var MoveSource string

//go:embed network.go
var NetworkSource string

//go:embed packages.go
var PackagesSource string

//go:embed remove.go
var RemoveSource string

//go:embed replace.go
var ReplaceSource string

//go:embed repos.go
var ReposSource string

//go:embed shell.go
var ShellSource string

//go:embed systemd.go
var SystemdSource string

// GetAllSources возвращает все исходники моделей
func GetAllSources() []string {
	return []string{
		BrandingSource,
		CopySource,
		GitSource,
		IncludeSource,
		KernelSource,
		LinkSource,
		MergeSource,
		MkdirSource,
		MoveSource,
		NetworkSource,
		PackagesSource,
		RemoveSource,
		ReplaceSource,
		ReposSource,
		ShellSource,
		SystemdSource,
	}
}
