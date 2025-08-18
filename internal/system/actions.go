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
	"apm/internal/common/apt"
	"apm/internal/common/reply"
	_package "apm/internal/system/package"
	"apm/internal/system/service"
	"apm/lib"
	"context"
	"errors"
	"fmt"
	"strings"
	"syscall"
)

// Actions объединяет методы для выполнения системных действий.
type Actions struct {
	serviceHostImage       *service.HostImageService
	serviceAptActions      *_package.Actions
	serviceAptDatabase     *_package.PackageDBService
	serviceHostDatabase    *service.HostDBService
	serviceHostConfig      *service.HostConfigService
	serviceTemporaryConfig *service.TemporaryConfigService
	serviceStplr           *_package.StplrService
}

// NewActionsWithDeps создаёт новый экземпляр Actions с ручными управлением зависимостями
func NewActionsWithDeps(
	aptDB *_package.PackageDBService,
	aptActions *_package.Actions,
	hostImage *service.HostImageService,
	hostDB *service.HostDBService,
	hostConfig *service.HostConfigService,
	temporaryConfig *service.TemporaryConfigService,
) *Actions {
	return &Actions{
		serviceHostImage:       hostImage,
		serviceAptActions:      aptActions,
		serviceAptDatabase:     aptDB,
		serviceHostDatabase:    hostDB,
		serviceHostConfig:      hostConfig,
		serviceTemporaryConfig: temporaryConfig,
	}
}

// NewActions создаёт новый экземпляр Actions.
func NewActions() *Actions {
	hostPackageDBSvc, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		lib.Log.Fatal(err)
	}
	hostDBSvc, err := service.NewHostDBService(lib.GetDB(true))
	if err != nil {
		lib.Log.Fatal(err)
	}

	hostConfigSvc := service.NewHostConfigService(lib.Env.PathImageFile, hostDBSvc)
	hostTemporarySvc := service.NewTemporaryConfigService("/tmp/apm.tmp")
	hostImageSvc := service.NewHostImageService(hostConfigSvc)
	hostALRSvc := _package.NewSTPLRService()
	hostAptSvc := _package.NewActions(hostPackageDBSvc, hostALRSvc)

	return &Actions{
		serviceHostImage:       hostImageSvc,
		serviceAptActions:      hostAptSvc,
		serviceAptDatabase:     hostPackageDBSvc,
		serviceHostDatabase:    hostDBSvc,
		serviceHostConfig:      hostConfigSvc,
		serviceStplr:           hostALRSvc,
		serviceTemporaryConfig: hostTemporarySvc,
	}
}

type ImageStatus struct {
	Image  service.HostImage `json:"image"`
	Status string            `json:"status"`
	Config service.Config    `json:"config"`
}

// CheckRemove проверяем пакеты перед удалением
func (a *Actions) CheckRemove(ctx context.Context, packages []string, purge bool) (*reply.APIResponse, error) {
	packageParse, aptError := a.serviceAptActions.CheckRemove(ctx, packages, purge)
	if aptError != nil {
		return nil, aptError
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

// UpdateKernel обновление ядра
func (a *Actions) UpdateKernel(ctx context.Context) (*reply.APIResponse, error) {
	if lib.Env.IsAtomic {
		return nil, errors.New(lib.T_("This option is available only for a non-atomic system"))
	}

	_, err := a.serviceAptActions.Update(ctx)
	if err != nil {
		return nil, err
	}

	packageParse, checkErrs := a.serviceAptActions.CheckUpdateKernel(ctx)
	criticalError := apt.FindCriticalError(checkErrs)
	if criticalError != nil {
		return nil, criticalError
	}

	runErrs := a.serviceAptActions.UpdateKernel(ctx)
	if critical := apt.FindCriticalError(runErrs); critical != nil {
		return nil, critical
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message": lib.T_("Kernel update finished"),
			"info":    packageParse,
		},
		Error: false,
	}

	return &resp, nil
}

// CheckUpdateKernel проверяем обновление ядра
func (a *Actions) CheckUpdateKernel(ctx context.Context) (*reply.APIResponse, error) {
	if lib.Env.IsAtomic {
		return nil, errors.New(lib.T_("This option is available only for a non-atomic system"))
	}

	packageParse, aptErrors := a.serviceAptActions.CheckUpdateKernel(ctx)
	criticalError := apt.FindCriticalError(aptErrors)
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

// CheckUpgrade проверяем пакеты перед обновлением системы
func (a *Actions) CheckUpgrade(ctx context.Context) (*reply.APIResponse, error) {
	packageParse, aptError := a.serviceAptActions.CheckUpgrade(ctx)
	if aptError != nil {
		return nil, aptError
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
	packageParse, aptError := a.serviceAptActions.CheckInstall(ctx, packages)
	if aptError != nil {
		return nil, aptError
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
func (a *Actions) Remove(ctx context.Context, packages []string, purge bool) (*reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return nil, err
	}

	err = a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	if len(packages) == 0 {
		errPackageNotFound := errors.New(lib.T_("At least one package must be specified"))

		return nil, errPackageNotFound
	}

	_, packageNames, packagesInfo, packageParse, errFind := a.serviceAptActions.FindPackage(ctx, []string{}, packages, purge)
	if errFind != nil {
		return nil, errFind
	}

	if packageParse.RemovedCount == 0 {
		messageNothingDo := lib.T_("No candidates for removal found")
		return nil, errors.New(messageNothingDo)
	}

	reply.StopSpinnerForDialog()
	dialogStatus, err := _package.NewDialog(packagesInfo, *packageParse, _package.ActionRemove)
	if err != nil {
		return nil, err
	}

	if !dialogStatus {
		errDialog := errors.New(lib.T_("Cancel dialog"))

		return nil, errDialog
	}

	reply.CreateSpinner()
	err = a.serviceAptActions.Remove(ctx, packageNames, purge)
	if err != nil {
		return nil, err
	}

	removePackageNames := strings.Join(packageParse.RemovedPackages, ", ")
	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return nil, err
	}

	messageAnswer := fmt.Sprintf(lib.TN_("%s removed successfully", "%s removed successfully", packageParse.RemovedCount), removePackageNames)

	if lib.Env.IsAtomic {
		messageAnswer += lib.T_(". The system image has not been changed. To apply the changes, run: apm s image apply")
		errSave := a.saveChange([]string{}, packageNames)
		if errSave != nil {
			return nil, errSave
		}
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
func (a *Actions) Install(ctx context.Context, packages []string) (*reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return nil, err
	}

	err = a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	if len(packages) == 0 {
		errPackageNotFound := errors.New(lib.T_("You must specify at least one package"))
		return nil, errPackageNotFound
	}

	packagesInstall, packagesRemove, errPrepare := a.serviceAptActions.PrepareInstallPackages(ctx, packages)
	if errPrepare != nil {
		return nil, errPrepare
	}

	packagesInstall, packagesRemove, packagesInfo, packageParse, errFind := a.serviceAptActions.FindPackage(ctx, packagesInstall, packagesRemove, false)
	if errFind != nil {
		return nil, errFind
	}

	if packageParse.NewInstalledCount == 0 && packageParse.UpgradedCount == 0 && packageParse.RemovedCount == 0 {
		return nil, errors.New(lib.T_("The operation will not make any changes"))
	}

	if len(packagesInfo) > 0 {
		reply.StopSpinnerForDialog()

		action := _package.ActionInstall
		if packageParse.RemovedCount > 0 {
			action = _package.ActionMultiInstall
		}

		dialogStatus, errDialog := _package.NewDialog(packagesInfo, *packageParse, action)
		if errDialog != nil {
			return nil, errDialog
		}

		if !dialogStatus {
			errDialog = errors.New(lib.T_("Cancel dialog"))

			return nil, errDialog
		}

		reply.CreateSpinner()
	}

	err = a.serviceAptActions.AptUpdate(ctx)
	if err != nil {
		return nil, err
	}

	errInstall := a.serviceAptActions.CombineInstallRemovePackages(ctx, packagesInstall, packagesRemove)
	if errInstall != nil {
		var matchedErr *apt.MatchedError
		if errors.As(errInstall, &matchedErr) && matchedErr.NeedUpdate() {
			_, err = a.serviceAptActions.Update(ctx)
			if err != nil {
				return nil, err
			}

			errAptRepo := errors.New(lib.T_("A repository connection error occurred. The package list has been updated, please try running the command again"))

			return nil, errAptRepo
		}

		return nil, errInstall
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

	if lib.Env.IsAtomic {
		messageAnswer += lib.T_(". The system image has not been changed. To apply the changes, run: apm s image apply")
		errSave := a.saveChange(packagesInstall, packagesRemove)
		if errSave != nil {
			return nil, errSave
		}
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

	packageParse, aptError := a.serviceAptActions.CheckUpgrade(ctx)
	if aptError != nil {
		return nil, aptError
	}

	if packageParse.NewInstalledCount == 0 && packageParse.UpgradedCount == 0 && packageParse.RemovedCount == 0 {
		return &reply.APIResponse{
			Data: map[string]interface{}{
				"message": lib.T_("The operation will not make any changes"),
			},
			Error: false,
		}, nil
	}

	reply.StopSpinnerForDialog()

	dialogStatus, err := _package.NewDialog([]_package.Package{}, *packageParse, _package.ActionUpgrade)
	if err != nil {
		return nil, err
	}

	if !dialogStatus {
		errDialog := errors.New(lib.T_("Cancel dialog"))

		return nil, errDialog
	}

	reply.CreateSpinner()

	errUpgrade := a.serviceAptActions.Upgrade(ctx)
	if errUpgrade != nil {
		return nil, errUpgrade
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
	//packageName = helper.CleanPackageName(packageName)
	if packageName == "" {
		errMsg := lib.T_("Package name must be specified, for example info package")
		return nil, errors.New(errMsg)
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
			return nil, errors.New(errorFindPackage)
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
		return nil, errors.New(lib.T_("Nothing found"))
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

// GetFilterFields возвращает список свойств для фильтрации по названию контейнера. Метод для DBUS
func (a *Actions) GetFilterFields(ctx context.Context) (*reply.APIResponse, error) {
	if err := a.validateDB(ctx); err != nil {
		return nil, err
	}

	fieldList := _package.AllowedFilterFields

	type FilterField struct {
		Name string                          `json:"name"`
		Text string                          `json:"text"`
		Info map[_package.PackageType]string `json:"info"`
		Type string                          `json:"type"`
	}

	var fields []FilterField

	for _, field := range fieldList {
		ff := FilterField{
			Name: field,
			Text: reply.TranslateKey(field),
			Type: "STRING",
		}

		switch field {
		case "size":
			ff.Type = "INTEGER"
		case "installedSize":
			ff.Type = "INTEGER"
		case "installed", "isApp":
			ff.Type = "BOOL"

		case "typePackage":
			ff.Type = "ENUM"
			ff.Info = map[_package.PackageType]string{
				_package.PackageTypeSystem: lib.T_("System package"),
				_package.PackageTypeStplr:  lib.T_("Stplr package"),
			}
		}

		fields = append(fields, ff)
	}

	return &reply.APIResponse{
		Data:  fields,
		Error: false,
	}, nil
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
		return nil, errors.New(errMsg)
	}

	packages, err := a.serviceAptDatabase.SearchPackagesByNameLike(ctx, "%"+packageName+"%", installed)
	if err != nil {
		return nil, err
	}

	if len(packages) == 0 {
		return nil, errors.New(lib.T_("Nothing found"))
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

	err = a.serviceTemporaryConfig.LoadConfig()
	if err != nil {
		return nil, err
	}

	err = a.serviceHostConfig.LoadConfig()
	if err != nil {
		return nil, err
	}

	if len(a.serviceTemporaryConfig.Config.Packages.Install) > 0 || len(a.serviceTemporaryConfig.Config.Packages.Remove) > 0 {
		reply.StopSpinnerForDialog()
		// Показываем диалог выбора пакетов
		result, errDialog := _package.NewPackageSelectionDialog(
			a.serviceTemporaryConfig.Config.Packages.Install,
			a.serviceTemporaryConfig.Config.Packages.Remove,
		)
		if errDialog != nil {
			return nil, errDialog
		}

		if result.Canceled {
			errDialog = errors.New(lib.T_("Cancel dialog"))
			return nil, errDialog
		}

		reply.CreateSpinner()
		for _, pkg := range result.InstallPackages {
			err = a.serviceHostConfig.AddInstallPackage(pkg)
			if err != nil {
				return nil, err
			}
		}
		for _, pkg := range result.RemovePackages {
			err = a.serviceHostConfig.AddRemovePackage(pkg)
			if err != nil {
				return nil, err
			}
		}
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

	_ = a.serviceTemporaryConfig.DeleteFile()

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

// ImageGetConfig получить конфиг
func (a *Actions) ImageGetConfig() (*reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return nil, err
	}

	err = a.serviceHostConfig.LoadConfig()
	if err != nil {
		return nil, err
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"config": a.serviceHostConfig.Config,
		},
		Error: false,
	}

	return &resp, nil
}

// ImageSaveConfig сохранить конфиг
func (a *Actions) ImageSaveConfig(config service.Config) (*reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return nil, err
	}

	err = a.serviceHostConfig.LoadConfig()
	if err != nil {
		return nil, err
	}

	a.serviceHostConfig.Config = &config

	err = a.serviceHostConfig.SaveConfig()
	if err != nil {
		return nil, err
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"config": a.serviceHostConfig.Config,
		},
		Error: false,
	}

	return &resp, nil
}

// checkRoot проверяет, запущен ли установщик от имени root
func (a *Actions) checkRoot() error {
	if syscall.Geteuid() != 0 {
		return errors.New(lib.T_("Elevated rights are required to perform this action. Please use sudo or su"))
	}

	if lib.Env.IsAtomic {
		err := a.serviceHostImage.EnableOverlay()
		if err != nil {
			return err
		}
	}

	return nil
}

// saveChange применяет изменения к образу системы
func (a *Actions) saveChange(packagesInstall []string, packagesRemove []string) error {
	if !lib.Env.IsAtomic {
		return errors.New(lib.T_("This option is only available for an atomic system"))
	}

	if err := a.serviceTemporaryConfig.LoadConfig(); err != nil {
		return err
	}

	// Вспомогательная функция для обработки списка пакетов
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

	// Обрабатываем пакеты на установку
	if err := processPackages(packagesInstall, a.serviceTemporaryConfig.AddInstallPackage); err != nil {
		return err
	}

	// Обрабатываем пакеты на удаление
	if err := processPackages(packagesRemove, a.serviceTemporaryConfig.AddRemovePackage); err != nil {
		return err
	}

	return a.serviceTemporaryConfig.SaveConfig()
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
