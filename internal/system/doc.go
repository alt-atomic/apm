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

package system

import (
	_package "apm/internal/common/apt/package"
	aptlib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/doc"
	"apm/internal/system/service"
	"context"
	_ "embed"
	"reflect"
)

//go:embed dbus.go
var dbusSource string

// FilterField структура поля для фильтрации
type FilterField struct {
	Name string                          `json:"name"`
	Text string                          `json:"text"`
	Info map[_package.PackageType]string `json:"info"`
	Type string                          `json:"type"`
}

// InstallResponse структура ответа для Install/Remove методов
type InstallResponse struct {
	Message string                `json:"message"`
	Info    aptlib.PackageChanges `json:"info"`
}

// InfoResponse структура ответа для Info метода
type InfoResponse struct {
	Message     string           `json:"message"`
	PackageInfo _package.Package `json:"packageInfo"`
}

// ListResponse структура ответа для List/Search методов
type ListResponse struct {
	Message    string             `json:"message"`
	Packages   []_package.Package `json:"packages,omitempty"`
	TotalCount int                `json:"totalCount,omitempty"`
}

// UpdateResponse структура ответа для Update метода
type UpdateResponse struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// CheckResponse структура ответа для Check* методов
type CheckResponse struct {
	Message string                `json:"message"`
	Info    aptlib.PackageChanges `json:"info"`
}

// ImageStatusResponse структура ответа для ImageStatus метода
type ImageStatusResponse struct {
	Message     string      `json:"message"`
	BootedImage ImageStatus `json:"bootedImage"`
}

// ImageUpdateResponse структура ответа для ImageUpdate метода
type ImageUpdateResponse struct {
	Message     string      `json:"message"`
	BootedImage ImageStatus `json:"bootedImage"`
}

// ImageApplyResponse структура ответа для ImageApply метода
type ImageApplyResponse struct {
	Message     string      `json:"message"`
	BootedImage ImageStatus `json:"bootedImage"`
}

// ImageHistoryResponse структура ответа для ImageHistory метода
type ImageHistoryResponse struct {
	Message    string                 `json:"message"`
	History    []service.ImageHistory `json:"history"`
	TotalCount int64                  `json:"totalCount"`
}

// ImageConfigResponse структура ответа для ImageGetConfig/ImageSaveConfig методов
type ImageConfigResponse struct {
	Config service.Config `json:"config"`
}

// GetFilterFieldsResponse структура ответа для GetFilterFields метода
type GetFilterFieldsResponse []FilterField

// getDocConfig возвращает конфигурацию документации для системного модуля
func getDocConfig() doc.Config {
	return doc.Config{
		ModuleName:    "System",
		DBusInterface: "org.altlinux.APM.system",
		ServerPort:    "8081",
		DBusWrapper:   (*DBusWrapper)(nil),
		SourceCode:    dbusSource,
		DBusSession:   "system",
		ResponseTypes: map[string]reflect.Type{
			"InstallResponse":         reflect.TypeOf(InstallResponse{}),
			"GetFilterFieldsResponse": reflect.TypeOf(GetFilterFieldsResponse{}),
			"UpdateResponse":          reflect.TypeOf(UpdateResponse{}),
			"ListResponse":            reflect.TypeOf(ListResponse{}),
			"InfoResponse":            reflect.TypeOf(InfoResponse{}),
			"CheckResponse":           reflect.TypeOf(CheckResponse{}),
			"ImageApplyResponse":      reflect.TypeOf(ImageApplyResponse{}),
			"ImageHistoryResponse":    reflect.TypeOf(ImageHistoryResponse{}),
			"ImageUpdateResponse":     reflect.TypeOf(ImageUpdateResponse{}),
			"ImageStatusResponse":     reflect.TypeOf(ImageStatusResponse{}),
			"ImageConfigResponse":     reflect.TypeOf(ImageConfigResponse{}),
		},
	}
}

// startDocServer запускает веб-сервер с документацией
func startDocServer(ctx context.Context) error {
	generator := doc.NewGenerator(getDocConfig())
	return generator.StartDocServer(ctx)
}
