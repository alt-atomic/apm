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

package distrobox

import (
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	"apm/internal/common/filter"
	"apm/internal/common/icon"
	"apm/internal/distrobox/service"
	"context"
	"errors"
	"fmt"
	"strings"
)

type Actions struct {
	servicePackage        *service.PackageService
	serviceDistroDatabase *service.DistroDBService
	serviceDistroAPI      *service.DistroAPIService
	iconService           *icon.Service
}

func NewActions(appConfig *app.Config) *Actions {
	distroDBSvc := service.NewDistroDBService(appConfig.DatabaseManager)

	commandPrefix := appConfig.ConfigManager.GetConfig().CommandPrefix
	distroPackageSvc := service.NewPackageService(distroDBSvc, commandPrefix)
	distroAPISvc := service.NewDistroAPIService(commandPrefix)
	iconSvc := icon.NewIconService(appConfig.DatabaseManager, commandPrefix)

	return &Actions{
		servicePackage:        distroPackageSvc,
		serviceDistroDatabase: distroDBSvc,
		serviceDistroAPI:      distroAPISvc,
		iconService:           iconSvc,
	}
}

// GetIconService возвращает сервис иконок
func (a *Actions) GetIconService() *icon.Service {
	return a.iconService
}

// GetIconByPackage возвращает иконку приложения. Параметр container можно передать пустым.
func (a *Actions) GetIconByPackage(_ context.Context, packageName, container string) ([]byte, error) {
	data, err := a.iconService.GetIcon(packageName, container)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeNotFound, err)
	}
	return data, nil
}

// Update обновляет и синхронизирует список пакетов в контейнере.
func (a *Actions) Update(ctx context.Context, container string) (*UpdateResponse, error) {
	osInfo, err := a.validateContainer(ctx, container, false)
	if err != nil {
		return nil, err
	}

	packages, err := a.servicePackage.UpdatePackages(ctx, osInfo)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	return &UpdateResponse{
		Message:   app.T_("Package list successfully updated"),
		Container: osInfo,
		Count:     len(packages),
	}, nil
}

// Info возвращает информацию о пакете.
func (a *Actions) Info(ctx context.Context, container string, packageName string) (*InfoResponse, error) {
	osInfo, err := a.validateContainer(ctx, container)
	if err != nil {
		return nil, err
	}
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf(app.T_("You must specify the package name, for example `%s package`"), "info"))
	}
	packageInfo, err := a.servicePackage.GetInfoPackage(ctx, osInfo, packageName)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeNotFound, err)
	}

	return &InfoResponse{
		Message:     app.T_("Package found"),
		PackageInfo: packageInfo,
	}, nil
}

// Search выполняет поиск пакета по названию.
func (a *Actions) Search(ctx context.Context, container string, packageName string) (*SearchResponse, error) {
	var osInfo service.ContainerInfo
	var err error
	if len(container) > 0 {
		osInfo, err = a.validateContainer(ctx, container)
		if err != nil {
			return nil, err
		}
	} else {
		err = a.validateDatabase(ctx)
		if err != nil {
			return nil, err
		}
	}

	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf(app.T_("You must specify the package name, for example `%s package`"), "search"))
	}

	queryResult, err := a.servicePackage.GetPackageByName(ctx, osInfo, packageName)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	return &SearchResponse{
		Message:  fmt.Sprintf(app.TN_("%d record found", "%d records found", len(queryResult.Packages)), len(queryResult.Packages)),
		Packages: queryResult.Packages,
	}, nil
}

// ListParams задаёт параметры для запроса списка пакетов.
type ListParams struct {
	Container   string          `json:"container"`
	Sort        string          `json:"sort"`
	Order       string          `json:"order"`
	Limit       int             `json:"limit"`
	Offset      int             `json:"offset"`
	Filters     []filter.Filter `json:"filters"`
	ForceUpdate bool            `json:"forceUpdate"`
}

// List возвращает список пакетов согласно заданным параметрам.
func (a *Actions) List(ctx context.Context, params ListParams) (*ListResponse, error) {
	var osInfo service.ContainerInfo
	var err error

	if len(params.Container) > 0 {
		osInfo, err = a.validateContainer(ctx, params.Container)
		if err != nil {
			return nil, err
		}
	} else {
		err = a.validateDatabase(ctx)
		if err != nil {
			return nil, err
		}
	}

	builder := service.PackageQueryBuilder{
		ForceUpdate: params.ForceUpdate,
		Limit:       params.Limit,
		Offset:      params.Offset,
		SortField:   params.Sort,
		SortOrder:   params.Order,
		Filters:     params.Filters,
	}

	queryResult, err := a.servicePackage.GetPackagesQuery(ctx, osInfo, builder)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	return &ListResponse{
		Message:    fmt.Sprintf(app.TN_("%d record found", "%d records found", len(queryResult.Packages)), len(queryResult.Packages)),
		Packages:   queryResult.Packages,
		TotalCount: queryResult.TotalCount,
	}, nil
}

// Install устанавливает указанный пакет и опционально экспортирует его.
func (a *Actions) Install(ctx context.Context, container string, packageName string, export bool) (*InstallResponse, error) {
	osInfo, err := a.validateContainer(ctx, container)
	if err != nil {
		return nil, err
	}
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf(app.T_("You must specify the package name, for example `%s package`"), "install"))
	}

	packageInfo, err := a.servicePackage.GetInfoPackage(ctx, osInfo, packageName)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeNotFound, err)
	}
	if !packageInfo.Package.Installed {
		err = a.servicePackage.InstallPackage(ctx, osInfo, packageName)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeContainer, err)
		}
		packageInfo.Package.Installed = true
		a.serviceDistroDatabase.UpdatePackageField(ctx, osInfo.ContainerName, packageName, "installed", true)
		packageInfo, _ = a.servicePackage.GetInfoPackage(ctx, osInfo, packageName)
	}
	if export && !packageInfo.Package.Exporting {
		errExport := a.serviceDistroAPI.ExportingApp(ctx, osInfo, packageName, packageInfo.DesktopPaths, packageInfo.ConsolePaths, false)
		if errExport != nil {
			return nil, apmerr.New(apmerr.ErrorTypeContainer, errExport)
		}
		if len(packageInfo.DesktopPaths) > 0 || len(packageInfo.ConsolePaths) > 0 {
			packageInfo.Package.Exporting = true
			a.serviceDistroDatabase.UpdatePackageField(ctx, osInfo.ContainerName, packageName, "exporting", true)
		}
	}

	return &InstallResponse{
		Message:     fmt.Sprintf(app.T_("Package %s installed"), packageName),
		PackageInfo: packageInfo,
	}, nil
}

// Remove удаляет указанный пакет. Если onlyExport равен true, удаляется только экспорт.
func (a *Actions) Remove(ctx context.Context, container string, packageName string, onlyExport bool) (*RemoveResponse, error) {
	osInfo, err := a.validateContainer(ctx, container)
	if err != nil {
		return nil, err
	}

	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf(app.T_("You must specify the package name, for example `%s package`"), "remove"))
	}

	packageInfo, err := a.servicePackage.GetInfoPackage(ctx, osInfo, packageName)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeNotFound, err)
	}

	if packageInfo.Package.Exporting {
		_ = a.serviceDistroAPI.ExportingApp(ctx, osInfo, packageName, packageInfo.DesktopPaths, packageInfo.ConsolePaths, true)
		packageInfo.Package.Exporting = false
		a.serviceDistroDatabase.UpdatePackageField(ctx, osInfo.ContainerName, packageName, "exporting", false)
	}

	if !onlyExport && packageInfo.Package.Installed {
		err = a.servicePackage.RemovePackage(ctx, osInfo, packageName)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeContainer, err)
		}
		packageInfo.Package.Installed = false
		a.serviceDistroDatabase.UpdatePackageField(ctx, osInfo.ContainerName, packageName, "installed", false)
	}

	return &RemoveResponse{
		Message:     fmt.Sprintf(app.T_("Package %s removed"), packageName),
		PackageInfo: packageInfo,
	}, nil
}

// ContainerList возвращает список контейнеров.
func (a *Actions) ContainerList(ctx context.Context) (*ContainerListResponse, error) {
	containers, err := a.serviceDistroAPI.GetContainerList(ctx, true)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeContainer, err)
	}

	if len(containers) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNotFound, errors.New(app.T_("No containers found")))
	}

	return &ContainerListResponse{
		Containers: containers,
	}, nil
}

// ContainerAdd создаёт новый контейнер.
func (a *Actions) ContainerAdd(ctx context.Context, image string, name string, additionalPackages, initHooks string) (*ContainerAddResponse, error) {
	image = strings.TrimSpace(image)
	name = strings.TrimSpace(name)
	if image == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("You must specify the image link (--image)")))
	}

	if name == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("You must specify the container name (--name)")))
	}

	osInfo, err := a.serviceDistroAPI.CreateContainer(ctx, image, name, additionalPackages, initHooks)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeContainer, err)
	}

	_, err = a.servicePackage.UpdatePackages(ctx, osInfo)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	return &ContainerAddResponse{
		Message:       fmt.Sprintf(app.T_("Container %s successfully created"), name),
		ContainerInfo: osInfo,
	}, nil
}

// ContainerRemove удаляет контейнер по имени.
func (a *Actions) ContainerRemove(ctx context.Context, name string) (*ContainerRemoveResponse, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("You must specify the container name (--name)")))
	}

	result, err := a.serviceDistroAPI.RemoveContainer(ctx, name)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeContainer, err)
	}

	err = a.serviceDistroDatabase.DeletePackagesFromContainer(ctx, name)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, fmt.Errorf(app.T_("Error deleting container: %v"), err))
	}

	return &ContainerRemoveResponse{
		Message:       fmt.Sprintf(app.T_("Container %s successfully deleted"), name),
		ContainerInfo: result,
	}, nil
}

// GetFilterFields возвращает список свойств для фильтрации. Метод для DBUS
func (a *Actions) GetFilterFields(_ context.Context) (GetFilterFieldsResponse, error) {
	return service.DistroFilterConfig.FieldsInfo(), nil
}

// validateDatabase проверяет, что таблица содержит какие-то записи
func (a *Actions) validateDatabase(ctx context.Context) error {
	if err := a.serviceDistroDatabase.DatabaseExist(ctx); err != nil {
		return apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	return nil
}

// validateContainer проверяет, что имя контейнера не пустой и обновляет пакеты, если нужно.
func (a *Actions) validateContainer(ctx context.Context, container string, autoUpdate ...bool) (service.ContainerInfo, error) {
	shouldAutoUpdate := len(autoUpdate) == 0 || autoUpdate[0]
	container = strings.TrimSpace(container)
	if container == "" {
		return service.ContainerInfo{}, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("You must specify the container name")))
	}

	// Если контейнер не найден через API, проверяем наличие записей в базе данных
	osInfo, errInfo := a.serviceDistroAPI.GetContainerOsInfo(ctx, container)
	if errInfo != nil {
		if err := a.serviceDistroDatabase.ContainerDatabaseExist(ctx, container); err == nil {
			if err = a.serviceDistroDatabase.DeletePackagesFromContainer(ctx, container); err != nil {
				return service.ContainerInfo{}, apmerr.New(apmerr.ErrorTypeDatabase, fmt.Errorf(app.T_("Failed to delete container records: %w"), err))
			}
		}

		return service.ContainerInfo{}, apmerr.New(apmerr.ErrorTypeNotFound, errInfo)
	}

	// Если база не содержит данные, обновляем пакеты.
	if shouldAutoUpdate {
		if err := a.serviceDistroDatabase.ContainerDatabaseExist(ctx, container); err != nil {
			if _, err = a.servicePackage.UpdatePackages(ctx, osInfo); err != nil {
				return service.ContainerInfo{}, apmerr.New(apmerr.ErrorTypeDatabase, err)
			}
		}
	}

	return osInfo, nil
}

// GenerateOnlineDoc запускает веб-сервер с HTML документацией для DBus API
func (a *Actions) GenerateOnlineDoc(ctx context.Context) error {
	return startDocServer(ctx)
}
