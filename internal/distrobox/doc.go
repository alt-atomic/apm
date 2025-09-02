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
	"apm/internal/common/doc"
	"apm/internal/distrobox/service"
	"context"
	"reflect"
)

// UpdateResponse структура ответа для Update метода
type UpdateResponse struct {
	Message   string                `json:"message"`
	Container service.ContainerInfo `json:"container"`
	Count     int                   `json:"count"`
}

// InfoResponse структура ответа для Info метода
type InfoResponse struct {
	Message     string              `json:"message"`
	PackageInfo service.PackageInfo `json:"packageInfo"`
}

// SearchResponse структура ответа для Search метода
type SearchResponse struct {
	Message  string                `json:"message"`
	Packages []service.PackageInfo `json:"packages"`
}

// ListResponse структура ответа для List метода
type ListResponse struct {
	Message    string                `json:"message"`
	Packages   []service.PackageInfo `json:"packages"`
	TotalCount int64                 `json:"totalCount"`
}

// InstallResponse структура ответа для Install метода
type InstallResponse struct {
	Message     string              `json:"message"`
	PackageInfo service.PackageInfo `json:"packageInfo"`
}

// RemoveResponse структура ответа для Remove метода
type RemoveResponse struct {
	Message     string              `json:"message"`
	PackageInfo service.PackageInfo `json:"packageInfo"`
}

// ContainerListResponse структура ответа для ContainerList метода
type ContainerListResponse struct {
	Containers []service.ContainerInfo `json:"containers"`
}

// ContainerAddResponse структура ответа для ContainerAdd метода
type ContainerAddResponse struct {
	Message       string                `json:"message"`
	ContainerInfo service.ContainerInfo `json:"containerInfo"`
}

// ContainerRemoveResponse структура ответа для ContainerRemove метода
type ContainerRemoveResponse struct {
	Message       string                `json:"message"`
	ContainerInfo service.ContainerInfo `json:"containerInfo"`
}

// FilterField структура поля для фильтрации
type FilterField struct {
	Name   string   `json:"name"`
	Text   string   `json:"text"`
	Type   string   `json:"type"`
	Choice []string `json:"choice"`
}

// GetFilterFieldsResponse структура ответа для GetFilterFields метода
type GetFilterFieldsResponse []FilterField

// getDocConfig возвращает конфигурацию документации для distrobox модуля
func getDocConfig() doc.Config {
	return doc.Config{
		ModuleName:    "Distrobox",
		DBusInterface: "org.altlinux.APM.distrobox",
		ServerPort:    "8082",
		DBusWrapper:   (*DBusWrapper)(nil),
		DBusMethods: map[string]reflect.Type{
			"GetFilterFields": reflect.TypeOf(GetFilterFieldsResponse{}),
			"Update":          reflect.TypeOf(UpdateResponse{}),
			"Info":            reflect.TypeOf(InfoResponse{}),
			"Search":          reflect.TypeOf(SearchResponse{}),
			"List":            reflect.TypeOf(ListResponse{}),
			"Install":         reflect.TypeOf(InstallResponse{}),
			"Remove":          reflect.TypeOf(RemoveResponse{}),
			"ContainerList":   reflect.TypeOf(ContainerListResponse{}),
			"ContainerAdd":    reflect.TypeOf(ContainerAddResponse{}),
			"ContainerRemove": reflect.TypeOf(ContainerRemoveResponse{}),
		},
	}
}

// startDocServer запускает веб-сервер с документацией
func startDocServer(ctx context.Context) error {
	generator := doc.NewGenerator(getDocConfig())
	return generator.StartDocServer(ctx)
}
