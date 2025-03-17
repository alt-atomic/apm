package system

import (
	"apm/cmd/common/helper"
	"apm/cmd/common/reply"
	"apm/cmd/system/apt"
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
	serviceAptActions   *apt.Actions
	serviceAptDatabase  *apt.PackageDBService
	serviceHostDatabase *service.HostDBService
	serviceHostConfig   *service.HostConfigService
}

// NewActionsWithDeps создаёт новый экземпляр Actions с ручными управлением зависимостями
func NewActionsWithDeps(
	aptDB *apt.PackageDBService,
	aptActions *apt.Actions,
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
	hostPackageDBSvc := apt.NewPackageDBService(lib.GetDB())
	hostDBSvc := service.NewHostDBService(lib.GetDB())
	hostConfigSvc := service.NewHostConfigService(lib.Env.PathImageFile, hostDBSvc)
	hostImageSvc := service.NewHostImageService(hostConfigSvc)
	hostAptSvc := apt.NewActions(hostPackageDBSvc)

	return &Actions{
		serviceHostImage:    hostImageSvc,
		serviceAptActions:   hostAptSvc,
		serviceAptDatabase:  hostPackageDBSvc,
		serviceHostDatabase: hostDBSvc,
		serviceHostConfig:   hostConfigSvc,
	}
}

type ImageStatus struct {
	Image  service.HostImage `json:"image"`
	Status string            `json:"status"`
	Config service.Config    `json:"config"`
}

// CheckRemove проверяем пакеты перед удалением
func (a *Actions) CheckRemove(ctx context.Context, packages []string) (reply.APIResponse, error) {
	allPackageNames := strings.Join(packages, " ")
	packageParse, aptErrors := a.serviceAptActions.Check(ctx, allPackageNames, "remove")
	criticalError := apt.FindCriticalError(aptErrors)
	if criticalError != nil {
		return a.newErrorResponse(criticalError.Error()), criticalError
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": "Информация о проверке",
			"info":    packageParse,
		},
		Error: false,
	}, nil
}

// CheckInstall проверяем пакеты перед установкой
func (a *Actions) CheckInstall(ctx context.Context, packages []string) (reply.APIResponse, error) {
	allPackageNames := strings.Join(packages, " ")
	packageParse, aptErrors := a.serviceAptActions.Check(ctx, allPackageNames, "install")
	criticalError := apt.FindCriticalError(aptErrors)
	if criticalError != nil {
		return a.newErrorResponse(criticalError.Error()), criticalError
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": "Информация о проверке",
			"info":    packageParse,
		},
		Error: false,
	}, nil
}

// Remove удаляет системный пакет.
func (a *Actions) Remove(ctx context.Context, packages []string, apply bool) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	err = a.validateDB(ctx)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if len(packages) == 0 {
		errPackageNotFound := fmt.Errorf("необходимо указать хотя бы один пакет, например remove package")

		return a.newErrorResponse(errPackageNotFound.Error()), errPackageNotFound
	}

	var names []string
	var packagesInfo []apt.Package
	for _, pkg := range packages {
		packageInfo, err := a.serviceAptDatabase.GetPackageByName(ctx, pkg)
		if err != nil {
			return a.newErrorResponse(err.Error()), err
		}

		packagesInfo = append(packagesInfo, packageInfo)
		names = append(names, packageInfo.Name)
	}

	allPackageNames := strings.Join(names, " ")
	packageParse, aptErrors := a.serviceAptActions.Check(ctx, allPackageNames, "remove")
	criticalError := apt.FindCriticalError(aptErrors)
	if criticalError != nil {
		return a.newErrorResponse(criticalError.Error()), criticalError
	}

	// Достанем все кастомные ошибки apt
	var customErrorList []*apt.MatchedError
	for _, err = range aptErrors {
		var matchedErr *apt.MatchedError
		if errors.As(err, &matchedErr) {
			customErrorList = append(customErrorList, matchedErr)
		}
	}

	if packageParse.RemovedCount == 0 {
		messageNothingDo := "Кандидатов на удаление не найдено"
		var alreadyRemovedPackages []string

		for _, customError := range customErrorList {
			if customError.Entry.Code == apt.ErrPackageNotInstalled && apply && lib.Env.IsAtomic {
				alreadyRemovedPackages = append(alreadyRemovedPackages, customError.Params[0])
			}
		}

		if apply && lib.Env.IsAtomic {
			diffPackageFound := false
			err = a.serviceHostConfig.LoadConfig()
			if err != nil {
				return newErrorResponse(err.Error()), nil
			}

			for _, removedPkg := range alreadyRemovedPackages {
				if !a.serviceHostConfig.IsRemoved(removedPkg) {
					diffPackageFound = true
					err = a.serviceHostConfig.AddRemovePackage(removedPkg)
					if err != nil {
						return newErrorResponse(err.Error()), nil
					}
				}
			}

			if diffPackageFound {
				err = a.applyChange(ctx, packages, false)
				if err != nil {
					return a.newErrorResponse(err.Error()), err
				}

				messageNothingDo += ".\nНайдено отличие списка пакетов в локальной конфигурации, образ был обновлён"
			}
		}

		return a.newErrorResponse(messageNothingDo), fmt.Errorf(messageNothingDo)
	}

	reply.StopSpinner()
	dialogStatus, err := apt.NewDialog(packagesInfo, packageParse, apt.ActionRemove)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if !dialogStatus {
		errDialog := fmt.Errorf("отмена диалога удаления")

		return a.newErrorResponse(errDialog.Error()), errDialog
	}

	reply.CreateSpinner()
	errList := a.serviceAptActions.Remove(ctx, allPackageNames)
	criticalError = apt.FindCriticalError(errList)
	if criticalError != nil {
		var matchedErr *apt.MatchedError
		if errors.As(criticalError, &matchedErr) && matchedErr.NeedUpdate() {
			_, err = a.serviceAptActions.Update(ctx)
			if err != nil {
				return newErrorResponse(err.Error()), err
			}

			errAptRepo := fmt.Errorf("возникла ошибка связи с репозиторием. Был актуализирован список пакетов, попробуйте повторно вызвать команду")

			return a.newErrorResponse(errAptRepo.Error()), errAptRepo
		}

		return a.newErrorResponse(criticalError.Error()), criticalError
	}

	removePackageNames := strings.Join(packageParse.RemovedPackages, ",")
	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	messageAnswer := fmt.Sprintf("%s успешно %s",
		removePackageNames,
		helper.DeclOfNum(packageParse.RemovedCount, []string{"удалён", "удалены", "удалены"}))
	if apply {
		err = a.applyChange(ctx, packages, false)
		if err != nil {
			return a.newErrorResponse(err.Error()), err
		}
		messageAnswer += ". Образ системы был изменён"
	}

	if !apply && lib.Env.IsAtomic {
		messageAnswer += ". Образ системы не был изменён! Для применения изменений необходим запуск с флагом -a"
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": messageAnswer,
			"info":    packageParse,
		},
		Error: false,
	}, nil
}

// Install осуществляет установку системного пакета.
func (a *Actions) Install(ctx context.Context, packages []string, apply bool) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	err = a.validateDB(ctx)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if len(packages) == 0 {
		errPackageNotFound := fmt.Errorf("необходимо указать хотя бы один пакет, например remove package")

		return a.newErrorResponse(errPackageNotFound.Error()), errPackageNotFound
	}

	isMultiInstall := false
	var packageNames []string
	var packagesInfo []apt.Package
	for _, pkg := range packages {
		originalPkg := pkg
		var packageInfo apt.Package

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
				return a.newErrorResponse(errFind.Error()), errFind
			}

			if len(alternativePackages) == 0 {
				errorFindPackage := fmt.Sprintf("не удалось получить информацию о пакете %s", originalPkg)
				return a.newErrorResponse(errorFindPackage), fmt.Errorf(errorFindPackage)
			}

			var altNames []string
			for _, altPkg := range alternativePackages {
				altNames = append(altNames, altPkg.Name)
			}

			message := err.Error() + ". Может быть, Вы искали: "

			errPackageNotFound := fmt.Errorf(message+"%s", strings.Join(altNames, " "))

			return a.newErrorResponse(errPackageNotFound.Error()), errPackageNotFound
		}
		packagesInfo = append(packagesInfo, packageInfo)
		packageNames = append(packageNames, originalPkg)
	}

	allPackageNames := strings.Join(packageNames, " ")
	packageParse, aptErrors := a.serviceAptActions.Check(ctx, allPackageNames, "install")
	criticalError := apt.FindCriticalError(aptErrors)
	if criticalError != nil {
		return a.newErrorResponse(criticalError.Error()), criticalError
	}

	// Достанем все кастомные ошибки apt
	var customErrorList []*apt.MatchedError
	for _, err = range aptErrors {
		var matchedErr *apt.MatchedError
		if errors.As(err, &matchedErr) {
			customErrorList = append(customErrorList, matchedErr)
		}
	}

	if len(customErrorList) > 0 && packageParse.NewInstalledCount == 0 && packageParse.UpgradedCount == 0 && packageParse.RemovedCount == 0 {
		messageNothingDo := "Операция не выполнит никаких изменений. Причины: \n"
		var alreadyInstalledPackages []string
		var alreadyRemovedPackages []string

		for _, customError := range customErrorList {
			if customError.Entry.Code == apt.ErrPackageIsAlreadyNewest && apply && lib.Env.IsAtomic {
				alreadyInstalledPackages = append(alreadyInstalledPackages, customError.Params[0])
			}

			if customError.Entry.Code == apt.ErrPackageNotInstalled && apply && lib.Env.IsAtomic {
				alreadyRemovedPackages = append(alreadyRemovedPackages, customError.Params[0])
			}

			messageNothingDo += customError.Error() + "\n"
		}

		if apply && lib.Env.IsAtomic {
			diffPackageFound := false
			err = a.serviceHostConfig.LoadConfig()
			if err != nil {
				return newErrorResponse(err.Error()), nil
			}

			for _, removedPkg := range alreadyRemovedPackages {
				cleanName := a.serviceAptActions.CleanPackageName(removedPkg, packageNames)
				if !a.serviceHostConfig.IsRemoved(cleanName) {
					diffPackageFound = true
					err = a.serviceHostConfig.AddRemovePackage(cleanName)
					if err != nil {
						return newErrorResponse(err.Error()), nil
					}
				}
			}

			for _, installedPkg := range alreadyInstalledPackages {
				cleanName := a.serviceAptActions.CleanPackageName(installedPkg, packageNames)
				if !a.serviceHostConfig.IsInstalled(cleanName) {
					diffPackageFound = true
					err = a.serviceHostConfig.AddInstallPackage(cleanName)
					if err != nil {
						return newErrorResponse(err.Error()), nil
					}
				}
			}

			if diffPackageFound {
				err = a.applyChange(ctx, packages, true)
				if err != nil {
					return a.newErrorResponse(err.Error()), err
				}

				messageNothingDo += "Найдено отличие списка пакетов в локальной конфигурации, образ был обновлён"
			}
		}

		return a.newErrorResponse(messageNothingDo), fmt.Errorf(messageNothingDo)
	}

	reply.StopSpinner()
	dialogAction := apt.ActionInstall
	if isMultiInstall {
		dialogAction = apt.ActionMultiInstall
	}

	dialogStatus, err := apt.NewDialog(packagesInfo, packageParse, dialogAction)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if !dialogStatus {
		errDialog := fmt.Errorf("отмена диалога удаления")

		return a.newErrorResponse(errDialog.Error()), errDialog
	}

	reply.CreateSpinner()

	errList := a.serviceAptActions.Install(ctx, allPackageNames)
	criticalError = apt.FindCriticalError(errList)
	if criticalError != nil {
		var matchedErr *apt.MatchedError
		if errors.As(criticalError, &matchedErr) && matchedErr.NeedUpdate() {
			_, err = a.serviceAptActions.Update(ctx)
			if err != nil {
				return newErrorResponse(err.Error()), err
			}

			errAptRepo := fmt.Errorf("возникла ошибка связи с репозиторием. Был актуализирован список пакетов, попробуйте повторно вызвать команду")

			return a.newErrorResponse(errAptRepo.Error()), errAptRepo
		}

		return a.newErrorResponse(criticalError.Error()), criticalError
	}

	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	messageAnswer := fmt.Sprintf("%d %s успешно %s и %d %s",
		packageParse.NewInstalledCount,
		helper.DeclOfNum(packageParse.NewInstalledCount, []string{"пакет", "пакета", "пакетов"}),
		helper.DeclOfNum(packageParse.NewInstalledCount, []string{"установлен", "установлено", "установлены"}),
		packageParse.UpgradedCount,
		helper.DeclOfNum(packageParse.UpgradedCount, []string{"обновлён", "обновлено", "обновилось"}))

	if apply {
		err = a.applyChange(ctx, packageNames, true)
		if err != nil {
			return a.newErrorResponse(err.Error()), err
		}

		messageAnswer += ". Образ системы был изменён"
	}

	if !apply && lib.Env.IsAtomic {
		messageAnswer += ". Образ системы не был изменён! Для применения изменений необходим запуск с флагом -a"
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": messageAnswer,
			"info":    packageParse,
		},
		Error: false,
	}, nil
}

// Update обновляет информацию или базу данных пакетов.
func (a *Actions) Update(ctx context.Context) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	err = a.validateDB(ctx)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	packages, err := a.serviceAptActions.Update(ctx)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": "Список пакетов успешно обновлён",
			"count":   len(packages),
		},
		Error: false,
	}, nil
}

// Info возвращает информацию о системном пакете.
func (a *Actions) Info(ctx context.Context, packageName string, isFullFormat bool) (reply.APIResponse, error) {
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := "необходимо указать название пакета, например info package"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}

	err := a.validateDB(ctx)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	packageInfo, err := a.serviceAptDatabase.GetPackageByName(ctx, packageName)
	if err != nil {
		filters := map[string]interface{}{
			"provides": packageName,
		}

		alternativePackages, errFind := a.serviceAptDatabase.QueryHostImagePackages(ctx, filters, "", "", 5, 0)
		if errFind != nil {
			return a.newErrorResponse(err.Error()), errFind
		}

		if len(alternativePackages) == 0 {
			errorFindPackage := fmt.Sprintf("не удалось получить информацию о пакете %s", packageName)
			return a.newErrorResponse(errorFindPackage), fmt.Errorf(errorFindPackage)
		}

		var altNames []string
		for _, altPkg := range alternativePackages {
			altNames = append(altNames, altPkg.Name)
		}

		message := err.Error() + ". Может быть, Вы искали: "

		errPackageNotFound := fmt.Errorf(message+"%s", strings.Join(altNames, " "))

		return a.newErrorResponse(errPackageNotFound.Error()), errPackageNotFound
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":     "Найден пакет",
			"packageInfo": a.FormatPackageOutput(packageInfo, isFullFormat),
		},
		Error: false,
	}, nil
}

// ListParams задаёт параметры для запроса списка пакетов.
type ListParams struct {
	Sort        string   `json:"sort"`
	Order       string   `json:"order"`
	Limit       int64    `json:"limit"`
	Offset      int64    `json:"offset"`
	Filters     []string `json:"filters"`
	ForceUpdate bool     `json:"forceUpdate"`
}

func (a *Actions) List(ctx context.Context, params ListParams, isFullFormat bool) (reply.APIResponse, error) {
	if params.ForceUpdate {
		_, err := a.serviceAptActions.Update(ctx)
		if err != nil {
			return a.newErrorResponse(err.Error()), err
		}
	}
	err := a.validateDB(ctx)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
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
		return a.newErrorResponse(err.Error()), err
	}

	packages, err := a.serviceAptDatabase.QueryHostImagePackages(ctx, filters, params.Sort, params.Order, params.Limit, params.Offset)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if len(packages) == 0 {
		return a.newErrorResponse("ничего не найдено"), fmt.Errorf("ничего не найдено")
	}

	msg := fmt.Sprintf(
		"%s %d %s",
		helper.DeclOfNum(len(packages), []string{"Найдена", "Найдено", "Найдены"}),
		len(packages),
		helper.DeclOfNum(len(packages), []string{"запись", "записи", "записей"}),
	)

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":    msg,
			"packages":   a.FormatPackageOutput(packages, isFullFormat),
			"totalCount": int(totalCount),
		},
		Error: false,
	}, nil
}

// Search осуществляет поиск системного пакета по названию.
func (a *Actions) Search(ctx context.Context, packageName string, installed bool, isFullFormat bool) (reply.APIResponse, error) {
	err := a.validateDB(ctx)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := "необходимо указать название пакета, например search package"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}

	packages, err := a.serviceAptDatabase.SearchPackagesByName(ctx, packageName, installed)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if len(packages) == 0 {
		return a.newErrorResponse("Ничего не найдено"), fmt.Errorf("ничего не найдено")
	}

	msg := fmt.Sprintf(
		"%s %d %s",
		helper.DeclOfNum(len(packages), []string{"Найдена", "Найдено", "Найдены"}),
		len(packages),
		helper.DeclOfNum(len(packages), []string{"запись", "записи", "записей"}),
	)

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":  msg,
			"packages": a.FormatPackageOutput(packages, isFullFormat),
		},
		Error: false,
	}, nil
}

// ImageStatus возвращает статус актуального образа
func (a *Actions) ImageStatus(ctx context.Context) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	imageStatus, err := a.getImageStatus(ctx)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":     "Статус образа",
			"bootedImage": imageStatus,
		},
		Error: false,
	}, nil
}

// ImageUpdate обновляет образ.
func (a *Actions) ImageUpdate(ctx context.Context) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	err = a.serviceHostConfig.LoadConfig()
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	err = a.serviceHostImage.CheckAndUpdateBaseImage(ctx, true, *a.serviceHostConfig.Config)
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	imageStatus, err := a.getImageStatus(ctx)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":     "Команда успешно выполнена",
			"bootedImage": imageStatus,
		},
		Error: false,
	}, nil
}

// ImageApply применить изменения к хосту
func (a *Actions) ImageApply(ctx context.Context) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	err = a.serviceHostConfig.LoadConfig()
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	err = a.serviceHostConfig.GenerateDockerfile()
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	imageStatus, err := a.getImageStatus(ctx)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	err = a.serviceHostImage.BuildAndSwitch(ctx, true, *a.serviceHostConfig.Config, true)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":     "Изменения успешно применены. Необходима перезагрузка",
			"bootedImage": imageStatus,
		},
		Error: false,
	}, nil
}

// ImageHistory история изменений образа
func (a *Actions) ImageHistory(ctx context.Context, imageName string, limit int64, offset int64) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	history, err := a.serviceHostDatabase.GetImageHistoriesFiltered(ctx, imageName, limit, offset)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	totalCount, err := a.serviceHostDatabase.CountImageHistoriesFiltered(ctx, imageName)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	msg := fmt.Sprintf(
		"%s %d %s",
		helper.DeclOfNum(len(history), []string{"Найдена", "Найдено", "Найдены"}),
		len(history),
		helper.DeclOfNum(len(history), []string{"запись", "записи", "записей"}),
	)

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":    msg,
			"history":    history,
			"totalCount": totalCount,
		},
		Error: false,
	}, nil
}

// checkRoot проверяет, запущен ли установщик от имени root
func (a *Actions) checkRoot() error {
	if syscall.Geteuid() != 0 {
		return fmt.Errorf("для выполнения необходимы права администратора, используйте sudo или su")
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
		return fmt.Errorf("опция доступна только для атомарной системы")
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

	err = a.serviceHostImage.BuildAndSwitch(ctx, true, *a.serviceHostConfig.Config, false)
	if err != nil {
		return err
	}

	return nil
}

// newErrorResponse создаёт ответ с ошибкой.
func (a *Actions) newErrorResponse(message string) reply.APIResponse {
	return reply.APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	}
}

// validateDB проверяет, существует ли база данных
func (a *Actions) validateDB(ctx context.Context) error {
	// Если база не содержит данные - запускаем процесс обновления
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

func (a *Actions) getImageStatus(ctx context.Context) (ImageStatus, error) {
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
			Status: "Изменённый образ. Файл конфигурации: " + lib.Env.PathImageFile,
			Image:  hostImage,
			Config: *a.serviceHostConfig.Config,
		}, nil
	}

	return ImageStatus{
		Status: "Облачный образ без изменений",
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
	case apt.Package:
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
	case []apt.Package:
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
