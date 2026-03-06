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

package appstream

import (
	"apm/internal/common/filter"
	"apm/internal/common/swcat"
)

// UpdateResponse структура ответа для метода Update.
type UpdateResponse struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// ListResponse структура ответа для метода List.
type ListResponse struct {
	Message    string              `json:"message"`
	Components []swcat.DBAppStream `json:"components"`
	TotalCount int                 `json:"totalCount"`
}

// ListParams параметры запроса списка AppStream компонентов.
type ListParams struct {
	Sort    string          `json:"sort"`
	Order   string          `json:"order"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
	Filters []filter.Filter `json:"filters"`
}

// InfoResponse структура ответа для метода Info.
type InfoResponse struct {
	Message    string            `json:"message"`
	PkgName    string            `json:"pkgname"`
	Components []swcat.Component `json:"components"`
}

// FilterFieldsAppStreamResponse структура ответа для GetFilterFields.
type FilterFieldsAppStreamResponse []filter.FieldInfo
