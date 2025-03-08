package service

import (
	"apm/cmd/common/reply"
	"apm/lib"
	"bufio"
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
		cmd := exec.Command("bootc", "usr-overlay")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ошибка активации usr-overlay: %s", string(output))
		}
	}

	return nil
}

// BuildImage сборка образа
func BuildImage(pullImage bool) (string, error) {
	reply.CreateEventNotification(reply.StateBefore)
	defer reply.CreateEventNotification(reply.StateAfter)
	command := "podman build --squash -t os /var"
	if pullImage {
		command = "podman build --pull=always --squash -t os /var"
	}

	cmd := exec.Command("sh", "-c", command)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ошибка сборки образа: %s", string(output))
	}

	cmd = exec.Command("podman", "images", "-q", "os")
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
func SwitchImage(podmanImageID string) error {
	reply.CreateEventNotification(reply.StateBefore)
	defer reply.CreateEventNotification(reply.StateAfter)
	cmd := exec.Command("bootc", "switch", "--transport", "containers-storage", podmanImageID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ошибка переключения на новый образ: %s", string(output))
	}

	return nil
}

// CheckAndUpdateBaseImage проверяет обновление базового образа.
func CheckAndUpdateBaseImage(pullImage bool) error {
	reply.CreateEventNotification(reply.StateBefore)
	defer reply.CreateEventNotification(reply.StateAfter)
	image, err := GetHostImage()
	if err != nil {
		return fmt.Errorf("ошибка получения информации: %v", err)
	}

	if image.Status.Booted.Image.Image.Transport != "containers-storage" {
		cmd := exec.Command("bootc", "upgrade", "--check")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("bootc upgrade --check failed: %s", string(output))
		}

		if strings.Contains(string(output), "No changes in:") {
			return bootcUpgrade()
		}
		return nil
	}

	if _, err := os.Stat(ContainerPath); err != nil {
		return fmt.Errorf("ошибка, файл %s не найден", ContainerPath)
	}

	return BuildAndSwitch(pullImage)
}

func bootcUpgrade() error {
	reply.CreateEventNotification(reply.StateBefore)
	defer reply.CreateEventNotification(reply.StateAfter)

	cmd := exec.Command("bootc", "upgrade")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("bootc upgrade failed: %s", string(output))
	}

	return nil
}

// BuildAndSwitch перестраивает и переключает систему на новый образ.
func BuildAndSwitch(pullImage bool) error {
	idImage, err := BuildImage(pullImage)
	if err != nil {
		return err
	}

	err = SwitchImage(idImage)
	if err != nil {
		return err
	}

	return pruneOldImages()
}
