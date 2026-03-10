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
	"sort"
	"strings"
)

// AltProvider реализует методы для работы с пакетами в ALT linux
type AltProvider struct {
	servicePackage *PackageService
	runner         command.Runner
}

// NewAltProvider возвращает новый экземпляр AltProvider.
func NewAltProvider(servicePackage *PackageService, runner command.Runner) *AltProvider {
	return &AltProvider{
		servicePackage: servicePackage,
		runner:         runner,
	}
}

// GetPackages обновляет базу пакетов, выполняет поиск и отмечает установленные пакеты.
func (p *AltProvider) GetPackages(ctx context.Context, containerInfo ContainerInfo) ([]PackageInfo, error) {
	if _, stderr, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "sudo", "apt-get", "update"}, command.WithQuiet()); err != nil {
		return nil, fmt.Errorf(app.T_("Failed to update package database: %v, stderr: %s"), err, stderr)
	}

	stdout, _, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "apt-cache", "dumpavail"}, command.WithQuiet())
	if err != nil {
		return nil, fmt.Errorf(app.T_("Error executing command: %w"), err)
	}

	// Получаем список экспортированных пакетов.
	exportingPackages, err := p.servicePackage.GetAllApplicationsByContainer(ctx, containerInfo)
	if err != nil {
		app.Log.Error(app.T_("Error retrieving installed packages: "), err)
		exportingPackages = []string{}
	}

	// Получаем карту установленных пакетов
	installedPackages, err := p.getInstalledPackages(ctx, containerInfo)
	if err != nil {
		installedPackages = []string{}
	}

	// Преобразуем срез в карту для быстрого поиска
	installedMap := make(map[string]bool)
	for _, pkg := range installedPackages {
		installedMap[pkg] = true
	}

	// Формируем карту для быстрого поиска установленных пакетов.
	exportingMap := make(map[string]bool)
	for _, name := range exportingPackages {
		exportingMap[name] = true
	}

	var packages []PackageInfo
	var pkg PackageInfo
	var currentKey string

	for _, rawLine := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(rawLine)

		if line == "" {
			if pkg.Name != "" {
				packages = append(packages, pkg)
				pkg = PackageInfo{}
				currentKey = ""
			}
			continue
		}

		if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			currentKey = key

			switch key {
			case "Package":
				pkg.Name = value
			case "Version":
				versionValue, errVersion := helper.GetVersionFromAptCache(value)
				if errVersion != nil {
					pkg.Version = value
				} else {
					pkg.Version = versionValue
				}
			case "Description":
				pkg.Description = value
			default:
			}
		} else {
			if currentKey == "Description" {
				pkg.Description += "\n" + line
			}
		}
	}

	if pkg.Name != "" {
		packages = append(packages, pkg)
	}

	for i := range packages {
		if installedMap[packages[i].Name] {
			packages[i].Installed = true
		}
		if exportingMap[packages[i].Name] {
			packages[i].Exporting = true
		}
		packages[i].Manager = "apt-get"
		packages[i].Container = containerInfo.ContainerName
	}

	return packages, nil
}

// RemovePackage удаляет указанный пакет с помощью apt-get remove.
func (p *AltProvider) RemovePackage(ctx context.Context, containerInfo ContainerInfo, packageName string) error {
	if err := validatePackageName(packageName); err != nil {
		return err
	}
	_, stderr, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "sudo", "apt-get", "remove", "-y", packageName})
	if err != nil {
		return fmt.Errorf(app.T_("Failed to remove package %s: %v, stderr: %s"), packageName, err, stderr)
	}
	return nil
}

// InstallPackage устанавливает указанный пакет с помощью apt-get install.
func (p *AltProvider) InstallPackage(ctx context.Context, containerInfo ContainerInfo, packageName string) error {
	if err := validatePackageName(packageName); err != nil {
		return err
	}
	_, stderr, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "sudo", "apt-get", "install", "-y", packageName})
	if err != nil {
		return fmt.Errorf(app.T_("Failed to install package %s: %v, stderr: %s"), packageName, err, stderr)
	}
	return nil
}

// GetPathByPackageName возвращает список путей для файла пакета, найденных через rpm -ql.
func (p *AltProvider) GetPathByPackageName(ctx context.Context, containerInfo ContainerInfo, packageName, filePath string) ([]string, error) {
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

	stdout, stderr, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "rpm", "-ql", packageName}, command.WithQuiet())
	if err != nil {
		app.Log.Debugf(app.T_("Command execution error: %s %s"), stderr, err.Error())
	}

	filtered := helper.FilterLines(stdout, filePath)
	paths := parseOutput(filtered)
	if len(paths) == 0 {
		qaStdout, qaStderr, qaErr := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "rpm", "-qa"}, command.WithQuiet())
		if qaErr != nil {
			app.Log.Debugf(app.T_("Fallback command execution error: %s %s"), qaStderr, qaErr.Error())
			return []string{}, nil
		}

		matchedPackages := helper.FilterLinesPrefix(qaStdout, packageName)
		var allPaths []string
		for _, pkg := range strings.Split(matchedPackages, "\n") {
			pkg = strings.TrimSpace(pkg)
			if pkg == "" {
				continue
			}
			qlStdout, _, qlErr := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "rpm", "-ql", pkg}, command.WithQuiet())
			if qlErr != nil {
				continue
			}
			filteredLines := helper.FilterLines(qlStdout, filePath)
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

// GetPackageOwner определяет пакет-владельца файла через rpm -qf.
func (p *AltProvider) GetPackageOwner(ctx context.Context, containerInfo ContainerInfo, filePath string) (string, error) {
	stdout, stderr, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "rpm", "-qf", "--queryformat", "%{NAME}", filePath}, command.WithQuiet())
	if err != nil {
		app.Log.Debugf(app.T_("Command execution error: %s %s"), stderr, err.Error())
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

// getInstalledPackages возвращает карту установленных пакетов
func (p *AltProvider) getInstalledPackages(ctx context.Context, containerInfo ContainerInfo) ([]string, error) {
	stdout, _, err := p.runner.Run(ctx, []string{"distrobox", "enter", containerInfo.ContainerName, "--", "rpm", "-qia"}, command.WithEnv("LC_ALL=C"), command.WithQuiet())
	if err != nil {
		return nil, fmt.Errorf(app.T_("Error executing command rpm -qia: %w"), err)
	}

	var packages []string
	for _, rawLine := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "Name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				packages = append(packages, name)
			}
		}
	}
	return packages, nil
}
