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

package apt

import (
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// InstalledPackageInfo информация об установленном пакете из rpm
type InstalledPackageInfo struct {
	Name    string
	Version string
	Arch    string
}

// KernelRPMInfo информация о ядре из rpm
type KernelRPMInfo struct {
	Name      string
	Version   string
	Release   string
	BuildTime string
}

// RpmGetInstalledPackages возвращает карту установленных пакетов (имя -> версия)
func (a *Actions) RpmGetInstalledPackages(ctx context.Context, commandPrefix string, noLock ...bool) (map[string]string, error) {
	var result map[string]string
	skipLock := len(noLock) > 0 && noLock[0]

	err := a.operationWrapperWithOptions(skipLock, func() error {
		command := fmt.Sprintf("%s rpm -qia", commandPrefix)
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Env = []string{"LC_ALL=C"}

		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			return fmt.Errorf(app.T_("Error executing the rpm -qia command: %w"), cmdErr)
		}

		var parseErr error
		result, parseErr = parseRpmQiaOutput(string(output))
		return parseErr
	})

	return result, err
}

// RpmQueryKernelPackages возвращает список установленных ядер через rpm
func (a *Actions) RpmQueryKernelPackages(ctx context.Context) ([]KernelRPMInfo, error) {
	var result []KernelRPMInfo

	err := a.operationWrapper(func() error {
		cmd := exec.CommandContext(ctx, "rpm", "-qa", "--queryformat",
			"%{NAME}\t%{VERSION}\t%{RELEASE}\t%{BUILDTIME}\n", "kernel-image-*")
		cmd.Env = []string{"LC_ALL=C"}

		output, cmdErr := cmd.Output()
		if cmdErr != nil {
			return fmt.Errorf(app.T_("failed to query installed kernels: %s"), cmdErr.Error())
		}

		var parseErr error
		result, parseErr = parseKernelRpmOutput(string(output))
		return parseErr
	})

	return result, err
}

// RpmIsPackageInstalled проверяет установлен ли пакет
func (a *Actions) RpmIsPackageInstalled(packageName string) (bool, error) {
	var installed bool

	err := a.operationWrapper(func() error {
		cmd := exec.Command("rpm", "-q", packageName)
		cmdErr := cmd.Run()
		installed = cmdErr == nil
		return nil // Не возвращаем ошибку - отсутствие пакета это не ошибка
	})

	return installed, err
}

// RpmIsAnyPackageInstalled проверяет установлен ли хотя бы один из пакетов
func (a *Actions) RpmIsAnyPackageInstalled(packageNames []string) (bool, error) {
	var installed bool

	err := a.operationWrapper(func() error {
		for _, pkgName := range packageNames {
			cmd := exec.Command("rpm", "-q", pkgName)
			if cmd.Run() == nil {
				installed = true
				return nil
			}
		}
		return nil
	})

	return installed, err
}

// parseRpmQiaOutput парсит вывод rpm -qia и возвращает карту имя -> версия
func parseRpmQiaOutput(output string) (map[string]string, error) {
	installed := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(output))
	var currentName, currentVersion, currentArch string

	flushCurrent := func() {
		if currentName == "" {
			return
		}
		name := currentName
		if strings.HasPrefix(name, "i586-") && (currentArch == "i586" || currentArch == "i386") {
			name = strings.TrimPrefix(name, "i586-")
		}

		// Если пакет уже есть, выбираем более новую версию
		if existingVersion, exists := installed[name]; exists {
			if helper.CompareVersions(currentVersion, existingVersion) > 0 {
				installed[name] = currentVersion
			}
		} else {
			installed[name] = currentVersion
		}

		currentName, currentVersion, currentArch = "", "", ""
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "Name") {
			flushCurrent()
		}
		if line == "" {
			flushCurrent()
			continue
		}

		if strings.HasPrefix(line, "Name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentName = strings.TrimSpace(parts[1])
			}
			continue
		}

		if strings.HasPrefix(line, "Version") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentVersion = strings.TrimSpace(parts[1])
			}
			continue
		}

		if strings.HasPrefix(line, "Architecture") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentArch = strings.TrimSpace(parts[1])
			}
			continue
		}
	}

	flushCurrent()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf(app.T_("error scanning RPM output: %w"), err)
	}

	return installed, nil
}

// parseKernelRpmOutput парсит вывод rpm -qa для ядер
func parseKernelRpmOutput(output string) ([]KernelRPMInfo, error) {
	var kernels []KernelRPMInfo
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) != 4 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		if strings.Contains(name, "debuginfo") {
			continue
		}

		kernels = append(kernels, KernelRPMInfo{
			Name:      name,
			Version:   strings.TrimSpace(parts[1]),
			Release:   strings.TrimSpace(parts[2]),
			BuildTime: strings.TrimSpace(parts[3]),
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf(app.T_("error scanning RPM output: %s"), err.Error())
	}

	return kernels, nil
}
