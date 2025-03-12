package system

import (
	"apm/cmd/common/helper"
	"apm/cmd/common/reply"
	"apm/cmd/system/apt"
	"apm/cmd/system/service"
	"apm/lib"
	"context"
	"fmt"
	"strings"
	"syscall"
)

// Actions объединяет методы для выполнения системных действий.
type Actions struct {
	serviceHostImage  *service.HostImageService
	serviceAptActions *apt.Actions
}

// NewActions создаёт новый экземпляр Actions.
func NewActions() *Actions {
	return &Actions{
		serviceHostImage:  service.NewHostImageService(),
		serviceAptActions: apt.NewActions(),
	}
}

type ImageStatus struct {
	Image  service.HostImage `json:"image"`
	Status string            `json:"status"`
	Config service.Config    `json:"config"`
}

func (a *Actions) CheckRemove(ctx context.Context, packages []string) (reply.APIResponse, error) {
	allPackageNames := strings.Join(packages, " ")
	packageParse, _, err := a.serviceAptActions.Check(ctx, allPackageNames, "remove")
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if packageParse.RemovedCount == 0 {
		return reply.APIResponse{
			Data: map[string]interface{}{
				"message": "Кандидатов на удаление не найдено",
			},
			Error: true,
		}, nil
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": "Информация о проверке",
			"info":    packageParse,
		},
		Error: false,
	}, nil
}

func (a *Actions) CheckInstall(ctx context.Context, packages []string) (reply.APIResponse, error) {
	allPackageNames := strings.Join(packages, " ")
	packageParse, output, err := a.serviceAptActions.Check(ctx, allPackageNames, "install")
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if strings.Contains(output, "is already the newest version") && packageParse.NewInstalledCount == 0 && packageParse.UpgradedCount == 0 {
		return reply.APIResponse{
			Data: map[string]interface{}{
				"message": fmt.Sprintf("%s %s %s самой последней версии",
					helper.DeclOfNum(len(packages), []string{"пакет", "пакета", "пакетов"}),
					allPackageNames,
					helper.DeclOfNum(len(packages), []string{"установлен", "установлены", "установлены"})),
			},
			Error: true,
		}, nil
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
		return reply.APIResponse{
			Data: map[string]interface{}{
				"message": "Необходимо указать хотя бы один пакет, например remove package",
			},
			Error: true,
		}, nil
	}

	var names []string
	var packagesInfo []apt.Package
	for _, pkg := range packages {
		packageInfo, err := apt.GetPackageByName(ctx, pkg)
		if err != nil {
			return a.newErrorResponse(err.Error()), err
		}

		packagesInfo = append(packagesInfo, packageInfo)
		names = append(names, packageInfo.Name)
	}

	allPackageNames := strings.Join(names, " ")
	packageParse, _, err := a.serviceAptActions.Check(ctx, allPackageNames, "remove")
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if packageParse.RemovedCount == 0 {
		return reply.APIResponse{
			Data: map[string]interface{}{
				"message": "Кандидатов на удаление не найдено",
			},
			Error: true,
		}, nil
	}

	reply.StopSpinner()
	dialogStatus, err := apt.NewDialog(packagesInfo, packageParse, apt.ActionRemove)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if !dialogStatus {
		return reply.APIResponse{
			Data: map[string]interface{}{
				"message": "Отмена диалога удаления",
			},
			Error: false,
		}, nil
	}

	reply.CreateSpinner()
	err = a.serviceAptActions.Remove(ctx, allPackageNames)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
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
		return reply.APIResponse{
			Data: map[string]interface{}{
				"message": "Необходимо указать хотя бы один пакет, например install package",
			},
			Error: true,
		}, nil
	}

	var names []string
	var packagesInfo []apt.Package
	for _, pkg := range packages {
		packageInfo, err := apt.GetPackageByName(ctx, pkg)
		if err != nil {
			return a.newErrorResponse(err.Error()), err
		}
		packagesInfo = append(packagesInfo, packageInfo)
		names = append(names, packageInfo.Name)
	}
	allPackageNames := strings.Join(names, " ")
	packageParse, output, err := a.serviceAptActions.Check(ctx, allPackageNames, "install")
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if strings.Contains(output, "is already the newest version") && packageParse.NewInstalledCount == 0 && packageParse.UpgradedCount == 0 {
		messageAlreadyExist := fmt.Sprintf("%s %s %s самой последней версии",
			helper.DeclOfNum(len(packagesInfo), []string{"пакет", "пакета", "пакетов"}),
			allPackageNames,
			helper.DeclOfNum(len(packagesInfo), []string{"установлен", "установлены", "установлены"}))

		if apply && lib.Env.IsAtomic {
			config, errConfig := service.ParseConfig()
			if errConfig != nil {
				return a.newErrorResponse(errConfig.Error()), err
			}

			// обнаруживаем пакеты, которые были ранее установлены, но не зафиксированы в образе
			// добавляем такие пакеты в конфиг и запускаем сборку
			notFoundPackage := false
			for _, pkg := range packagesInfo {
				found := false
				for _, configPkg := range config.Packages.Install {
					if pkg.Name == configPkg {
						found = true
						break
					}
				}
				if !found {
					if err = config.AddInstallPackage(pkg.Name); err != nil {
						return a.newErrorResponse(err.Error()), err
					}
					notFoundPackage = true
				}
			}

			if notFoundPackage {
				err = a.applyChange(ctx, packages, true)
				if err != nil {
					return a.newErrorResponse(err.Error()), err
				}

				messageAlreadyExist += ". Но пакет не был найден в образе, поэтому образ был изменён"
			}
		}

		return reply.APIResponse{
			Data: map[string]interface{}{
				"message": messageAlreadyExist,
			},
			Error: true,
		}, nil
	}

	reply.StopSpinner()

	dialogStatus, err := apt.NewDialog(packagesInfo, packageParse, apt.ActionInstall)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if !dialogStatus {
		return reply.APIResponse{
			Data: map[string]interface{}{
				"message": "Отмена диалога установки",
			},
			Error: false,
		}, nil
	}

	reply.CreateSpinner()

	err = a.serviceAptActions.Install(ctx, allPackageNames)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
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
		err = a.applyChange(ctx, packages, true)
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

type PackageResponse struct {
	Name             string   `json:"name"`
	Section          string   `json:"section"`
	InstalledSize    string   `json:"installedSize"`
	Maintainer       string   `json:"maintainer"`
	Version          string   `json:"version"`
	VersionInstalled string   `json:"versionInstalled"`
	Depends          []string `json:"depends"`
	Size             string   `json:"size"`
	Filename         string   `json:"filename"`
	Description      string   `json:"description"`
	Installed        bool     `json:"installed"`
}

// Info возвращает информацию о системном пакете.
func (a *Actions) Info(ctx context.Context, packageName string) (reply.APIResponse, error) {
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := "необходимо указать название пакета, например info package"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}

	err := a.validateDB(ctx)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	packageInfo, err := apt.GetPackageByName(ctx, packageName)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	resp := PackageResponse{
		Name:             packageInfo.Name,
		Section:          packageInfo.Section,
		InstalledSize:    helper.AutoSize(packageInfo.InstalledSize),
		Maintainer:       packageInfo.Maintainer,
		Version:          packageInfo.Version,
		VersionInstalled: packageInfo.VersionInstalled,
		Depends:          packageInfo.Depends,
		Size:             helper.AutoSize(packageInfo.Size),
		Filename:         packageInfo.Filename,
		Description:      packageInfo.Description,
		Installed:        packageInfo.Installed,
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":     "Найден пакет",
			"packageInfo": resp,
		},
		Error: false,
	}, nil
}

// ListParams задаёт параметры для запроса списка пакетов.
type ListParams struct {
	Sort        string `json:"sort"`
	Order       string `json:"order"`
	Limit       int64  `json:"limit"`
	Offset      int64  `json:"offset"`
	FilterField string `json:"filterField"`
	FilterValue string `json:"filterValue"`
	ForceUpdate bool   `json:"forceUpdate"`
}

func (a *Actions) List(ctx context.Context, params ListParams) (reply.APIResponse, error) {
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

	// 4. Формируем фильтры (map[string]interface{}) из входных параметров
	filters := make(map[string]interface{})
	if strings.TrimSpace(params.FilterField) != "" && strings.TrimSpace(params.FilterValue) != "" {
		filters[params.FilterField] = params.FilterValue
	}
	totalCount, err := apt.CountHostImagePackages(ctx, filters)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	packages, err := apt.QueryHostImagePackages(ctx, filters, params.Sort, params.Order, params.Limit, params.Offset)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if len(packages) == 0 {
		return a.newErrorResponse("ничего не найдено"), fmt.Errorf("ничего не найдено")
	}

	var respPackages []PackageResponse
	for _, packageInfo := range packages {
		respPackages = append(respPackages, PackageResponse{
			Name:             packageInfo.Name,
			Section:          packageInfo.Section,
			InstalledSize:    helper.AutoSize(packageInfo.InstalledSize),
			Maintainer:       packageInfo.Maintainer,
			Version:          packageInfo.Version,
			VersionInstalled: packageInfo.VersionInstalled,
			Depends:          packageInfo.Depends,
			Size:             helper.AutoSize(packageInfo.Size),
			Filename:         packageInfo.Filename,
			Description:      packageInfo.Description,
			Installed:        packageInfo.Installed,
		})
	}

	msg := fmt.Sprintf(
		"%s %d %s",
		helper.DeclOfNum(len(respPackages), []string{"Найдена", "Найдено", "Найдены"}),
		len(respPackages),
		helper.DeclOfNum(len(respPackages), []string{"запись", "записи", "записей"}),
	)

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":    msg,
			"packages":   respPackages,
			"totalCount": int(totalCount),
		},
		Error: false,
	}, nil
}

// Search осуществляет поиск системного пакета по названию.
func (a *Actions) Search(ctx context.Context, packageName string, installed bool) (reply.APIResponse, error) {
	err := a.validateDB(ctx)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := "необходимо указать название пакета, например search package"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}

	packages, err := apt.SearchPackagesByName(ctx, packageName, installed)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if len(packages) == 0 {
		return a.newErrorResponse("Ничего не найдено"), fmt.Errorf("ничего не найдено")
	}

	var respPackages []PackageResponse
	for _, packageInfo := range packages {
		respPackages = append(respPackages, PackageResponse{
			Name:             packageInfo.Name,
			Section:          packageInfo.Section,
			InstalledSize:    helper.AutoSize(packageInfo.InstalledSize),
			Maintainer:       packageInfo.Maintainer,
			Version:          packageInfo.Version,
			VersionInstalled: packageInfo.VersionInstalled,
			Depends:          packageInfo.Depends,
			Size:             helper.AutoSize(packageInfo.Size),
			Filename:         packageInfo.Filename,
			Description:      packageInfo.Description,
			Installed:        packageInfo.Installed,
		})
	}

	msg := fmt.Sprintf(
		"%s %d %s",
		helper.DeclOfNum(len(respPackages), []string{"Найдена", "Найдено", "Найдены"}),
		len(respPackages),
		helper.DeclOfNum(len(respPackages), []string{"запись", "записи", "записей"}),
	)

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":  msg,
			"packages": respPackages,
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

	config, err := service.ParseConfig()
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	err = a.serviceHostImage.CheckAndUpdateBaseImage(ctx, true, config)
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

	config, err := service.ParseConfig()
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	err = config.GenerateDockerfile()
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	imageStatus, err := a.getImageStatus(ctx)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	err = a.serviceHostImage.BuildAndSwitch(ctx, true, config, true)
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

	history, err := service.GetImageHistoriesFiltered(ctx, imageName, limit, offset)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	totalCount, err := service.CountImageHistoriesFiltered(ctx, imageName)
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

	config, err := service.ParseConfig()
	if err != nil {
		return err
	}

	for _, pkg := range packages {
		if isInstall {
			err = config.AddInstallPackage(pkg)
			if err != nil {
				return err
			}
		} else {
			err = config.AddRemovePackage(pkg)
			if err != nil {
				return err
			}
		}
	}

	err = config.GenerateDockerfile()
	if err != nil {
		return err
	}

	err = a.serviceHostImage.BuildAndSwitch(ctx, true, config, false)
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
	if err := apt.PackageDatabaseExist(ctx); err != nil {
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
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)
	installedPackages, err := apt.GetInstalledPackages()
	if err != nil {
		return err
	}

	err = apt.SyncPackageInstallationInfo(ctx, installedPackages)
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

	config, err := service.ParseConfig()
	if err != nil {
		return ImageStatus{}, err
	}

	if hostImage.Status.Booted.Image.Image.Transport == "containers-storage" {
		return ImageStatus{
			Status: "Изменённый образ. Файл конфигурации: " + lib.Env.PathImageFile,
			Image:  hostImage,
			Config: config,
		}, nil
	}

	return ImageStatus{
		Status: "Облачный образ без изменений",
		Image:  hostImage,
		Config: config,
	}, nil
}
