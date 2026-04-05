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
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	"apm/internal/common/apt"
	_package "apm/internal/common/apt/package"
	aptBinding "apm/internal/common/binding/apt"
	"apm/internal/common/build"
	"apm/internal/common/build/altfiles"
	"apm/internal/common/build/lint"
	"apm/internal/common/command"
	"apm/internal/common/filter"
	"apm/internal/common/reply"
	"apm/internal/common/swcat"
	kservice "apm/internal/domain/kernel/service"
	reposervice "apm/internal/domain/repository/service"
	"apm/internal/domain/system/dialog"
	"apm/internal/domain/system/service"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
)

// Actions объединяет методы для выполнения системных действий.
type Actions struct {
	appConfig              *app.Config
	serviceHostImage       hostImageService
	serviceAptActions      aptActionsService
	serviceAptDatabase     aptDatabaseService
	serviceHostDatabase    hostDatabaseService
	serviceHostConfig      hostConfigService
	serviceTemporaryConfig temporaryConfigService
	serviceAppStreamDB     appStreamService
}

// NewActions создаёт новый экземпляр Actions.
func NewActions(appConfig *app.Config) *Actions {
	hostPackageDBSvc := _package.NewPackageDBService(appConfig.DatabaseManager)
	hostDBSvc := build.NewHostDBService(appConfig.DatabaseManager)

	cfg := appConfig.ConfigManager.GetConfig()
	runner := command.NewRunner(cfg.CommandPrefix, cfg.Verbose)
	hostImageSvc := build.NewHostImageService(
		cfg,
		appConfig.ConfigManager.GetPathImageContainerFile(),
		runner,
	)
	hostConfigSvc := build.NewHostConfigService(
		appConfig.ConfigManager.GetPathImageFile(),
		hostDBSvc,
		hostImageSvc,
	)
	hostTemporarySvc := service.NewTemporaryConfigService(
		appConfig.ConfigManager.GetTemporaryImageFile(),
	)
	hostAptSvc := _package.NewActions(hostPackageDBSvc, appConfig)

	appStreamDBSvc := swcat.NewAppStreamDBService(appConfig.DatabaseManager)

	return &Actions{
		appConfig:              appConfig,
		serviceHostImage:       hostImageSvc,
		serviceAptActions:      hostAptSvc,
		serviceAptDatabase:     hostPackageDBSvc,
		serviceHostDatabase:    hostDBSvc,
		serviceHostConfig:      hostConfigSvc,
		serviceTemporaryConfig: hostTemporarySvc,
		serviceAppStreamDB:     appStreamDBSvc,
	}
}

// SetAptConfigOverrides устанавливает переопределения конфигурации APT
func (a *Actions) SetAptConfigOverrides(overrides map[string]string) (*AptConfigResponse, error) {
	a.serviceAptActions.SetAptConfigOverrides(overrides)
	return &AptConfigResponse{Options: overrides}, nil
}

// GetAptConfigOverrides возвращает текущие переопределения конфигурации APT
func (a *Actions) GetAptConfigOverrides() (*AptConfigResponse, error) {
	overrides := a.serviceAptActions.GetAptConfigOverrides()
	if overrides == nil {
		overrides = map[string]string{}
	}
	return &AptConfigResponse{Options: overrides}, nil
}

type ImageStatus struct {
	Image  build.HostImage `json:"image"`
	Status string          `json:"status"`
	Config build.Config    `json:"config"`
}

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
func (a *Actions) Install(ctx context.Context, packages []string, confirm bool, downloadOnly bool) (*InstallRemoveResponse, error) {
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

	allLocalRpm := len(packagesInstall) > 0 && len(packagesRemove) == 0
	if allLocalRpm {
		for _, pkg := range packagesInstall {
			if !apt.IsRegularFileAndIsPackage(pkg) {
				allLocalRpm = false
				break
			}
		}
	}
	// Проверяем, все ли пакеты на вход являются RPM файлами
	if !allLocalRpm {
		err = a.serviceAptActions.AptUpdate(ctx)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeApt, err)
		}
	}

	errInstall := a.serviceAptActions.CombineInstallRemovePackages(ctx, packagesInstall, packagesRemove, false, false, downloadOnly)
	if errInstall != nil {
		var matchedErr *apt.MatchedError
		if errors.As(errInstall, &matchedErr) && matchedErr.NeedUpdate() {
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
		var matchedErr *apt.MatchedError
		if errors.As(errReinstall, &matchedErr) && matchedErr.NeedUpdate() {
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

// ImageBuild Update Сборка образа
func (a *Actions) ImageBuild(ctx context.Context) (*ImageBuild, error) {
	a.appConfig.ConfigManager.EnableVerbose()
	reply.StopSpinner(a.appConfig)

	if err := os.MkdirAll(a.appConfig.ConfigManager.GetResourcesDir(), 0644); err != nil {
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
	hostPackageDBSvc := _package.NewPackageDBService(a.appConfig.DatabaseManager)
	aptActions := aptBinding.NewActions()
	kernelManager := kservice.NewKernelManager(hostPackageDBSvc, aptActions, runner)
	repoService := reposervice.NewRepoService(hostPackageDBSvc, runner)
	buildConfigSvc := build.NewConfigService(a.appConfig, a.serviceAptActions, hostPackageDBSvc, kernelManager, repoService, a.serviceHostConfig, runner)

	err = buildConfigSvc.Build(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	return &ImageBuild{
		Message: app.T_("DONE"),
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
		messageAnswer := fmt.Sprintf(
			app.TN_("%d package successfully downloaded", "%d packages successfully downloaded", total),
			total,
		)

		return &UpgradeResponse{
			Message: app.T_("Download complete"),
			Result:  &messageAnswer,
		}, nil
	}

	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	messageAnswer := fmt.Sprintf(
		"%s %s %s",
		fmt.Sprintf(app.TN_("%d package successfully installed", "%d packages successfully installed", packageParse.NewInstalledCount), packageParse.NewInstalledCount),
		app.T_("and"),
		fmt.Sprintf(app.TN_("%d updated", "%d updated", packageParse.UpgradedCount), packageParse.UpgradedCount),
	)

	return &UpgradeResponse{
		Message: app.T_("The system has been upgrade successfully"),
		Result:  &messageAnswer,
	}, nil
}

// Info возвращает информацию о системном пакете.
func (a *Actions) Info(ctx context.Context, packageName string) (*InfoResponse, error) {
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Package name must be specified, for example info package")))
	}

	err := a.validateDB(ctx, false)
	if err != nil {
		return nil, err
	}

	packageInfo, err := a.serviceAptDatabase.GetPackageByName(ctx, packageName)
	if err != nil {
		filters := []filter.Filter{
			{Field: "provides", Op: filter.OpContains, Value: packageName},
		}

		alternativePackages, errFind := a.serviceAptDatabase.QueryHostImagePackages(ctx, filters, "", "", 5, 0)
		if errFind != nil {
			return nil, apmerr.New(apmerr.ErrorTypeDatabase, errFind)
		}

		if len(alternativePackages) == 0 {
			errorFindPackage := fmt.Sprintf(app.T_("Failed to retrieve information about the package %s"), packageName)
			return nil, apmerr.New(apmerr.ErrorTypeNotFound, errors.New(errorFindPackage))
		} else if len(alternativePackages) == 1 {
			packageInfo = alternativePackages[0]
		} else {
			var altNames []string
			for _, altPkg := range alternativePackages {
				altNames = append(altNames, altPkg.Name)
			}

			message := err.Error() + app.T_(". Maybe you were looking for: ")

			return nil, apmerr.New(apmerr.ErrorTypeNotFound, fmt.Errorf(message+"%s", strings.Join(altNames, " ")))
		}
	}

	pkgs := []_package.Package{packageInfo}
	a.enrichWithAppStream(ctx, pkgs)
	packageInfo = pkgs[0]

	return &InfoResponse{
		Message:     app.T_("Package found"),
		PackageInfo: packageInfo,
	}, nil
}

// MultiInfo возвращает информацию о нескольких пакетах одним запросом.
func (a *Actions) MultiInfo(ctx context.Context, packageNames []string) (*MultiInfoResponse, error) {
	if len(packageNames) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Package list must not be empty")))
	}

	err := a.validateDB(ctx, false)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(packageNames))
	for _, name := range packageNames {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}

	if len(names) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Package list must not be empty")))
	}

	packages, err := a.serviceAptDatabase.GetPackagesByNames(ctx, names)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	foundNames := make(map[string]bool, len(packages))
	for _, pkg := range packages {
		foundNames[pkg.Name] = true
	}

	var missing []string
	for _, name := range names {
		if !foundNames[name] {
			missing = append(missing, name)
		}
	}

	var notFound []string
	for _, name := range missing {
		providesPackages, err := a.serviceAptDatabase.QueryHostImagePackages(ctx, []filter.Filter{
			{Field: "provides", Op: filter.OpContains, Value: name},
		}, "", "", 1, 0)
		if err != nil || len(providesPackages) == 0 {
			notFound = append(notFound, name)
			continue
		}
		packages = append(packages, providesPackages[0])
	}

	a.enrichWithAppStream(ctx, packages)

	return &MultiInfoResponse{
		Message:  fmt.Sprintf(app.T_("Found %d out of %d packages"), len(packages), len(names)),
		Packages: packages,
		NotFound: notFound,
	}, nil
}

// ListParams задаёт параметры для запроса списка пакетов.
type ListParams struct {
	Sort        string          `json:"sort"`
	Order       string          `json:"order"`
	Limit       int             `json:"limit"`
	Offset      int             `json:"offset"`
	Filters     []filter.Filter `json:"filters"`
	ForceUpdate bool            `json:"forceUpdate"`
	Full        bool            `json:"full"`
}

// List возвращает список пакетов
func (a *Actions) List(ctx context.Context, params ListParams) (*ListResponse, error) {
	if params.ForceUpdate {
		_, err := a.serviceAptActions.Update(ctx)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeApt, err)
		}
	}
	err := a.validateDB(ctx, false)
	if err != nil {
		return nil, err
	}

	totalCount, err := a.serviceAptDatabase.CountHostImagePackages(ctx, params.Filters)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	packages, err := a.serviceAptDatabase.QueryHostImagePackages(ctx, params.Filters, params.Sort, params.Order, params.Limit, params.Offset)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	if len(packages) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNotFound, errors.New(app.T_("Nothing found")))
	}

	a.enrichWithAppStream(ctx, packages)

	msg := fmt.Sprintf(app.TN_("%d record found", "%d records found", len(packages)), len(packages))

	return &ListResponse{
		Message:    msg,
		Packages:   packages,
		TotalCount: int(totalCount),
	}, nil
}

// GetFilterFields возвращает список свойств для фильтрации
func (a *Actions) GetFilterFields(ctx context.Context) (GetFilterFieldsResponse, error) {
	if err := a.validateDB(ctx, false); err != nil {
		return nil, err
	}

	return _package.SystemFilterConfig.FieldsInfo(), nil
}

// Sections возвращает список всех уникальных секций пакетов.
func (a *Actions) Sections(ctx context.Context) (*SectionsResponse, error) {
	if err := a.validateDB(ctx, false); err != nil {
		return nil, err
	}

	sections, err := a.serviceAptDatabase.GetSections(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	return &SectionsResponse{
		Message:  fmt.Sprintf(app.TN_("%d section found", "%d sections found", len(sections)), len(sections)),
		Sections: sections,
	}, nil
}

// Search осуществляет поиск системного пакета по названию
func (a *Actions) Search(ctx context.Context, packageName string, installed bool) (*SearchResponse, error) {
	err := a.validateDB(ctx, false)
	if err != nil {
		return nil, err
	}

	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf(app.T_("You must specify the package name, for example `%s package`"), "search"))
	}

	packages, err := a.serviceAptDatabase.SearchPackagesByNameLike(ctx, "%"+packageName+"%", installed)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	if len(packages) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNotFound, errors.New(app.T_("Nothing found")))
	}

	a.enrichWithAppStream(ctx, packages)

	msg := fmt.Sprintf(app.TN_("%d record found", "%d records found", len(packages)), len(packages))

	return &SearchResponse{
		Message:  msg,
		Packages: packages,
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
func (a *Actions) ImageApply(ctx context.Context, pullImage bool, hostCache bool) (*ImageApplyResponse, error) {
	err := a.checkOverlay(ctx)
	if err != nil {
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
		reply.StopSpinner(a.appConfig)
		// Показываем диалог выбора пакетов
		result, errDialog := dialog.NewPackageSelectionDialog(
			a.appConfig,
			a.serviceTemporaryConfig.GetConfig().Packages.Install,
			a.serviceTemporaryConfig.GetConfig().Packages.Remove,
		)
		if errDialog != nil {
			return nil, errDialog
		}

		if result.Canceled {
			return nil, apmerr.New(apmerr.ErrorTypeCanceled, errors.New(app.T_("Cancel dialog")))
		}

		reply.CreateSpinner(a.appConfig)
		for _, pkg := range result.InstallPackages {
			err = a.serviceHostConfig.AddInstallPackage(pkg)
			if err != nil {
				return nil, apmerr.New(apmerr.ErrorTypeImage, err)
			}
		}
		for _, pkg := range result.RemovePackages {
			err = a.serviceHostConfig.AddRemovePackage(pkg)
			if err != nil {
				return nil, apmerr.New(apmerr.ErrorTypeImage, err)
			}
		}
	}

	imageStatus, err := a.getImageStatus(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	if len(a.serviceHostConfig.GetConfig().Modules) > 0 {
		err = a.serviceHostConfig.GenerateDockerfile(hostCache)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeImage, err)
		}

		err = a.serviceHostImage.BuildAndSwitch(ctx, pullImage, true, a.serviceHostConfig)
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

	return &ImageApplyResponse{
		Message:     app.T_("Changes applied successfully. A reboot is required"),
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
	svc := lint.New(rootfs)
	result, err := svc.Analyze(ctx, fix)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeImage, err)
	}

	resp := &ImageLintResponse{Message: result.Message}

	if result.TmpFiles != nil {
		resp.Tmpfiles = &ImageLintTmpfiles{
			Missing:     result.TmpFiles.Missing,
			Unsupported: result.TmpFiles.Unsupported,
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
func (a *Actions) ImageSaveConfig(_ context.Context, config build.Config) (*ImageConfigResponse, error) {
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
		Message:        app.T_("nss-altfiles configuration applied successfully"),
		EtcPasswdCount: result.EtcPasswdCount,
		LibPasswdCount: result.LibPasswdCount,
		EtcGroupCount:  result.EtcGroupCount,
		LibGroupCount:  result.LibGroupCount,
	}, nil
}

// ImageSyncGroups синхронизирует группы пользователей из YAML-конфигов
func (a *Actions) ImageSyncGroups(_ context.Context, configDirs []string) (*ImageSyncGroupsResponse, error) {
	if !a.appConfig.ConfigManager.GetConfig().IsAtomic {
		return nil, apmerr.New(apmerr.ErrorTypeImage, errors.New(app.T_("This option is only available for an atomic system")))
	}

	svc := altfiles.NewDefault()

	configs, err := svc.ReadSyncConfigsDirs(configDirs)
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
		Skipped: result.Skipped,
	}, nil
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
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemUpdateAllPackagesDB))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemUpdateAllPackagesDB))

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

// enrichWithAppStream подтягивает AppStream данные из отдельной таблицы в пакеты
func (a *Actions) enrichWithAppStream(ctx context.Context, packages []_package.Package) {
	format := a.appConfig.ConfigManager.GetConfig().Format
	if format == app.FormatText {
		return
	}

	names := make([]string, 0, len(packages))
	for i := range packages {
		if packages[i].HasAppStream {
			names = append(names, packages[i].Name)
		}
	}
	if len(names) == 0 {
		return
	}
	compMap, err := a.serviceAppStreamDB.GetByPkgNames(ctx, names)
	if err != nil {
		app.Log.Debugf("enrichWithAppStream: %v", err)
		return
	}
	for i := range packages {
		if comps, ok := compMap[packages[i].Name]; ok {
			packages[i].AppStream = comps
		}
	}
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

// ShortPackageResponse Определяем структуру для короткого представления пакета
type ShortPackageResponse struct {
	Name       string `json:"name"`
	Summary    string `json:"summary"`
	Installed  bool   `json:"installed"`
	Version    string `json:"version"`
	Maintainer string `json:"maintainer"`
}

// FormatPackageOutput принимает данные (один пакет или срез пакетов) и флаг full.
// Если full == true, то возвращается полный вывод, иначе – сокращённый.
func (a *Actions) FormatPackageOutput(data interface{}, full bool) interface{} {
	switch v := data.(type) {
	case _package.Package:
		if full {
			return v
		}
		return ShortPackageResponse{
			Name:       v.Name,
			Summary:    v.Summary,
			Version:    v.Version,
			Installed:  v.Installed,
			Maintainer: v.Maintainer,
		}
	case []_package.Package:
		if full {
			return v
		}
		shortList := make([]ShortPackageResponse, 0, len(v))
		for _, pkg := range v {
			shortList = append(shortList, ShortPackageResponse{
				Name:       pkg.Name,
				Summary:    pkg.Summary,
				Version:    pkg.Version,
				Installed:  pkg.Installed,
				Maintainer: pkg.Maintainer,
			})
		}
		return shortList
	default:
		return nil
	}
}

// GenerateOnlineDoc запускает веб-сервер с HTML документацией для DBus API
func (a *Actions) GenerateOnlineDoc(ctx context.Context) error {
	return startDocServer(ctx)
}
