package service

import (
	"apm/cmd/common/reply"
	"apm/lib"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type HostImage struct {
	Spec struct {
		Image ImageInfo `json:"image"`
	} `json:"spec"`
	Status struct {
		Staged *ImageStatus `json:"staged"`
		Booted ImageStatus  `json:"booted"`
	} `json:"status"`
}

type ImageInfo struct {
	Image     string `json:"image"`
	Transport string `json:"transport"`
}

type ImageStatus struct {
	Image  Image  `json:"image"`
	Pinned bool   `json:"pinned"`
	Store  string `json:"store"`
}

type Image struct {
	Image       ImageInfo `json:"image"`
	Version     *string   `json:"version"`
	Timestamp   string    `json:"timestamp"`
	ImageDigest string    `json:"imageDigest"`
}

const ContainerPath = "/var/Containerfile"

func GetHostImage() (HostImage, error) {
	var host HostImage

	command := fmt.Sprintf("%s bootc status --format json", lib.Env.CommandPrefix)
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = append(os.Environ(), "PATH=/home/linuxbrew/.linuxbrew/bin:"+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return host, fmt.Errorf("не удалось выполнить команду bootc: %v", string(output))
	}

	if err := json.Unmarshal(output, &host); err != nil {
		return host, fmt.Errorf("не удалось распарсить JSON: %v", err)
	}

	transport := strings.TrimSpace(host.Status.Booted.Image.Image.Transport)
	// Если образ пуст или начинается с "containers-storage", ищем в файле
	if strings.HasPrefix(transport, "containers-storage") {
		file, err := os.Open(ContainerPath)
		if err != nil {
			return host, fmt.Errorf("не удалось открыть файл %s: %v", ContainerPath, err.Error())
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		found := false
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "FROM ") {
				candidate := strings.TrimSpace(line[len("FROM "):])
				candidate = strings.Trim(candidate, "\"")
				if candidate != "" {
					host.Status.Booted.Image.Image.Image = candidate
					found = true
					break
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return host, fmt.Errorf("ошибка чтения файла Containerfile: %v", err)
		}
		if !found {
			return host, fmt.Errorf("не удалось определить образ дистрибутива")
		}
	}

	return host, nil
}

// EnableOverlay проверяет и активирует наложение файловой системы.
func EnableOverlay() error {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return fmt.Errorf("ошибка доступа к /proc/mounts: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	runOverlay := true
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		device, mountpoint := fields[0], fields[1]
		if device == "overlay" && mountpoint == "/usr" {
			runOverlay = false
			break
		}
	}
	if scanner.Err() != nil {
		return scanner.Err()
	}

	if runOverlay {
		command := fmt.Sprintf("%s bootc usr-overlay", lib.Env.CommandPrefix)
		cmd := exec.Command("sh", "-c", command)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ошибка активации usr-overlay: %s", string(output))
		}
	}

	return nil
}

// BuildImage сборка образа
func BuildImage(ctx context.Context, pullImage bool) (string, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)
	command := fmt.Sprintf("%s podman build --squash -t os /var", lib.Env.CommandPrefix)
	if pullImage {
		command = fmt.Sprintf("%s podman build --pull=always --squash -t os /var", lib.Env.CommandPrefix)
	}

	err := PullAndProgress(ctx, command)
	if err != nil {
		return "", fmt.Errorf("ошибка сборки образа: %v", err)
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s podman images -q os", lib.Env.CommandPrefix))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ошибка podman образ: %v", err)
	}

	podmanImageID := strings.TrimSpace(string(output))
	if podmanImageID == "" {
		return "", fmt.Errorf("нет валидных образов с тегом tag 'os'. Сначало соберите образ")
	}

	return podmanImageID, nil
}

// SwitchImage переключение образа
func SwitchImage(ctx context.Context, podmanImageID string) error {
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)

	command := fmt.Sprintf("%s bootc switch --transport containers-storage %s", lib.Env.CommandPrefix, podmanImageID)
	cmd := exec.Command("sh", "-c", command)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ошибка переключения на новый образ: %s", string(output))
	}

	return nil
}

// CheckAndUpdateBaseImage проверяет обновление базового образа.
func CheckAndUpdateBaseImage(ctx context.Context, pullImage bool, config Config) error {
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)
	image, err := GetHostImage()
	if err != nil {
		return fmt.Errorf("ошибка получения информации: %v", err)
	}

	if image.Status.Booted.Image.Image.Transport != "containers-storage" {
		command := fmt.Sprintf("%s bootc upgrade --check", lib.Env.CommandPrefix)
		cmd := exec.Command("sh", "-c", command)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("bootc upgrade --check failed: %s", string(output))
		}

		if !strings.Contains(string(output), "No changes in:") {
			return bootcUpgrade(ctx)
		}
		return nil
	}

	if _, err := os.Stat(ContainerPath); err != nil {
		return fmt.Errorf("ошибка, файл %s не найден", ContainerPath)
	}

	return BuildAndSwitch(ctx, pullImage, config)
}

func bootcUpgrade(ctx context.Context) error {
	reply.CreateEventNotification(ctx, reply.StateBefore)
	defer reply.CreateEventNotification(ctx, reply.StateAfter)

	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s bootc upgrade", lib.Env.CommandPrefix))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("bootc upgrade failed: %s", string(output))
	}

	return nil
}

// BuildAndSwitch перестраивает и переключает систему на новый образ.
func BuildAndSwitch(ctx context.Context, pullImage bool, config Config) error {
	idImage, err := BuildImage(ctx, pullImage)
	if err != nil {
		return err
	}

	err = SwitchImage(ctx, idImage)
	if err != nil {
		return err
	}

	err = config.SaveToDB(ctx)
	if err != nil {
		return err
	}

	return pruneOldImages(ctx)
}
