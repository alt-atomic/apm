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

package repo

import (
	aptlib "apm/internal/common/binding/apt/lib"
	"apm/internal/repo/service"
)

// ListResponse структура ответа для List метода
type ListResponse struct {
	Message      string               `json:"message"`
	Repositories []service.Repository `json:"repositories"`
	Count        int                  `json:"count"`
}

// AddRemoveResponse структура ответа для Add/Remove методов
type AddRemoveResponse struct {
	Message string   `json:"message"`
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

// SetResponse структура ответа для Set метода
type SetResponse struct {
	Message string   `json:"message"`
	Branch  string   `json:"branch"`
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

// SimulateResponse структура ответа для симуляции операций
type SimulateResponse struct {
	Message    string   `json:"message"`
	WillAdd    []string `json:"willAdd,omitempty"`
	WillRemove []string `json:"willRemove,omitempty"`
}

// BranchesResponse структура ответа для GetBranches метода
type BranchesResponse struct {
	Message  string   `json:"message"`
	Branches []string `json:"branches"`
}

// TaskPackagesResponse структура ответа для GetTaskPackages метода
type TaskPackagesResponse struct {
	Message  string   `json:"message"`
	TaskNum  string   `json:"taskNum"`
	Packages []string `json:"packages"`
	Count    int      `json:"count"`
}

// TestTaskResponse структура ответа для TestTask метода
type TestTaskResponse struct {
	Message string                `json:"message"`
	TaskNum string                `json:"taskNum"`
	Info    aptlib.PackageChanges `json:"info"`
}
