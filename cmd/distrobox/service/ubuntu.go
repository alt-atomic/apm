package service

import (
	"apm/cmd/distrobox/api"
	"apm/lib"
	"context"
	"fmt"
	"regexp"
	"strings"
)

// UbuntuProvider реализует интерфейс PackageProvider для Ubuntu
type UbuntuProvider struct{}

// NewUbuntuProvider возвращает новый экземпляр UbuntuProvider.
func NewUbuntuProvider() *UbuntuProvider {
	return &UbuntuProvider{}
}

// GetPackages получает список пакетов через выполнение команды "apt search ."
// и парсит вывод с учётом установленных пакетов.
func (p *UbuntuProvider) GetPackages(ctx context.Context, containerInfo api.ContainerInfo) ([]PackageInfo, error) {
	// Обновляем базу пакетов.
	updateCmd := fmt.Sprintf("%s distrobox enter %s -- sudo apt-get update", lib.Env.CommandPrefix, containerInfo.ContainerName)
	_, stderr, err := RunCommand(updateCmd)
	if err != nil {
		return nil, fmt.Errorf("не удалось обновить базу пакетов: %v, stderr: %s", err, stderr)
	}

	searchCmd := fmt.Sprintf("%s distrobox enter %s -- apt search .", lib.Env.CommandPrefix, containerInfo.ContainerName)
	stdout, stderr, err := RunCommand(searchCmd)
	if err != nil {
		return nil, fmt.Errorf("не удалось выполнить apt search: %v, stderr: %s", err, stderr)
	}

	installedPackages, err := GetAllApplicationsByContainer(ctx, containerInfo)
	if err != nil {
		lib.Log.Error("Ошибка получения установленных пакетов: ", err)
		installedPackages = []string{}
	}

	packages := p.parseAptOutput(stdout, installedPackages)
	for i := range packages {
		packages[i].Manager = "apt"
	}
	return packages, nil
}

// GetPathByPackageName возвращает список путей для файла пакета, найденных через dpkg -L.
func (p *UbuntuProvider) GetPathByPackageName(ctx context.Context, containerInfo api.ContainerInfo, packageName, filePath string) ([]string, error) {
	command := fmt.Sprintf("%s distrobox enter %s -- dpkg -L %s | grep '%s'", lib.Env.CommandPrefix, containerInfo.ContainerName, packageName, filePath)
	stdout, stderr, err := RunCommand(command)
	if err != nil {
		lib.Log.Debugf("Ошибка выполнения команды: %s %s", stderr, err.Error())
		return []string{}, err
	}

	lines := strings.Split(stdout, "\n")
	var paths []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasSuffix(trimmed, "/") {
			paths = append(paths, trimmed)
		}
	}
	return paths, nil
}

// GetPackageOwner определяет пакет-владельца файла через dpkg -S.
func (p *UbuntuProvider) GetPackageOwner(ctx context.Context, containerInfo api.ContainerInfo, filePath string) (string, error) {
	command := fmt.Sprintf("%s distrobox enter %s -- dpkg -S %s", lib.Env.CommandPrefix, containerInfo.ContainerName, filePath)
	stdout, _, err := RunCommand(command)
	if err != nil {
		return "", err
	}
	// Ожидаемый вывод: "<package>: /usr/bin/<fileName>"
	re := regexp.MustCompile(`^([^:]+):`)
	matches := re.FindStringSubmatch(stdout)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1]), nil
	}
	return "", nil
}

// InstallPackage устанавливает указанный пакет внутри контейнера через apt-get install.
func (p *UbuntuProvider) InstallPackage(ctx context.Context, containerInfo api.ContainerInfo, packageName string) error {
	command := fmt.Sprintf("%s distrobox enter %s -- sudo apt-get install -y %s", lib.Env.CommandPrefix, containerInfo.ContainerName, packageName)
	_, stderr, err := RunCommand(command)
	if err != nil {
		return fmt.Errorf("не удалось установить пакет %s: %v, stderr: %s", packageName, err, stderr)
	}

	return nil
}

// RemovePackage удаляет указанный пакет внутри контейнера через apt-get remove.
func (p *UbuntuProvider) RemovePackage(ctx context.Context, containerInfo api.ContainerInfo, packageName string) error {
	command := fmt.Sprintf("%s distrobox enter %s -- sudo apt-get remove -y %s", lib.Env.CommandPrefix, containerInfo.ContainerName, packageName)
	_, stderr, err := RunCommand(command)
	if err != nil {
		return fmt.Errorf("не удалось удалить пакет %s: %v, stderr: %s", packageName, err, stderr)
	}

	return nil
}

// parseAptOutput парсит вывод команды apt search . и возвращает срез PackageInfo.
func (p *UbuntuProvider) parseAptOutput(output string, installedPackages []string) []PackageInfo {
	lines := strings.Split(output, "\n")
	var packages []PackageInfo
	var currentPkg *PackageInfo

	// Регулярное выражение для строки с информацией о пакете.
	// Пример строки: "vim/focal 2:8.1.2269-1ubuntu5 amd64 [installed] Vi IMproved, a highly configurable, improved version of the vi text editor"
	pkgRegex := regexp.MustCompile(`^([^/\s]+)\/(\S+)\s+(\S+)\s+(amd64|i386|all|arm64|armhf)(?:\s+(.*))?$`)

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		// Определяем, установлен ли пакет.
		isInstalled := strings.Contains(line, "[installed")
		// Удаляем маркеры [installed ...] и [upgradable from: ...]
		line = regexp.MustCompile(`\[[^\]]+\]`).ReplaceAllString(line, "")
		line = strings.TrimSpace(line)

		match := pkgRegex.FindStringSubmatch(line)
		if match != nil {
			if currentPkg != nil {
				packages = append(packages, *currentPkg)
			}
			currentPkg = &PackageInfo{
				PackageName: match[1],
				Version:     match[3],
				Description: strings.TrimSpace(match[5]),
				Installed:   isInstalled,
				Exporting:   contains(installedPackages, match[1]),
			}
		} else {
			if currentPkg != nil {
				if currentPkg.Description != "" {
					currentPkg.Description += " "
				}
				currentPkg.Description += line
			}
		}
	}
	if currentPkg != nil {
		packages = append(packages, *currentPkg)
	}
	return packages
}

// contains проверяет, содержится ли значение в срезе.
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
