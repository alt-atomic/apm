package distrobox

import (
	"apm/cmd/common/reply"
	"apm/cmd/distrobox/api"
	"apm/cmd/distrobox/service"
	"apm/lib"
	"fmt"
	"strings"
)

type Actions struct{}

func NewActions() *Actions {
	return &Actions{}
}

// Update обновляет и синхронизирует список пакетов в контейнере.
func (a *Actions) Update(container string) (reply.APIResponse, error) {
	cont, err := a.validateContainer(container)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	osInfo, err := api.GetContainerOsInfo(cont)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packages, err := service.UpdatePackages(osInfo)
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
func (a *Actions) Info(container, packageName string) (reply.APIResponse, error) {
	cont, err := a.validateContainer(container)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := "необходимо указать название пакета, например info package"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}
	osInfo, err := api.GetContainerOsInfo(cont)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packageInfo, err := service.GetInfoPackage(osInfo, packageName)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":     "Информация о пакете",
			"packageInfo": packageInfo,
		},
		Error: false,
	}
	return resp, nil
}

// Search выполняет поиск пакета по названию.
func (a *Actions) Search(container, packageName string) (reply.APIResponse, error) {
	cont, err := a.validateContainer(container)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := "необходимо указать название пакета, например search package"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}
	osInfo, err := api.GetContainerOsInfo(cont)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	queryResult, err := service.GetPackageByName(osInfo, packageName)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	word := a.pluralizePackage(queryResult.TotalCount)
	msg := fmt.Sprintf("Найден %d %s", queryResult.TotalCount, word)
	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":  msg,
			"packages": queryResult.Packages,
			"count":    queryResult.TotalCount,
		},
		Error: false,
	}
	return resp, nil
}

// ListParams задаёт параметры для запроса списка пакетов.
type ListParams struct {
	Container   string
	Sort        string
	Order       string
	Limit       int64
	Offset      int64
	FilterField string
	FilterValue string
	ForceUpdate bool
}

// List возвращает список пакетов согласно заданным параметрам.
func (a *Actions) List(params ListParams) (reply.APIResponse, error) {
	cont, err := a.validateContainer(params.Container)
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
	if strings.TrimSpace(params.FilterField) != "" && strings.TrimSpace(params.FilterValue) != "" {
		builder.Filters[params.FilterField] = params.FilterValue
	}
	osInfo, err := api.GetContainerOsInfo(cont)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	queryResult, err := service.GetPackagesQuery(osInfo, builder)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	word := a.pluralizePackage(queryResult.TotalCount)
	msg := fmt.Sprintf("Найдено: %d %s", queryResult.TotalCount, word)
	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message":  msg,
			"packages": queryResult.Packages,
			"count":    queryResult.TotalCount,
		},
		Error: false,
	}
	return resp, nil
}

// Install устанавливает указанный пакет и опционально экспортирует его.
func (a *Actions) Install(container, packageName string, export bool) (reply.APIResponse, error) {
	cont, err := a.validateContainer(container)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := "необходимо указать название пакета, например install package"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}
	osInfo, err := api.GetContainerOsInfo(cont)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packageInfo, err := service.GetInfoPackage(osInfo, packageName)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	if !packageInfo.Package.Installed {
		err = service.InstallPackage(osInfo, packageName)
		if err != nil {
			return a.newErrorResponse(err.Error()), err
		}
		packageInfo.Package.Installed = true
		service.UpdatePackageField(osInfo.ContainerName, packageName, "installed", true)
		packageInfo, _ = service.GetInfoPackage(osInfo, packageName)
	}
	if export && !packageInfo.Package.Exporting {
		errExport := api.ExportingApp(osInfo, packageName, packageInfo.IsConsole, packageInfo.Paths, false)
		if errExport != nil {
			return a.newErrorResponse(errExport.Error()), errExport
		}
		packageInfo.Package.Exporting = true
		service.UpdatePackageField(osInfo.ContainerName, packageName, "exporting", true)
	}
	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Пакет %s установлен", packageName),
			"package": packageInfo,
		},
		Error: false,
	}
	return resp, nil
}

// Remove удаляет указанный пакет. Если onlyExport равен true, удаляется только экспорт.
func (a *Actions) Remove(container, packageName string, onlyExport bool) (reply.APIResponse, error) {
	cont, err := a.validateContainer(container)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		errMsg := "необходимо указать название пакета, например remove package"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}
	osInfo, err := api.GetContainerOsInfo(cont)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	packageInfo, err := service.GetInfoPackage(osInfo, packageName)
	if err != nil {
		return a.newErrorResponse(err.Error()), err
	}
	if packageInfo.Package.Exporting {
		errExport := api.ExportingApp(osInfo, packageName, packageInfo.IsConsole, packageInfo.Paths, true)
		if errExport != nil {
			return a.newErrorResponse(errExport.Error()), errExport
		}
		packageInfo.Package.Exporting = false
		service.UpdatePackageField(osInfo.ContainerName, packageName, "exporting", false)
	}
	if !onlyExport && packageInfo.Package.Installed {
		err = service.RemovePackage(osInfo, packageName)
		if err != nil {
			return a.newErrorResponse(err.Error()), err
		}
		packageInfo.Package.Installed = false
		service.UpdatePackageField(osInfo.ContainerName, packageName, "installed", false)
	}
	resp := reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Пакет %s удалён", packageName),
			"package": packageInfo,
		},
		Error: false,
	}
	return resp, nil
}

// ContainerList возвращает список контейнеров.
func (a *Actions) ContainerList() (reply.APIResponse, error) {
	containers, err := api.GetContainerList(true)
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
func (a *Actions) ContainerAdd(image, name, additionalPackages, initHooks string) (reply.APIResponse, error) {
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
	result, err := api.CreateContainer(image, name, additionalPackages, initHooks)
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
func (a *Actions) ContainerRemove(name string) (reply.APIResponse, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		errMsg := "необходимо указать название контейнера (--name)"
		return a.newErrorResponse(errMsg), fmt.Errorf(errMsg)
	}
	result, err := api.RemoveContainer(name)
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

// pluralizePackage возвращает слово "пакет" в правильной форме.
func (a *Actions) pluralizePackage(n int) string {
	if n%10 == 1 && n%100 != 11 {
		return "пакет"
	} else if n%10 >= 2 && n%10 <= 4 && !(n%100 >= 12 && n%100 <= 14) {
		return "пакета"
	}
	return "пакетов"
}

// validateContainer проверяет, что имя контейнера не пустое и обновляет пакеты, если нужно.
func (a *Actions) validateContainer(container string) (string, error) {
	container = strings.TrimSpace(container)
	if container == "" {
		return "", fmt.Errorf("необходимо указать название контейнера")
	}
	// Если база не содержит данные, обновляем пакеты.
	if err := service.ContainerDatabaseExist(container); err != nil {
		osInfo, err := api.GetContainerOsInfo(container)
		if err != nil {
			return "", err
		}
		if _, err = service.UpdatePackages(osInfo); err != nil {
			return "", err
		}
	}
	return container, nil
}
