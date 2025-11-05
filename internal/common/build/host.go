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

package build

import (
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"bufio"
	"context"
	"encoding/json"
	"errors"
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

// HostImageService — единый сервис для операций с образом (build, switch и т.д.).
type HostImageService struct {
	appConfig     *app.Configuration
	containerPath string
}

// NewHostImageService — конструктор сервиса
func NewHostImageService(appConfig *app.Configuration, containerPath string) *HostImageService {
	return &HostImageService{
		appConfig:     appConfig,
		containerPath: containerPath,
	}
}

func (h *HostImageService) GetHostImage() (HostImage, error) {
	var host HostImage

	command := fmt.Sprintf("%s bootc status --format json", h.appConfig.CommandPrefix)
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return host, fmt.Errorf(app.T_("Failed to execute bootc command: %v"), string(output))
	}

	if err = json.Unmarshal(output, &host); err != nil {
		return host, fmt.Errorf(app.T_("Failed to parse JSON: %v"), err)
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
		return "", fmt.Errorf(app.T_("Failed to open file %s: %w"), h.containerPath, err)
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			app.Log.Error(err)
		}
	}(file)

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
		return "", fmt.Errorf(app.T_("Error reading file %s: %w"), h.containerPath, err)
	}

	return "", fmt.Errorf(app.T_("Failed to determine the distribution image, please specify it manually in the file: %s"), h.containerPath)
}

// EnableOverlay проверяет и активирует наложение файловой системы.
func (h *HostImageService) EnableOverlay() error {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return fmt.Errorf(app.T_("Access error to /proc/mounts: %v"), err)
	}

	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			app.Log.Debug(fmt.Sprintf(app.T_("Failed to close /proc/mounts: %v"), err))
		}
	}(file)

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

	// запускаем если находимся НЕ в контейнере
	if runOverlay && !helper.IsRunningInContainer() {
		command := fmt.Sprintf("%s bootc usr-overlay", h.appConfig.CommandPrefix)
		cmd := exec.Command("sh", "-c", command)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf(app.T_("Error activating usr-overlay: %s"), string(output))
		}
	}

	return nil
}

// BuildImage сборка образа
func (h *HostImageService) BuildImage(ctx context.Context, pullImage bool) (string, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.BuildImage"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.BuildImage"))
	command := fmt.Sprintf("%s podman build --squash -t os /var", h.appConfig.CommandPrefix)
	if pullImage {
		command = fmt.Sprintf("%s podman build --pull=always --squash -t os /var", h.appConfig.CommandPrefix)
	}

	stdout, err := PullAndProgress(ctx, command)
	if err != nil {
		return "", fmt.Errorf(app.T_("Error building image: %s status: %d"), stdout, err)
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s podman images -q os", h.appConfig.CommandPrefix))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf(app.T_("Error podman image: %v"), err)
	}

	podmanImageID := strings.TrimSpace(string(output))
	if podmanImageID == "" {
		return "", errors.New(app.T_("No valid images with tag 'os'. Please build the image first."))
	}

	return podmanImageID, nil
}

// SwitchImage переключение образа
func (h *HostImageService) SwitchImage(ctx context.Context, podmanImageID string) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.SwitchImage"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.SwitchImage"))

	command := fmt.Sprintf("%s bootc switch --transport containers-storage %s", h.appConfig.CommandPrefix, podmanImageID)
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(app.T_("Error switching to the new image: %s"), string(output))
	}

	return nil
}

// CheckAndUpdateBaseImage проверяет обновление базового образа.
func (h *HostImageService) CheckAndUpdateBaseImage(ctx context.Context, pullImage bool, config Config) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.CheckAndUpdateBaseImage"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.CheckAndUpdateBaseImage"))
	image, err := h.GetHostImage()
	if err != nil {
		return fmt.Errorf(app.T_("Error retrieving information: %v"), err)
	}

	if image.Status.Booted.Image.Image.Transport != "containers-storage" {
		command := fmt.Sprintf("%s bootc upgrade --check", h.appConfig.CommandPrefix)
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
		return fmt.Errorf(app.T_("Error, file %s not found"), h.containerPath)
	}

	var (
		remoteDigest  string
		localDigest   string
		errCheckImage error
	)

	remoteDigest, errCheckImage = h.getRemoteImageInfo(ctx, config.Image, false)
	if errCheckImage != nil {
		return errCheckImage
	}
	localDigest, errCheckImage = h.getRemoteImageInfo(ctx, config.Image, true)
	if errCheckImage != nil {
		return errCheckImage
	}

	if remoteDigest == localDigest {
		return nil
	}

	return h.buildAndSwitchSimple(ctx, pullImage)
}

type SkopeoInspectInfo struct {
	Digest string   `json:"Digest"`
	Layers []string `json:"Layers"`
}

// getRemoteImageDigest используя skopeo, смотрим Layers удалённого или локально образа для сравнения
func (h *HostImageService) getRemoteImageInfo(ctx context.Context, imageName string, checkLocal bool) (string, error) {
	command := fmt.Sprintf("%s skopeo inspect docker://%s", h.appConfig.CommandPrefix, imageName)
	if checkLocal {
		command = fmt.Sprintf("%s skopeo inspect containers-storage:%s", h.appConfig.CommandPrefix, imageName)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Env = []string{"LC_ALL=C"}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf(app.T_("Skopeo inspect error: %v"), err)
	}

	var info SkopeoInspectInfo
	if err = json.Unmarshal(out, &info); err != nil {
		return "", fmt.Errorf(app.T_("Failed to parse skopeo inspect: %w"), err)
	}

	return strings.Join(info.Layers, ","), nil
}

func (h *HostImageService) bootcUpgrade(ctx context.Context) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.bootcUpgrade"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.bootcUpgrade"))

	cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%s bootc upgrade", h.appConfig.CommandPrefix))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(app.T_("Bootc upgrade failed: %s"), string(output))
	}

	return nil
}

// GenerateDefaultConfig генерирует конфигурацию по умолчанию, если файл не существует.
func (h *HostImageService) GenerateDefaultConfig() (Config, error) {
	var cfg Config
	imageName, err := h.GetImageFromDocker()
	if err != nil {
		return cfg, err
	}

	cfg.Image = imageName

	return cfg, nil
}

// GenerateDockerfile генерирует содержимое Dockerfile, формируя apm команды с модификаторами для пакетов.
func (h *HostImageService) GenerateDockerfile(config Config) error {
	// Формирование Dockerfile.
	var dockerfileLines []string
	dockerfileLines = append(dockerfileLines, fmt.Sprintf("FROM \"%s\"", config.Image))
	dockerfileLines = append(dockerfileLines, fmt.Sprintf("COPY \"%s\" \"%s\"", h.appConfig.PathResourcesDir, h.appConfig.PathResourcesDir))
	dockerfileLines = append(dockerfileLines, fmt.Sprintf("COPY \"%s\" \"%s\"", h.appConfig.PathImageFile, h.appConfig.PathImageFile))
	dockerfileLines = append(dockerfileLines, "RUN apm system image build")
	dockerfileLines = append(dockerfileLines, fmt.Sprintf("RUN rm -rf \"%s\" \"%s\"", h.appConfig.PathResourcesDir, h.appConfig.PathImageFile))

	dockerStr := strings.Join(dockerfileLines, "\n") + "\n"
	err := os.WriteFile(h.containerPath, []byte(dockerStr), 0644)
	if err != nil {
		return err
	}

	return nil
}

// BuildAndSwitch перестраивает и переключает систему на новый образ. checkSame - включена ли проверка на изменение конфигурации
func (h *HostImageService) BuildAndSwitch(ctx context.Context, pullImage bool, checkSame bool, hostConfigService *HostConfigService) error {
	statusSame, err := hostConfigService.ConfigIsChanged(ctx)
	if err != nil {
		return err
	}
	if !statusSame && checkSame {
		return errors.New(app.T_("The image has not changed, build paused"))
	}

	idImage, err := h.BuildImage(ctx, pullImage)
	if err != nil {
		return err
	}

	err = h.SwitchImage(ctx, idImage)
	if err != nil {
		return err
	}

	err = hostConfigService.SaveConfigToDB(ctx)
	if err != nil {
		return err
	}

	return pruneOldImages(ctx)
}

// buildAndSwitchSimple упрощенная версия BuildAndSwitch без проверки изменений и сохранения в БД
func (h *HostImageService) buildAndSwitchSimple(ctx context.Context, pullImage bool) error {
	idImage, err := h.BuildImage(ctx, pullImage)
	if err != nil {
		return err
	}

	err = h.SwitchImage(ctx, idImage)
	if err != nil {
		return err
	}

	return pruneOldImages(ctx)
}

// uniqueStrings возвращает новый срез, содержащий только уникальные элементы исходного среза.
func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range input {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// splitCommand разбивает команду на строки длиной не более 80 символов с отступом.
func splitCommand(prefix, cmd string) []string {
	if strings.TrimSpace(cmd) == "" {
		return nil
	}

	const maxLineLength = 80
	words := strings.Fields(cmd)
	var lines []string
	currentLine := prefix
	for _, word := range words {
		if len(currentLine)+len(word)+1 > maxLineLength {
			lines = append(lines, currentLine+" \\")
			currentLine = "    " + word
		} else {
			if currentLine == prefix {
				currentLine += word
			} else {
				currentLine += " " + word
			}
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return lines
}
