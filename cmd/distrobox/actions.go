package distrobox

import (
	"apm/cmd/common/helper"
	"apm/cmd/common/reply"
	"apm/cmd/distrobox/service"
	"apm/lib"
	"context"
	"fmt"
	"strings"
	"syscall"
)

type Actions struct {
	servicePackage        *service.PackageService
	serviceDistroDatabase *service.DistroDBService
	serviceDistroAPI      *service.DistroAPIService
}

// NewActionsWithDeps создаёт новый экземпляр Actions с ручными управлением зависимостями
func NewActionsWithDeps(
	servicePackage *service.PackageService,
	serviceDistroDatabase *service.DistroDBService,
	serviceDistroAPI *service.DistroAPIService,
) *Actions {
	return &Actions{
		servicePackage:        servicePackage,
		serviceDistroDatabase: serviceDistroDatabase,
		serviceDistroAPI:      serviceDistroAPI,
	}
}

func NewActions() *Actions {
	distroDBSvc := service.NewDistroDBService(lib.GetDB())
	distroPackageSvc := service.NewPackageService(distroDBSvc)
	distroAPISvc := service.NewDistroAPIService()

	return &Actions{
		servicePackage:        distroPackageSvc,
		serviceDistroDatabase: distroDBSvc,
		serviceDistroAPI:      distroAPISvc,
	}
}

// Update обновляет и синхронизирует список пакетов в контейнере.
func (a *Actions) Update(ctx context.Context, container string) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	cont, err := a.validateContainer(ctx, container)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	osInfo, err := a.serviceDistroAPI.GetContainerOsInfo(ctx, cont)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packages, err := a.servicePackage.UpdatePackages(ctx, osInfo)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":   "Список пакетов успешно обновлён",
			"container": osInfo,
			"count":     len(packages),
		},
		Error: false,
	}
	return resp, nil
}

// Info возвращает информацию о пакете.
func (a *Actions) Info(ctx context.Context, container string, packageName string) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	cont, err := a.validateContainer(ctx, container)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := "необходимо указать название пакета, например info package"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}
	osInfo, err := a.serviceDistroAPI.GetContainerOsInfo(ctx, cont)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packageInfo, err := a.servicePackage.GetInfoPackage(ctx, osInfo, packageName)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":     "Найден пакет",
			"packageInfo": packageInfo,
		},
		Error: false,
	}
	return resp, nil
}

// Search выполняет поиск пакета по названию.
func (a *Actions) Search(ctx context.Context, container string, packageName string) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	cont, err := a.validateContainer(ctx, container)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := "необходимо указать название пакета, например search package"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}
	osInfo, err := a.serviceDistroAPI.GetContainerOsInfo(ctx, cont)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	queryResult, err := a.servicePackage.GetPackageByName(ctx, osInfo, packageName)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	msg := fmt.Sprintf(
		"%s %d %s",
		helper.DeclOfNum(len(queryResult.Packages), []string{"Найдена", "Найдено", "Найдены"}),
		len(queryResult.Packages),
		helper.DeclOfNum(len(queryResult.Packages), []string{"запись", "записи", "записей"}),
	)
	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":  msg,
			"packages": queryResult.Packages,
		},
		Error: false,
	}

	return resp, nil
}

// ListParams задаёт параметры для запроса списка пакетов.
type ListParams struct {
	Container   string   `json:"container"`
	Sort        string   `json:"sort"`
	Order       string   `json:"order"`
	Limit       int64    `json:"limit"`
	Offset      int64    `json:"offset"`
	Filters     []string `json:"filters"`
	ForceUpdate bool     `json:"forceUpdate"`
}

// List возвращает список пакетов согласно заданным параметрам.
func (a *Actions) List(ctx context.Context, params ListParams) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	cont, err := a.validateContainer(ctx, params.Container)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	builder := service.PackageQueryBuilder{
		ForceUpdate: params.ForceUpdate,
		Limit:       params.Limit,
		Offset:      params.Offset,
		SortField:   params.Sort,
		SortOrder:   params.Order,
		Filters:     make(map[string]interface{}),
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

	builder.Filters = filters

	osInfo, err := a.serviceDistroAPI.GetContainerOsInfo(ctx, cont)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	queryResult, err := a.servicePackage.GetPackagesQuery(ctx, osInfo, builder)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	msg := fmt.Sprintf(
		"%s %d %s",
		helper.DeclOfNum(len(queryResult.Packages), []string{"Найдена", "Найдено", "Найдены"}),
		len(queryResult.Packages),
		helper.DeclOfNum(len(queryResult.Packages), []string{"запись", "записи", "записей"}),
	)
	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":    msg,
			"packages":   queryResult.Packages,
			"totalCount": queryResult.TotalCount,
		},
		Error: false,
	}

	return resp, nil
}

// Install устанавливает указанный пакет и опционально экспортирует его.
func (a *Actions) Install(ctx context.Context, container string, packageName string, export bool) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	cont, err := a.validateContainer(ctx, container)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := "необходимо указать название пакета, например install package"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}
	osInfo, err := a.serviceDistroAPI.GetContainerOsInfo(ctx, cont)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packageInfo, err := a.servicePackage.GetInfoPackage(ctx, osInfo, packageName)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	if !packageInfo.Package.Installed {
		err = a.servicePackage.InstallPackage(ctx, osInfo, packageName)
		if err != nil {
			return a.newErrorResponse(err.Error()), err
		}
		packageInfo.Package.Installed = true
		a.serviceDistroDatabase.UpdatePackageField(ctx, osInfo.ContainerName, packageName, "installed", true)
		packageInfo, _ = a.servicePackage.GetInfoPackage(ctx, osInfo, packageName)
	}
	if export && !packageInfo.Package.Exporting {
		errExport := a.serviceDistroAPI.ExportingApp(ctx, osInfo, packageName, packageInfo.IsConsole, packageInfo.Paths, false)
		if errExport != nil {
			return a.newErrorResponse(errExport.Error()), errExport
		}
		packageInfo.Package.Exporting = true
		a.serviceDistroDatabase.UpdatePackageField(ctx, osInfo.ContainerName, packageName, "exporting", true)
	}
	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":     fmt.Sprintf("Пакет %s установлен", packageName),
			"packageInfo": packageInfo,
		},
		Error: false,
	}

	return resp, nil
}

// Remove удаляет указанный пакет. Если onlyExport равен true, удаляется только экспорт.
func (a *Actions) Remove(ctx context.Context, container string, packageName string, onlyExport bool) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	cont, err := a.validateContainer(ctx, container)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := "необходимо указать название пакета, например remove package"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}

	osInfo, err := a.serviceDistroAPI.GetContainerOsInfo(ctx, cont)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	packageInfo, err := a.servicePackage.GetInfoPackage(ctx, osInfo, packageName)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	if packageInfo.Package.Exporting {
		errExport := a.serviceDistroAPI.ExportingApp(ctx, osInfo, packageName, packageInfo.IsConsole, packageInfo.Paths, true)
		if errExport != nil {
			return a.newErrorResponse(errExport.Error()), errExport
		}
		packageInfo.Package.Exporting = false
		a.serviceDistroDatabase.UpdatePackageField(ctx, osInfo.ContainerName, packageName, "exporting", false)
	}

	if !onlyExport && packageInfo.Package.Installed {
		err = a.servicePackage.RemovePackage(ctx, osInfo, packageName)
		if err != nil {
			return a.newErrorResponse(err.Error()), err
		}
		packageInfo.Package.Installed = false
		a.serviceDistroDatabase.UpdatePackageField(ctx, osInfo.ContainerName, packageName, "installed", false)
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":     fmt.Sprintf("Пакет %s удалён", packageName),
			"packageInfo": packageInfo,
		},
		Error: false,
	}

	return resp, nil
}

// ContainerList возвращает список контейнеров.
func (a *Actions) ContainerList(ctx context.Context) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	containers, err := a.serviceDistroAPI.GetContainerList(ctx, true)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"containers": containers,
		},
		Error: false,
	}

	return resp, nil
}

// ContainerAdd создаёт новый контейнер.
func (a *Actions) ContainerAdd(ctx context.Context, image string, name string, additionalPackages, initHooks string) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	image = strings.TrimSpace(image)
	name = strings.TrimSpace(name)
	if image == "" {
		errMsg := "необходимо указать ссылку на образ (--image)"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}

	if name == "" {
		errMsg := "необходимо указать название контейнера (--name)"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}

	result, err := a.serviceDistroAPI.CreateContainer(ctx, image, name, additionalPackages, initHooks)
	if err != nil {
		return a.newErrorResponse(fmt.Sprintf("Ошибка создания контейнера: %v", err)), err
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":       fmt.Sprintf("Контейнер %s успешно создан", name),
			"containerInfo": result,
		},
		Error: false,
	}

	return resp, nil
}

// ContainerRemove удаляет контейнер по имени.
func (a *Actions) ContainerRemove(ctx context.Context, name string) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		errMsg := "необходимо указать название контейнера (--name)"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}

	result, err := a.serviceDistroAPI.RemoveContainer(ctx, name)
	if err != nil {
		return a.newErrorResponse(fmt.Sprintf("Ошибка удаления контейнера: %v", err)), err
	}

	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":       fmt.Sprintf("Контейнер %s успешно удалён", name),
			"containerInfo": result,
		},
		Error: false,
	}

	err = a.serviceDistroDatabase.DeleteContainerTable(ctx, name)
	if err != nil {
		return a.newErrorResponse(fmt.Sprintf("Ошибка удаления контейнера: %v", err)), err
	}

	return resp, nil
}

// GetFilterFields возвращает список свойств для фильтрации по названию контейнера. Метод для DBUS
func (a *Actions) GetFilterFields(ctx context.Context, container string) (reply.APIResponse, error) {
	err := a.checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	cont, err := a.validateContainer(ctx, container)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	osInfo, err := a.serviceDistroAPI.GetContainerOsInfo(ctx, cont)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}

	fieldList := []string{"name", "version", "description", "installed", "exporting", "manager"}
	type FiltersField struct {
		Name   string   `json:"name"`
		Text   string   `json:"text"`
		Type   string   `json:"type"`
		Choice []string `json:"choice"`
	}

	var fields []FiltersField
	var manager []string
	lowerOsName := strings.ToLower(osInfo.OS)
	switch {
	case strings.Contains(lowerOsName, "arch"):
		manager = append(manager, "pacman")
	case strings.Contains(lowerOsName, "alt"):
		manager = append(manager, "apt-get")
	case strings.Contains(lowerOsName, "ubuntu"):
		manager = append(manager, "apt")
	}

	for _, field := range fieldList {
		fieldType := "STRING"
		if field == "installed" || field == "exporting" {
			fieldType = "BOOL"
		}

		var choice []string
		if field == "manager" {
			choice = manager
		}

		fields = append(fields, FiltersField{
			Name:   field,
			Text:   lib.T("response."+field, field),
			Type:   fieldType,
			Choice: choice,
		})
	}

	resp := reply.APIResponse{
		Data:  fields,
		Error: false,
	}

	return resp, nil
}

// newErrorResponse создаёт ответ с ошибкой.
func (a *Actions) newErrorResponse(message string) reply.APIResponse {
	lib.Log.Error(message)

	return reply.APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	}
}

// validateContainer проверяет, что имя контейнера не пустое и обновляет пакеты, если нужно.
func (a *Actions) validateContainer(ctx context.Context, container string) (string, error) {
	container = strings.TrimSpace(container)
	if container == "" {
		return "", fmt.Errorf("необходимо указать название контейнера")
	}

	// Если база не содержит данные, обновляем пакеты.
	if err := a.serviceDistroDatabase.ContainerDatabaseExist(ctx, container); err != nil {
		osInfo, errInfo := a.serviceDistroAPI.GetContainerOsInfo(ctx, container)
		if errInfo != nil {
			return "", errInfo
		}
		if _, err = a.servicePackage.UpdatePackages(ctx, osInfo); err != nil {
			return "", err
		}
	}

	return container, nil
}

// checkRoot проверяет, запущен ли apm от имени root
func (a *Actions) checkRoot() error {
	if syscall.Geteuid() == 0 {
		return fmt.Errorf("запрошенную команду нельзя выполнить от имени root, пожалуйста не используйте sudo/su")
	}

	return nil
}
