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
	"strings"
	"time"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/apt"
	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/reply"
	"altlinux.space/alt-atomic/apm/internal/domain/system/dialog"
)

// aptListsTTL максимальный возраст списков пакетов, при котором Install не делает повторный update
const aptListsTTL = 4 * time.Hour

// CheckRemove проверяем пакеты перед удалением
func (a *Actions) CheckRemove(ctx context.Context, packages []string, purge bool, depends bool) (*CheckResponse, error) {
	packageParse, aptError := a.serviceAptActions.CheckRemove(ctx, packages, purge, depends)
	if aptError != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, aptError)
	}

	return &CheckResponse{
		Message: app.T_("Inspection information"),
		Info:    *packageParse,
	}, nil
}

// CheckUpgrade проверяем пакеты перед обновлением системы
func (a *Actions) CheckUpgrade(ctx context.Context) (*CheckResponse, error) {
	packageParse, aptError := a.serviceAptActions.CheckUpgrade(ctx)
	if aptError != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, aptError)
	}

	return &CheckResponse{
		Message: app.T_("Inspection information"),
		Info:    *packageParse,
	}, nil
}

// CheckInstall проверяем пакеты перед установкой
func (a *Actions) CheckInstall(ctx context.Context, packages []string) (*CheckResponse, error) {
	if len(packages) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("You must specify at least one package")))
	}

	err := a.validateDB(ctx, false)
	if err != nil {
		return nil, err
	}

	packagesInstall, packagesRemove, errPrepare := a.serviceAptActions.PrepareInstallPackages(ctx, packages)
	if errPrepare != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, errPrepare)
	}

	_, _, _, packageParse, errFind := a.serviceAptActions.FindPackage(
		ctx,
		packagesInstall,
		packagesRemove,
		false,
		false,
		false,
	)
	if errFind != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, errFind)
	}

	return &CheckResponse{
		Message: app.T_("Inspection information"),
		Info:    *packageParse,
	}, nil
}

// Remove удаляет системный пакет.
func (a *Actions) Remove(ctx context.Context, packages []string, purge bool, depends bool, confirm bool) (*InstallRemoveResponse, error) {
	err := a.checkOverlay(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	err = a.validateDB(ctx, false)
	if err != nil {
		return nil, err
	}

	if len(packages) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("At least one package must be specified")))
	}

	_, packageNames, packagesInfo, packageParse, errFind := a.serviceAptActions.FindPackage(ctx,
		[]string{}, packages, purge, depends, false)
	if errFind != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, errFind)
	}

	if packageParse.RemovedCount == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNotFound, errors.New(app.T_("No candidates for removal found")))
	}

	if !confirm {
		reply.StopSpinner(a.appConfig)
		dialogStatus, err := dialog.NewDialog(a.appConfig, packagesInfo, *packageParse, dialog.ActionRemove)
		if err != nil {
			return nil, err
		}

		if !dialogStatus {
			return nil, apmerr.New(apmerr.ErrorTypeCanceled, errors.New(app.T_("Cancel dialog")))
		}

		reply.CreateSpinner(a.appConfig)
	}

	err = a.serviceAptActions.Remove(ctx, packageNames, purge, depends)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, err)
	}

	removePackageNames := strings.Join(packageParse.RemovedPackages, ", ")
	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	messageAnswer := fmt.Sprintf(app.TN_("%s removed successfully", "%s removed successfully", packageParse.RemovedCount), removePackageNames)

	if a.appConfig.ConfigManager.GetConfig().IsAtomic {
		messageAnswer += app.T_(". The system image has not been changed. To apply the changes, run: apm s image apply")
		errSave := a.saveChange(ctx, []string{}, packageNames)
		if errSave != nil {
			return nil, apmerr.New(apmerr.ErrorTypeImage, errSave)
		}
	}

	return &InstallRemoveResponse{
		Message: messageAnswer,
		Info:    *packageParse,
	}, nil
}

// Install осуществляет установку системного пакета.
func (a *Actions) Install(ctx context.Context, packages []string, confirm bool, downloadOnly bool, noUpdate bool) (*InstallRemoveResponse, error) {
	err := a.checkOverlay(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	err = a.validateDB(ctx, false)
	if err != nil {
		return nil, err
	}

	if len(packages) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("You must specify at least one package")))
	}

	packagesInstall, packagesRemove, errPrepare := a.serviceAptActions.PrepareInstallPackages(ctx, packages)
	if errPrepare != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, errPrepare)
	}

	// Обновляем индексы ДО симуляции
	allLocalRpm := len(packagesInstall) > 0 && len(packagesRemove) == 0
	if allLocalRpm {
		for _, pkg := range packagesInstall {
			if !apt.IsRegularFileAndIsPackage(pkg) {
				allLocalRpm = false
				break
			}
		}
	}
	if !allLocalRpm && !noUpdate {
		err = a.serviceAptActions.AptUpdateIfStale(ctx, aptListsTTL)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeApt, err)
		}
	}

	packagesInstall, packagesRemove, packagesInfo, packageParse, errFind := a.serviceAptActions.FindPackage(
		ctx,
		packagesInstall,
		packagesRemove,
		false,
		false,
		false,
	)
	if errFind != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, errFind)
	}

	if packageParse.NewInstalledCount == 0 && packageParse.UpgradedCount == 0 && packageParse.RemovedCount == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNoOperation, errors.New(app.T_("The operation will not make any changes")))
	}

	if len(packagesInfo) > 0 && !confirm {
		reply.StopSpinner(a.appConfig)

		var action dialog.Action
		if downloadOnly {
			action = dialog.ActionDownload
		} else if packageParse.RemovedCount > 0 {
			action = dialog.ActionMultiInstall
		} else {
			action = dialog.ActionInstall
		}

		dialogStatus, errDialog := dialog.NewDialog(a.appConfig, packagesInfo, *packageParse, action)
		if errDialog != nil {
			return nil, errDialog
		}

		if !dialogStatus {
			return nil, apmerr.New(apmerr.ErrorTypeCanceled, errors.New(app.T_("Cancel dialog")))
		}

		reply.CreateSpinner(a.appConfig)
	}

	errInstall := a.serviceAptActions.CombineInstallRemovePackages(ctx, packagesInstall, packagesRemove, false, false, downloadOnly)
	if errInstall != nil {
		if matchedErr, ok := errors.AsType[*apt.MatchedError](errInstall); ok && matchedErr.NeedUpdate() {
			_, err = a.serviceAptActions.Update(ctx)
			if err != nil {
				return nil, apmerr.New(apmerr.ErrorTypeApt, err)
			}

			return nil, apmerr.New(apmerr.ErrorTypeRepository, errors.New(app.T_("A repository connection error occurred. The package list has been updated, please try running the command again")))
		}

		return nil, apmerr.New(apmerr.ErrorTypeApt, errInstall)
	}

	var messageAnswer string

	if downloadOnly {
		messageAnswer = fmt.Sprintf(
			app.TN_("%d package successfully downloaded", "%d packages successfully downloaded", packageParse.NewInstalledCount+packageParse.UpgradedCount),
			packageParse.NewInstalledCount+packageParse.UpgradedCount,
		)
	} else {
		err = a.updateAllPackagesDB(ctx)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
		}

		messageAnswer = fmt.Sprintf(
			"%s %s %s",
			fmt.Sprintf(app.TN_("%d package successfully installed", "%d packages successfully installed", packageParse.NewInstalledCount), packageParse.NewInstalledCount),
			app.T_("and"),
			fmt.Sprintf(app.TN_("%d updated", "%d updated", packageParse.UpgradedCount), packageParse.UpgradedCount),
		)

		if a.appConfig.ConfigManager.GetConfig().IsAtomic {
			messageAnswer += app.T_(". The system image has not been changed. To apply the changes, run: apm s image apply")
			errSave := a.saveChange(ctx, packagesInstall, packagesRemove)
			if errSave != nil {
				return nil, apmerr.New(apmerr.ErrorTypeImage, errSave)
			}
		}
	}

	return &InstallRemoveResponse{
		Message: messageAnswer,
		Info:    *packageParse,
	}, nil
}

// Source скачивает исходные пакеты .src.rpm и устанавливает их в сборочное дерево rpm
func (a *Actions) Source(ctx context.Context, packages []string, downloadOnly bool) (*SourceResponse, error) {
	if len(packages) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("You must specify at least one package")))
	}

	sources, err := a.serviceAptActions.DownloadSource(ctx, packages, "")
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, err)
	}

	messageAnswer := app.T_("Source packages successfully downloaded")
	if !downloadOnly {
		files := make([]string, 0, len(sources))
		for _, src := range sources {
			files = append(files, src.File)
		}
		if errInstall := a.serviceAptActions.InstallSourcePackages(ctx, files); errInstall != nil {
			return nil, apmerr.New(apmerr.ErrorTypeApt, errInstall)
		}
		messageAnswer = app.T_("Source packages successfully downloaded and installed")
	}

	resp := &SourceResponse{Message: messageAnswer}
	for _, src := range sources {
		resp.Packages = append(resp.Packages, SourcePackageInfo{
			Name:    src.Name,
			Version: src.Version,
			File:    src.File,
		})
	}

	return resp, nil
}

// CheckReinstall проверяем пакеты перед переустановкой
func (a *Actions) CheckReinstall(ctx context.Context, packages []string) (*CheckResponse, error) {
	if len(packages) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("You must specify at least one package")))
	}

	packagesInstall, packagesRemove, errPrepare := a.serviceAptActions.PrepareInstallPackages(ctx, packages)
	if errPrepare != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, errPrepare)
	}

	_, _, _, packageParse, errFind := a.serviceAptActions.FindPackage(
		ctx,
		packagesInstall,
		packagesRemove,
		false,
		false,
		true,
	)
	if errFind != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, errFind)
	}

	return &CheckResponse{
		Message: app.T_("Inspection information"),
		Info:    *packageParse,
	}, nil
}

// Reinstall осуществляет переустановку системного пакета.
func (a *Actions) Reinstall(ctx context.Context, packages []string, confirm bool) (*InstallRemoveResponse, error) {
	err := a.checkOverlay(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	err = a.validateDB(ctx, false)
	if err != nil {
		return nil, err
	}

	if len(packages) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("You must specify at least one package")))
	}

	packagesInstall, _, errPrepare := a.serviceAptActions.PrepareInstallPackages(ctx, packages)
	if errPrepare != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, errPrepare)
	}

	packagesInstall, _, packagesInfo, packageParse, errFind := a.serviceAptActions.FindPackage(
		ctx,
		packagesInstall,
		nil,
		false,
		false,
		true,
	)
	if errFind != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, errFind)
	}

	if packageParse.NewInstalledCount == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNoOperation, errors.New(app.T_("The operation will not make any changes")))
	}

	if !confirm {
		reply.StopSpinner(a.appConfig)

		dialogStatus, errDialog := dialog.NewDialog(a.appConfig, packagesInfo, *packageParse, dialog.ActionInstall)
		if errDialog != nil {
			return nil, errDialog
		}

		if !dialogStatus {
			return nil, apmerr.New(apmerr.ErrorTypeCanceled, errors.New(app.T_("Cancel dialog")))
		}

		reply.CreateSpinner(a.appConfig)
	}

	errReinstall := a.serviceAptActions.ReinstallPackages(ctx, packagesInstall)
	if errReinstall != nil {
		if matchedErr, ok := errors.AsType[*apt.MatchedError](errReinstall); ok && matchedErr.NeedUpdate() {
			_, err = a.serviceAptActions.Update(ctx)
			if err != nil {
				return nil, apmerr.New(apmerr.ErrorTypeApt, err)
			}

			return nil, apmerr.New(apmerr.ErrorTypeRepository, errors.New(app.T_("A repository connection error occurred. The package list has been updated, please try running the command again")))
		}

		return nil, apmerr.New(apmerr.ErrorTypeApt, errReinstall)
	}

	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	messageAnswer := fmt.Sprintf(
		app.TN_("%d package successfully reinstalled", "%d packages successfully reinstalled", packageParse.NewInstalledCount),
		packageParse.NewInstalledCount,
	)

	return &InstallRemoveResponse{
		Message: messageAnswer,
		Info:    *packageParse,
	}, nil
}

// Update обновляет информацию или базу данных пакетов.
func (a *Actions) Update(ctx context.Context, noLock bool, onlyDB bool) (*UpdateResponse, error) {
	err := a.checkOverlay(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	err = a.validateDB(ctx, noLock)
	if err != nil {
		return nil, err
	}

	if onlyDB {
		packages, err := a.serviceAptActions.UpdateDBOnly(ctx, noLock)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeApt, err)
		}
		if err = a.serviceAptDatabase.UpdateAppStreamLinks(ctx); err != nil {
			app.Log.Debugf("UpdateAppStreamLinks: %v", err)
		}
		return &UpdateResponse{
			Message: app.T_("Installed package status updated"),
			Count:   len(packages),
		}, nil
	}

	packages, err := a.serviceAptActions.Update(ctx, noLock)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, err)
	}

	if err = a.serviceAptDatabase.UpdateAppStreamLinks(ctx); err != nil {
		app.Log.Debugf("UpdateAppStreamLinks: %v", err)
	}

	return &UpdateResponse{
		Message: app.T_("Package list updated successfully"),
		Count:   len(packages),
	}, nil
}

// Upgrade общее обновление системы
func (a *Actions) Upgrade(ctx context.Context, downloadOnly bool) (*UpgradeResponse, error) {
	err := a.checkOverlay(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	err = a.validateDB(ctx, false)
	if err != nil {
		return nil, err
	}

	_, err = a.serviceAptActions.Update(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, err)
	}

	packageParse, aptError := a.serviceAptActions.CheckUpgrade(ctx)
	if aptError != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, aptError)
	}

	if packageParse.NewInstalledCount == 0 && packageParse.UpgradedCount == 0 && packageParse.RemovedCount == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNoOperation, errors.New(app.T_("The operation will not make any changes")))
	}

	reply.StopSpinner(a.appConfig)

	action := dialog.ActionUpgrade
	if downloadOnly {
		action = dialog.ActionDownload
	}

	dialogStatus, err := dialog.NewDialog(a.appConfig, []_package.Package{}, *packageParse, action)
	if err != nil {
		return nil, err
	}

	if !dialogStatus {
		return nil, apmerr.New(apmerr.ErrorTypeCanceled, errors.New(app.T_("Cancel dialog")))
	}

	reply.CreateSpinner(a.appConfig)

	errUpgrade := a.serviceAptActions.Upgrade(ctx, downloadOnly)
	if errUpgrade != nil {
		return nil, apmerr.New(apmerr.ErrorTypeApt, errUpgrade)
	}

	if downloadOnly {
		total := packageParse.NewInstalledCount + packageParse.UpgradedCount

		return &UpgradeResponse{
			Message: app.T_("Download complete"),
			Result: new(fmt.Sprintf(
				app.TN_("%d package successfully downloaded", "%d packages successfully downloaded", total),
				total,
			)),
		}, nil
	}

	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	return &UpgradeResponse{
		Message: app.T_("The system has been upgrade successfully"),
		Result: new(fmt.Sprintf(
			"%s %s %s",
			fmt.Sprintf(app.TN_("%d package successfully installed", "%d packages successfully installed", packageParse.NewInstalledCount), packageParse.NewInstalledCount),
			app.T_("and"),
			fmt.Sprintf(app.TN_("%d updated", "%d updated", packageParse.UpgradedCount), packageParse.UpgradedCount),
		)),
	}, nil
}

// saveChange применяет изменения к образу системы
func (a *Actions) saveChange(_ context.Context, packagesInstall []string, packagesRemove []string) error {
	if !a.appConfig.ConfigManager.GetConfig().IsAtomic {
		return apmerr.New(apmerr.ErrorTypeImage, errors.New(app.T_("This option is only available for an atomic system")))
	}

	if err := a.serviceTemporaryConfig.LoadConfig(); err != nil {
		return err
	}

	processPackages := func(packages []string, addFunc func(string) error) error {
		for _, pkg := range packages {
			if pkg = strings.TrimSpace(pkg); pkg != "" {
				if err := addFunc(pkg); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := processPackages(packagesInstall, a.serviceTemporaryConfig.AddInstallPackage); err != nil {
		return err
	}

	if err := processPackages(packagesRemove, a.serviceTemporaryConfig.AddRemovePackage); err != nil {
		return err
	}

	return a.serviceTemporaryConfig.SaveConfig()
}
