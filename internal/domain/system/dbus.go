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
	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/reply"
	"altlinux.space/alt-atomic/apm/internal/domain/system/appstream"
)

// DBusServices общие сервисы домена для обеих версий D-Bus API.
// Actions один на демон: переопределения APT и временный конфиг живут в
// экземпляре, на нескольких копиях интерфейсы разошлись бы между собой.
type DBusServices struct {
	appConfig        *app.Config
	actions          *Actions
	appstreamActions *appstream.Actions
}

// NewDBusServices собирает сервисы домена для экспорта в D-Bus.
func NewDBusServices(appConfig *app.Config, reporter *reply.Reporter) *DBusServices {
	return &DBusServices{
		appConfig:        appConfig,
		actions:          NewActions(appConfig, reporter),
		appstreamActions: appstream.NewActions(appConfig, reporter),
	}
}
