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
	"apm/internal/common/build/core"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BuildahOptions опции для сборки через buildah
type BuildahOptions struct {
	Tag           string
	BaseImage     string
	ConfigPath    string
	ResourcesPath string
	NoCache       bool
	EnvVars       []string
}

// BuildahBuilder управляет сборкой через buildah bud
type BuildahBuilder struct {
	appConfig *app.Configuration
	config    *Config
	options   BuildahOptions
}

// NewBuildahBuilder создаёт новый BuildahBuilder
func NewBuildahBuilder(appConfig *app.Configuration, config *Config, opts BuildahOptions) *BuildahBuilder {
	// Определяем пути
	if opts.ConfigPath == "" {
		opts.ConfigPath = appConfig.PathImageFile
	}
	if opts.ResourcesPath == "" {
		opts.ResourcesPath = appConfig.PathResourcesDir
	}

	return &BuildahBuilder{
		appConfig: appConfig,
		config:    config,
		options:   opts,
	}
}

// Build выполняет сборку образа через buildah bud
func (b *BuildahBuilder) Build(ctx context.Context) (string, error) {
	if _, err := exec.LookPath("buildah"); err != nil {
		return "", fmt.Errorf("buildah not found: %w", err)
	}

	baseImage := b.config.Image
	if b.options.BaseImage != "" {
		baseImage = b.options.BaseImage
	}
	if baseImage == "" {
		return "", errors.New("base image is required")
	}

	// Раскрываем все include модули
	resourcesDir := b.options.ResourcesPath
	flatModules, err := core.FlattenModules(b.config.Modules, resourcesDir, b.options.ConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to flatten modules: %w", err)
	}

	app.Log.Info(fmt.Sprintf("Building image from %s with %d modules", baseImage, len(flatModules)))

	// Генерируем Containerfile
	containerfile, err := b.generateContainerfile(baseImage, flatModules)
	if err != nil {
		return "", fmt.Errorf("failed to generate Containerfile: %w", err)
	}

	// Создаём временный файл для Containerfile
	tmpDir, err := os.MkdirTemp("", "apm-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	containerfilePath := filepath.Join(tmpDir, "Containerfile")
	if err = os.WriteFile(containerfilePath, []byte(containerfile), 0644); err != nil {
		return "", fmt.Errorf("failed to write Containerfile: %w", err)
	}

	app.Log.Debug(fmt.Sprintf("Generated Containerfile:\n%s", containerfile))

	imageID, err := b.runBuildahBud(ctx, containerfilePath)
	if err != nil {
		return "", err
	}

	app.Log.Info(fmt.Sprintf("Build complete: %s", b.options.Tag))
	return imageID, nil
}

// generateContainerfile генерирует Containerfile из модулей
func (b *BuildahBuilder) generateContainerfile(baseImage string, modules []core.FlatModule) (string, error) {
	var lines []string

	lines = append(lines, fmt.Sprintf("FROM %s", baseImage))
	lines = append(lines, "")

	// Передаём только указанные переменные окружения
	for _, envSpec := range b.options.EnvVars {
		if strings.Contains(envSpec, "=") {
			parts := strings.SplitN(envSpec, "=", 2)
			value := strings.ReplaceAll(parts[1], "\\", "\\\\")
			value = strings.ReplaceAll(value, "\"", "\\\"")
			value = strings.ReplaceAll(value, "\n", "\\n")
			lines = append(lines, fmt.Sprintf("ENV %s=\"%s\"", parts[0], value))
		} else {
			if value, ok := os.LookupEnv(envSpec); ok {
				value = strings.ReplaceAll(value, "\\", "\\\\")
				value = strings.ReplaceAll(value, "\"", "\\\"")
				value = strings.ReplaceAll(value, "\n", "\\n")
				lines = append(lines, fmt.Sprintf("ENV %s=\"%s\"", envSpec, value))
			}
		}
	}
	if len(b.options.EnvVars) > 0 {
		lines = append(lines, "")
	}

	// Генерируем RUN инструкции для каждого модуля
	for i, fm := range modules {
		label := fm.Module.GetLabel()
		lines = append(lines, fmt.Sprintf("# [%d/%d] %s", i+1, len(modules), label))

		runCmd := b.generateRunCommand(fm, i)
		lines = append(lines, fmt.Sprintf("RUN %s", runCmd))
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n"), nil
}

// generateRunCommand генерирует команду RUN для модуля
func (b *BuildahBuilder) generateRunCommand(fm core.FlatModule, index int) string {
	// Формируем команду с рабочей директорией
	cmd := fmt.Sprintf(
		"apm system image build --config /etc/apm/image.yml --resources /etc/apm/resources --flat-index %d",
		index,
	)

	// Добавляем cd в рабочую директорию если она отличается от resources
	if fm.BaseDir != "" && fm.BaseDir != b.options.ResourcesPath {
		relPath, err := filepath.Rel(b.options.ResourcesPath, fm.BaseDir)
		if err == nil && !strings.HasPrefix(relPath, "..") {
			workDir := filepath.Join("/etc/apm/resources", relPath)
			cmd = fmt.Sprintf("cd %s && %s", workDir, cmd)
		}
	}

	return cmd
}

// runBuildahBud запускает buildah bud
func (b *BuildahBuilder) runBuildahBud(ctx context.Context, containerfilePath string) (string, error) {
	args := []string{
		"bud",
		"--layers",
		"-f", containerfilePath,
		"-t", b.options.Tag,
	}

	if b.options.NoCache {
		args = append(args, "--no-cache")
	}

	// Монтируем конфиг
	args = append(args, "--volume", fmt.Sprintf("%s:/etc/apm/image.yml:ro", b.options.ConfigPath))

	// Монтируем ресурсы
	args = append(args, "--volume", fmt.Sprintf("%s:/etc/apm/resources:ro", b.options.ResourcesPath))

	// Монтируем APM бинарник
	if apmPath, err := exec.LookPath("apm"); err == nil {
		args = append(args, "--volume", fmt.Sprintf("%s:/usr/bin/apm:ro", apmPath))
	}

	// Монтируем системные includes
	if _, err := os.Stat("/usr/share/apm"); err == nil {
		args = append(args, "--volume", "/usr/share/apm:/usr/share/apm:ro")
	}

	// Контекст сборки - текущая директория (не используется, но требуется)
	args = append(args, ".")

	app.Log.Debug(fmt.Sprintf("Running: buildah %s", strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, "buildah", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("buildah bud failed: %w", err)
	}

	return b.options.Tag, nil
}
