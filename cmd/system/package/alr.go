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
	"regexp"
	"strings"
)

type AlrService struct{}

func NewALRService() *AlrService {
	return &AlrService{}
}

// PreInstall собираем пакет, но не устанавливаем в систему, возвращаем путь к rpm
func (a *AlrService) PreInstall(ctx context.Context, packageName string) (string, error) {
	packageName = helper.ClearALRPackageName(packageName)

	reply.CreateEventNotification(ctx, reply.StateBefore,
		reply.WithEventName("alr-build-"+packageName),
		reply.WithProgress(true),
		reply.WithProgressPercent(float64(50)),
		reply.WithEventView(fmt.Sprintf(lib.T_("Build ALR package %s"), packageName)),
	)

	defer reply.CreateEventNotification(ctx, reply.StateAfter,
		reply.WithEventName("alr-build-"+packageName),
		reply.WithProgress(true),
		reply.WithProgressDoneText(fmt.Sprintf(lib.T_("Build ALR package %s"), packageName)),
		reply.WithProgressPercent(100),
	)

	env := os.Environ()
	env = append(env, "LC_ALL=C", "HOME=/root", "XDG_CACHE_HOME=/root/.cache")

	cmd := exec.CommandContext(ctx, "sh", "-c",
		fmt.Sprintf("%s alr -i='false' -P -s install %s", lib.Env.CommandPrefix, packageName))
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf(lib.T_("Error alr command '%s': %s"), "alr install", string(output))
	}

	re := regexp.MustCompile(`for '([^']+)'`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return "", fmt.Errorf(lib.T_("Unable to extract RPM path from output: %s"), string(output))
	}

	rpmPath := matches[1]
	return rpmPath, nil
}

// UpdateWithAlrPackages получает список alr пакетов и добавляет их к срезу packages.
func (a *AlrService) UpdateWithAlrPackages(ctx context.Context, packages []Package) ([]Package, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.UpdateALR"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.UpdateALR"))

	env := os.Environ()
	env = append(env, "LC_ALL=C", "HOME=/root", "XDG_CACHE_HOME=/root/.cache")
	update := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%s alr fix && alr ref", lib.Env.CommandPrefix))
	update.Env = env
	outputUpdate, errUpdate := update.CombinedOutput()
	if errUpdate != nil {
		return packages, fmt.Errorf(lib.T_("Error alr command '%s': %s"), "alr fix && alr ref", string(outputUpdate))
	}

	command := fmt.Sprintf("%s alr list", lib.Env.CommandPrefix)
	cmdList := exec.CommandContext(ctx, "sh", "-c", command)
	cmdList.Env = env
	outputList, err := cmdList.CombinedOutput()
	if err != nil {
		return packages, fmt.Errorf(lib.T_("Error alr command '%s': %s"), "alr list", string(outputList))
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
		return packages, fmt.Errorf(lib.T_("Error alr command '%s': %w"), "alr list", err)
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

		cmdInfo := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%s alr info %s", lib.Env.CommandPrefix, cleanedPkgIdentifier))
		cmdInfo.Env = env
		outputInfo, err := cmdInfo.CombinedOutput()
		if err != nil {
			lib.Log.Warning(err)
			continue
		}

		var pkg Package
		pkg.IsAlr = true

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
				case "description":
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

		pkg.Name = cleanedPkgIdentifier + "+alr-aides"
		packages = append(packages, pkg)
	}

	return packages, nil
}
