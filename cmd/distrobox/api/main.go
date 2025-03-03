package api

import (
	"apm/config"
	"apm/logger"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

type ContainerInfo struct {
	OS            string
	ContainerName string
	Active        bool
}

// GetContainerList возвращает список объектов ContainerInfo с именами контейнеров.
func GetContainerList(getFullInfo bool) ([]ContainerInfo, error) {
	// Формируем команду с префиксом из конфигурации
	command := fmt.Sprintf("%s distrobox ls", config.Env.CommandPrefix)

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
	for _, line := range lines[1:] {
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimSpace(parts[1])
		if name != "" {
			var osInfo ContainerInfo
			var err error
			if getFullInfo {
				osInfo, err = GetContainerOsInfo(name)
				if err != nil {
					logger.Log.Error(err)
					continue
				}
			} else {
				osInfo = ContainerInfo{
					ContainerName: name,
					OS:            "",
					Active:        false,
				}
			}

			containers = append(containers, osInfo)
		}
	}

	return containers, nil
}

// ExportingApp экспортирует пакет в хост-систему.
// Если isConsole == false, формируется команда экспорта GUI приложения;
// если isConsole == true, формируются команды для каждого пути из pathList.
func ExportingApp(containerInfo ContainerInfo, packageName string, isConsole bool, pathList []string, deleteApp bool) error {
	// Определяем суффикс: "-d", если deleteApp == true, иначе пустая строка.
	suffix := ""
	if deleteApp {
		suffix = "-d"
	}

	var commands []string

	if !isConsole {
		// Команда экспорта GUI-приложения
		appCommand := fmt.Sprintf("%s distrobox enter %s -- distrobox-export --app %s %s",
			config.Env.CommandPrefix, containerInfo.ContainerName, packageName, suffix)
		commands = append(commands, appCommand)
	} else {
		// Формируем команду для каждого пути консольного приложения
		for _, path := range pathList {
			pathCommand := fmt.Sprintf("%s distrobox enter %s -- distrobox-export -b %s %s",
				config.Env.CommandPrefix, containerInfo.ContainerName, path, suffix)
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
				errChan <- fmt.Errorf("ошибка выполнения команды %q: %v, stderr: %s", command, err, stderr)
			}
		}(cmdStr)
	}

	wg.Wait()
	close(errChan)
	// Если произошла хотя бы одна ошибка, возвращаем первую найденную
	for err := range errChan {
		return err
	}

	return nil
}

// GetContainerOsInfo возвращает объект с информацией о контейнере
func GetContainerOsInfo(containerName string) (ContainerInfo, error) {
	containers, errContainerList := GetContainerList(false)
	if errContainerList != nil {
		logger.Log.Error(errContainerList.Error())

		return ContainerInfo{ContainerName: containerName, OS: "", Active: false}, errContainerList
	}

	var found *ContainerInfo
	for _, c := range containers {
		if c.ContainerName == containerName {
			found = &c
			break
		}
	}

	if found == nil {
		return ContainerInfo{ContainerName: containerName, OS: "", Active: false},
			fmt.Errorf("не удалось найти контейнер: %s", containerName)
	}

	command := fmt.Sprintf("%s distrobox enter %s -- cat /etc/os-release", config.Env.CommandPrefix, containerName)

	// Выполняем команду через оболочку.
	cmd := exec.Command("sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logger.Log.Error("Ошибка получения информации об ОС контейнера %s: %v, stderr: %s", containerName, err, stderr.String())
		return ContainerInfo{ContainerName: containerName, OS: containerName, Active: false}, err
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

	lowerOsName := strings.ToLower(osName)
	if strings.Contains(lowerOsName, "arch") {
		osName = "Arch"
		return ContainerInfo{ContainerName: containerName, OS: osName, Active: true}, nil
	} else if strings.Contains(lowerOsName, "alt") {
		osName = "Altlinux"
		return ContainerInfo{ContainerName: containerName, OS: osName, Active: false}, nil
	} else if strings.Contains(lowerOsName, "ubuntu") {
		osName = "Ubuntu"
		return ContainerInfo{ContainerName: containerName, OS: osName, Active: true}, nil
	}

	return ContainerInfo{ContainerName: containerName, OS: osName, Active: false}, nil
}

// CreateContainer создает контейнер, выполняя команду создания, и затем возвращает информацию о контейнере.
func CreateContainer(image, containerName string, addPkg string) (ContainerInfo, error) {
	containers, errContainerList := GetContainerList(false)
	if errContainerList != nil {
		logger.Log.Error(errContainerList.Error())

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

	command := fmt.Sprintf("%s distrobox create -i %s -n %s --yes --additional-packages %s", config.Env.CommandPrefix, image, containerName, addPkg)
	cmd := exec.Command("sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Выполняем команду создания контейнера
	if err := cmd.Run(); err != nil {
		return ContainerInfo{}, fmt.Errorf("не удалось создать контейнер %s: %v, stderr: %s", containerName, err, stderr.String())
	}

	return GetContainerOsInfo(containerName)
}

// RemoveContainer удаление контейнера
func RemoveContainer(containerName string) (ContainerInfo, error) {
	command := fmt.Sprintf("%s distrobox rm %s", config.Env.CommandPrefix, containerName)
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
