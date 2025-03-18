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

var ContainerFile = "/var/Containerfile"

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

// HostImageService — единый сервис для операций с образом (build, switch и т.д.).
type HostImageService struct {
	commandPrefix     string
	containerPath     string
	serviceHostConfig *HostConfigService
}

// NewHostImageService — конструктор сервиса
func NewHostImageService(hostConfigService *HostConfigService) *HostImageService {
	return &HostImageService{
		commandPrefix:     lib.Env.CommandPrefix,
		containerPath:     ContainerFile,
		serviceHostConfig: hostConfigService,
	}
}

func (h *HostImageService) GetHostImage() (HostImage, error) {
	var host HostImage

	command := fmt.Sprintf("%s bootc status --format json", lib.Env.CommandPrefix)
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return host, fmt.Errorf("не удалось выполнить команду bootc: %v", string(output))
	}

	if err = json.Unmarshal(output, &host); err != nil {
		return host, fmt.Errorf("не удалось распарсить JSON: %v", err)
	}

	return host, nil
}

// GetImageFromDocker ищет название образа в docker-файле.
func (h *HostImageService) GetImageFromDocker() (string, error) {
	host, err := h.GetHostImage()
	if err != nil {
		return "", err
	}

	transport := strings.TrimSpace(host.Status.Booted.Image.Image.Transport)
	if !strings.HasPrefix(transport, "containers-storage") {
		return host.Status.Booted.Image.Image.Image, nil
	}

	file, err := os.Open(h.containerPath)
	if err != nil {
		return "", fmt.Errorf("не удалось открыть файл %s: %w", h.containerPath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "FROM ") {
			candidate := strings.Trim(strings.TrimSpace(line[len("FROM "):]), "\"")
			if candidate != "" {
				return candidate, nil
			}
		}
	}

	if err = scanner.Err(); err != nil {
		return "", fmt.Errorf("ошибка чтения файла %s: %w", h.containerPath, err)
	}

	return "", fmt.Errorf("не удалось определить образ дистрибутива, укажите его вручную в файле: %s", lib.Env.PathImageFile)
}

// EnableOverlay проверяет и активирует наложение файловой системы.
func (h *HostImageService) EnableOverlay() error {
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
func (h *HostImageService) BuildImage(ctx context.Context, pullImage bool) (string, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.BuildImage"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.BuildImage"))
	command := fmt.Sprintf("%s podman build --squash -t os /var", lib.Env.CommandPrefix)
	if pullImage {
		command = fmt.Sprintf("%s podman build --pull=always --squash -t os /var", lib.Env.CommandPrefix)
	}

	stdout, err := PullAndProgress(ctx, command)
	if err != nil {
		return "", fmt.Errorf("ошибка сборки образа: %s статус: %d", stdout, err)
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
func (h *HostImageService) SwitchImage(ctx context.Context, podmanImageID string) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.SwitchImage"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.SwitchImage"))

	command := fmt.Sprintf("%s bootc switch --transport containers-storage %s", lib.Env.CommandPrefix, podmanImageID)
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ошибка переключения на новый образ: %s", string(output))
	}

	return nil
}

// CheckAndUpdateBaseImage проверяет обновление базового образа.
func (h *HostImageService) CheckAndUpdateBaseImage(ctx context.Context, pullImage bool, config Config) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.CheckAndUpdateBaseImage"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.CheckAndUpdateBaseImage"))
	image, err := h.GetHostImage()
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
			return h.bootcUpgrade(ctx)
		}
		return nil
	}

	if _, err = os.Stat(h.containerPath); err != nil {
		return fmt.Errorf("ошибка, файл %s не найден", h.containerPath)
	}

	return h.BuildAndSwitch(ctx, pullImage, config, false)
}

func (h *HostImageService) bootcUpgrade(ctx context.Context) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.bootcUpgrade"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.bootcUpgrade"))

	cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%s bootc upgrade", lib.Env.CommandPrefix))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("bootc upgrade failed: %s", string(output))
	}

	return nil
}

// BuildAndSwitch перестраивает и переключает систему на новый образ. checkSame - включена ли проверка на изменение конфигурации
func (h *HostImageService) BuildAndSwitch(ctx context.Context, pullImage bool, config Config, checkSame bool) error {
	statusSame, err := h.serviceHostConfig.ConfigIsChanged(ctx)
	if !statusSame && checkSame {
		return fmt.Errorf("образ не изменился, сборка приостановлена")
	}

	idImage, err := h.BuildImage(ctx, pullImage)
	if err != nil {
		return err
	}

	err = h.SwitchImage(ctx, idImage)
	if err != nil {
		return err
	}

	err = h.serviceHostConfig.SaveConfigToDB(ctx)
	if err != nil {
		return err
	}

	return pruneOldImages(ctx)
}
