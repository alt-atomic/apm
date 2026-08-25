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
	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/authz"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/jobs"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
	"altlinux.space/alt-atomic/apm/internal/common/polkit"
	"altlinux.space/alt-atomic/apm/internal/common/reply"
	pkgbuild "altlinux.space/alt-atomic/apm/pkg/build"

	"github.com/godbus/dbus/v5"
)

// imageModuleV2 модуль интерфейса org.altlinux.APM2.Image.
func imageModuleV2(appConfig *app.Config, reporter *reply.Reporter) dbusv2.Module {
	return dbusv2.Module{
		Iface:         protocol.ImageIface,
		Introspection: imageIntrospectionV2,
		Build: func(ctx context.Context, reg *jobs.Registry, az authz.Authorizer) any {
			return &ImageV2{
				ctx:     ctx,
				actions: NewActions(appConfig, reporter),
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

// startJob авторизует отправителя и регистрирует фоновую задачу образа.
func (w *ImageV2) startJob(msg dbus.Message, kind string, fn func(ctx context.Context) (wire.Dict, error)) (uint32, *dbus.Error) {
	if err := w.az.Authorize(msg, protocol.ActionImageManage); err != nil {
		return 0, wire.Error(err)
	}
	return w.jobs.Start("image", kind, polkit.Sender(msg), protocol.ActionImageManage, fn), nil
}

// Status возвращает статус загруженного образа.
func (w *ImageV2) Status() (wire.Dict, *dbus.Error) {
	resp, err := w.actions.ImageStatus(w.ctx)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(wire.StructDict(resp))
}

// Update обновляет базовый образ фоновой задачей.
func (w *ImageV2) Update(msg dbus.Message, options wire.Dict) (uint32, *dbus.Error) {
	var noCache bool
	if err := wire.ParseOptions(options, map[string]any{
		"no_cache": &noCache,
	}); err != nil {
		return 0, wire.Error(err)
	}
	return w.startJob(msg, "Update", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.ImageUpdate(ctx, !noCache)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// Apply применяет изменения к образу хоста фоновой задачей.
func (w *ImageV2) Apply(msg dbus.Message, options wire.Dict) (uint32, *dbus.Error) {
	var pull, noCache bool
	var configPath, workdir string
	if err := wire.ParseOptions(options, map[string]any{
		"pull":        &pull,
		"no_cache":    &noCache,
		"config_path": &configPath,
		"workdir":     &workdir,
	}); err != nil {
		return 0, wire.Error(err)
	}
	return w.startJob(msg, "Apply", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.ImageApply(ctx, pull, !noCache, true, configPath, workdir)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// Switch переключает систему на другой базовый образ фоновой задачей.
func (w *ImageV2) Switch(msg dbus.Message, image string, options wire.Dict) (uint32, *dbus.Error) {
	var pull, noCache bool
	if err := wire.ParseOptions(options, map[string]any{
		"pull":     &pull,
		"no_cache": &noCache,
	}); err != nil {
		return 0, wire.Error(err)
	}
	return w.startJob(msg, "Switch", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.ImageSwitch(ctx, image, pull, !noCache)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// History возвращает страницу истории изменений образа.
func (w *ImageV2) History(image string, limit uint32, offset uint32) (uint32, []wire.Dict, *dbus.Error) {
	pageSize, err := wire.PageLimit(limit)
	if err != nil {
		return 0, nil, wire.Error(err)
	}
	resp, err := w.actions.ImageHistory(w.ctx, image, pageSize, int(offset))
	if err != nil {
		return 0, nil, wire.Error(err)
	}
	history, err := wire.StructDicts(resp.History)
	if err != nil {
		return 0, nil, wire.Error(err)
	}
	return uint32(max(resp.TotalCount, 0)), history, nil
}

// GetConfig возвращает конфигурацию образа как YAML-документ.
func (w *ImageV2) GetConfig() (string, *dbus.Error) {
	resp, err := w.actions.ImageGetConfig(w.ctx)
	if err != nil {
		return "", wire.Error(err)
	}
	data, err := resp.Config.MarshalYaml()
	if err != nil {
		return "", wire.Error(apmerr.New(apmerr.ErrorTypeImage, err))
	}
	return string(data), nil
}

// SaveConfig сохраняет конфигурацию образа из YAML-документа.
func (w *ImageV2) SaveConfig(msg dbus.Message, yaml string) *dbus.Error {
	return wire.Error(authz.Guard(w.az, msg, protocol.ActionImageManage, func() error {
		config, err := pkgbuild.ParseYamlConfigData([]byte(yaml))
		if err != nil {
			return apmerr.New(apmerr.ErrorTypeValidation, err)
		}
		_, err = w.actions.ImageSaveConfig(w.ctx, config)
		return err
	}))
}

// SyncGroups синхронизирует группы пользователей из YAML-конфигов.
func (w *ImageV2) SyncGroups(msg dbus.Message) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, msg, protocol.ActionImageManage, func() (wire.Dict, error) {
		resp, err := w.actions.ImageSyncGroups(w.ctx)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// FixNss исправляет /etc/passwd и /etc/group на атомарной системе.
func (w *ImageV2) FixNss(msg dbus.Message) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, msg, protocol.ActionImageManage, func() (wire.Dict, error) {
		resp, err := w.actions.ImageFixNss(w.ctx)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}
