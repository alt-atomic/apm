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
type Actions struct{}

// NewActions создаёт новый экземпляр Actions.
func NewActions() *Actions {
	return &Actions{}
}

type ImageStatus struct {
	Image  service.HostImage `json:"image"`
	Status string            `json:"status"`
	Config service.Config    `json:"config"`
}

// Remove удаляет системный пакет.
func (a *Actions) Remove(ctx context.Context, packages []string) (reply.APIResponse, error) {
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
	packageParse, _, err := apt.NewActions().Check(ctx, allPackageNames, "remove")
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
	err = apt.NewActions().Remove(ctx, allPackageNames)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	removePackageNames := strings.Join(packageParse.RemovedPackages, ",")

	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("%s успешно %s",
				removePackageNames,
				helper.DeclOfNum(packageParse.RemovedCount, []string{"удалён", "удалены", "удалены"}),
			),
			"info": packageParse,
		},
		Error: false,
	}, nil
}

// Install осуществляет установку системного пакета.
func (a *Actions) Install(ctx context.Context, packages []string) (reply.APIResponse, error) {
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
	packageParse, output, err := apt.NewActions().Check(ctx, allPackageNames, "install")
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if strings.Contains(output, "is already the newest version") && packageParse.NewInstalledCount == 0 && packageParse.UpgradedCount == 0 {
		return reply.APIResponse{
			Data: map[string]interface{}{
				"message": fmt.Sprintf("%s %s %s самой последней версии",
					helper.DeclOfNum(len(packagesInfo), []string{"пакет", "пакета", "пакетов"}),
					allPackageNames,
					helper.DeclOfNum(len(packagesInfo), []string{"установлен", "установлены", "установлены"})),
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
	err = apt.NewActions().Install(ctx, allPackageNames)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("%d %s успешно %s и %d %s",
				packageParse.NewInstalledCount,
				helper.DeclOfNum(packageParse.NewInstalledCount, []string{"пакет", "пакета", "пакетов"}),
				helper.DeclOfNum(packageParse.NewInstalledCount, []string{"установлен", "установлено", "установлены"}),
				packageParse.UpgradedCount,
				helper.DeclOfNum(packageParse.UpgradedCount, []string{"обновлён", "обновлено", "обновилось"}),
			),
			"info": packageParse,
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

	packages, err := apt.NewActions().Update(ctx)
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
	Changelog        string   `json:"lastChangelog"`
	Installed        bool     `json:"installed"`
}

// Info возвращает информацию о системном пакете.
func (a *Actions) Info(ctx context.Context, packageName string) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := "необходимо указать название пакета, например info package"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}
	err = a.validateDB(ctx)
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
		Changelog:        packageInfo.Changelog,
		Installed:        packageInfo.Installed,
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":     fmt.Sprintf("Информация о пакете %s", packageInfo.Name),
			"packageInfo": resp,
		},
		Error: false,
	}, nil
}

// Search осуществляет поиск системного пакета по названию.
func (a *Actions) Search(ctx context.Context, packageName string) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	err = a.validateDB(ctx)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	// Пустая реализация
	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Search action вызван для пакета '%s'", packageName),
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

	imageStatus, err := a.getImageStatus()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":     "Состояние образа",
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

	err = service.CheckAndUpdateBaseImage(ctx, true, config)
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	imageStatus, err := a.getImageStatus()
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

	imageStatus, err := a.getImageStatus()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	err = service.BuildAndSwitch(ctx, true, config)
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
	count, err := service.CountImageHistoriesFiltered(ctx, imageName)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": "История изменений образа",
			"history": history,
			"count":   count,
		},
		Error: false,
	}, nil
}

// checkRoot проверяет, запущен ли установщик от имени root
func (a *Actions) checkRoot() error {
	if syscall.Geteuid() != 0 {
		return fmt.Errorf("для выполнения необходимы права администратора, используйте sudo или su")
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
		aptAction := apt.NewActions()
		_, err = aptAction.Update(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// updateAllPackagesDB обновляет состояние всех пакетов в базе данных
func (a *Actions) updateAllPackagesDB(ctx context.Context) error {
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

func (a *Actions) getImageStatus() (ImageStatus, error) {
	hostImage, err := service.GetHostImage()
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
