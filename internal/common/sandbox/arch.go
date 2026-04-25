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

package sandbox

import (
	"apm/internal/common/app"
	"apm/internal/common/command"
	"apm/internal/common/helper"
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// ArchProvider реализует методы для работы с пакетами в Arch
type ArchProvider struct {
	servicePackage *PackageService
	runner         command.Runner
}

// NewArchProvider возвращает новый экземпляр ArchProvider.
func NewArchProvider(servicePackage *PackageService, runner command.Runner) *ArchProvider {
	return &ArchProvider{
		servicePackage: servicePackage,
		runner:         runner,
	}
}

// GetPackages обновляет базу пакетов и выполняет поиск:
func (p *ArchProvider) GetPackages(ctx context.Context, containerInfo ContainerInfo) ([]PackageInfo, error) {
	if _, stderr, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "sudo", "pacman", "-Sy", "--noconfirm"}, command.WithQuiet()); err != nil {
		return nil, fmt.Errorf(app.T_("Failed to update package database: %v, stderr: %s"), err, stderr)
	}

	// Получаем пакеты из официальных репозиториев
	stdoutSs, stderrSs, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "pacman", "-Ss"}, command.WithEnv("LC_ALL=C"), command.WithQuiet())
	if err != nil {
		return nil, fmt.Errorf(app.T_("Failed to search packages (pacman -Ss): %v, stderr: %s"), err, stderrSs)
	}

	exportingPackages, err := p.servicePackage.GetAllApplicationsByContainer(ctx, containerInfo)
	if err != nil {
		app.Log.Error(app.T_("Error retrieving exporting packages: "), err)
		exportingPackages = []string{}
	}

	installedPackages, err := p.getInstalledPackages(ctx, containerInfo)
	if err != nil {
		app.Log.Error(app.T_("Error retrieving installed packages: "), err)
		installedPackages = []string{}
	}
	installedMap := make(map[string]bool, len(installedPackages))
	for _, pkg := range installedPackages {
		installedMap[pkg] = true
	}

	packagesOfficial, err := p.parseOutput(stdoutSs, exportingPackages, installedMap)
	if err != nil {
		app.Log.Errorf(app.T_("Error parsing official packages: %v"), err)
		return nil, err
	}

	for i := range packagesOfficial {
		packagesOfficial[i].Container = containerInfo.ContainerName
	}

	return packagesOfficial, nil
}

// getInstalledPackages возвращает список установленных пакетов через pacman -Qq.
func (p *ArchProvider) getInstalledPackages(ctx context.Context, containerInfo ContainerInfo) ([]string, error) {
	stdout, _, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "pacman", "-Qq"}, command.WithEnv("LC_ALL=C"), command.WithQuiet())
	if err != nil {
		return nil, fmt.Errorf(app.T_("Error executing command pacman -Qq: %w"), err)
	}

	var packages []string
	for _, rawLine := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(rawLine)
		if line != "" {
			packages = append(packages, line)
		}
	}
	return packages, nil
}

// RemovePackage удаляет указанный пакет с помощью pacman -R.
func (p *ArchProvider) RemovePackage(ctx context.Context, containerInfo ContainerInfo, packageName string) error {
	if err := validatePackageName(packageName); err != nil {
		return err
	}
	_, stderr, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "sudo", "pacman", "-Rs", "--noconfirm", packageName})
	if err != nil {
		return fmt.Errorf(app.T_("Failed to remove package %s: %v, stderr: %s"), packageName, err, stderr)
	}
	return nil
}

// InstallPackage устанавливает указанный пакет с помощью pacman -S.
func (p *ArchProvider) InstallPackage(ctx context.Context, containerInfo ContainerInfo, packageName string) error {
	if err := validatePackageName(packageName); err != nil {
		return err
	}
	_, stderr, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "sudo", "pacman", "-S", "--noconfirm", packageName})
	if err != nil {
		return fmt.Errorf(app.T_("Failed to install package %s: %v, stderr: %s"), packageName, err, stderr)
	}
	return nil
}

// GetPackageOwner определяет, какому пакету принадлежит указанный файл.
// Сначала используется pacman -Qo для поиска установленного пакета,
// затем, если не найден, выполняется поиск через pacman -F.
func (p *ArchProvider) GetPackageOwner(ctx context.Context, containerInfo ContainerInfo, fileName string) (string, error) {
	// Попытка через pacman -Qo.
	stdout, _, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "pacman", "-Qo", fileName}, command.WithEnv("LC_ALL=C"), command.WithQuiet())
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
		return "", fmt.Errorf(app.T_("Failed to recognize the owner for file '%s'"), fileName)
	}

	// Если не найдено, пробуем через pacman -F.
	stdout, stderr, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "pacman", "-F", fileName}, command.WithEnv("LC_ALL=C"), command.WithQuiet())
	if err != nil {
		return "", fmt.Errorf(app.T_("Failed to find a package for file '%s': %v, stderr: %s"), fileName, err, stderr)
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
	return "", fmt.Errorf(app.T_("Failed to determine the package for file '%s'"), fileName)
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

	stdout, stderr, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "pacman", "-Ql", packageName}, command.WithQuiet())
	if err != nil {
		app.Log.Debugf(app.T_("Command execution error: %s %s"), stderr, err.Error())
	}

	filtered := helper.FilterLines(stdout, filePath)
	paths := parseOutput(filtered)
	if len(paths) == 0 {
		qqStdout, qqStderr, qqErr := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "pacman", "-Qq"}, command.WithQuiet())
		if qqErr != nil {
			app.Log.Debugf(app.T_("Fallback command execution error: %s %s"), qqStderr, qqErr.Error())
			return paths, nil
		}

		matchedPackages := helper.FilterLinesPrefix(qqStdout, packageName)
		var allPaths []string
		for _, pkg := range strings.Split(matchedPackages, "\n") {
			pkg = strings.TrimSpace(pkg)
			if pkg == "" {
				continue
			}
			qlStdout, _, qlErr := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "pacman", "-Ql", pkg}, command.WithQuiet())
			if qlErr != nil {
				continue
			}
			filteredLines := helper.FilterLines(qlStdout, filePath)
			for _, line := range strings.Split(filteredLines, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasSuffix(trimmed, "/") {
					parts := strings.Fields(trimmed)
					if len(parts) > 1 {
						allPaths = append(allPaths, parts[1])
					}
				}
			}
		}
		sort.Strings(allPaths)
		if len(allPaths) > 0 {
			app.Log.Debugf(app.T_("Fallback search found %d files"), len(allPaths))
			return allPaths, nil
		}
	}

	return paths, nil
}

// parseOutput парсит вывод команды
func (p *ArchProvider) parseOutput(output string, exportingPackages []string, installedMap map[string]bool) ([]PackageInfo, error) {
	re := regexp.MustCompile(`^([a-z]+)/([\w.-]+)\s+(\S+)`)
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

		pkgName := matches[2]
		version := matches[3]
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
			Installed:   installedMap[pkgName],
			Exporting:   slices.Contains(exportingPackages, pkgName),
			Manager:     "pacman",
		})
		i++
	}

	return results, nil
}
