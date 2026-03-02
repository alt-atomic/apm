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
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// AltProvider реализует методы для работы с пакетами в ALT linux
type AltProvider struct {
	servicePackage *PackageService
	commandPrefix  string
}

// NewAltProvider возвращает новый экземпляр AltProvider.
func NewAltProvider(servicePackage *PackageService, commandPrefix string) *AltProvider {
	return &AltProvider{
		servicePackage: servicePackage,
		commandPrefix:  commandPrefix,
	}
}

// GetPackages обновляет базу пакетов, выполняет поиск и отмечает установленные пакеты.
func (p *AltProvider) GetPackages(ctx context.Context, containerInfo ContainerInfo) ([]PackageInfo, error) {
	updateArgs := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "sudo", "apt-get", "update")
	if _, stderr, err := helper.RunCommand(ctx, updateArgs); err != nil {
		return nil, fmt.Errorf(app.T_("Failed to update package database: %v, stderr: %s"), err, stderr)
	}

	args := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "apt-cache", "dumpavail")
	cmd := exec.Command(args[0], args[1:]...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf(app.T_("Error opening stdout pipe: %w"), err)
	}
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf(app.T_("Error executing command: %w"), err)
	}

	// Получаем список экспортированных пакетов.
	exportingPackages, err := p.servicePackage.GetAllApplicationsByContainer(ctx, containerInfo)
	if err != nil {
		app.Log.Error(app.T_("Error retrieving installed packages: "), err)
		exportingPackages = []string{}
	}

	// Получаем карту установленных пакетов
	installedPackages, err := p.getInstalledPackages(containerInfo)
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

	const maxCapacity = 1024 * 1024 * 350 // 350MB
	buf := make([]byte, maxCapacity)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(buf, maxCapacity)

	var packages []PackageInfo
	var pkg PackageInfo
	var currentKey string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

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

	if err = scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf(app.T_("String too large: (over %dMB) - "), maxCapacity/(1024*1024))
		}
		return nil, fmt.Errorf(app.T_("Scanner error: %w"), err)
	}

	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf(app.T_("Command execution error: %w"), err)
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
	args := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "sudo", "apt-get", "remove", "-y", packageName)
	_, stderr, err := helper.RunCommand(ctx, args)
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
	args := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "sudo", "apt-get", "install", "-y", packageName)
	_, stderr, err := helper.RunCommand(ctx, args)
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

	args := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "rpm", "-ql", packageName)
	stdout, stderr, err := helper.RunCommand(ctx, args)
	if err != nil {
		app.Log.Debugf(app.T_("Command execution error: %s %s"), stderr, err.Error())
	}

	filtered := helper.FilterLines(stdout, filePath)
	paths := parseOutput(filtered)
	if len(paths) == 0 {
		qaArgs := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "rpm", "-qa")
		qaStdout, qaStderr, qaErr := helper.RunCommand(ctx, qaArgs)
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
			qlArgs := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "rpm", "-ql", pkg)
			qlStdout, _, qlErr := helper.RunCommand(ctx, qlArgs)
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
	args := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "rpm", "-qf", "--queryformat", "%{NAME}", filePath)
	stdout, stderr, err := helper.RunCommand(ctx, args)
	if err != nil {
		app.Log.Debugf(app.T_("Command execution error: %s %s"), stderr, err.Error())
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

// getInstalledPackages возвращает карту установленных пакетов
func (p *AltProvider) getInstalledPackages(containerInfo ContainerInfo) ([]string, error) {
	args := helper.BuildDistroboxArgs(p.commandPrefix, "distrobox", "enter", containerInfo.ContainerName, "--", "rpm", "-qia")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = []string{"LC_ALL=C"}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf(app.T_("Error executing command rpm -qia: %w"), err)
	}

	var packages []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				packages = append(packages, name)
			}
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf(app.T_("Error scanning rpm output: %w"), err)
	}
	return packages, nil
}
