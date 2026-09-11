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
	"errors"
	"syscall"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/app"
	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/imagesvc"
	"altlinux.space/alt-atomic/apm/internal/common/reply"
	"altlinux.space/alt-atomic/apm/internal/common/swcat"
	"altlinux.space/alt-atomic/apm/internal/domain/system/temporary"
	"altlinux.space/alt-atomic/apm/pkg/command"
)

// Actions объединяет методы для выполнения системных действий.
type Actions struct {
	appConfig              *app.Config
	reporter               *reply.Reporter
	serviceHostImage       hostImageService
	serviceAptActions      aptActionsService
	serviceAptDatabase     aptDatabaseService
	serviceHostDatabase    hostDatabaseService
	serviceHostConfig      hostConfigService
	serviceTemporaryConfig temporaryConfigService
	serviceAppStreamDB     appStreamService
}

// NewActions создаёт новый экземпляр Actions.
func NewActions(appConfig *app.Config, reporter *reply.Reporter) *Actions {
	hostPackageDBSvc := _package.NewPackageDBService(appConfig.DatabaseManager, reporter)
	hostDBSvc := imagesvc.NewHostDBService(appConfig.DatabaseManager, reporter)

	cfg := appConfig.ConfigManager.GetConfig()
	runner := command.NewRunner(cfg.CommandPrefix, cfg.Verbose)
	hostImageSvc := imagesvc.NewHostImageService(
		cfg,
		appConfig.ConfigManager.GetPathImageContainerFile(),
		runner,
		reporter,
	)
	hostConfigSvc := imagesvc.NewHostConfigService(
		hostDBSvc,
		hostImageSvc,
	)
	hostTemporarySvc := temporary.NewManager(
		appConfig.ConfigManager.GetTemporaryImageFile(),
	)
	hostAptSvc := _package.New(hostPackageDBSvc, appConfig, reporter)

	appStreamDBSvc := swcat.NewAppStreamDBService(appConfig.DatabaseManager, reporter)

	return &Actions{
		appConfig:              appConfig,
		reporter:               reporter,
		serviceHostImage:       hostImageSvc,
		serviceAptActions:      hostAptSvc,
		serviceAptDatabase:     hostPackageDBSvc,
		serviceHostDatabase:    hostDBSvc,
		serviceHostConfig:      hostConfigSvc,
		serviceTemporaryConfig: hostTemporarySvc,
		serviceAppStreamDB:     appStreamDBSvc,
	}
}

// checkOverlay проверяет, включен ли overlay
func (a *Actions) checkOverlay(_ context.Context) error {
	if a.appConfig.ConfigManager.GetConfig().IsAtomic {
		err := a.serviceHostImage.EnableOverlay()
		if err != nil {
			return err
		}
	}

	return nil
}

// validateDB проверяет, существует ли база данных
func (a *Actions) validateDB(ctx context.Context, noLock bool) error {
	if err := a.serviceAptDatabase.PackageDatabaseExist(ctx); err != nil {
		if syscall.Geteuid() != 0 {
			return apmerr.New(apmerr.ErrorTypePermission, errors.New(app.T_("package database is empty. Run 'apm system update' with elevated rights to create it")))
		}

		_, err = a.serviceAptActions.Update(ctx, noLock)
		if err != nil {
			return apmerr.New(apmerr.ErrorTypeDatabase, err)
		}
	}

	return nil
}

// updateAllPackagesDB обновляет состояние всех пакетов в базе данных
func (a *Actions) updateAllPackagesDB(ctx context.Context) error {
	a.reporter.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemUpdateAllPackagesDB))
	defer a.reporter.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemUpdateAllPackagesDB))

	installedPackages, err := a.serviceAptActions.GetInstalledPackages(ctx)
	if err != nil {
		return apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	err = a.serviceAptDatabase.SyncPackageInstallationInfo(ctx, installedPackages)
	if err != nil {
		return apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	return nil
}
