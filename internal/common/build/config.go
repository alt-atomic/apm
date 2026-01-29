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
	"apm/internal/common/version"
	"context"
	"errors"
	"os"
	"sync"
	"time"
)

type Config = core.Config

var (
	ParseJsonConfigData = core.ParseJsonConfigData
)

// HostConfigService — сервис для работы с конфигурацией хоста.
type HostConfigService struct {
	Config              *Config
	serviceHostDatabase *HostDBService
	pathImageFile       string
	hostImageService    *HostImageService
}

func NewHostConfigService(pathImageFile string, hostDBService *HostDBService, hostImageService *HostImageService) *HostConfigService {
	return &HostConfigService{
		serviceHostDatabase: hostDBService,
		pathImageFile:       pathImageFile,
		hostImageService:    hostImageService,
	}
}

// syncYamlMutex защищает операции работы с файлом.
var syncYamlMutex sync.Mutex

// LoadConfig загружает конфигурацию из файла и сохраняет в поле config.
func (s *HostConfigService) LoadConfig() error {
	var (
		cfg Config
		err error
	)

	if _, err = os.Stat(s.pathImageFile); os.IsNotExist(err) {
		if cfg, err = s.hostImageService.GenerateDefaultConfig(); err != nil {
			return err
		}
		s.Config = &cfg
		return s.SaveConfig()
	}

	if cfg, err = core.ReadAndParseConfigYamlFile(s.pathImageFile); err != nil {
		return err
	}

	// Рекурсивная валидация всех include файлов
	basePath := s.hostImageService.appConfig.PathResourcesDir
	if err = core.ValidateConfigRecursive(&cfg, basePath); err != nil {
		return err
	}

	s.Config = &cfg
	return nil
}

// LoadConfigFromPathWithResources загружает конфигурацию с указанием пути к ресурсам
func (s *HostConfigService) LoadConfigFromPathWithResources(configPath, resourcesPath string) error {
	cfg, err := core.ReadAndParseConfigYamlFile(configPath)
	if err != nil {
		return err
	}

	if err = core.ValidateConfigRecursive(&cfg, resourcesPath); err != nil {
		return err
	}

	s.Config = &cfg
	return nil
}

func (s *HostConfigService) GetConfigEnvVars() (map[string]string, error) {
	var (
		envs core.Envs
		err  error
	)

	if _, err = os.Stat(s.pathImageFile); os.IsNotExist(err) {
		return map[string]string{}, nil
	}

	if envs, err = core.ReadAndParseConfigEnvYamlFile(s.pathImageFile, version.ParseVersion(s.hostImageService.appConfig.Version)); err != nil {
		return map[string]string{}, err
	}
	return envs.Env, nil
}

// SaveConfig сохраняет текущую конфигурацию сервиса в файл.
func (s *HostConfigService) SaveConfig() error {
	if s.Config == nil {
		return errors.New(app.T_("Configuration not loaded"))
	}
	syncYamlMutex.Lock()
	defer syncYamlMutex.Unlock()

	return s.Config.Save(s.pathImageFile)
}

// GenerateDockerfile делегирует генерацию Dockerfile к HostImageService
func (s *HostConfigService) GenerateDockerfile() error {
	return s.hostImageService.GenerateDockerfile(*s.Config)
}

// ConfigIsChanged проверяет, изменился ли новый конфиг, используя сервис для работы с базой.
func (s *HostConfigService) ConfigIsChanged(ctx context.Context) (bool, error) {
	statusSame, err := s.serviceHostDatabase.IsLatestConfigSame(ctx, *s.Config)
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
		ImageName: s.Config.Image,
		Config:    s.Config,
		ImageDate: time.Now().Format(time.RFC3339),
	}
	return s.serviceHostDatabase.SaveImageToDB(ctx, history)
}

// IsInstalled проверяет наличие пакета в списке для установки.
func (s *HostConfigService) IsInstalled(pkg string) bool {
	return s.Config.IsInstalled(pkg)
}

// IsRemoved проверяет наличие пакета в списке для удаления.
func (s *HostConfigService) IsRemoved(pkg string) bool {
	return s.Config.IsRemoved(pkg)
}

// AddInstallPackage добавляет пакет в список для установки и сохраняет изменения в файл.
func (s *HostConfigService) AddInstallPackage(pkg string) error {
	s.Config.AddInstallPackage(pkg)
	return s.SaveConfig()
}

// AddRemovePackage добавляет пакет в список для удаления и сохраняет изменения в файл.
func (s *HostConfigService) AddRemovePackage(pkg string) error {
	s.Config.AddRemovePackage(pkg)
	return s.SaveConfig()
}
