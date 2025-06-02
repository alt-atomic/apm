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
	"apm/cmd/common/reply"
	"apm/cmd/system/package"
	"apm/cmd/system/service"
	"apm/lib"
	"context"
	"errors"
	"fmt"
	"strings"
	"syscall"
)

// Actions объединяет методы для выполнения системных действий.
type Actions struct {
	serviceHostImage    *service.HostImageService
	serviceAptActions   *_package.Actions
	serviceAptDatabase  *_package.PackageDBService
	serviceHostDatabase *service.HostDBService
	serviceHostConfig   *service.HostConfigService
	serviceAlr          *_package.AlrService
}

// NewActionsWithDeps создаёт новый экземпляр Actions с ручными управлением зависимостями
func NewActionsWithDeps(
	aptDB *_package.PackageDBService,
	aptActions *_package.Actions,
	hostImage *service.HostImageService,
	hostDB *service.HostDBService,
	hostConfig *service.HostConfigService,
) *Actions {
	return &Actions{
		serviceHostImage:    hostImage,
		serviceAptActions:   aptActions,
		serviceAptDatabase:  aptDB,
		serviceHostDatabase: hostDB,
		serviceHostConfig:   hostConfig,
	}
}

// NewActions создаёт новый экземпляр Actions.
func NewActions() *Actions {
	hostPackageDBSvc, err := _package.NewPackageDBService(lib.GetDB(true))
	hostDBSvc, err := service.NewHostDBService(lib.GetDB(true))
	if err != nil {
		lib.Log.Fatal(err)
	}

	hostConfigSvc := service.NewHostConfigService(lib.Env.PathImageFile, hostDBSvc)
	hostImageSvc := service.NewHostImageService(hostConfigSvc)
	hostALRSvc := _package.NewALRService()
	hostAptSvc := _package.NewActions(hostPackageDBSvc, hostALRSvc)

	return &Actions{
		serviceHostImage:    hostImageSvc,
		serviceAptActions:   hostAptSvc,
		serviceAptDatabase:  hostPackageDBSvc,
		serviceHostDatabase: hostDBSvc,
		serviceHostConfig:   hostConfigSvc,
		serviceAlr:          hostALRSvc,
	}
}

type ImageStatus struct {
	Image  service.HostImage `json:"image"`
	Status string            `json:"status"`
	Config service.Config    `json:"config"`
}

// CheckRemove проверяем пакеты перед удалением
func (a *Actions) CheckRemove(ctx context.Context, packages []string) (*reply.APIResponse, error) {
	allPackageNames := strings.Join(packages, " ")
	packageParse, aptErrors := a.serviceAptActions.Check(ctx, allPackageNames, "remove")
	criticalError := _package.FindCriticalError(aptErrors)
	if criticalError != nil {
		return nil, criticalError
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message": lib.T_("Inspection information"),
			"info":    packageParse,
		},
		Error: false,
	}

	return &resp, nil
}

// CheckInstall проверяем пакеты перед установкой
func (a *Actions) CheckInstall(ctx context.Context, packages []string) (*reply.APIResponse, error) {
	allPackageNames := strings.Join(packages, " ")
	packageParse, aptErrors := a.serviceAptActions.Check(ctx, allPackageNames, "install")
	criticalError := _package.FindCriticalError(aptErrors)
	if criticalError != nil {
		return nil, criticalError
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message": lib.T_("Inspection information"),
			"info":    packageParse,
		},
		Error: false,
	}

	return &resp, nil
}

// Remove удаляет системный пакет.
func (a *Actions) Remove(ctx context.Context, packages []string, apply bool) (*reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return nil, err
	}

	err = a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	if len(packages) == 0 {
		errPackageNotFound := fmt.Errorf(lib.T_("At least one package must be specified, for example, remove package"))

		return nil, errPackageNotFound
	}

	var names []string
	var packagesInfo []_package.Package
	for _, pkg := range packages {
		packageInfo, err := a.serviceAptDatabase.GetPackageByName(ctx, pkg)
		if err != nil {
			return nil, err
		}

		packagesInfo = append(packagesInfo, packageInfo)
		names = append(names, packageInfo.Name)
	}

	allPackageNames := strings.Join(names, " ")
	packageParse, aptErrors := a.serviceAptActions.Check(ctx, allPackageNames, "remove")
	criticalError := _package.FindCriticalError(aptErrors)
	if criticalError != nil {
		return nil, criticalError
	}

	// Достанем все кастомные ошибки apt
	var customErrorList []*_package.MatchedError
	for _, err = range aptErrors {
		var matchedErr *_package.MatchedError
		if errors.As(err, &matchedErr) {
			customErrorList = append(customErrorList, matchedErr)
		}
	}

	if packageParse.RemovedCount == 0 {
		messageNothingDo := lib.T_("No candidates for removal found")
		var alreadyRemovedPackages []string

		for _, customError := range customErrorList {
			if customError.Entry.Code == _package.ErrPackageNotInstalled && apply && lib.Env.IsAtomic {
				alreadyRemovedPackages = append(alreadyRemovedPackages, customError.Params[0])
			}
		}

		if apply && lib.Env.IsAtomic {
			diffPackageFound := false
			err = a.serviceHostConfig.LoadConfig()
			if err != nil {
				return nil, err
			}

			for _, removedPkg := range alreadyRemovedPackages {
				if !a.serviceHostConfig.IsRemoved(removedPkg) {
					diffPackageFound = true
					err = a.serviceHostConfig.AddRemovePackage(removedPkg)
					if err != nil {
						return nil, err
					}
				}
			}

			if diffPackageFound {
				err = a.applyChange(ctx, packages, false)
				if err != nil {
					return nil, err
				}

				messageNothingDo += lib.T_(".\nA difference in the package list was found in the local configuration, the image has been updated")
			}
		}

		return nil, fmt.Errorf(messageNothingDo)
	}

	reply.StopSpinner()
	dialogStatus, err := _package.NewDialog(packagesInfo, packageParse, _package.ActionRemove)
	if err != nil {
		return nil, err
	}

	if !dialogStatus {
		errDialog := fmt.Errorf(lib.T_("Cancel dialog"))

		return nil, errDialog
	}

	reply.CreateSpinner()
	errList := a.serviceAptActions.Remove(ctx, allPackageNames)
	criticalError = _package.FindCriticalError(errList)
	if criticalError != nil {
		var matchedErr *_package.MatchedError
		if errors.As(criticalError, &matchedErr) && matchedErr.NeedUpdate() {
			_, err = a.serviceAptActions.Update(ctx)
			if err != nil {
				return nil, err
			}

			errAptRepo := fmt.Errorf(lib.T_("A communication error with the repository occurred. The package list has been updated, please try running the command again"))

			return nil, errAptRepo
		}

		return nil, criticalError
	}

	removePackageNames := strings.Join(packageParse.RemovedPackages, ", ")
	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return nil, err
	}

	messageAnswer := fmt.Sprintf(lib.TN_("%s removed successfully", "%s removed successfully", packageParse.RemovedCount), removePackageNames)
	if apply {
		err = a.applyChange(ctx, packages, false)
		if err != nil {
			return nil, err
		}
		messageAnswer += lib.T_(". The system image has been modified")
	}

	if !apply && lib.Env.IsAtomic {
		messageAnswer += lib.T_(". The system image has not been modified! To apply changes, run with the -a flag")
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message": messageAnswer,
			"info":    packageParse,
		},
		Error: false,
	}

	return &resp, nil
}

// Install осуществляет установку системного пакета.
func (a *Actions) Install(ctx context.Context, packages []string, apply bool) (*reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return nil, err
	}

	err = a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	if len(packages) == 0 {
		errPackageNotFound := fmt.Errorf(lib.T_("You must specify at least one package, for example, remove package"))

		return nil, errPackageNotFound
	}

	isMultiInstall := false
	var packageNames []string
	var packagesInfo []_package.Package
	for _, pkg := range packages {
		originalPkg := pkg
		var packageInfo _package.Package

		packageInfo, err = a.serviceAptDatabase.GetPackageByName(ctx, pkg)
		if err != nil {
			if len(pkg) > 0 {
				lastChar := pkg[len(pkg)-1]
				if lastChar == '+' || lastChar == '-' {
					cleanedPkg := pkg[:len(pkg)-1]
					packageInfo, err = a.serviceAptDatabase.GetPackageByName(ctx, cleanedPkg)
					if err == nil {
						isMultiInstall = true
					}
				}
			}
		}

		if err != nil {
			filters := map[string]interface{}{
				"provides": originalPkg,
			}

			alternativePackages, errFind := a.serviceAptDatabase.QueryHostImagePackages(ctx, filters, "", "", 5, 0)
			if errFind != nil {
				return nil, errFind
			}

			if len(alternativePackages) == 0 {
				errorFindPackage := fmt.Sprintf(lib.T_("Failed to retrieve information about the package %s"), originalPkg)
				return nil, fmt.Errorf(errorFindPackage)
			}

			var altNames []string
			for _, altPkg := range alternativePackages {
				altNames = append(altNames, altPkg.Name)
			}

			message := err.Error() + lib.T_(". Maybe you were looking for: ")

			errPackageNotFound := fmt.Errorf(message+"%s", strings.Join(altNames, " "))

			return nil, errPackageNotFound
		}

		packagesInfo = append(packagesInfo, packageInfo)

		// Обработка ALR-пакетов
		if packageInfo.IsAlr {
			action := "install"
			if len(originalPkg) > 0 {
				lastChar := originalPkg[len(originalPkg)-1]
				if lastChar == '-' {
					action = "remove"
				} else if lastChar == '+' {
					action = "install"
				}
			}

			if action == "install" {
				pkgForPreInstall := originalPkg
				if len(originalPkg) > 0 {
					lastChar := originalPkg[len(originalPkg)-1]
					if lastChar == '+' || lastChar == '-' {
						pkgForPreInstall = originalPkg[:len(originalPkg)-1]
					}
				}

				rpmPath, err := a.serviceAlr.PreInstall(ctx, pkgForPreInstall)
				if err != nil {
					return nil, err
				}
				packageNames = append(packageNames, rpmPath)
			} else {
				packageNames = append(packageNames, originalPkg)
			}
		} else {
			packageNames = append(packageNames, originalPkg)
		}
	}

	allPackageNames := strings.Join(packageNames, " ")
	packageParse, aptErrors := a.serviceAptActions.Check(ctx, allPackageNames, "install")
	criticalError := _package.FindCriticalError(aptErrors)
	if criticalError != nil {
		return nil, criticalError
	}

	// Достанем все кастомные ошибки apt
	var customErrorList []*_package.MatchedError
	for _, err = range aptErrors {
		var matchedErr *_package.MatchedError
		if errors.As(err, &matchedErr) {
			customErrorList = append(customErrorList, matchedErr)
		}
	}

	if len(customErrorList) > 0 && packageParse.NewInstalledCount == 0 && packageParse.UpgradedCount == 0 && packageParse.RemovedCount == 0 {
		messageNothingDo := lib.T_("The operation will not make any changes. Reasons: \n")
		var alreadyInstalledPackages []string
		var alreadyRemovedPackages []string

		for _, customError := range customErrorList {
			if customError.Entry.Code == _package.ErrPackageIsAlreadyNewest && apply && lib.Env.IsAtomic {
				alreadyInstalledPackages = append(alreadyInstalledPackages, customError.Params[0])
			}

			if customError.Entry.Code == _package.ErrPackageNotInstalled && apply && lib.Env.IsAtomic {
				alreadyRemovedPackages = append(alreadyRemovedPackages, customError.Params[0])
			}

			messageNothingDo += customError.Error() + "\n"
		}

		if apply && lib.Env.IsAtomic {
			diffPackageFound := false
			err = a.serviceHostConfig.LoadConfig()
			if err != nil {
				return nil, err
			}

			for _, removedPkg := range alreadyRemovedPackages {
				cleanName := a.serviceAptActions.CleanPackageName(removedPkg, packageNames)
				if !a.serviceHostConfig.IsRemoved(cleanName) {
					diffPackageFound = true
					err = a.serviceHostConfig.AddRemovePackage(cleanName)
					if err != nil {
						return nil, err
					}
				}
			}

			for _, installedPkg := range alreadyInstalledPackages {
				cleanName := a.serviceAptActions.CleanPackageName(installedPkg, packageNames)
				if !a.serviceHostConfig.IsInstalled(cleanName) {
					diffPackageFound = true
					err = a.serviceHostConfig.AddInstallPackage(cleanName)
					if err != nil {
						return nil, err
					}
				}
			}

			if diffPackageFound {
				err = a.applyChange(ctx, packages, true)
				if err != nil {
					return nil, err
				}

				messageNothingDo += lib.T_("Found a discrepancy in the package list in the local configuration, the image has been updated")
			}
		}

		return nil, fmt.Errorf(messageNothingDo)
	}

	reply.StopSpinner()
	dialogAction := _package.ActionInstall
	if isMultiInstall {
		dialogAction = _package.ActionMultiInstall
	}

	dialogStatus, err := _package.NewDialog(packagesInfo, packageParse, dialogAction)
	if err != nil {
		return nil, err
	}

	if !dialogStatus {
		errDialog := fmt.Errorf(lib.T_("Cancel dialog"))

		return nil, errDialog
	}

	reply.CreateSpinner()

	errList := a.serviceAptActions.Install(ctx, allPackageNames)
	criticalError = _package.FindCriticalError(errList)
	if criticalError != nil {
		var matchedErr *_package.MatchedError
		if errors.As(criticalError, &matchedErr) && matchedErr.NeedUpdate() {
			_, err = a.serviceAptActions.Update(ctx)
			if err != nil {
				return nil, err
			}

			errAptRepo := fmt.Errorf(lib.T_("A repository connection error occurred. The package list has been updated, please try running the command again"))

			return nil, errAptRepo
		}

		return nil, criticalError
	}

	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return nil, err
	}

	messageAnswer := fmt.Sprintf(
		"%s %s %s",
		fmt.Sprintf(lib.TN_("%d package successfully installed", "%d packages successfully installed", packageParse.NewInstalledCount), packageParse.NewInstalledCount),
		lib.T_("and"),
		fmt.Sprintf(lib.TN_("%d updated", "%d updated", packageParse.UpgradedCount), packageParse.UpgradedCount),
	)

	if apply {
		err = a.applyChange(ctx, packages, true)
		if err != nil {
			return nil, err
		}

		messageAnswer += lib.T_(". The system image has been changed.")
	}

	if !apply && lib.Env.IsAtomic {
		messageAnswer += lib.T_(". The system image has not been changed! To apply changes, you need to run with the -a flag.")
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message": messageAnswer,
			"info":    packageParse,
		},
		Error: false,
	}

	return &resp, nil
}

// Update обновляет информацию или базу данных пакетов.
func (a *Actions) Update(ctx context.Context) (*reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return nil, err
	}

	err = a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	packages, err := a.serviceAptActions.Update(ctx)
	if err != nil {
		return nil, err
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message": lib.T_("Package list updated successfully"),
			"count":   len(packages),
		},
		Error: false,
	}

	return &resp, nil
}

// Upgrade общее обновление системы
func (a *Actions) Upgrade(ctx context.Context) (*reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return nil, err
	}

	err = a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	_, err = a.serviceAptActions.Update(ctx)
	if err != nil {
		return nil, err
	}

	packageParse, aptErrors := a.serviceAptActions.Check(ctx, "", "dist-upgrade")
	criticalError := _package.FindCriticalError(aptErrors)
	if criticalError != nil {
		return nil, criticalError
	}

	if packageParse.NewInstalledCount == 0 && packageParse.UpgradedCount == 0 && packageParse.RemovedCount == 0 {
		return &reply.APIResponse{
			Data: map[string]interface{}{
				"message": lib.T_("The operation will not make any changes"),
			},
			Error: false,
		}, nil
	}

	reply.StopSpinner()

	dialogStatus, err := _package.NewDialog([]_package.Package{}, packageParse, _package.ActionUpgrade)
	if err != nil {
		return nil, err
	}

	if !dialogStatus {
		errDialog := fmt.Errorf(lib.T_("Cancel dialog"))

		return nil, errDialog
	}

	reply.CreateSpinner()

	errUpgrade := a.serviceAptActions.Upgrade(ctx)
	criticalError = _package.FindCriticalError(errUpgrade)
	if criticalError != nil {
		return nil, criticalError
	}

	messageAnswer := fmt.Sprintf(
		"%s %s %s",
		fmt.Sprintf(lib.TN_("%d package successfully installed", "%d packages successfully installed", packageParse.NewInstalledCount), packageParse.NewInstalledCount),
		lib.T_("and"),
		fmt.Sprintf(lib.TN_("%d updated", "%d updated", packageParse.UpgradedCount), packageParse.UpgradedCount),
	)

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message": lib.T_("The system has been upgrade successfully"),
			"result":  messageAnswer,
		},
		Error: false,
	}

	return &resp, nil
}

// Info возвращает информацию о системном пакете.
func (a *Actions) Info(ctx context.Context, packageName string, isFullFormat bool) (*reply.APIResponse, error) {
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := lib.T_("Package name must be specified, for example info package")
		return nil, fmt.Errorf(errMsg)
	}

	err := a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	packageInfo, err := a.serviceAptDatabase.GetPackageByName(ctx, packageName)
	if err != nil {
		filters := map[string]interface{}{
			"provides": packageName,
		}

		alternativePackages, errFind := a.serviceAptDatabase.QueryHostImagePackages(ctx, filters, "", "", 5, 0)
		if errFind != nil {
			return nil, errFind
		}

		if len(alternativePackages) == 0 {
			errorFindPackage := fmt.Sprintf(lib.T_("Failed to retrieve information about the package %s"), packageName)
			return nil, fmt.Errorf(errorFindPackage)
		}

		var altNames []string
		for _, altPkg := range alternativePackages {
			altNames = append(altNames, altPkg.Name)
		}

		message := err.Error() + lib.T_(". Maybe you were looking for: ")

		errPackageNotFound := fmt.Errorf(message+"%s", strings.Join(altNames, " "))

		return nil, errPackageNotFound
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":     lib.T_("Package found"),
			"packageInfo": a.FormatPackageOutput(packageInfo, isFullFormat),
		},
		Error: false,
	}

	return &resp, nil
}

// ListParams задаёт параметры для запроса списка пакетов.
type ListParams struct {
	Sort        string   `json:"sort"`
	Order       string   `json:"order"`
	Limit       int      `json:"limit"`
	Offset      int      `json:"offset"`
	Filters     []string `json:"filters"`
	ForceUpdate bool     `json:"forceUpdate"`
}

func (a *Actions) List(ctx context.Context, params ListParams, isFullFormat bool) (*reply.APIResponse, error) {
	if params.ForceUpdate {
		_, err := a.serviceAptActions.Update(ctx)
		if err != nil {
			return nil, err
		}
	}
	err := a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	// Формируем фильтры (map[string]interface{})
	filters := make(map[string]interface{})
	for _, filter := range params.Filters {
		filter = strings.TrimSpace(filter)
		if filter == "" {
			continue
		}
		parts := strings.SplitN(filter, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key != "" && value != "" {
			filters[key] = value
		}
	}

	totalCount, err := a.serviceAptDatabase.CountHostImagePackages(ctx, filters)
	if err != nil {
		return nil, err
	}

	packages, err := a.serviceAptDatabase.QueryHostImagePackages(ctx, filters, params.Sort, params.Order, params.Limit, params.Offset)
	if err != nil {
		return nil, err
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf(lib.T_("Nothing found"))
	}

	msg := fmt.Sprintf(lib.TN_("%d record found", "%d records found", len(packages)), len(packages))

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":    msg,
			"packages":   a.FormatPackageOutput(packages, isFullFormat),
			"totalCount": int(totalCount),
		},
		Error: false,
	}

	return &resp, nil
}

// Search осуществляет поиск системного пакета по названию.
func (a *Actions) Search(ctx context.Context, packageName string, installed bool, isFullFormat bool) (*reply.APIResponse, error) {
	err := a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := fmt.Sprintf(lib.T_("You must specify the package name, for example `%s package`"), "search")
		return nil, fmt.Errorf(errMsg)
	}

	packages, err := a.serviceAptDatabase.SearchPackagesByName(ctx, packageName, installed)
	if err != nil {
		return nil, err
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf(lib.T_("Nothing found"))
	}

	msg := fmt.Sprintf(lib.TN_("%d record found", "%d records found", len(packages)), len(packages))

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":  msg,
			"packages": a.FormatPackageOutput(packages, isFullFormat),
		},
		Error: false,
	}

	return &resp, nil
}

// ImageStatus возвращает статус актуального образа
func (a *Actions) ImageStatus(ctx context.Context) (*reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return nil, err
	}

	imageStatus, err := a.getImageStatus(ctx)
	if err != nil {
		return nil, err
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":     lib.T_("Image status"),
			"bootedImage": imageStatus,
		},
		Error: false,
	}

	return &resp, nil
}

// ImageUpdate обновляет образ.
func (a *Actions) ImageUpdate(ctx context.Context) (*reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return nil, err
	}

	err = a.serviceHostConfig.LoadConfig()
	if err != nil {
		return nil, err
	}

	err = a.serviceHostImage.CheckAndUpdateBaseImage(ctx, true, *a.serviceHostConfig.Config)
	if err != nil {
		return nil, err
	}

	imageStatus, err := a.getImageStatus(ctx)
	if err != nil {
		return nil, err
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":     lib.T_("Command executed successfully"),
			"bootedImage": imageStatus,
		},
		Error: false,
	}

	return &resp, nil
}

// ImageApply применить изменения к хосту
func (a *Actions) ImageApply(ctx context.Context) (*reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return nil, err
	}

	err = a.serviceHostConfig.LoadConfig()
	if err != nil {
		return nil, err
	}

	err = a.serviceHostConfig.GenerateDockerfile()
	if err != nil {
		return nil, err
	}

	imageStatus, err := a.getImageStatus(ctx)
	if err != nil {
		return nil, err
	}

	err = a.serviceHostImage.BuildAndSwitch(ctx, false, true)
	if err != nil {
		return nil, err
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":     lib.T_("Changes applied successfully. A reboot is required"),
			"bootedImage": imageStatus,
		},
		Error: false,
	}

	return &resp, nil
}

// ImageHistory история изменений образа
func (a *Actions) ImageHistory(ctx context.Context, imageName string, limit int, offset int) (*reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return nil, err
	}

	history, err := a.serviceHostDatabase.GetImageHistoriesFiltered(ctx, imageName, limit, offset)
	if err != nil {
		return nil, err
	}

	totalCount, err := a.serviceHostDatabase.CountImageHistoriesFiltered(ctx, imageName)
	if err != nil {
		return nil, err
	}

	msg := fmt.Sprintf(lib.TN_("%d record found", "%d records found", len(history)), len(history))

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":    msg,
			"history":    history,
			"totalCount": totalCount,
		},
		Error: false,
	}

	return &resp, nil
}

// checkRoot проверяет, запущен ли установщик от имени root
func (a *Actions) checkRoot() error {
	if syscall.Geteuid() != 0 {
		return fmt.Errorf(lib.T_("Elevated rights are required to perform this action. Please use sudo or su"))
	}

	if lib.Env.IsAtomic {
		err := a.serviceHostImage.EnableOverlay()
		if err != nil {
			return err
		}
	}

	return nil
}

// applyChange применяет изменения к образу системы
func (a *Actions) applyChange(ctx context.Context, packages []string, isInstall bool) error {
	if !lib.Env.IsAtomic {
		return fmt.Errorf(lib.T_("This option is only available for an atomic system"))
	}

	err := a.serviceHostConfig.LoadConfig()
	if err != nil {
		return err
	}

	for _, pkg := range packages {
		if len(pkg) == 0 {
			continue
		}

		originalPkg := pkg
		canonicalPkg := pkg

		if _, errFull := a.serviceAptDatabase.GetPackageByName(ctx, canonicalPkg); errFull != nil {
			for len(canonicalPkg) > 0 && (canonicalPkg[len(canonicalPkg)-1] == '+' || canonicalPkg[len(canonicalPkg)-1] == '-') {
				canonicalPkg = canonicalPkg[:len(canonicalPkg)-1]
				if _, errTmp := a.serviceAptDatabase.GetPackageByName(ctx, canonicalPkg); errTmp == nil {
					break
				}
			}
		}

		if originalPkg[len(originalPkg)-1] == '+' {
			err = a.serviceHostConfig.AddInstallPackage(canonicalPkg)
		} else if originalPkg[len(originalPkg)-1] == '-' {
			err = a.serviceHostConfig.AddRemovePackage(canonicalPkg)
		} else {
			if isInstall {
				err = a.serviceHostConfig.AddInstallPackage(canonicalPkg)
			} else {
				err = a.serviceHostConfig.AddRemovePackage(canonicalPkg)
			}
		}
		if err != nil {
			return err
		}
	}

	err = a.serviceHostConfig.GenerateDockerfile()
	if err != nil {
		return err
	}

	err = a.serviceHostImage.BuildAndSwitch(ctx, false, false)
	if err != nil {
		return err
	}

	return nil
}

// validateDB проверяет, существует ли база данных
func (a *Actions) validateDB(ctx context.Context) error {
	if err := a.serviceAptDatabase.PackageDatabaseExist(ctx); err != nil {
		err = a.checkRoot()
		if err != nil {
			return err
		}

		_, err = a.serviceAptActions.Update(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// updateAllPackagesDB обновляет состояние всех пакетов в базе данных
func (a *Actions) updateAllPackagesDB(ctx context.Context) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.updateAllPackagesDB"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.updateAllPackagesDB"))

	installedPackages, err := a.serviceAptActions.GetInstalledPackages(ctx)
	if err != nil {
		return err
	}

	err = a.serviceAptDatabase.SyncPackageInstallationInfo(ctx, installedPackages)
	if err != nil {
		return err
	}

	return nil
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
			Status: lib.T_("Modified image. Configuration file: ") + lib.Env.PathImageFile,
			Image:  hostImage,
			Config: *a.serviceHostConfig.Config,
		}, nil
	}

	return ImageStatus{
		Status: lib.T_("Cloud image without changes"),
		Image:  hostImage,
		Config: *a.serviceHostConfig.Config,
	}, nil
}

// ShortPackageResponse Определяем структуру для короткого представления пакета
type ShortPackageResponse struct {
	Name        string `json:"name"`
	Installed   bool   `json:"installed"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// FormatPackageOutput принимает данные (один пакет или срез пакетов) и флаг full.
// Если full == true, то возвращается полный вывод, иначе – сокращённый.
func (a *Actions) FormatPackageOutput(data interface{}, full bool) interface{} {
	switch v := data.(type) {
	// Если передан один пакет
	case _package.Package:
		if full {
			return v
		}
		return ShortPackageResponse{
			Name:        v.Name,
			Version:     v.Version,
			Installed:   v.Installed,
			Description: v.Description,
		}
	// Если передан срез пакетов
	case []_package.Package:
		if full {
			return v
		}
		shortList := make([]ShortPackageResponse, 0, len(v))
		for _, pkg := range v {
			shortList = append(shortList, ShortPackageResponse{
				Name:        pkg.Name,
				Version:     pkg.Version,
				Installed:   pkg.Installed,
				Description: pkg.Description,
			})
		}
		return shortList
	default:
		return nil
	}
}
