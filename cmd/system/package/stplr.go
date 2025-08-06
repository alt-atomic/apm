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

package _package

import (
	"apm/cmd/common/helper"
	"apm/cmd/common/reply"
	"apm/lib"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type StplrService struct{}

func NewSTPLRService() *StplrService {
	return &StplrService{}
}

var workDir = "/root"
var buildDir = "/root/.cache"

// PreInstall собираем пакет, но не устанавливаем в систему, возвращаем путь к rpm
func (a *StplrService) PreInstall(ctx context.Context, packageName string) (string, error) {
	// Очищаем постфикс и удаляем старые rpm
	packageName = helper.ClearALRPackageName(packageName)

	rpmPattern := filepath.Join(buildDir, fmt.Sprintf("%s*.rpm", packageName))
	oldRpms, err := filepath.Glob(rpmPattern)
	if err != nil {
		return "", fmt.Errorf(lib.T_("Error searching for old RPMs %w"), err)
	}

	for _, f := range oldRpms {
		_ = os.Remove(f)
	}

	// Уведомление о старте сборки
	reply.CreateEventNotification(ctx, reply.StateBefore,
		reply.WithEventName("stplr-build-"+packageName),
		reply.WithProgress(true),
		reply.WithProgressPercent(float64(50)),
		reply.WithEventView(fmt.Sprintf(lib.T_("Build STPLR package %s"), packageName)),
	)
	defer reply.CreateEventNotification(ctx, reply.StateAfter,
		reply.WithEventName("stplr-build-"+packageName),
		reply.WithProgress(true),
		reply.WithProgressDoneText(fmt.Sprintf(lib.T_("Build STPLR package %s"), packageName)),
		reply.WithProgressPercent(100),
	)

	env := append(os.Environ(),
		"LC_ALL=C",
		fmt.Sprintf("HOME=%s", workDir),
		fmt.Sprintf("XDG_CACHE_HOME=%s", buildDir),
	)

	cmd := exec.CommandContext(ctx, "sh", "-c",
		fmt.Sprintf("%s stplr -i='false' build --package %s", lib.Env.CommandPrefix, packageName))
	cmd.Env = env
	cmd.Dir = buildDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf(lib.T_("Error stplr command '%s': %s: %w"), "stplr build", strings.TrimSpace(string(output)), err)
	}

	newRpms, err := filepath.Glob(rpmPattern)
	if err != nil {
		return "", fmt.Errorf(lib.T_("Not found rpm file %s"), err.Error())
	}

	if len(newRpms) == 0 {
		return "", fmt.Errorf(lib.T_("Unable to find built RPM for package %s in %s"), packageName, buildDir)
	}

	rpmPath := newRpms[0]
	return rpmPath, nil
}

// UpdateWithStplrPackages получает список alr пакетов и добавляет их к срезу packages.
func (a *StplrService) UpdateWithStplrPackages(ctx context.Context, packages []Package) ([]Package, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.UpdateSTPLR"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.UpdateSTPLR"))

	env := append(os.Environ(),
		"LC_ALL=C",
		fmt.Sprintf("HOME=%s", workDir),
		fmt.Sprintf("XDG_CACHE_HOME=%s", buildDir),
	)
	update := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%s stplr fix && stplr ref", lib.Env.CommandPrefix))
	update.Env = env
	outputUpdate, errUpdate := update.CombinedOutput()
	if errUpdate != nil {
		return packages, fmt.Errorf(lib.T_("Error stplr command '%s': %s"), "stplr fix && stplr ref", string(outputUpdate))
	}

	command := fmt.Sprintf("%s stplr list", lib.Env.CommandPrefix)
	cmdList := exec.CommandContext(ctx, "sh", "-c", command)
	cmdList.Env = env
	outputList, err := cmdList.CombinedOutput()
	if err != nil {
		return packages, fmt.Errorf(lib.T_("Error stplr command '%s': %s"), "stplr list", string(outputList))
	}

	scanner := bufio.NewScanner(bytes.NewReader(outputList))
	var alrPackages []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			alrPackages = append(alrPackages, fields[0])
		}
	}

	if err = scanner.Err(); err != nil {
		return packages, fmt.Errorf(lib.T_("Error stplr command '%s': %w"), "stplr list", err)
	}

	for _, pkgIdentifier := range alrPackages {
		// Обрабатываем только пакеты, начинающиеся с "aides/"
		if !strings.HasPrefix(pkgIdentifier, "aides/") {
			continue
		}

		parts := strings.SplitN(pkgIdentifier, "/", 2)
		if len(parts) < 2 {
			continue
		}
		cleanedPkgIdentifier := parts[1]

		cmdInfo := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%s stplr info %s", lib.Env.CommandPrefix, cleanedPkgIdentifier))
		cmdInfo.Env = env
		outputInfo, err := cmdInfo.CombinedOutput()
		if err != nil {
			lib.Log.Warning(err)
			continue
		}

		var pkg Package
		pkg.TypePackage = int(PackageTypeStplr)

		var currentKey string
		scannerInfo := bufio.NewScanner(bytes.NewReader(outputInfo))
		for scannerInfo.Scan() {
			line := scannerInfo.Text()
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine == "" || strings.HasPrefix(trimmedLine, "//") {
				continue
			}

			if currentKey != "" && strings.HasPrefix(trimmedLine, "- ") {
				item := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- "))
				switch currentKey {
				case "provides":
					pkg.Provides = append(pkg.Provides, item)
				case "depends":
					pkg.Depends = append(pkg.Depends, item)
				}
				continue
			}

			if strings.Contains(line, ":") {
				parts = strings.SplitN(line, ":", 2)
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				currentKey = key

				switch key {
				case "name":
					// Не используем полученное имя, т.к. формируем по шаблону внизу
				case "version":
					pkg.Version = value
				case "release":
				case "epoch":
				case "summary":
					pkg.Description = value
				case "maintainer":
					pkg.Maintainer = value
				case "provides":
					if value != "" && value != "[]" {
						pkg.Provides = append(pkg.Provides, value)
					}
				case "depends":
					if value != "" && value != "[]" {
						pkg.Depends = append(pkg.Depends, value)
					}
				default:
				}
				continue
			}

			if currentKey == "description" {
				pkg.Description += "\n" + trimmedLine
			}
		}
		if err = scannerInfo.Err(); err != nil {
			continue
		}

		pkg.Name = cleanedPkgIdentifier + "+stplr-aides"
		packages = append(packages, pkg)
	}

	return packages, nil
}
