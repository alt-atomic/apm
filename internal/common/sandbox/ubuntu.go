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
	"apm/internal/common/helper"
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// UbuntuProvider реализует интерфейс PackageProvider для Ubuntu
type UbuntuProvider struct {
	servicePackage *PackageService
	commandPrefix  string
}

// NewUbuntuProvider возвращает новый экземпляр UbuntuProvider.
func NewUbuntuProvider(servicePackage *PackageService, commandPrefix string) *UbuntuProvider {
	return &UbuntuProvider{
		servicePackage: servicePackage,
		commandPrefix:  commandPrefix,
	}
}

// GetPackages получает список пакетов через выполнение команды "apt search ."
// и парсит вывод с учётом установленных пакетов.
func (p *UbuntuProvider) GetPackages(ctx context.Context, containerInfo ContainerInfo) ([]PackageInfo, error) {
	// Обновляем базу пакетов.
	updateArgs := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "sudo", "apt-get", "update")
	_, stderr, err := helper.RunCommand(ctx, updateArgs)
	if err != nil {
		return nil, fmt.Errorf(app.T_("Failed to update package database: %v, stderr: %s"), err, stderr)
	}

	searchArgs := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "apt", "search", ".")
	stdout, stderr, err := helper.RunCommand(ctx, searchArgs)
	if err != nil {
		return nil, fmt.Errorf(app.T_("Failed to execute apt search: %v, stderr: %s"), err, stderr)
	}

	exportingPackages, err := p.servicePackage.GetAllApplicationsByContainer(ctx, containerInfo)
	if err != nil {
		app.Log.Error(app.T_("Failed to retrieve installed packages: "), err)
		exportingPackages = []string{}
	}

	packages := p.parseAptOutput(stdout, exportingPackages)
	for i := range packages {
		packages[i].Manager = "apt"
		packages[i].Container = containerInfo.ContainerName
	}
	return packages, nil
}

// GetPathByPackageName возвращает список путей для файла пакета, найденных через dpkg -L.
func (p *UbuntuProvider) GetPathByPackageName(ctx context.Context, containerInfo ContainerInfo, packageName, filePath string) ([]string, error) {
	parseOutput := func(output string) []string {
		lines := strings.Split(output, "\n")
		var paths []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasSuffix(trimmed, "/") {
				paths = append(paths, trimmed)
			}
		}
		return paths
	}

	args := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "dpkg", "-L", packageName)
	stdout, stderr, err := helper.RunCommand(ctx, args)
	if err != nil {
		app.Log.Debugf(app.T_("Command execution error: %s %s"), stderr, err.Error())
	}

	filtered := helper.FilterLines(stdout, filePath)
	paths := parseOutput(filtered)
	if len(paths) == 0 {
		dlArgs := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "dpkg", "-l")
		dlStdout, dlStderr, dlErr := helper.RunCommand(ctx, dlArgs)
		if dlErr != nil {
			app.Log.Debugf(app.T_("Fallback command execution error: %s %s"), dlStderr, dlErr.Error())
			return paths, nil
		}

		var matchedPackages []string
		for _, line := range strings.Split(dlStdout, "\n") {
			if strings.HasPrefix(line, "ii") && strings.Contains(line, packageName) {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					matchedPackages = append(matchedPackages, fields[1])
				}
			}
		}

		var allPaths []string
		for _, pkg := range matchedPackages {
			pkgArgs := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "dpkg", "-L", pkg)
			pkgStdout, _, pkgErr := helper.RunCommand(ctx, pkgArgs)
			if pkgErr != nil {
				continue
			}
			filteredLines := helper.FilterLines(pkgStdout, filePath)
			for _, line := range strings.Split(filteredLines, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasSuffix(trimmed, "/") {
					allPaths = append(allPaths, trimmed)
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

// GetPackageOwner определяет пакет-владельца файла через dpkg -S.
func (p *UbuntuProvider) GetPackageOwner(ctx context.Context, containerInfo ContainerInfo, filePath string) (string, error) {
	args := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "dpkg", "-S", filePath)
	stdout, _, err := helper.RunCommand(ctx, args)
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
func (p *UbuntuProvider) InstallPackage(ctx context.Context, containerInfo ContainerInfo, packageName string) error {
	if err := validatePackageName(packageName); err != nil {
		return err
	}
	args := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "sudo", "apt-get", "install", "-y", packageName)
	_, stderr, err := helper.RunCommand(ctx, args)
	if err != nil {
		return fmt.Errorf(app.T_("Failed to install package %s: %v, stderr: %s"), packageName, err, stderr)
	}

	return nil
}

// RemovePackage удаляет указанный пакет внутри контейнера через apt-get remove.
func (p *UbuntuProvider) RemovePackage(ctx context.Context, containerInfo ContainerInfo, packageName string) error {
	if err := validatePackageName(packageName); err != nil {
		return err
	}
	args := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "sudo", "apt-get", "remove", "-y", packageName)
	_, stderr, err := helper.RunCommand(ctx, args)
	if err != nil {
		return fmt.Errorf(app.T_("Failed to remove package %s: %v, stderr: %s"), packageName, err, stderr)
	}

	return nil
}

// parseAptOutput парсит вывод команды apt search . и возвращает срез PackageInfo.
func (p *UbuntuProvider) parseAptOutput(output string, exportingPackages []string) []PackageInfo {
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
				Name:        match[1],
				Version:     match[3],
				Description: strings.TrimSpace(match[5]),
				Installed:   isInstalled,
				Exporting:   slices.Contains(exportingPackages, match[1]),
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
