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
	"apm/internal/common/reply"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// DistroAPIService реализует методы для работы с пакетами в Arch
type DistroAPIService struct {
	commandPrefix string
}

// NewDistroAPIService возвращает новый экземпляр DistroAPIService.
func NewDistroAPIService(commandPrefix string) *DistroAPIService {
	return &DistroAPIService{
		commandPrefix: commandPrefix,
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
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.GetContainerList"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.GetContainerList"))

	command := fmt.Sprintf("%s distrobox ls", d.commandPrefix)
	stdout, stderr, err := helper.RunCommand(ctx, command)
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
				info, err := d.fetchOsInfo(n)
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

	return containers, nil
}

// ExportingApp экспортирует пакет в хост-систему.
// Принимает отдельные списки для desktop и консольных приложений и обрабатывает каждый тип соответственно.
func (d *DistroAPIService) ExportingApp(ctx context.Context, containerInfo ContainerInfo, _ string, desktopPaths, consolePaths []string, deleteApp bool) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.ExportingApp"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.ExportingApp"))
	// Определяем суффикс: "-d", если deleteApp == true, иначе пустая строка.
	suffix := ""
	if deleteApp {
		suffix = "-d"
	}

	var commands []string

	// Обрабатываем desktop приложения
	for _, path := range desktopPaths {
		appCommand := fmt.Sprintf("%s distrobox enter %s -- distrobox-export --app %s %s",
			d.commandPrefix, containerInfo.ContainerName, path, suffix)
		commands = append(commands, appCommand)
	}

	// Обрабатываем консольные приложения
	for _, path := range consolePaths {
		pathCommand := fmt.Sprintf("%s distrobox enter %s -- distrobox-export -b %s %s",
			d.commandPrefix, containerInfo.ContainerName, path, suffix)
		commands = append(commands, pathCommand)
	}

	// Выполняем все команды параллельно
	var wg sync.WaitGroup
	errChan := make(chan error, len(commands))

	for _, cmdStr := range commands {
		wg.Add(1)
		go func(command string) {
			defer wg.Done()
			cmd := exec.Command("sh", "-c", command)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			if err := cmd.Run(); err != nil {
				errChan <- fmt.Errorf(app.T_("Error executing command %q: %v"), command, err)
			}
		}(cmdStr)
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
func (d *DistroAPIService) fetchOsInfo(containerName string) (ContainerInfo, error) {
	command := fmt.Sprintf("%s distrobox enter %s -- cat /etc/os-release", d.commandPrefix, containerName)
	cmd := exec.Command("sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		app.Log.Errorf(app.T_("Error getting OS information for container %s: %v, stderr: %s"), containerName, err, stderr.String())
		return ContainerInfo{ContainerName: containerName, OS: "", Active: false}, err
	}

	osReleaseContent := strings.TrimSpace(stdout.String())
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
// Зачем мы запрашиваем список контейнеров внутри? Потому что distrobox будет создавать контейнер автоматически
// если не указать правильно название.
func (d *DistroAPIService) GetContainerOsInfo(ctx context.Context, containerName string) (ContainerInfo, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.GetContainerOsInfo"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.GetContainerOsInfo"))

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

	return d.fetchOsInfo(containerName)
}

// CreateContainer создает контейнер, выполняя команду создания, и затем возвращает информацию о контейнере.
func (d *DistroAPIService) CreateContainer(ctx context.Context, image, containerName string, addPkg string, hook string) (ContainerInfo, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.CreateContainer"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.CreateContainer"))

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

	// Формирование базовой части команды
	cmdParts := []string{
		d.commandPrefix,
		"distrobox",
		"create",
		"-i", image,
		"-n", containerName,
		"--yes",
	}

	// Добавляем параметр --additional-packages, если переменная addPkg не пустая
	if addPkg != "" {
		cmdParts = append(cmdParts, "--additional-packages", fmt.Sprintf("'%s'", addPkg))
	}

	// Добавляем параметр --init-hooks, если переменная hook не пустая
	if hook != "" {
		cmdParts = append(cmdParts, "--init-hooks", fmt.Sprintf("'%s'", hook))
	}

	command := strings.Join(cmdParts, " ")

	app.Log.Debug(command)
	cmd := exec.Command("sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Выполнение команды создания контейнера
	if err := cmd.Run(); err != nil {
		app.Log.Errorf(app.T_("Failed to create container %s: %v, stderr: %s"), containerName, err, stderr.String())
		return ContainerInfo{}, fmt.Errorf(app.T_("Failed to create container %s: %v"), containerName, stderr.String())
	}

	return d.GetContainerOsInfo(ctx, containerName)
}

// RemoveContainer удаление контейнера
func (d *DistroAPIService) RemoveContainer(ctx context.Context, containerName string) (ContainerInfo, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("distro.RemoveContainer"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("distro.RemoveContainer"))
	command := fmt.Sprintf("%s distrobox rm --yes --force %s", d.commandPrefix, containerName)
	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	osInfo, err := d.GetContainerOsInfo(ctx, containerName)
	if err != nil {
		return osInfo, err
	}

	if err = cmd.Run(); err != nil {
		return ContainerInfo{}, fmt.Errorf(app.T_("Failed to delete container %s: %v, stderr: %s"), containerName, err, stderr.String())
	}

	return osInfo, nil
}
