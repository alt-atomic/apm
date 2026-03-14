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

package distrobox

import (
	"apm/internal/common/filter"
	"apm/internal/common/sandbox"
)

// UpdateResponse структура ответа для Update метода
type UpdateResponse struct {
	Message   string                `json:"message"`
	Container sandbox.ContainerInfo `json:"container"`
	Count     int                   `json:"count"`
}

// InfoResponse структура ответа для Info метода
type InfoResponse struct {
	Message     string                    `json:"message"`
	PackageInfo sandbox.InfoPackageAnswer `json:"packageInfo"`
}

// SearchResponse структура ответа для Search метода
type SearchResponse struct {
	Message  string                `json:"message"`
	Packages []sandbox.PackageInfo `json:"packages"`
}

// ListFiltersBody тело запроса для List — только фильтры.
type ListFiltersBody struct {
	Filters []filter.Filter `json:"filters"`
}

// ListResponse структура ответа для List метода
type ListResponse struct {
	Message    string                `json:"message"`
	Packages   []sandbox.PackageInfo `json:"packages"`
	TotalCount int                   `json:"totalCount"`
}

// InstallResponse структура ответа для Install метода
type InstallResponse struct {
	Message     string                    `json:"message"`
	PackageInfo sandbox.InfoPackageAnswer `json:"packageInfo"`
}

// RemoveResponse структура ответа для Remove метода
type RemoveResponse struct {
	Message     string                    `json:"message"`
	PackageInfo sandbox.InfoPackageAnswer `json:"packageInfo"`
}

// ContainerListResponse структура ответа для ContainerList метода
type ContainerListResponse struct {
	Containers []sandbox.ContainerInfo `json:"containers"`
}

// ContainerAddResponse структура ответа для ContainerAdd метода
type ContainerAddResponse struct {
	Message       string                `json:"message"`
	ContainerInfo sandbox.ContainerInfo `json:"containerInfo"`
}

// ContainerRemoveResponse структура ответа для ContainerRemove метода
type ContainerRemoveResponse struct {
	Message       string                `json:"message"`
	ContainerInfo sandbox.ContainerInfo `json:"containerInfo"`
}

// GetFilterFieldsResponse структура ответа для GetFilterFields метода
type GetFilterFieldsResponse []filter.FieldInfo

// BackgroundTaskResponse структура ответа при запуске фоновой задачи
type BackgroundTaskResponse struct {
	Message     string `json:"message"`
	Transaction string `json:"transaction"`
}
