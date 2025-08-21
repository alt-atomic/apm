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
	"apm/internal/common/helper"
	"apm/lib"
	"context"
	"fmt"
	"regexp"
	"strings"
)

// ArchProvider реализует методы для работы с пакетами в Arch
type ArchProvider struct {
	servicePackage *PackageService
}

// NewArchProvider возвращает новый экземпляр ArchProvider.
func NewArchProvider(servicePackage *PackageService) *ArchProvider {
	return &ArchProvider{
		servicePackage: servicePackage,
	}
}

// GetPackages обновляет базу пакетов и выполняет поиск:
func (p *ArchProvider) GetPackages(ctx context.Context, containerInfo ContainerInfo) ([]PackageInfo, error) {
	// Обновляем базу пакетов и базу владельцев файлов.
	updateCmd := fmt.Sprintf("%s distrobox enter %s -- sudo pacman -Suy ", lib.Env.CommandPrefix, containerInfo.ContainerName)
	if _, stderr, err := helper.RunCommand(ctx, updateCmd); err != nil {
		return nil, fmt.Errorf(lib.T_("Failed to update package database: %v, stderr: %s"), err, stderr)
	}

	// Получаем пакеты из официальных репозиториев
	commandSs := fmt.Sprintf("%s distrobox enter %s -- sudo pacman -Ss", lib.Env.CommandPrefix, containerInfo.ContainerName)
	stdoutSs, stderrSs, err := helper.RunCommand(ctx, commandSs)
	if err != nil {
		return nil, fmt.Errorf(lib.T_("Failed to search packages (pacman -Ss): %v, stderr: %s"), err, stderrSs)
	}

	exportingPackages, err := p.servicePackage.GetAllApplicationsByContainer(ctx, containerInfo)
	if err != nil {
		lib.Log.Error(lib.T_("Error retrieving installed packages: "), err)
		exportingPackages = []string{}
	}

	packagesOfficial, err := p.parseOutput(stdoutSs, exportingPackages)
	if err != nil {
		lib.Log.Errorf(lib.T_("Error parsing official packages: %v"), err)
		return nil, err
	}

	for i := range packagesOfficial {
		packagesOfficial[i].Container = containerInfo.ContainerName
	}

	return packagesOfficial, nil
}

// RemovePackage удаляет указанный пакет с помощью pacman -R.
func (p *ArchProvider) RemovePackage(ctx context.Context, containerInfo ContainerInfo, packageName string) error {
	cmdStr := fmt.Sprintf("%s distrobox enter %s -- sudo pacman -Rs --noconfirm %s", lib.Env.CommandPrefix, containerInfo.ContainerName, packageName)
	_, stderr, err := helper.RunCommand(ctx, cmdStr)
	if err != nil {
		return fmt.Errorf(lib.T_("Failed to remove package %s: %v, stderr: %s"), packageName, err, stderr)
	}
	return nil
}

// InstallPackage устанавливает указанный пакет с помощью pacman -S.
func (p *ArchProvider) InstallPackage(ctx context.Context, containerInfo ContainerInfo, packageName string) error {
	cmdStr := fmt.Sprintf("%s distrobox enter %s -- sudo pacman -S --noconfirm %s", lib.Env.CommandPrefix, containerInfo.ContainerName, packageName)
	_, stderr, err := helper.RunCommand(ctx, cmdStr)
	if err != nil {
		return fmt.Errorf(lib.T_("Failed to install package %s: %v, stderr: %s"), packageName, err, stderr)
	}
	return nil
}

// GetPackageOwner определяет, какому пакету принадлежит указанный файл.
// Сначала используется pacman -Qo для поиска установленного пакета,
// затем, если не найден, выполняется поиск через pacman -F.
func (p *ArchProvider) GetPackageOwner(ctx context.Context, containerInfo ContainerInfo, fileName string) (string, error) {
	// Попытка через pacman -Qo.
	cmdStr := fmt.Sprintf("%s distrobox enter %s -- pacman -Qo %s", lib.Env.CommandPrefix, containerInfo.ContainerName, fileName)
	stdout, _, err := helper.RunCommand(ctx, cmdStr)
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
		return "", fmt.Errorf(lib.T_("Failed to recognize the owner for file '%s'"), fileName)
	}

	// Если не найдено, пробуем через pacman -F.
	cmdStr = fmt.Sprintf("%s distrobox enter %s -- pacman -F %s", lib.Env.CommandPrefix, containerInfo.ContainerName, fileName)
	stdout, stderr, err := helper.RunCommand(ctx, cmdStr)
	if err != nil {
		return "", fmt.Errorf(lib.T_("Failed to find a package for file '%s': %v, stderr: %s"), fileName, err, stderr)
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
	return "", fmt.Errorf(lib.T_("Failed to determine the package for file '%s'"), fileName)
}

// GetPathByPackageName возвращает список путей для файла, принадлежащего указанному пакету,
// используя команду pacman -Ql и фильтрацию по filePath.
func (p *ArchProvider) GetPathByPackageName(ctx context.Context, containerInfo ContainerInfo, packageName, filePath string) ([]string, error) {
	parseOutput := func(output string) []string {
		lines := strings.Split(output, "\n")
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
		return paths
	}

	cmdStr := fmt.Sprintf("%s distrobox enter %s -- pacman -Ql %s | grep '%s'", lib.Env.CommandPrefix, containerInfo.ContainerName, packageName, filePath)
	stdout, stderr, err := helper.RunCommand(ctx, cmdStr)
	if err != nil {
		lib.Log.Debugf(lib.T_("Command execution error: %s %s"), stderr, err.Error())
	}

	paths := parseOutput(stdout)
	if len(paths) == 0 {
		fallbackCommand := fmt.Sprintf("%s distrobox enter %s -- pacman -Qq | grep '^%s' | xargs pacman -Ql | grep '%s' | sort",
			lib.Env.CommandPrefix, containerInfo.ContainerName, packageName, filePath)
		fallbackStdout, fallbackStderr, fallbackErr := helper.RunCommand(ctx, fallbackCommand)
		if fallbackErr != nil {
			lib.Log.Debugf(lib.T_("Fallback command execution error: %s %s"), fallbackStderr, fallbackErr.Error())
			return paths, nil
		}

		fallbackPaths := parseOutput(fallbackStdout)
		if len(fallbackPaths) > 0 {
			lib.Log.Debugf(lib.T_("Fallback search found %d files"), len(fallbackPaths))
			return fallbackPaths, nil
		}
	}

	return paths, nil
}

// parseOutput парсит вывод команды
func (p *ArchProvider) parseOutput(output string, exportingPackages []string) ([]PackageInfo, error) {
	// Регулярное выражение для парсинга строки:
	// Пример: "core/vim 8.2.3456-1 [installed] Vi IMproved, a highly configurable, improved version of the vi text editor"
	re := regexp.MustCompile(`^([a-z]+)\/([\w\.-]+)\s+([^\s]+)`)
	lines := strings.Split(output, "\n")
	var results []PackageInfo

	seen := make(map[string]bool)

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		// Пробуем найти совпадение по регулярному выражению.
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			i++
			continue
		}

		//repo := matches[1]
		//if repo != "core" && repo != "extra" {
		//	i++
		//	continue
		//}
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

		// Если пакет с таким именем уже добавлен, пропускаем его
		if seen[pkgName] {
			i++
			continue
		}
		seen[pkgName] = true

		results = append(results, PackageInfo{
			Name:        pkgName,
			Version:     version,
			Description: description,
			Installed:   installed,
			Exporting:   contains(exportingPackages, pkgName),
			Manager:     "pacman",
		})
		i++
	}

	return results, nil
}
