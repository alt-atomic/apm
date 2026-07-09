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

package imagesvc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/build/modules"
	pkgbuild "altlinux.space/alt-atomic/apm/pkg/build"
	_ "altlinux.space/alt-atomic/apm/pkg/build/modules"
)

type Config = pkgbuild.Config

var (
	ParseJsonConfigData = pkgbuild.ParseJsonConfigData
)

// HostConfigService предоставляет сервис для работы с конфигурацией хоста.
type HostConfigService struct {
	config              *Config
	serviceHostDatabase *HostDBService
	hostImageService    *HostImageService
}

func NewHostConfigService(hostDBService *HostDBService, hostImageService *HostImageService) *HostConfigService {
	return &HostConfigService{
		serviceHostDatabase: hostDBService,
		hostImageService:    hostImageService,
	}
}

// ApplyPathOverrides переопределяет путь к файлу конфигурации и/или к рабочей директории сборки.
func (s *HostConfigService) ApplyPathOverrides(configPath, workdir string) error {
	if configPath != "" {
		abs, err := filepath.Abs(configPath)
		if err != nil {
			return err
		}
		s.hostImageService.appConfig.PathImageFile = abs
	}
	if workdir != "" {
		abs, err := filepath.Abs(workdir)
		if err != nil {
			return err
		}
		s.hostImageService.appConfig.PathResourcesDir = abs
	}
	return nil
}

// syncYamlMutex защищает операции работы с файлом.
var syncYamlMutex sync.Mutex

// LoadConfig загружает конфигурацию из файла и сохраняет в поле config.
func (s *HostConfigService) LoadConfig() error {
	var (
		cfg Config
		err error
	)

	if _, err = os.Stat(s.hostImageService.appConfig.PathImageFile); os.IsNotExist(err) {
		if cfg, err = s.hostImageService.GenerateDefaultConfig(); err != nil {
			return err
		}
		s.config = &cfg
		return s.SaveConfig()
	}

	if cfg, err = pkgbuild.ReadAndParseConfig(s.hostImageService.appConfig.PathImageFile); err != nil {
		return err
	}

	// Рекурсивная валидация всех include файлов
	basePath := s.hostImageService.appConfig.PathResourcesDir
	if err = pkgbuild.ValidateConfigRecursive(&cfg, basePath, s.hostImageService.appConfig.ParsedVersion.ToBuildVersion()); err != nil {
		return err
	}

	s.config = &cfg
	return nil
}

func (s *HostConfigService) GetConfigEnvVars() (map[string]string, error) {
	var (
		envs pkgbuild.Envs
		err  error
	)

	if _, err = os.Stat(s.hostImageService.appConfig.PathImageFile); os.IsNotExist(err) {
		return map[string]string{}, nil
	}

	if envs, err = pkgbuild.ReadAndParseConfigEnv(s.hostImageService.appConfig.PathImageFile, s.hostImageService.appConfig.ParsedVersion.ToBuildVersion()); err != nil {
		return map[string]string{}, err
	}
	return envs.Env, nil
}

// SaveConfig сохраняет текущую конфигурацию сервиса в файл.
func (s *HostConfigService) SaveConfig() error {
	if s.config == nil {
		return errors.New(app.T_("Configuration not loaded"))
	}
	syncYamlMutex.Lock()
	defer syncYamlMutex.Unlock()

	return s.config.Save(s.hostImageService.appConfig.PathImageFile)
}

// GenerateDockerfile делегирует генерацию Dockerfile к HostImageService
func (s *HostConfigService) GenerateDockerfile(hostCache bool) error {
	return s.hostImageService.GenerateDockerfile(*s.config, hostCache)
}

// ConfigIsChanged проверяет, изменился ли новый конфиг, используя сервис для работы с базой.
func (s *HostConfigService) ConfigIsChanged(ctx context.Context) (bool, error) {
	statusSame, err := s.serviceHostDatabase.IsLatestConfigSame(ctx, *s.config)
	if err != nil {
		return false, err
	}
	return !statusSame, nil
}

// SaveConfigToDB сохраняет историю конфигурации в базу, если конфиг изменился.
func (s *HostConfigService) SaveConfigToDB(ctx context.Context) error {
	changed, err := s.ConfigIsChanged(ctx)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	history := ImageHistory{
		ImageName: s.config.Image,
		Config:    s.config,
		ImageDate: time.Now().Format(time.RFC3339),
	}
	return s.serviceHostDatabase.SaveImageToDB(ctx, history)
}

// IsInstalled проверяет наличие пакета в списке для установки.
func (s *HostConfigService) IsInstalled(pkg string) bool {
	return isInstalled(s.config, pkg)
}

// IsRemoved проверяет наличие пакета в списке для удаления.
func (s *HostConfigService) IsRemoved(pkg string) bool {
	return isRemoved(s.config, pkg)
}

// SetImage меняет базовый образ и сохраняет изменения в файл.
func (s *HostConfigService) SetImage(image string) error {
	s.config.Image = image
	return s.SaveConfig()
}

// AddInstallPackage добавляет пакет в список для установки и сохраняет изменения в файл.
func (s *HostConfigService) AddInstallPackage(pkg string) error {
	addInstallPackage(s.config, pkg)
	return s.SaveConfig()
}

// AddRemovePackage добавляет пакет в список для удаления и сохраняет изменения в файл.
func (s *HostConfigService) AddRemovePackage(pkg string) error {
	addRemovePackage(s.config, pkg)
	return s.SaveConfig()
}

// GetConfig возвращает текущую конфигурацию.
func (s *HostConfigService) GetConfig() *Config {
	return s.config
}

// SetConfig устанавливает конфигурацию.
func (s *HostConfigService) SetConfig(config *Config) {
	s.config = config
}

const imageApplyModuleName = "image-apply-results"

func totalInstall(cfg *Config) []string {
	var installed []string
	for _, module := range cfg.Modules {
		if module.Type == modules.TypePackages {
			body, ok := module.Body.(*modules.PackagesBody)
			if !ok {
				continue
			}
			installed = append(installed, body.Install...)
			for _, p := range body.Remove {
				installed = removeByValue(installed, p)
			}
		}
	}
	return installed
}

func totalRemove(cfg *Config) []string {
	var removed []string
	for _, module := range cfg.Modules {
		if module.Type == modules.TypePackages {
			body, ok := module.Body.(*modules.PackagesBody)
			if !ok {
				continue
			}
			for _, p := range body.Install {
				removed = removeByValue(removed, p)
			}
			removed = append(removed, body.Remove...)
		}
	}
	return removed
}

// applyPackagesModule возвращает существующий модуль image-apply-results.
func applyPackagesModule(cfg *Config) *pkgbuild.Module {
	for i := range cfg.Modules {
		if cfg.Modules[i].Type == modules.TypePackages && cfg.Modules[i].Name == imageApplyModuleName {
			return &cfg.Modules[i]
		}
	}

	cfg.Modules = append(cfg.Modules, pkgbuild.Module{
		Name: imageApplyModuleName,
		Type: modules.TypePackages,
		Body: &modules.PackagesBody{
			Install: []string{},
			Remove:  []string{},
		},
	})

	return &cfg.Modules[len(cfg.Modules)-1]
}

func addInstallPackage(cfg *Config, pkg string) {
	if slices.Contains(totalInstall(cfg), pkg) {
		return
	}

	module := applyPackagesModule(cfg)
	body, ok := module.Body.(*modules.PackagesBody)
	if !ok {
		body = &modules.PackagesBody{}
		module.Body = body
	}

	if slices.Contains(body.Remove, pkg) {
		body.Remove = removeByValue(body.Remove, pkg)
	} else {
		body.Install = append(body.Install, pkg)
	}
}

func addRemovePackage(cfg *Config, pkg string) {
	if slices.Contains(totalRemove(cfg), pkg) {
		return
	}

	module := applyPackagesModule(cfg)
	body, ok := module.Body.(*modules.PackagesBody)
	if !ok {
		body = &modules.PackagesBody{}
		module.Body = body
	}

	if slices.Contains(body.Install, pkg) {
		body.Install = removeByValue(body.Install, pkg)
	} else {
		body.Remove = append(body.Remove, pkg)
	}
}

func isInstalled(cfg *Config, pkg string) bool {
	return slices.Contains(totalInstall(cfg), pkg)
}

func isRemoved(cfg *Config, pkg string) bool {
	return slices.Contains(totalRemove(cfg), pkg)
}

func removeByValue(arr []string, value string) []string {
	return slices.DeleteFunc(arr, func(s string) bool {
		return s == value
	})
}

// SwitchableConfig — контракт конфигурации для HostImageService.BuildAndSwitch.
type SwitchableConfig interface {
	ConfigIsChanged(ctx context.Context) (bool, error)
	SaveConfigToDB(ctx context.Context) error
}
