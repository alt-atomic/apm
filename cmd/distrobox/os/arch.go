package os

import (
	"apm/config"
	"apm/logger"
	"fmt"
	"regexp"
	"strings"

	"apm/cmd/distrobox/api"
)

// ArchProvider реализует методы для работы с пакетами в Arch
type ArchProvider struct{}

// NewArchProvider возвращает новый экземпляр ArchProvider.
func NewArchProvider() *ArchProvider {
	return &ArchProvider{}
}

// GetPackages обновляет базу пакетов и выполняет поиск:
func (p *ArchProvider) GetPackages(containerInfo api.ContainerInfo) ([]PackageInfo, error) {
	// Обновляем базу пакетов и базу владельцев файлов.
	updateCmd := fmt.Sprintf("%s distrobox enter %s -- sudo pacman -Sy ", config.Env.CommandPrefix, containerInfo.ContainerName)
	if _, stderr, err := RunCommand(updateCmd); err != nil {
		return nil, fmt.Errorf("не удалось обновить базу пакетов: %v, stderr: %s", err, stderr)
	}

	// Получаем пакеты из официальных репозиториев
	commandSs := fmt.Sprintf("%s distrobox enter %s -- sudo pacman -Ss", config.Env.CommandPrefix, containerInfo.ContainerName)
	stdoutSs, stderrSs, err := RunCommand(commandSs)
	if err != nil {
		return nil, fmt.Errorf("не удалось выполнить поиск пакетов (pacman -Ss): %v, stderr: %s", err, stderrSs)
	}

	installedPackages, err := GetAllApplicationsByContainer(containerInfo)
	if err != nil {
		logger.Log.Error("Ошибка получения установленных пакетов: ", err)
		installedPackages = []string{}
	}

	packagesOfficial, err := p.parseOutput(stdoutSs, installedPackages)
	if err != nil {
		logger.Log.Errorf("Ошибка парсинга официальных пакетов: %v", err)
		return nil, err
	}

	return packagesOfficial, nil
}

// RemovePackage удаляет указанный пакет с помощью pacman -R.
func (p *ArchProvider) RemovePackage(containerInfo api.ContainerInfo, packageName string) error {
	cmdStr := fmt.Sprintf("%s distrobox enter %s -- sudo sudo pacman -Rs --noconfirm %s", config.Env.CommandPrefix, containerInfo.ContainerName, packageName)
	_, stderr, err := RunCommand(cmdStr)
	if err != nil {
		return fmt.Errorf("не удалось удалить пакет %s: %v, stderr: %s", packageName, err, stderr)
	}
	return nil
}

// InstallPackage устанавливает указанный пакет с помощью pacman -S.
func (p *ArchProvider) InstallPackage(containerInfo api.ContainerInfo, packageName string) error {
	cmdStr := fmt.Sprintf("%s distrobox enter %s -- sudo sudo pacman -S --noconfirm %s", config.Env.CommandPrefix, containerInfo.ContainerName, packageName)
	_, stderr, err := RunCommand(cmdStr)
	if err != nil {
		return fmt.Errorf("не удалось установить пакет %s: %v, stderr: %s", packageName, err, stderr)
	}
	return nil
}

// GetPackageOwner определяет, какому пакету принадлежит указанный файл.
// Сначала используется pacman -Qo для поиска установленного пакета,
// затем, если не найден, выполняется поиск через pacman -F.
func (p *ArchProvider) GetPackageOwner(containerInfo api.ContainerInfo, fileName string) (string, error) {
	// Попытка через pacman -Qo.
	cmdStr := fmt.Sprintf("%s distrobox enter %s -- pacman -Qo %s", config.Env.CommandPrefix, containerInfo.ContainerName, fileName)
	stdout, _, err := RunCommand(cmdStr)
	if err == nil {
		ownerInfo := strings.TrimSpace(stdout)
		const marker = " is owned by "
		idx := strings.Index(ownerInfo, marker)
		if idx != -1 {
			ownedPart := ownerInfo[idx+len(marker):]
			fields := strings.Fields(ownedPart)
			if len(fields) >= 1 {
				return fields[0], nil
			}
		}
		return "", fmt.Errorf("не удалось распознать владельца для файла '%s'", fileName)
	}

	// Если не найдено, пробуем через pacman -F.
	cmdStr = fmt.Sprintf("%s distrobox enter %s -- pacman -F %s", config.Env.CommandPrefix, containerInfo.ContainerName, fileName)
	stdout, stderr, err := RunCommand(cmdStr)
	if err != nil {
		return "", fmt.Errorf("не удалось найти пакет для файла '%s': %v, stderr: %s", fileName, err, stderr)
	}
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "error:") || strings.HasPrefix(line, "warning:") {
			continue
		}
		if strings.Contains(line, "/") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			repoAndName := parts[0]
			slashIdx := strings.Index(repoAndName, "/")
			if slashIdx == -1 {
				continue
			}
			pkgName := repoAndName[slashIdx+1:]
			return pkgName, nil
		}
	}
	return "", fmt.Errorf("не удалось определить пакет для файла '%s'", fileName)
}

// GetPathByPackageName возвращает список путей для файла, принадлежащего указанному пакету,
// используя команду pacman -Ql и фильтрацию по filePath.
func (p *ArchProvider) GetPathByPackageName(containerInfo api.ContainerInfo, packageName, filePath string) ([]string, error) {
	cmdStr := fmt.Sprintf("%s distrobox enter %s -- pacman -Ql %s | grep '%s'", config.Env.CommandPrefix, containerInfo.ContainerName, packageName, filePath)
	stdout, stderr, err := RunCommand(cmdStr)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения команды: %v, stderr: %s", err, stderr)
	}
	lines := strings.Split(stdout, "\n")
	var paths []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasSuffix(trimmed, "/") {
			parts := strings.Fields(trimmed)
			if len(parts) > 1 {
				paths = append(paths, parts[1])
			}
		}
	}
	return paths, nil
}

// parseOutput парсит вывод команды
func (p *ArchProvider) parseOutput(output string, installedPackages []string) ([]PackageInfo, error) {
	// Регулярное выражение для парсинга строки:
	// Пример: "core/vim 8.2.3456-1 [installed] Vi IMproved, a highly configurable, improved version of the vi text editor"
	re := regexp.MustCompile(`^([a-z]+)\/([\w\.-]+)\s+([^\s]+)`)
	lines := strings.Split(output, "\n")
	var results []PackageInfo
	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		// Пропускаем записи AUR.
		if strings.HasPrefix(line, "aur/") {
			i++
			continue
		}
		// Пробуем найти совпадение по регулярному выражению.
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			i++
			continue
		}

		repo := matches[1]
		if repo != "core" && repo != "extra" {
			i++
			continue
		}
		pkgName := matches[2]
		version := matches[3]
		installed := strings.Contains(line, "[installed") || strings.Contains(line, "(установлено:")
		description := ""
		if i+1 < len(lines) && !strings.Contains(lines[i+1], "/") {
			description = strings.TrimSpace(lines[i+1])
			i++
		}
		if description == "" {
			description = "-"
		}

		results = append(results, PackageInfo{
			PackageName: pkgName,
			Version:     version,
			Description: description,
			Installed:   installed,
			Exporting:   contains(installedPackages, pkgName),
			Manager:     "pacman",
		})
		i++
	}
	return results, nil
}
