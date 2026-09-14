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
	"context"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/authz"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/jobs"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
	"altlinux.space/alt-atomic/apm/internal/common/imagesvc"
	"altlinux.space/alt-atomic/apm/internal/common/polkit"

	"github.com/godbus/dbus/v5"
)

// imageModuleV2 модуль интерфейса org.altlinux.APM2.Image.
func imageModuleV2(actions *Actions) dbusv2.Module {
	return dbusv2.Module{
		Iface:         protocol.ImageIface,
		Introspection: imageIntrospectionV2,
		Build: func(ctx context.Context, reg *jobs.Registry, az authz.Authorizer) any {
			return &ImageV2{
				ctx:     ctx,
				actions: actions,
				jobs:    reg,
				az:      az,
			}
		},
	}
}

// ImageV2 DBus-объект интерфейса Image.
type ImageV2 struct {
	ctx     context.Context
	actions *Actions
	jobs    *jobs.Registry
	az      authz.Authorizer
}

// guard проверяет единое право на управление образом.
func (w *ImageV2) guard(msg dbus.Message) *dbus.Error {
	return wire.Error(w.az.Authorize(msg, protocol.ActionImageManage))
}

// startJob регистрирует фоновую задачу образа; отправитель уже авторизован.
func (w *ImageV2) startJob(msg dbus.Message, kind string, fn func(ctx context.Context) (string, error)) string {
	return w.jobs.Start(jobs.ResourceImage, "image", kind, polkit.Sender(msg), protocol.ActionImageManage, fn)
}

// Status возвращает статус загруженного образа.
func (w *ImageV2) Status() (string, *dbus.Error) {
	resp, err := w.actions.ImageStatus(w.ctx)
	return wire.JSONReply(resp, err)
}

// Update обновляет базовый образ фоновой задачей.
func (w *ImageV2) Update(msg dbus.Message, optionsJSON string) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	var options struct {
		NoCache bool `json:"noCache"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startJob(msg, "Update", wire.JSONTask(func(ctx context.Context) (*ImageUpdateResponse, error) {
		return w.actions.ImageUpdate(ctx, !options.NoCache)
	})), nil
}

// Apply применяет изменения к образу хоста фоновой задачей.
func (w *ImageV2) Apply(msg dbus.Message, optionsJSON string) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	var options struct {
		Pull       bool   `json:"pull"`
		NoCache    bool   `json:"noCache"`
		ConfigPath string `json:"configPath"`
		Workdir    string `json:"workdir"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startJob(msg, "Apply", wire.JSONTask(func(ctx context.Context) (*ImageApplyResponse, error) {
		return w.actions.ImageApply(ctx, options.Pull, !options.NoCache, true, options.ConfigPath, options.Workdir)
	})), nil
}

// Switch переключает систему на другой базовый образ фоновой задачей.
func (w *ImageV2) Switch(msg dbus.Message, image string, optionsJSON string) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	var options struct {
		Pull    bool `json:"pull"`
		NoCache bool `json:"noCache"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startJob(msg, "Switch", wire.JSONTask(func(ctx context.Context) (*ImageSwitchResponse, error) {
		return w.actions.ImageSwitch(ctx, image, options.Pull, !options.NoCache)
	})), nil
}

// History возвращает страницу истории изменений образа.
func (w *ImageV2) History(image string, requestJSON string) (string, *dbus.Error) {
	var request wire.PageRequest
	if err := wire.DecodeJSON(requestJSON, &request); err != nil {
		return "", wire.Error(err)
	}

	page, err := request.Validate()
	if err != nil {
		return "", wire.Error(err)
	}

	resp, err := w.actions.ImageHistory(w.ctx, image, page.Limit, page.Offset)
	return wire.JSONReply(resp, err)
}

// GetConfig возвращает конфигурацию образа.
func (w *ImageV2) GetConfig() (string, *dbus.Error) {
	resp, err := w.actions.ImageGetConfig(w.ctx)
	return wire.JSONReply(resp, err)
}

// SaveConfig сохраняет конфигурацию образа из JSON-документа.
func (w *ImageV2) SaveConfig(msg dbus.Message, configJSON string) *dbus.Error {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return dbusErr
	}
	config, err := imagesvc.ParseJsonConfigData([]byte(configJSON))
	if err != nil {
		return wire.Error(apmerr.New(apmerr.ErrorTypeValidation, err))
	}
	_, err = w.actions.ImageSaveConfig(w.ctx, config)
	return wire.Error(err)
}

// SyncGroups синхронизирует группы пользователей из YAML-конфигов.
func (w *ImageV2) SyncGroups(msg dbus.Message) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	resp, err := w.actions.ImageSyncGroups(w.ctx)
	return wire.JSONReply(resp, err)
}

// FixNss исправляет /etc/passwd и /etc/group на атомарной системе.
func (w *ImageV2) FixNss(msg dbus.Message) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	resp, err := w.actions.ImageFixNss(w.ctx)
	return wire.JSONReply(resp, err)
}
