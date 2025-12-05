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
	"apm/internal/common/build"
)

// CheckResponse структура ответа для Check* методов
type CheckResponse struct {
	Message string                `json:"message"`
	Info    aptlib.PackageChanges `json:"info"`
}

// InstallRemoveResponse структура ответа для Install/Remove методов
type InstallRemoveResponse struct {
	Message string                `json:"message"`
	Info    aptlib.PackageChanges `json:"info"`
}

// UpdateResponse структура ответа для Update метода
type UpdateResponse struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// UpgradeResponse структура ответа для Upgrade метода
type UpgradeResponse struct {
	Message string  `json:"message"`
	Result  *string `json:"result"`
}

// InfoResponse структура ответа для Info метода
// Для DBUS API всегда возвращается полный пакет
type InfoResponse struct {
	Message     string           `json:"message"`
	PackageInfo _package.Package `json:"packageInfo"`
}

// ListResponse структура ответа для List метода
type ListResponse struct {
	Message    string             `json:"message"`
	Packages   []_package.Package `json:"packages,omitempty"`
	TotalCount int                `json:"totalCount,omitempty"`
}

// SearchResponse структура ответа для Search метода
type SearchResponse struct {
	Message  string             `json:"message"`
	Packages []_package.Package `json:"packages,omitempty"`
}

// ImageBuild структура ответа для ImageBuild
type ImageBuild struct {
	Message string `json:"message"`
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
	Message    string               `json:"message"`
	History    []build.ImageHistory `json:"history"`
	TotalCount int                  `json:"totalCount"`
}

// ImageConfigResponse структура ответа для ImageGetConfig/ImageSaveConfig методов
type ImageConfigResponse struct {
	Config build.Config `json:"config"`
}

// FilterField структура поля для фильтрации
type FilterField struct {
	Name string                          `json:"name"`
	Text string                          `json:"text"`
	Info map[_package.PackageType]string `json:"info"`
	Type string                          `json:"type"`
}

// GetFilterFieldsResponse структура ответа для GetFilterFields метода
type GetFilterFieldsResponse []FilterField
