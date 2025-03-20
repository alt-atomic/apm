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

package service

import (
	"apm/cmd/common/reply"
	"apm/lib"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type PackageService struct {
	serviceDistroDatabase *DistroDBService
}

// NewPackageService — конструктор сервиса.
func NewPackageService(serviceDistroDatabase *DistroDBService) *PackageService {
	return &PackageService{serviceDistroDatabase: serviceDistroDatabase}
}

// PackageInfo описывает информацию о пакете.
type PackageInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Container   string `json:"container"`
	Installed   bool   `json:"installed"`
	Exporting   bool   `json:"exporting"`
	Manager     string `json:"manager"`
}

// PackageQueryResult содержит срез найденных пакетов и общее количество записей, удовлетворяющих фильтрам.
type PackageQueryResult struct {
	Packages   []PackageInfo `json:"packages"`
	TotalCount int           `json:"totalCount"`
}

// PackageQueryBuilder задаёт параметры запроса.
type PackageQueryBuilder struct {
	ForceUpdate bool                   // Обновление перед тем как выполнить запрос
	Limit       int64                  // Если Limit <= 0, то ограничение не применяется
	Offset      int64                  // Если Offset < 0, то считается 0
	Filters     map[string]interface{} // фильтры вида "field": value; используется условие "="
	SortField   string                 // Поле сортировки (например, "packageName")
	SortOrder   string                 // "ASC" или "DESC"
}

type InfoPackageAnswer struct {
	Package   PackageInfo `json:"package"`
	Paths     []string    `json:"paths"`
	IsConsole bool        `json:"isConsole"`
}

// PackageProvider задаёт интерфейс для работы с пакетами в контейнере.
type PackageProvider interface {
	GetPackages(ctx context.Context, containerInfo ContainerInfo) ([]PackageInfo, error)
	RemovePackage(ctx context.Context, containerInfo ContainerInfo, packageName string) error
	InstallPackage(ctx context.Context, containerInfo ContainerInfo, packageName string) error
	GetPackageOwner(ctx context.Context, containerInfo ContainerInfo, fileName string) (string, error)
	GetPathByPackageName(ctx context.Context, containerInfo ContainerInfo, packageName, filePath string) ([]string, error)
}

// getProvider возвращает подходящий провайдер в зависимости от имени ОС контейнера.
func getProvider(servicePackage *PackageService, osName string) (PackageProvider, error) {

	lowerName := strings.ToLower(osName)
	if strings.Contains(lowerName, "ubuntu") || strings.Contains(lowerName, "debian") {
		return NewUbuntuProvider(servicePackage), nil
	} else if strings.Contains(lowerName, "arch") {
		return NewArchProvider(servicePackage), nil
	} else if strings.Contains(lowerName, "alt") {
		return NewAltProvider(servicePackage), nil
	} else {
		return nil, errors.New("Данный контейнер не поддерживается: " + osName)
	}
}

// InstallPackage установка пакета
func (p *PackageService) InstallPackage(ctx context.Context, containerInfo ContainerInfo, packageName string) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.InstallPackage"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.InstallPackage"))
	provider, err := getProvider(p, containerInfo.OS)
	if err != nil {
		return err
	}

	return provider.InstallPackage(ctx, containerInfo, packageName)
}

// RemovePackage удаление пакета
func (p *PackageService) RemovePackage(ctx context.Context, containerInfo ContainerInfo, packageName string) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.RemovePackage"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.RemovePackage"))
	provider, err := getProvider(p, containerInfo.OS)
	if err != nil {
		return err
	}

	return provider.RemovePackage(ctx, containerInfo, packageName)
}

// GetPackages получает список пакетов из контейнера.
func (p *PackageService) GetPackages(ctx context.Context, containerInfo ContainerInfo) ([]PackageInfo, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.GetPackages"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.GetPackages"))
	provider, err := getProvider(p, containerInfo.OS)
	if err != nil {
		return nil, err
	}

	return provider.GetPackages(ctx, containerInfo)
}

// GetPackageOwner получает название пакета, которому принадлежит указанный файл, из контейнера.
func (p *PackageService) GetPackageOwner(ctx context.Context, containerInfo ContainerInfo, fileName string) (string, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.GetPackageOwner"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.GetPackageOwner"))
	provider, err := getProvider(p, containerInfo.OS)
	if err != nil {
		return "", err
	}

	return provider.GetPackageOwner(ctx, containerInfo, fileName)
}

// GetPathByPackageName получает список путей для файла пакета из контейнера.
func (p *PackageService) GetPathByPackageName(ctx context.Context, containerInfo ContainerInfo, packageName, filePath string) ([]string, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.GetPathByPackageName"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.GetPathByPackageName"))
	provider, err := getProvider(p, containerInfo.OS)
	if err != nil {
		return nil, err
	}

	return provider.GetPathByPackageName(ctx, containerInfo, packageName, filePath)
}

// GetInfoPackage возвращает информацию о пакете
func (p *PackageService) GetInfoPackage(ctx context.Context, containerInfo ContainerInfo, packageName string) (InfoPackageAnswer, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.GetInfoPackage"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.GetInfoPackage"))
	// Получаем информацию о пакете из базы данных
	info, err := p.serviceDistroDatabase.GetPackageInfoByName(containerInfo.ContainerName, packageName)
	if err != nil {
		return InfoPackageAnswer{}, fmt.Errorf("не удалось получить информацию о пакете: %s", packageName)
	}

	// Пробуем получить пути для GUI-приложений
	desktopPaths, err := p.GetPathByPackageName(ctx, containerInfo, packageName, "/usr/share/applications/")
	if err != nil {
		lib.Log.Debugf(fmt.Sprintf("Ошибка получения desktop пути: %v", err))
	}

	if len(desktopPaths) > 0 {
		return InfoPackageAnswer{
			Package:   info,
			Paths:     desktopPaths,
			IsConsole: false,
		}, nil
	}

	// Если GUI-пути не найдены, ищем консольные приложения
	consolePaths, err := p.GetPathByPackageName(ctx, containerInfo, packageName, "/usr/bin/")
	if err != nil {
		lib.Log.Debugf(fmt.Sprintf("Ошибка получения консольного пути %v", err))
	}

	return InfoPackageAnswer{
		Package:   info,
		Paths:     consolePaths,
		IsConsole: len(consolePaths) > 0,
	}, nil
}

// UpdatePackages обновляет пакеты и записывает в базу данных
func (p *PackageService) UpdatePackages(ctx context.Context, containerInfo ContainerInfo) ([]PackageInfo, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.UpdatePackages"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.UpdatePackages"))
	packages, err := p.GetPackages(ctx, containerInfo)
	if err != nil {
		lib.Log.Error(err)
		return []PackageInfo{}, err
	}

	errorSave := p.serviceDistroDatabase.SavePackagesToDB(ctx, containerInfo.ContainerName, packages)
	if errorSave != nil {
		lib.Log.Error(errorSave)
		return []PackageInfo{}, errorSave
	}

	return packages, nil
}

// GetPackagesQuery получение списка пакетов с фильтрацией и сортировкой
func (p *PackageService) GetPackagesQuery(ctx context.Context, containerInfo ContainerInfo, builder PackageQueryBuilder) (PackageQueryResult, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.GetPackagesQuery"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.GetPackagesQuery"))
	if builder.ForceUpdate {
		if len(containerInfo.ContainerName) == 0 {
			return PackageQueryResult{}, fmt.Errorf("для операции принудительного обновления необходимо указать контейнер")
		}
		_, err := p.UpdatePackages(ctx, containerInfo)
		if err != nil {
			lib.Log.Error(err)
			return PackageQueryResult{}, err
		}
	}

	packages, err := p.serviceDistroDatabase.QueryPackages(containerInfo.ContainerName, builder.Filters, builder.SortField, builder.SortOrder, builder.Limit, builder.Offset)
	if err != nil {
		return PackageQueryResult{}, err
	}

	total, err := p.serviceDistroDatabase.CountTotalPackages(containerInfo.ContainerName, builder.Filters)
	if err != nil {
		return PackageQueryResult{}, err
	}

	return PackageQueryResult{
		Packages:   packages,
		TotalCount: total,
	}, nil
}

// GetPackageByName поиска пакета по неточному совпадению имени
func (p *PackageService) GetPackageByName(ctx context.Context, containerInfo ContainerInfo, packageName string) (PackageQueryResult, error) {
	packages, err := p.serviceDistroDatabase.FindPackagesByName(containerInfo.ContainerName, packageName)
	if err != nil {
		return PackageQueryResult{}, err
	}

	return PackageQueryResult{
		Packages:   packages,
		TotalCount: len(packages),
	}, nil
}

// GetAllApplicationsByContainer возвращает объединённый список приложений,
// найденных как среди десктопных, так и консольных, без дублей.
func (p *PackageService) GetAllApplicationsByContainer(ctx context.Context, containerInfo ContainerInfo) ([]string, error) {
	var wg sync.WaitGroup
	var desktopApps, consoleApps []string
	var errDesktop, errConsole error

	wg.Add(2)
	go func() {
		defer wg.Done()
		desktopApps, errDesktop = p.GetDesktopApplicationsByContainer(ctx, containerInfo)
	}()
	go func() {
		defer wg.Done()
		consoleApps, errConsole = p.GetConsoleApplicationsByContainer(ctx, containerInfo)
	}()
	wg.Wait()

	if errDesktop != nil {
		lib.Log.Error(fmt.Sprintf("Ошибка при получении desktop приложений для контейнера %s: %v", containerInfo.ContainerName, errDesktop))
	}
	if errConsole != nil {
		lib.Log.Error(fmt.Sprintf("Ошибка при получении консольных приложений для контейнера %s: %v", containerInfo.ContainerName, errConsole))
	}

	// Объединяем оба массива и удаляем дубли
	appsSet := make(map[string]struct{})
	for _, app := range desktopApps {
		appsSet[app] = struct{}{}
	}
	for _, app := range consoleApps {
		appsSet[app] = struct{}{}
	}
	var allApps []string
	for app := range appsSet {
		allApps = append(allApps, app)
	}

	return allApps, nil
}

// GetDesktopApplicationsByContainer ищет .desktop файлы в каталоге "~/.local/share/applications".
// Для каждого найденного файла, если его имя начинается с префикса контейнера,
// удаляет префикс и формирует путь "/usr/share/applications/<trimmedFileName>" для вызова GetPackageOwner.
func (p *PackageService) GetDesktopApplicationsByContainer(ctx context.Context, containerInfo ContainerInfo) ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("не удалось получить домашнюю директорию: %v", err)
	}

	localShareApps := filepath.Join(homeDir, ".local", "share", "applications")
	entries, err := os.ReadDir(localShareApps)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения директории %s: %v", localShareApps, err)
	}

	prefix := containerInfo.ContainerName + "-"
	suffix := ".desktop"
	packageNamesSet := make(map[string]struct{})

	for _, entry := range entries {
		if !entry.IsDir() {
			fileName := entry.Name()
			if strings.HasPrefix(fileName, prefix) && strings.HasSuffix(fileName, suffix) {
				trimmedFileName := strings.TrimPrefix(fileName, prefix)
				packagePath := filepath.Join("/usr/share/applications", trimmedFileName)
				ownerPackage, err := p.GetPackageOwner(ctx, containerInfo, packagePath)
				if err != nil {
					lib.Log.Error(fmt.Sprintf("Ошибка при получении владельца для файла %s: %v", fileName, err))
					continue
				}
				if ownerPackage != "" {
					packageNamesSet[ownerPackage] = struct{}{}
				}
			}
		}
	}

	var packageNames []string
	for pkg := range packageNamesSet {
		packageNames = append(packageNames, pkg)
	}
	return packageNames, nil
}

// GetConsoleApplicationsByContainer ищет исполняемые файлы в каталоге "~/.local/bin".
// Для каждого файла считывается его содержимое; если оно содержит маркер "# name: <containerName>",
// вызывается GetPackageOwner с путем "/usr/bin/<fileName>".
func (p *PackageService) GetConsoleApplicationsByContainer(ctx context.Context, containerInfo ContainerInfo) ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("не удалось получить домашнюю директорию: %v", err)
	}

	localBinApps := filepath.Join(homeDir, ".local", "bin")
	entries, err := os.ReadDir(localBinApps)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения директории %s: %v", localBinApps, err)
	}

	packageNamesSet := make(map[string]struct{})
	marker := "# name: " + containerInfo.ContainerName

	for _, entry := range entries {
		if !entry.IsDir() {
			fileName := entry.Name()
			fullPath := filepath.Join(localBinApps, fileName)
			contentBytes, err := os.ReadFile(fullPath)
			if err != nil {
				lib.Log.Error(fmt.Sprintf("Ошибка при обработке файла %s: %v", fileName, err))
				continue
			}
			content := string(contentBytes)
			if strings.Contains(content, marker) {
				ownerPackage, err := p.GetPackageOwner(ctx, containerInfo, filepath.Join("/usr/bin", fileName))
				if err != nil {
					lib.Log.Error(fmt.Sprintf("Ошибка при получении владельца для файла %s: %v", fileName, err))
					continue
				}
				if ownerPackage != "" {
					packageNamesSet[ownerPackage] = struct{}{}
				}
			}
		}
	}

	var packageNames []string
	for pkg := range packageNamesSet {
		packageNames = append(packageNames, pkg)
	}
	return packageNames, nil
}
