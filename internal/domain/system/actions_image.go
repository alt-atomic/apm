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
	"fmt"
	"os"
	"strings"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/app"
	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/build"
	"altlinux.space/alt-atomic/apm/internal/common/build/altfiles"
	"altlinux.space/alt-atomic/apm/internal/common/build/lint"
	"altlinux.space/alt-atomic/apm/internal/common/imagesvc"
	"altlinux.space/alt-atomic/apm/internal/common/reply"
	kservice "altlinux.space/alt-atomic/apm/internal/domain/kernel/service"
	"altlinux.space/alt-atomic/apm/internal/domain/system/dialog"
	aptBinding "altlinux.space/alt-atomic/apm/pkg/apt"
	reposervice "altlinux.space/alt-atomic/apm/pkg/aptrepo"
	"altlinux.space/alt-atomic/apm/pkg/command"
)

type ImageStatus struct {
	Image  imagesvc.HostImage `json:"image"`
	Status string             `json:"status"`
	Config imagesvc.Config    `json:"config"`
}

// ImageBuild Update Сборка образа
func (a *Actions) ImageBuild(ctx context.Context, configPath, workdir string, pretty bool) (*ImageBuild, error) {
	if !pretty {
		a.appConfig.ConfigManager.EnableVerbose()
		reply.StopSpinner(a.appConfig)
	}

	if err := a.serviceHostConfig.ApplyPathOverrides(configPath, workdir); err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	if err := os.MkdirAll(a.appConfig.ConfigManager.GetResourcesDir(), 0755); err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	err := os.Chdir(a.appConfig.ConfigManager.GetResourcesDir())
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	envVars, err := a.serviceHostConfig.GetConfigEnvVars()
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	for key, value := range envVars {
		if err = os.Setenv(key, value); err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeImage, err)
		}
	}

	err = a.serviceHostConfig.LoadConfig()
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	cfg := a.appConfig.ConfigManager.GetConfig()
	runner := command.NewRunner(cfg.CommandPrefix, cfg.Verbose)
	hostPackageDBSvc := _package.NewPackageDBService(a.appConfig.DatabaseManager, a.reporter)
	aptActions := aptBinding.NewActions()
	kernelManager := kservice.NewKernelManager(hostPackageDBSvc, aptActions, runner, a.reporter)
	hasPackage := func(ctx context.Context, name string) bool {
		pkg, err := hostPackageDBSvc.GetPackageByName(ctx, name)
		return err == nil && pkg.Installed
	}
	repoService := reposervice.NewRepoService(hasPackage, runner)
	buildConfigSvc := build.NewConfigService(a.appConfig, a.reporter, a.serviceAptActions, hostPackageDBSvc, kernelManager, repoService, a.serviceHostConfig, runner)

	err = buildConfigSvc.Build(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	message := app.T_("DONE")
	if pretty {
		if output := buildConfigSvc.CollectedOutput(); output != "" {
			message = output
		}
	}

	return &ImageBuild{
		Message: message,
	}, nil
}

// ImageStatus возвращает статус актуального образа
func (a *Actions) ImageStatus(ctx context.Context) (*ImageStatusResponse, error) {
	imageStatus, err := a.getImageStatus(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	return &ImageStatusResponse{
		Message:     app.T_("Image status"),
		BootedImage: imageStatus,
	}, nil
}

// ImageUpdate обновляет образ.
func (a *Actions) ImageUpdate(ctx context.Context, hostCache bool) (*ImageUpdateResponse, error) {
	if err := a.serviceHostConfig.LoadConfig(); err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	if err := a.serviceHostConfig.GetConfig().CheckImage(); err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	err := a.serviceHostImage.CheckAndUpdateBaseImage(ctx, true, hostCache, *a.serviceHostConfig.GetConfig())
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	imageStatus, err := a.getImageStatus(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	return &ImageUpdateResponse{
		Message:     app.T_("Command executed successfully"),
		BootedImage: imageStatus,
	}, nil
}

// ImageApply применить изменения к хосту
func (a *Actions) ImageApply(ctx context.Context, pullImage bool, hostCache bool, force bool, configPath, workdir string) (*ImageApplyResponse, error) {
	err := a.checkOverlay(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	if err = a.serviceHostConfig.ApplyPathOverrides(configPath, workdir); err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	if err = a.serviceHostConfig.LoadConfig(); err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	if err = a.serviceHostConfig.GetConfig().CheckImage(); err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	if err = a.serviceTemporaryConfig.LoadConfig(); err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	if len(a.serviceTemporaryConfig.GetConfig().Packages.Install) > 0 || len(a.serviceTemporaryConfig.GetConfig().Packages.Remove) > 0 {
		installPkgs := a.serviceTemporaryConfig.GetConfig().Packages.Install
		removePkgs := a.serviceTemporaryConfig.GetConfig().Packages.Remove

		// force применяет все временные пакеты без диалога
		if !force {
			reply.StopSpinner(a.appConfig)
			result, errDialog := dialog.NewPackageSelectionDialog(a.appConfig, installPkgs, removePkgs)
			if errDialog != nil {
				return nil, errDialog
			}

			if result.Canceled {
				return nil, apmerr.New(apmerr.ErrorTypeCanceled, errors.New(app.T_("Cancel dialog")))
			}

			reply.CreateSpinner(a.appConfig)
			installPkgs = result.InstallPackages
			removePkgs = result.RemovePackages
		}

		for _, pkg := range installPkgs {
			if err = a.serviceHostConfig.AddInstallPackage(pkg); err != nil {
				return nil, apmerr.New(apmerr.ErrorTypeImage, err)
			}
		}
		for _, pkg := range removePkgs {
			if err = a.serviceHostConfig.AddRemovePackage(pkg); err != nil {
				return nil, apmerr.New(apmerr.ErrorTypeImage, err)
			}
		}
	}

	if len(a.serviceHostConfig.GetConfig().Modules) > 0 {
		err = a.serviceHostConfig.GenerateDockerfile(hostCache)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeImage, err)
		}

		err = a.serviceHostImage.BuildAndSwitch(ctx, pullImage, !force, a.serviceHostConfig)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeImage, err)
		}
	} else {
		err = a.serviceHostImage.SwitchImage(ctx, a.serviceHostConfig.GetConfig().Image, false)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeImage, err)
		}
	}

	_ = a.serviceTemporaryConfig.DeleteFile()

	imageStatus, err := a.getImageStatus(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	return &ImageApplyResponse{
		Message:     app.T_("Changes applied successfully. A reboot is required"),
		BootedImage: imageStatus,
	}, nil
}

// ImageSwitch переключает систему на другой базовый образ.
func (a *Actions) ImageSwitch(ctx context.Context, image string, pullImage bool, hostCache bool) (*ImageSwitchResponse, error) {
	image = strings.TrimSpace(image)
	if image == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("You must specify the target image")))
	}

	if err := a.serviceHostConfig.LoadConfig(); err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	config := a.serviceHostConfig.GetConfig()
	if config.Image == image {
		active, err := a.serviceHostImage.IsImageActive(image, len(config.Modules) > 0)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeImage, err)
		}
		if active {
			return nil, apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf(app.T_("The system is already based on image %s"), image))
		}
	}

	if err := a.serviceHostImage.VerifyRemoteImage(ctx, image); err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	a.serviceHostConfig.SetImage(image)

	if len(config.Modules) > 0 {
		if err := a.serviceHostConfig.GenerateDockerfile(hostCache); err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeImage, err)
		}
		if err := a.serviceHostImage.BuildAndSwitch(ctx, pullImage, false, a.serviceHostConfig); err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeImage, err)
		}
	} else {
		if err := a.serviceHostImage.SwitchImage(ctx, image, false); err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeImage, err)
		}
	}

	if err := a.serviceHostConfig.SaveConfig(); err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	imageStatus, err := a.getImageStatus(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	return &ImageSwitchResponse{
		Message:     app.T_("Image switched successfully. A reboot is required"),
		BootedImage: imageStatus,
	}, nil
}

// ImageHistory история изменений образа
func (a *Actions) ImageHistory(ctx context.Context, imageName string, limit int, offset int) (*ImageHistoryResponse, error) {
	history, err := a.serviceHostDatabase.GetImageHistoriesFiltered(ctx, imageName, limit, offset)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	totalCount, err := a.serviceHostDatabase.CountImageHistoriesFiltered(ctx, imageName)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	msg := fmt.Sprintf(app.TN_("%d record found", "%d records found", len(history)), len(history))

	return &ImageHistoryResponse{
		Message:    msg,
		History:    history,
		TotalCount: totalCount,
	}, nil
}

// ImageLint линтер файлов и пакетной базы
func (a *Actions) ImageLint(ctx context.Context, rootfs string, fix bool) (*ImageLintResponse, error) {
	svc := lint.New(rootfs, a.reporter)
	result, err := svc.Analyze(ctx, fix)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	resp := &ImageLintResponse{Message: result.Message}

	if result.TmpFiles != nil {
		resp.Tmpfiles = &ImageLintTmpfiles{
			Missing:     result.TmpFiles.Missing,
			Unsupported: result.TmpFiles.Unsupported,
			Factory:     result.TmpFiles.Factory,
		}
	}

	if result.SysUsers != nil {
		resp.Sysusers = &ImageLintSysusers{Missing: result.SysUsers.Missing}
	}
	if result.RunTmp != nil {
		resp.RunTmp = &ImageLintRunTmp{Entries: result.RunTmp.Entries}
	}

	return resp, nil
}

// ImageGetConfig получить конфиг
func (a *Actions) ImageGetConfig(_ context.Context) (*ImageConfigResponse, error) {
	err := a.serviceHostConfig.LoadConfig()
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	return &ImageConfigResponse{
		Config: *a.serviceHostConfig.GetConfig(),
	}, nil
}

// ImageSaveConfig сохранить конфиг
func (a *Actions) ImageSaveConfig(_ context.Context, config imagesvc.Config) (*ImageConfigResponse, error) {
	err := a.serviceHostConfig.LoadConfig()
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	a.serviceHostConfig.SetConfig(&config)

	err = a.serviceHostConfig.SaveConfig()
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	return &ImageConfigResponse{
		Config: *a.serviceHostConfig.GetConfig(),
	}, nil
}

// ImageFixNss исправляет /etc/passwd и /etc/group на живой атомарной системе
func (a *Actions) ImageFixNss(_ context.Context) (*ImageFixNssResponse, error) {
	if !a.appConfig.ConfigManager.GetConfig().IsAtomic {
		return nil, apmerr.New(apmerr.ErrorTypeImage, errors.New(app.T_("This option is only available for an atomic system")))
	}

	svc := altfiles.NewDefault()
	result, err := svc.ApplyFix()
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	return &ImageFixNssResponse{
		Message:             app.T_("nss-altfiles configuration applied successfully"),
		EtcPasswdCount:      result.EtcPasswdCount,
		LibPasswdCount:      result.LibPasswdCount,
		EtcGroupCount:       result.EtcGroupCount,
		LibGroupCount:       result.LibGroupCount,
		RemovedUIDConflicts: result.RemovedUIDConflicts,
		RemovedGIDConflicts: result.RemovedGIDConflicts,
		NormalizedGids:      len(result.NormalizedGroups),
		NormalizedGroups:    result.NormalizedGroups,
	}, nil
}

// ImageSyncGroups синхронизирует группы пользователей из YAML-конфигов
func (a *Actions) ImageSyncGroups(_ context.Context) (*ImageSyncGroupsResponse, error) {
	if !a.appConfig.ConfigManager.GetConfig().IsAtomic {
		return nil, apmerr.New(apmerr.ErrorTypeImage, errors.New(app.T_("This option is only available for an atomic system")))
	}

	svc := altfiles.NewDefault()

	configs, err := svc.ReadSyncConfigsDirs(altfiles.DefaultSyncConfigDirs)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	if len(configs) == 0 {
		return &ImageSyncGroupsResponse{
			Message: app.T_("No configs found"),
		}, nil
	}

	result, err := svc.SyncGroups(configs)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	return &ImageSyncGroupsResponse{
		Message: app.T_("Groups synced successfully"),
		Added:   result.Added,
		Fixed:   result.Fixed,
		Removed: result.Removed,
		Skipped: result.Skipped,
	}, nil
}

func (a *Actions) getImageStatus(_ context.Context) (ImageStatus, error) {
	hostImage, err := a.serviceHostImage.GetHostImage()
	if err != nil {
		return ImageStatus{}, err
	}

	err = a.serviceHostConfig.LoadConfig()
	if err != nil {
		return ImageStatus{}, err
	}

	if hostImage.Status.Booted.Image.Image.Transport == "containers-storage" {
		return ImageStatus{
			Status: app.T_("Modified image. Configuration file: ") + a.appConfig.ConfigManager.GetConfig().PathImageFile,
			Image:  hostImage,
			Config: *a.serviceHostConfig.GetConfig(),
		}, nil
	}

	return ImageStatus{
		Status: app.T_("Cloud image without changes"),
		Image:  hostImage,
		Config: *a.serviceHostConfig.GetConfig(),
	}, nil
}
