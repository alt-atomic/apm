package os

import (
	"apm/logger"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"apm/cmd/distrobox/api"
)

// PackageInfo описывает информацию о пакете.
type PackageInfo struct {
	PackageName string `json:"packageName"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
	Exporting   bool   `json:"exporting"`
	Manager     string `json:"manager"`
}

// PackageQueryResult содержит срез найденных пакетов и общее количество записей, удовлетворяющих фильтрам.
type PackageQueryResult struct {
	Packages   []PackageInfo
	TotalCount int
}

// PackageQueryBuilder задаёт параметры запроса.
type PackageQueryBuilder struct {
	ForceUpdate bool                   // обновление перед тем как выполнить запрос
	Limit       int64                  // если Limit <= 0, то ограничение не применяется
	Offset      int64                  // если Offset < 0, то считается 0
	Filters     map[string]interface{} // фильтры вида "field": value; используется условие "="
	SortField   string                 // поле сортировки (например, "packageName")
	SortOrder   string                 // "ASC" или "DESC"
}

type InfoPackageAnswer struct {
	PackageInfo PackageInfo
	Paths       []string
	IsConsole   bool
}

// PackageProvider задаёт интерфейс для работы с пакетами в контейнере.
type PackageProvider interface {
	GetPackages(containerInfo api.ContainerInfo) ([]PackageInfo, error)
	RemovePackage(containerInfo api.ContainerInfo, packageName string) error
	InstallPackage(containerInfo api.ContainerInfo, packageName string) error
	GetPackageOwner(containerInfo api.ContainerInfo, fileName string) (string, error)
	GetPathByPackageName(containerInfo api.ContainerInfo, packageName, filePath string) ([]string, error)
}

// getProvider возвращает подходящий провайдер в зависимости от имени ОС контейнера.
func getProvider(osName string) (PackageProvider, error) {
	lowerName := strings.ToLower(osName)
	if strings.Contains(lowerName, "ubuntu") || strings.Contains(lowerName, "debian") {
		return NewUbuntuProvider(), nil
	} else if strings.Contains(lowerName, "arch") {
		return NewArchProvider(), nil
	} else {
		return nil, errors.New("Данный контейнер не поддерживается: " + osName)
	}
}

// InstallPackage установка пакета
func InstallPackage(containerInfo api.ContainerInfo, packageName string) error {
	provider, err := getProvider(containerInfo.OS)
	if err != nil {
		return err
	}
	return provider.InstallPackage(containerInfo, packageName)
}

// RemovePackage удаление пакета
func RemovePackage(containerInfo api.ContainerInfo, packageName string) error {
	provider, err := getProvider(containerInfo.OS)
	if err != nil {
		return err
	}
	return provider.RemovePackage(containerInfo, packageName)
}

// GetPackages получает список пакетов из контейнера.
func GetPackages(containerInfo api.ContainerInfo) ([]PackageInfo, error) {
	provider, err := getProvider(containerInfo.OS)
	if err != nil {
		return nil, err
	}
	return provider.GetPackages(containerInfo)
}

// GetPackageOwner получает название пакета, которому принадлежит указанный файл, из контейнера.
func GetPackageOwner(containerInfo api.ContainerInfo, fileName string) (string, error) {
	provider, err := getProvider(containerInfo.OS)
	if err != nil {
		return "", err
	}
	return provider.GetPackageOwner(containerInfo, fileName)
}

// GetPathByPackageName получает список путей для файла пакета из контейнера.
func GetPathByPackageName(containerInfo api.ContainerInfo, packageName, filePath string) ([]string, error) {
	provider, err := getProvider(containerInfo.OS)
	if err != nil {
		return nil, err
	}
	return provider.GetPathByPackageName(containerInfo, packageName, filePath)
}

// GetInfoPackage возвращает информацию о пакете
func GetInfoPackage(containerInfo api.ContainerInfo, packageName string) (InfoPackageAnswer, error) {
	// Получаем информацию о пакете из базы данных
	info, err := GetPackageInfoByName(containerInfo.ContainerName, packageName)
	if err != nil {
		return InfoPackageAnswer{}, fmt.Errorf("не удалось получить информацию о пакете: %s %v", packageName, err)
	}

	// Пробуем получить пути для GUI-приложений
	desktopPaths, err := GetPathByPackageName(containerInfo, packageName, "/usr/share/applications/")
	if err != nil {
		logger.Log.Debugf(fmt.Sprintf("Ошибка получения desktop пути: %v", err))
	}

	if len(desktopPaths) > 0 {
		return InfoPackageAnswer{
			PackageInfo: info,
			Paths:       desktopPaths,
			IsConsole:   false,
		}, nil
	}

	// Если GUI-пути не найдены, ищем консольные приложения
	consolePaths, err := GetPathByPackageName(containerInfo, packageName, "/usr/bin/")
	if err != nil {
		logger.Log.Debugf(fmt.Sprintf("Ошибка получения консольного пути %v", err))
	}

	return InfoPackageAnswer{
		PackageInfo: info,
		Paths:       consolePaths,
		IsConsole:   len(consolePaths) > 0,
	}, nil
}

// UpdatePackages обновляет пакеты и записывает в базу данных
func UpdatePackages(containerInfo api.ContainerInfo) ([]PackageInfo, error) {
	packages, err := GetPackages(containerInfo)
	if err != nil {
		logger.Log.Error(err)
		return []PackageInfo{}, err
	}

	errorSave := SavePackagesToDB(containerInfo.ContainerName, packages)
	if errorSave != nil {
		logger.Log.Error(errorSave)
		return []PackageInfo{}, errorSave
	}

	return packages, nil
}

// GetPackagesQuery получение списка пакетов с фильтрацией и сортировкой
func GetPackagesQuery(containerInfo api.ContainerInfo, builder PackageQueryBuilder) (PackageQueryResult, error) {
	if builder.ForceUpdate {
		_, err := UpdatePackages(containerInfo)
		if err != nil {
			logger.Log.Error(err)
			return PackageQueryResult{}, err
		}
	}

	packages, err := QueryPackages(containerInfo.ContainerName, builder.Filters, builder.SortField, builder.SortOrder, builder.Limit, builder.Offset)
	if err != nil {
		return PackageQueryResult{}, err
	}

	total, err := CountTotalPackages(containerInfo.ContainerName, builder.Filters)
	if err != nil {
		return PackageQueryResult{}, err
	}

	return PackageQueryResult{
		Packages:   packages,
		TotalCount: total,
	}, nil
}

// GetPackageByName поиска пакета по неточному совпадению имени
func GetPackageByName(containerInfo api.ContainerInfo, packageName string) (PackageQueryResult, error) {
	packages, err := FindPackagesByName(containerInfo.ContainerName, packageName)
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
func GetAllApplicationsByContainer(containerInfo api.ContainerInfo) ([]string, error) {
	var wg sync.WaitGroup
	var desktopApps, consoleApps []string
	var errDesktop, errConsole error

	wg.Add(2)
	go func() {
		defer wg.Done()
		desktopApps, errDesktop = GetDesktopApplicationsByContainer(containerInfo)
	}()
	go func() {
		defer wg.Done()
		consoleApps, errConsole = GetConsoleApplicationsByContainer(containerInfo)
	}()
	wg.Wait()

	if errDesktop != nil {
		logger.Log.Error(fmt.Sprintf("Ошибка при получении desktop приложений для контейнера %s: %v", containerInfo.ContainerName, errDesktop))
	}
	if errConsole != nil {
		logger.Log.Error(fmt.Sprintf("Ошибка при получении консольных приложений для контейнера %s: %v", containerInfo.ContainerName, errConsole))
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
func GetDesktopApplicationsByContainer(containerInfo api.ContainerInfo) ([]string, error) {
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
				ownerPackage, err := GetPackageOwner(containerInfo, packagePath)
				if err != nil {
					logger.Log.Error(fmt.Sprintf("Ошибка при получении владельца для файла %s: %v", fileName, err))
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
func GetConsoleApplicationsByContainer(containerInfo api.ContainerInfo) ([]string, error) {
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
				logger.Log.Error(fmt.Sprintf("Ошибка при обработке файла %s: %v", fileName, err))
				continue
			}
			content := string(contentBytes)
			if strings.Contains(content, marker) {
				ownerPackage, err := GetPackageOwner(containerInfo, filepath.Join("/usr/bin", fileName))
				if err != nil {
					logger.Log.Error(fmt.Sprintf("Ошибка при получении владельца для файла %s: %v", fileName, err))
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

// RunCommand выполняет команду и возвращает stdout, stderr и ошибку.
func RunCommand(command string) (string, string, error) {
	cmd := exec.Command("sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
