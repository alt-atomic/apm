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
	"apm/internal/common/reply"
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// DistroAPIService реализует методы для работы с пакетами в Arch
type DistroAPIService struct {
	runner command.Runner
}

// NewDistroAPIService возвращает новый экземпляр DistroAPIService.
func NewDistroAPIService(runner command.Runner) *DistroAPIService {
	return &DistroAPIService{
		runner: runner,
	}
}

type ContainerInfo struct {
	OS            string `json:"os"`
	ContainerName string `json:"name"`
	Active        bool   `json:"active"`
}

// GetContainerList получает список контейнеров, а если требуется полная информация (getFullInfo),
// то параллельно для каждого контейнера вызывается fetchOsInfo.
func (d *DistroAPIService) GetContainerList(ctx context.Context, getFullInfo bool) ([]ContainerInfo, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventDistroGetContainerList))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventDistroGetContainerList))

	stdout, stderr, err := d.runner.Run(ctx, []string{"distrobox", "ls"}, command.WithQuiet())
	if err != nil {
		return nil, errors.New(app.T_("Failed to retrieve the list of containers: ") + stderr)
	}

	output := strings.TrimSpace(stdout)
	if output == "" {
		return []ContainerInfo{}, nil
	}

	lines := strings.Split(output, "\n")
	if len(lines) <= 1 {
		return []ContainerInfo{}, nil
	}

	var containers []ContainerInfo

	var containerNames []string
	for _, line := range lines[1:] {
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimSpace(parts[1])
		if name != "" {
			containerNames = append(containerNames, name)
		}
	}

	if getFullInfo {
		var wg sync.WaitGroup
		mu := &sync.Mutex{}
		for _, name := range containerNames {
			wg.Add(1)
			go func(n string) {
				defer wg.Done()
				info, err := d.fetchOsInfo(ctx, n)
				if err != nil {
					app.Log.Error(err)
					info = ContainerInfo{ContainerName: n, OS: "", Active: false}
				}
				mu.Lock()
				containers = append(containers, info)
				mu.Unlock()
			}(name)
		}
		wg.Wait()
	} else {
		for _, name := range containerNames {
			containers = append(containers, ContainerInfo{
				ContainerName: name,
				OS:            "",
				Active:        false,
			})
		}
	}

	slices.SortFunc(containers, func(a, b ContainerInfo) int {
		return strings.Compare(a.ContainerName, b.ContainerName)
	})

	return containers, nil
}

// ExportingApp экспортирует пакет в хост-систему.
// Принимает отдельные списки для desktop и консольных приложений и обрабатывает каждый тип соответственно.
func (d *DistroAPIService) ExportingApp(ctx context.Context, containerInfo ContainerInfo, _ string, desktopPaths, consolePaths []string, deleteApp bool) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventDistroExportingApp))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventDistroExportingApp))
	// Определяем суффикс: "-d", если deleteApp == true, иначе пустая строка.
	suffix := ""
	if deleteApp {
		suffix = "-d"
	}

	type cmdEntry struct {
		args []string
	}
	var commands []cmdEntry

	// Обрабатываем desktop приложения
	for _, path := range desktopPaths {
		args := []string{"distrobox", "enter", containerInfo.ContainerName, "--", "distrobox-export", "--app", path}
		if suffix != "" {
			args = append(args, suffix)
		}
		commands = append(commands, cmdEntry{args: args})
	}

	// Обрабатываем консольные приложения
	for _, path := range consolePaths {
		args := []string{"distrobox", "enter", containerInfo.ContainerName, "--", "distrobox-export", "-b", path}
		if suffix != "" {
			args = append(args, suffix)
		}
		commands = append(commands, cmdEntry{args: args})
	}

	// Выполняем все команды параллельно
	var wg sync.WaitGroup
	errChan := make(chan error, len(commands))

	for _, cmd := range commands {
		wg.Add(1)
		go func(args []string) {
			defer wg.Done()
			_, stderr, err := d.runner.Run(ctx, args, command.WithQuiet())
			if err != nil {
				stderrStr := strings.TrimSpace(stderr)
				cmdStr := strings.Join(args, " ")
				if stderrStr != "" {
					errChan <- fmt.Errorf(app.T_("Error executing command %q: %s"), cmdStr, stderrStr)
				} else {
					errChan <- fmt.Errorf(app.T_("Error executing command %q: %v"), cmdStr, err)
				}
			}
		}(cmd.args)
	}

	wg.Wait()
	close(errChan)
	for err := range errChan {
		return err
	}

	return nil
}

// fetchOsInfo выполняет команду для получения информации об ОС контейнера
// и возвращает объект ContainerInfo.
func (d *DistroAPIService) fetchOsInfo(ctx context.Context, containerName string) (ContainerInfo, error) {
	stdout, stderr, err := d.runner.Run(ctx, []string{"distrobox", "enter", containerName, "--", "cat", "/etc/os-release"}, command.WithQuiet())
	if err != nil {
		stderrStr := strings.TrimSpace(stderr)
		errMsg := fmt.Errorf(app.T_("Error getting OS information for container %s: %v"), containerName, err)
		if stderrStr != "" {
			errMsg = fmt.Errorf(app.T_("Error getting OS information for container %s: %s"), containerName, stderrStr)
		}
		return ContainerInfo{ContainerName: containerName, OS: "", Active: false}, errMsg
	}

	osReleaseContent := strings.TrimSpace(stdout)
	lines := strings.Split(osReleaseContent, "\n")
	var osName string

	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			osName = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(line, "ID=", ""), "\"", ""))
			break
		}
	}
	if osName == "" {
		for _, line := range lines {
			if strings.HasPrefix(line, "NAME=") {
				osName = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(line, "NAME=", ""), "\"", ""))
				break
			}
		}
	}

	// Приводим имя ОС к нужному формату и определяем активность контейнера
	lowerOsName := strings.ToLower(osName)
	active := false
	switch {
	case strings.Contains(lowerOsName, "arch"):
		osName = "Arch"
		active = true
	case strings.Contains(lowerOsName, "alt"):
		osName = "ALT Linux"
		active = true
	case strings.Contains(lowerOsName, "ubuntu"):
		osName = "Ubuntu"
		active = true
	}

	return ContainerInfo{ContainerName: containerName, OS: osName, Active: active}, nil
}

// GetContainerOsInfo запрос информации о контейнере.
func (d *DistroAPIService) GetContainerOsInfo(ctx context.Context, containerName string) (ContainerInfo, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventDistroGetContainerInfo))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventDistroGetContainerInfo))

	// Получаем список контейнеров
	containers, err := d.GetContainerList(ctx, false)
	if err != nil {
		return ContainerInfo{}, fmt.Errorf(app.T_("Failed to get the list of containers: %v"), err)
	}

	var found bool
	for _, c := range containers {
		if c.ContainerName == containerName {
			found = true
			break
		}
	}

	if !found {
		return ContainerInfo{}, fmt.Errorf(app.T_("Container %s not found"), containerName)
	}

	return d.fetchOsInfo(ctx, containerName)
}

// CreateContainer создает контейнер, выполняя команду создания, и затем возвращает информацию о контейнере.
func (d *DistroAPIService) CreateContainer(ctx context.Context, image, containerName string, addPkg string, hook string) (ContainerInfo, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventDistroCreateContainer))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventDistroCreateContainer))

	if err := validateContainerName(containerName); err != nil {
		return ContainerInfo{}, err
	}
	if err := validateImageRef(image); err != nil {
		return ContainerInfo{}, err
	}
	if addPkg != "" {
		if err := validatePackageList(addPkg); err != nil {
			return ContainerInfo{}, err
		}
	}

	containers, errContainerList := d.GetContainerList(ctx, false)
	if errContainerList != nil {
		app.Log.Error(errContainerList.Error())
		return ContainerInfo{ContainerName: containerName, OS: "", Active: false}, errContainerList
	}

	// Проверка на существование контейнера
	for _, c := range containers {
		if c.ContainerName == containerName {
			return ContainerInfo{ContainerName: containerName, OS: "", Active: false},
				fmt.Errorf(app.T_("Container already exists: %s"), containerName)
		}
	}

	// Формирование аргументов команды без shell
	args := []string{"distrobox", "create", "-i", image, "-n", containerName, "--yes"}

	// Добавляем параметр --additional-packages, если переменная addPkg не пустая
	if addPkg != "" {
		args = append(args, "--additional-packages", addPkg)
	}

	// Добавляем параметр --init-hooks, если переменная hook не пустая
	if hook != "" {
		args = append(args, "--init-hooks", hook)
	}

	_, stderr, err := d.runner.Run(ctx, args)
	if err != nil {
		app.Log.Errorf(app.T_("Failed to create container %s: %v, stderr: %s"), containerName, err, stderr)
		return ContainerInfo{}, fmt.Errorf(app.T_("Failed to create container %s: %v"), containerName, stderr)
	}

	// Первый enter инициализирует контейнер (установка пакетов, настройка)
	if _, stderr, err = d.runner.Run(ctx, []string{"distrobox", "enter", containerName, "--", "true"}); err != nil {
		app.Log.Errorf(app.T_("Failed to initialize container %s: %v, stderr: %s"), containerName, err, stderr)
	}

	return d.GetContainerOsInfo(ctx, containerName)
}

// RemoveContainer удаление контейнера
func (d *DistroAPIService) RemoveContainer(ctx context.Context, containerName string) (ContainerInfo, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventDistroRemoveContainer))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventDistroRemoveContainer))

	if err := validateContainerName(containerName); err != nil {
		return ContainerInfo{}, err
	}

	osInfo, err := d.GetContainerOsInfo(ctx, containerName)
	if err != nil {
		return osInfo, err
	}

	_, stderr, errRm := d.runner.Run(ctx, []string{"distrobox", "rm", "--yes", "--force", containerName})
	if errRm != nil {
		return ContainerInfo{}, fmt.Errorf(app.T_("Failed to delete container %s: %v, stderr: %s"), containerName, errRm, stderr)
	}

	return osInfo, nil
}

var (
	packageNameRegex   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.+:-]*$`)
	containerNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	imageRefRegex      = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_./:@-]*$`)
)

// validatePackageName проверяет имя пакета на допустимые символы.
func validatePackageName(name string) error {
	if !packageNameRegex.MatchString(name) {
		return fmt.Errorf(app.T_("Invalid package name: %q"), name)
	}
	return nil
}

// validateContainerName проверяет имя контейнера на допустимые символы.
func validateContainerName(name string) error {
	if !containerNameRegex.MatchString(name) {
		return fmt.Errorf(app.T_("Invalid container name: %q"), name)
	}
	return nil
}

// validateImageRef проверяет ссылку на образ на допустимые символы.
func validateImageRef(ref string) error {
	if !imageRefRegex.MatchString(ref) {
		return fmt.Errorf(app.T_("Invalid image reference: %q"), ref)
	}
	return nil
}

// validatePackageList проверяет список пакетов (через пробел) на допустимые символы.
func validatePackageList(pkgList string) error {
	for _, pkg := range strings.Fields(pkgList) {
		if err := validatePackageName(pkg); err != nil {
			return err
		}
	}
	return nil
}
