package api

import (
	"apm/cmd/common/reply"
	"apm/lib"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

type ContainerInfo struct {
	OS            string `json:"os"`
	ContainerName string `json:"containerName"`
	Active        bool   `json:"active"`
}

// GetContainerList получает список контейнеров, а если требуется полная информация (getFullInfo),
// то параллельно для каждого контейнера вызывается fetchOsInfo.
func GetContainerList(getFullInfo bool) ([]ContainerInfo, error) {
	reply.CreateEventNotification(reply.StateBefore)
	defer reply.CreateEventNotification(reply.StateAfter)

	command := fmt.Sprintf("%s distrobox ls", lib.Env.CommandPrefix)
	cmd := exec.Command("sh", "-c", command)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, errors.New("Не удалось получить список контейнеров: " + err.Error())
	}

	output := strings.TrimSpace(out.String())
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
				info, err := fetchOsInfo(n)
				if err != nil {
					lib.Log.Error(err)
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
// Если isConsole == false, формируется команда экспорта GUI приложения;
// если isConsole == true, формируются команды для каждого пути из pathList.
func ExportingApp(containerInfo ContainerInfo, packageName string, isConsole bool, pathList []string, deleteApp bool) error {
	reply.CreateEventNotification(reply.StateBefore)
	defer reply.CreateEventNotification(reply.StateAfter)
	// Определяем суффикс: "-d", если deleteApp == true, иначе пустая строка.
	suffix := ""
	if deleteApp {
		suffix = "-d"
	}

	var commands []string

	if !isConsole {
		// Команда экспорта GUI-приложения
		appCommand := fmt.Sprintf("%s distrobox enter %s -- distrobox-export --app %s %s",
			lib.Env.CommandPrefix, containerInfo.ContainerName, packageName, suffix)
		commands = append(commands, appCommand)
	} else {
		// Формируем команду для каждого пути консольного приложения
		for _, path := range pathList {
			pathCommand := fmt.Sprintf("%s distrobox enter %s -- distrobox-export -b %s %s",
				lib.Env.CommandPrefix, containerInfo.ContainerName, path, suffix)
			commands = append(commands, pathCommand)
		}
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
				errChan <- fmt.Errorf("ошибка выполнения команды %q: %v", command, err)
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
func fetchOsInfo(containerName string) (ContainerInfo, error) {
	command := fmt.Sprintf("%s distrobox enter %s -- cat /etc/os-release", lib.Env.CommandPrefix, containerName)
	cmd := exec.Command("sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		lib.Log.Errorf("Ошибка получения информации об ОС контейнера %s: %v, stderr: %s", containerName, err, stderr.String())
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
		osName = "Altlinux"
	case strings.Contains(lowerOsName, "ubuntu"):
		osName = "Ubuntu"
		active = true
	}

	return ContainerInfo{ContainerName: containerName, OS: osName, Active: active}, nil
}

// GetContainerOsInfo теперь просто вызывает fetchOsInfo, что позволяет использовать её и отдельно.
func GetContainerOsInfo(containerName string) (ContainerInfo, error) {
	reply.CreateEventNotification(reply.StateBefore)
	defer reply.CreateEventNotification(reply.StateAfter)
	return fetchOsInfo(containerName)
}

// CreateContainer создает контейнер, выполняя команду создания, и затем возвращает информацию о контейнере.
func CreateContainer(image, containerName string, addPkg string, hook string) (ContainerInfo, error) {
	reply.CreateEventNotification(reply.StateBefore)
	defer reply.CreateEventNotification(reply.StateAfter)
	containers, errContainerList := GetContainerList(false)
	if errContainerList != nil {
		lib.Log.Error(errContainerList.Error())

		return ContainerInfo{ContainerName: containerName, OS: "", Active: false}, errContainerList
	}

	var found *ContainerInfo
	for _, c := range containers {
		if c.ContainerName == containerName {
			found = &c
			break
		}
	}

	if found != nil {
		return ContainerInfo{ContainerName: containerName, OS: "", Active: false},
			fmt.Errorf("контейнер уже сушествует: %s", containerName)
	}

	var command string
	if len(hook) > 0 {
		command = fmt.Sprintf("%s distrobox create -i %s -n %s --yes --additional-packages %s --init-hooks %s",
			lib.Env.CommandPrefix, image, containerName, addPkg, hook)
	} else {
		command = fmt.Sprintf("%s distrobox create -i %s -n %s --yes --additional-packages %s",
			lib.Env.CommandPrefix, image, containerName, addPkg)
	}
	cmd := exec.Command("sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Выполняем команду создания контейнера
	if err := cmd.Run(); err != nil {
		lib.Log.Errorf("не удалось создать контейнер %s: %v, stderr: %s", containerName, err, stderr.String())
		return ContainerInfo{}, fmt.Errorf("не удалось создать контейнер %s: %v", containerName, err)
	}

	return GetContainerOsInfo(containerName)
}

// RemoveContainer удаление контейнера
func RemoveContainer(containerName string) (ContainerInfo, error) {
	reply.CreateEventNotification(reply.StateBefore)
	defer reply.CreateEventNotification(reply.StateAfter)
	command := fmt.Sprintf("%s distrobox rm %s", lib.Env.CommandPrefix, containerName)
	cmd := exec.Command("sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	osInfo, err := GetContainerOsInfo(containerName)
	if err != nil {
		return osInfo, err
	}

	if err := cmd.Run(); err != nil {
		return ContainerInfo{}, fmt.Errorf("не удалось удалить контейнер %s: %v, stderr: %s", containerName, err, stderr.String())
	}

	return osInfo, nil
}
