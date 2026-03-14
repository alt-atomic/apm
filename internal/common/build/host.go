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
	"apm/internal/common/command"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

// HostImageService предоставляет единый сервис для работы с образами хоста.
type HostImageService struct {
	appConfig     *app.Configuration
	containerPath string
	runner        command.Runner
}

// NewHostImageService создаёт новый сервис для работы с образами хоста.
func NewHostImageService(appConfig *app.Configuration, containerPath string, runner command.Runner) *HostImageService {
	return &HostImageService{
		appConfig:     appConfig,
		containerPath: containerPath,
		runner:        runner,
	}
}

func (h *HostImageService) GetHostImage() (HostImage, error) {
	var host HostImage

	stdout, stderr, err := h.runner.Run(context.Background(), []string{"bootc", "status", "--format", "json"}, command.WithQuiet())
	if err != nil {
		return host, fmt.Errorf(app.T_("Failed to execute bootc command: %v"), stdout+stderr)
	}

	if err = json.Unmarshal([]byte(stdout), &host); err != nil {
		return host, fmt.Errorf(app.T_("Failed to parse JSON: %v"), err)
	}

	return host, nil
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
		stdout, stderr, err := h.runner.Run(context.Background(), []string{"bootc", "usr-overlay"})
		if err != nil {
			return fmt.Errorf(app.T_("Error activating usr-overlay: %s"), stdout+stderr)
		}
	}

	return nil
}

// BuildImage сборка образа
func (h *HostImageService) BuildImage(ctx context.Context, pullImage bool) (string, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemBuildImage))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemBuildImage))

	buildArgs := []string{"podman", "build"}
	if pullImage {
		buildArgs = append(buildArgs, "--pull=always")
	}
	buildArgs = append(buildArgs, "--squash", "-t", "os", "-f", h.containerPath, "/etc/apm")

	if h.appConfig.Verbose {
		_, _, err := h.runner.Run(ctx, buildArgs, command.WithEnv("TMPDIR=/var/tmp", "LC_ALL=C"))
		if err != nil {
			return "", fmt.Errorf(app.T_("Failed to build image. Please fix the configuration: %s"), h.appConfig.PathImageFile)
		}
	} else {
		fullCmd := strings.TrimSpace(h.appConfig.CommandPrefix + " " + strings.Join(buildArgs, " "))
		stdout, err := PullAndProgress(ctx, fullCmd)
		if err != nil {
			if apmLogs := extractAPMLogs(stdout); apmLogs != "" {
				return "", fmt.Errorf("%s\n%s\n%s",
					fmt.Sprintf(app.T_("Failed to build image. Please fix the configuration: %s"), h.appConfig.PathImageFile),
					app.T_("Build log:"),
					apmLogs)
			}
			return "", fmt.Errorf("%s\n%v", stdout, err)
		}
	}

	imgStdout, _, err := h.runner.Run(ctx, []string{"podman", "images", "-q", "os"}, command.WithQuiet())
	if err != nil {
		return "", fmt.Errorf(app.T_("Error podman image: %v"), err)
	}

	podmanImageID := strings.TrimSpace(imgStdout)
	if podmanImageID == "" {
		return "", errors.New(app.T_("No valid images with tag 'os'. Please build the image first."))
	}

	return podmanImageID, nil
}

// SwitchImage переключение образа
func (h *HostImageService) SwitchImage(ctx context.Context, podmanImageID string, isLocal bool) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemSwitchImage))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemSwitchImage))

	var args []string
	if isLocal {
		args = []string{"bootc", "switch", "--transport", "containers-storage", podmanImageID}
	} else {
		args = []string{"bootc", "switch", podmanImageID}
	}
	stdout, stderr, err := h.runner.Run(ctx, args)
	if err != nil {
		return fmt.Errorf(app.T_("Error switching to the new image: %s"), stdout+stderr)
	}

	return nil
}

// CheckAndUpdateBaseImage проверяет обновление базового образа.
func (h *HostImageService) CheckAndUpdateBaseImage(ctx context.Context, pullImage bool, hostCache bool, config Config) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemCheckUpdateBaseImage))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheckUpdateBaseImage))
	image, err := h.GetHostImage()
	if err != nil {
		return fmt.Errorf(app.T_("Error retrieving information: %v"), err)
	}

	if image.Status.Booted.Image.Image.Transport != "containers-storage" {
		stdout, stderr, err := h.runner.Run(ctx, []string{"bootc", "upgrade", "--check"}, command.WithQuiet())
		if err != nil {
			return fmt.Errorf("bootc upgrade --check failed: %s", stdout+stderr)
		}

		if !strings.Contains(stdout+stderr, "No changes in:") {
			return h.bootcUpgrade(ctx)
		}

		return nil
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

	// Нет модулей — просто переключаемся на базовый образ
	if len(config.Modules) == 0 {
		return h.SwitchImage(ctx, config.Image, false)
	}

	// Генерируем Containerfile если его нет
	if _, statErr := os.Stat(h.containerPath); statErr != nil {
		if err = h.GenerateDockerfile(config, hostCache); err != nil {
			return fmt.Errorf(app.T_("Failed to generate Containerfile: %w"), err)
		}
	}

	return h.buildAndSwitchSimple(ctx, pullImage)
}

type SkopeoInspectInfo struct {
	Digest string   `json:"Digest"`
	Layers []string `json:"Layers"`
}

// getRemoteImageDigest используя skopeo, смотрим Layers удалённого или локально образа для сравнения
func (h *HostImageService) getRemoteImageInfo(ctx context.Context, imageName string, checkLocal bool) (string, error) {
	var args []string
	if checkLocal {
		args = []string{"skopeo", "inspect", "containers-storage:" + imageName}
	} else {
		args = []string{"skopeo", "inspect", "docker://" + imageName}
	}

	stdout, stderr, err := h.runner.Run(ctx, args, command.WithEnv("LC_ALL=C"), command.WithQuiet())
	if err != nil {
		errMsg := strings.TrimSpace(stderr)
		if errMsg == "" {
			errMsg = fmt.Sprintf("%v", err)
		}
		return "", fmt.Errorf(app.T_("Skopeo inspect error: %s"), errMsg)
	}

	var info SkopeoInspectInfo
	if err = json.Unmarshal([]byte(stdout), &info); err != nil {
		return "", fmt.Errorf(app.T_("Failed to parse skopeo inspect: %w"), err)
	}

	return strings.Join(info.Layers, ","), nil
}

func (h *HostImageService) bootcUpgrade(ctx context.Context) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemBootcUpgrade))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemBootcUpgrade))

	upgradeCmd := fmt.Sprintf("%s bootc upgrade", h.appConfig.CommandPrefix)
	_, err := BootcUpgradeAndProgress(ctx, upgradeCmd)
	if err != nil {
		return fmt.Errorf(app.T_("Bootc upgrade failed: %v"), err)
	}

	return nil
}

// GenerateDefaultConfig генерирует конфигурацию по умолчанию, если файл не существует.
func (h *HostImageService) GenerateDefaultConfig() (Config, error) {
	var cfg Config

	host, err := h.GetHostImage()
	if err != nil {
		return cfg, err
	}

	transport := strings.TrimSpace(host.Status.Booted.Image.Image.Transport)
	if !strings.HasPrefix(transport, "containers-storage") {
		cfg.Image = host.Status.Booted.Image.Image.Image
		return cfg, nil
	}

	imageName, err := getImageFromDocker(h.containerPath)
	if err != nil {
		return cfg, fmt.Errorf(app.T_("Failed to determine the distribution image: %w. Please specify it in %s"), err, h.appConfig.PathImageFile)
	}

	cfg.Image = imageName
	return cfg, nil
}

// GenerateDockerfile генерирует содержимое Dockerfile, формируя apm команды с модификаторами для пакетов.
func (h *HostImageService) GenerateDockerfile(config Config, hostCache bool) error {
	var dockerfileLines []string
	dockerfileLines = append(dockerfileLines, fmt.Sprintf("FROM \"%s\"", config.Image))

	runCmd := "RUN --mount=type=bind,ro,source=image.yml,target=/etc/apm/image.yml" +
		" --mount=type=bind,ro,source=resources,target=/etc/apm/resources"
	if hostCache {
		runCmd += " --mount=type=cache,target=/var/cache/apt/archives,id=apt-cache" +
			" mkdir -p /var/cache/apt/archives/partial &&"
	}
	runCmd += " apm system image build"
	dockerfileLines = append(dockerfileLines, runCmd)

	dockerStr := strings.Join(dockerfileLines, "\n") + "\n"
	return os.WriteFile(h.containerPath, []byte(dockerStr), 0644)
}

// BuildAndSwitch перестраивает и переключает систему на новый образ. checkSame - включена ли проверка на изменение конфигурации
func (h *HostImageService) BuildAndSwitch(ctx context.Context, pullImage bool, checkSame bool, hostConfigService SwitchableConfig) error {
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

	err = h.SwitchImage(ctx, idImage, true)
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

	err = h.SwitchImage(ctx, idImage, true)
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

// getImageFromDocker ищет название образа в docker-файле
func getImageFromDocker(dockerFilePath string) (string, error) {
	file, err := os.Open(dockerFilePath)
	if err != nil {
		return "", fmt.Errorf(app.T_("Failed to open file %s: %w"), dockerFilePath, err)
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
		return "", fmt.Errorf(app.T_("Error reading file %s: %w"), dockerFilePath, err)
	}

	return "", fmt.Errorf(app.T_("Failed to determine the distribution image in %s"), dockerFilePath)
}

// extractAPMLogs извлекает логи APM из вывода сборки образа
func extractAPMLogs(output string) string {
	var apmLogs []string

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "time=") {
			apmLogs = append(apmLogs, trimmed)
			continue
		}

		if strings.Contains(trimmed, "╰──") {
			apmLogs = append(apmLogs, trimmed)
		}
	}

	if len(apmLogs) > 0 {
		return strings.Join(apmLogs, "\n")
	}

	return ""
}
