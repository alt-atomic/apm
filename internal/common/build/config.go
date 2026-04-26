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
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Config = core.Config

var (
	ParseJsonConfigData = core.ParseJsonConfigData
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

	if cfg, err = core.ReadAndParseConfigYamlFile(s.hostImageService.appConfig.PathImageFile); err != nil {
		return err
	}

	// Рекурсивная валидация всех include файлов
	basePath := s.hostImageService.appConfig.PathResourcesDir
	if err = core.ValidateConfigRecursive(&cfg, basePath); err != nil {
		return err
	}

	s.config = &cfg
	return nil
}

func (s *HostConfigService) GetConfigEnvVars() (map[string]string, error) {
	var (
		envs core.Envs
		err  error
	)

	if _, err = os.Stat(s.hostImageService.appConfig.PathImageFile); os.IsNotExist(err) {
		return map[string]string{}, nil
	}

	if envs, err = core.ReadAndParseConfigEnvYamlFile(s.hostImageService.appConfig.PathImageFile, *s.hostImageService.appConfig.ParsedVersion); err != nil {
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
	return s.config.IsInstalled(pkg)
}

// IsRemoved проверяет наличие пакета в списке для удаления.
func (s *HostConfigService) IsRemoved(pkg string) bool {
	return s.config.IsRemoved(pkg)
}

// AddInstallPackage добавляет пакет в список для установки и сохраняет изменения в файл.
func (s *HostConfigService) AddInstallPackage(pkg string) error {
	s.config.AddInstallPackage(pkg)
	return s.SaveConfig()
}

// AddRemovePackage добавляет пакет в список для удаления и сохраняет изменения в файл.
func (s *HostConfigService) AddRemovePackage(pkg string) error {
	s.config.AddRemovePackage(pkg)
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
