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
	"apm/internal/common/appstream"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"apm/lib"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/creack/pty"
)

type Package struct {
	Name             string               `json:"name"`
	Section          string               `json:"section"`
	InstalledSize    int                  `json:"installedSize"`
	Maintainer       string               `json:"maintainer"`
	Version          string               `json:"version"`
	VersionInstalled string               `json:"versionInstalled"`
	Depends          []string             `json:"depends"`
	Provides         []string             `json:"provides"`
	Size             int                  `json:"size"`
	Filename         string               `json:"filename"`
	Description      string               `json:"description"`
	AppStream        *appstream.Component `json:"appStream"`
	Changelog        string               `json:"lastChangelog"`
	Installed        bool                 `json:"installed"`
	TypePackage      int                  `json:"typePackage"`
}

var syncAptMutex sync.Mutex

const (
	TypeInstall = iota
	TypeRemove
	TypeChanged
)

func CommandWithProgress(ctx context.Context, command string, typeProcess int) []error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	// Запускаем команду через pty для захвата вывода в реальном времени.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return []error{err}
	}
	defer ptmx.Close()

	scanner := bufio.NewScanner(ptmx)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	})

	// Регулярное выражение для распознавания прогресса скачивания.
	downloadRegex := regexp.MustCompile(`(?P<global>\d+)%\s*\[(?P<order>\d+)\s+(?P<pkg>[\w\-\+]+)\s+(?P<data>[0-9]+\/[0-9]+[KMG]?B)\s+(?P<local>\d+)%\]`)
	// Регулярное выражение для распознавания прогресса установки.
	installRegex := regexp.MustCompile(`^(?P<step>\d+):\s+(?P<pkg>[\w\-\:\+]+).*?\[\s*(?P<percent>\d+)%\]`)

	// Мапы: ключ – уникальное имя события, значение – чистое имя пакета.
	downloadEvents := make(map[string]string)
	installEvents := make(map[string]string)

	var textStatus string
	switch typeProcess {
	case TypeRemove:
		textStatus = lib.T_("Removal")
	case TypeChanged:
		textStatus = lib.T_("Change")
	default:
		textStatus = lib.T_("Installation")
	}

	var outputLines []string
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for scanner.Scan() {
			line := scanner.Text()
			outputLines = append(outputLines, line)

			if downloadRegex.MatchString(line) {
				match := downloadRegex.FindStringSubmatch(line)
				pkgName := match[downloadRegex.SubexpIndex("pkg")]
				// Уникальное имя события
				eventName := fmt.Sprintf("system.downloadProgress-%s", pkgName)
				downloadEvents[eventName] = pkgName

				percentStr := match[downloadRegex.SubexpIndex("local")]
				if percent, err := strconv.Atoi(percentStr); err == nil {
					reply.CreateEventNotification(ctx, reply.StateBefore,
						reply.WithEventName(eventName),
						reply.WithProgress(true),
						reply.WithProgressPercent(float64(percent)),
						reply.WithEventView(fmt.Sprintf(lib.T_("Downloading: %s"), pkgName)),
					)
				}
			} else if installRegex.MatchString(line) {
				match := installRegex.FindStringSubmatch(line)
				pkgName := match[installRegex.SubexpIndex("pkg")]
				eventName := "system.installProgress"
				installEvents[eventName] = pkgName

				percentStr := match[installRegex.SubexpIndex("percent")]
				if percent, err := strconv.Atoi(percentStr); err == nil {
					reply.CreateEventNotification(ctx, reply.StateBefore,
						reply.WithEventName(eventName),
						reply.WithProgress(true),
						reply.WithProgressPercent(float64(percent)),
						reply.WithEventView(fmt.Sprintf("%s: %s", textStatus, pkgName)),
					)
				}
			}

			// При получении строки "Done." завершаем все события.
			if strings.Contains(line, "Done.") {
				for event, pkg := range downloadEvents {
					reply.CreateEventNotification(ctx, reply.StateAfter,
						reply.WithEventName(event),
						reply.WithProgress(true),
						reply.WithProgressDoneText(pkg),
						reply.WithProgressPercent(100),
					)
				}
				for event, pkg := range installEvents {
					reply.CreateEventNotification(ctx, reply.StateAfter,
						reply.WithEventName(event),
						reply.WithProgress(true),
						reply.WithProgressDoneText(pkg),
						reply.WithProgressPercent(100),
					)
				}
			}
		}
	}()

	// Ожидаем завершения выполнения команды.
	if err = cmd.Wait(); err != nil {
		wg.Wait()
		aptErrors := ErrorLinesAnalyseAll(outputLines)
		if len(aptErrors) > 0 {
			var errorsSlice []error
			for _, e := range aptErrors {
				errorsSlice = append(errorsSlice, e)
			}
			return errorsSlice
		}
		return []error{fmt.Errorf(lib.T_("Installation error: %v"), err)}
	}

	wg.Wait()

	aptErrors := ErrorLinesAnalyseAll(outputLines)
	if len(aptErrors) > 0 {
		var errorsSlice []error
		for _, e := range aptErrors {
			errorsSlice = append(errorsSlice, e)
		}
		return errorsSlice
	}

	return nil
}

func Update(ctx context.Context) ([]Package, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Update"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Update"))

	err := aptUpdate(ctx)
	if err != nil {
		return nil, err
	}

	command := fmt.Sprintf("%s apt-cache dumpavail", lib.Env.CommandPrefix)
	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf(lib.T_("Error opening stdout pipe: %w"), err)
	}
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf(lib.T_("Error executing command: %w"), err)
	}

	const maxCapacity = 1024 * 1024 * 350 // 350MB
	buf := make([]byte, maxCapacity)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(buf, maxCapacity)

	var packages []Package
	var pkg Package
	var currentKey string

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if line == "" {
			if pkg.Name != "" {
				packages = append(packages, pkg)
				pkg = Package{}
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
			case "Section":
				pkg.Section = value
			case "Installed Size":
				sizeValue, err := strconv.Atoi(value)
				if err != nil {
					sizeValue = 0
				}

				pkg.InstalledSize = sizeValue
			case "Maintainer":
				pkg.Maintainer = value
			case "Version":
				versionValue, errVersion := helper.GetVersionFromAptCache(value)
				if errVersion != nil {
					pkg.Version = value
				} else {
					pkg.Version = versionValue
				}
			case "Depends":
				depList := strings.Split(value, ",")
				seen := make(map[string]bool)
				var cleanedDeps []string
				for _, dep := range depList {
					cleanDep := CleanDependency(dep)
					if cleanDep != "" && !seen[cleanDep] {
						seen[cleanDep] = true
						cleanedDeps = append(cleanedDeps, cleanDep)
					}
				}
				pkg.Depends = cleanedDeps
			case "Provides":
				provList := strings.Split(value, ",")
				seen := make(map[string]bool)
				var cleanedProviders []string
				for _, prov := range provList {
					cleanProv := CleanDependency(prov)
					if cleanProv != "" && !seen[cleanProv] {
						seen[cleanProv] = true
						cleanedProviders = append(cleanedProviders, cleanProv)
					}
				}
				pkg.Provides = cleanedProviders
			case "Size":
				sizeValue, err := strconv.Atoi(value)
				if err != nil {
					sizeValue = 0
				}

				pkg.Size = sizeValue
			case "Filename":
				pkg.Filename = value
			case "Description":
				pkg.Description = value
			case "Changelog":
				pkg.Changelog = value
			default:
			}
		} else {
			switch currentKey {
			case "Description":
				pkg.Description += "\n" + line
			case "Changelog":
				pkg.Changelog += "\n" + line
			default:
			}
		}
	}

	if pkg.Name != "" {
		packages = append(packages, pkg)
	}

	if err = scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf(lib.T_("String too large: (over %dMB) - "), maxCapacity/(1024*1024))
		}
		return nil, fmt.Errorf(lib.T_("Scanner error: %w"), err)
	}

	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf(lib.T_("Command execution error: %w"), err)
	}

	// добавляем Changelog
	for i := range packages {
		packages[i].Changelog = extractLastMessage(packages[i].Changelog)
	}

	return packages, nil
}

func aptUpdate(ctx context.Context) error {
	syncAptMutex.Lock()
	defer syncAptMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.AptUpdate"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.AptUpdate"))

	command := fmt.Sprintf("%s apt-get update", lib.Env.CommandPrefix)
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = []string{"LC_ALL=C"}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	aptError := ErrorLinesAnalise(lines)
	if aptError != nil {
		return errors.New(aptError.Error())
	}
	if err != nil {
		return fmt.Errorf(lib.T_("Error updating packages: %v, output: %s"), err, string(output))
	}

	return nil
}

func extractLastMessage(changelog string) string {
	lines := strings.Split(changelog, "\n")
	var result []string
	found := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "*") {
			if !found {
				result = append(result, trimmed)
				found = true
			} else {
				break
			}
		} else if found {
			result = append(result, trimmed)
		}
	}

	return strings.Join(result, "\n")
}

var soNameRe = regexp.MustCompile(`^(.+?\.so(?:\.[0-9]+)*).*`)

func CleanDependency(s string) string {
	s = strings.TrimSpace(s)

	if m := soNameRe.FindStringSubmatch(s); len(m) > 1 && strings.HasPrefix(s, "lib") {
		return m[1]
	}

	if idx := strings.IndexByte(s, '('); idx != -1 {
		inner := s[idx+1:]
		if j := strings.IndexByte(inner, ')'); j != -1 {
			inner = inner[:j]
		}
		// Если скобка не закрыта, inner уже содержит всё до конца строки
		inner = strings.TrimSpace(inner)

		if inner != "" && (strings.Contains(inner, ".so") || strings.Contains(inner, "/")) {
			return s
		}
	}

	// Убираем версионные ограничения только для обычных зависимостей пакетов
	s = regexp.MustCompile(`\s*\((?i:64bit)\)$`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\s*\([<>!=][^)]*\)$`).ReplaceAllString(s, "")

	if idx := strings.IndexByte(s, '('); idx != -1 {
		inner := s[idx+1:]
		if j := strings.IndexByte(inner, ')'); j != -1 {
			inner = inner[:j]
		}
		inner = strings.TrimSpace(inner)
		if inner == "" {
			s = strings.TrimSpace(s[:idx])
		}
	}
	
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, ':'); idx > 0 && s[0] >= '0' && s[0] <= '9' {
		s = s[idx+1:]
	}

	return s
}
