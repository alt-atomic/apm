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

package service

import (
	"apm/internal/common/app"
	"errors"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// TemporaryConfig Config описывает структуру временного конфигурационного файла
type TemporaryConfig struct {
	Packages struct {
		Install []string `yaml:"install" json:"install"`
		Remove  []string `yaml:"remove" json:"remove"`
	} `yaml:"packages" json:"packages"`
}

// TemporaryConfigService HostConfigService — сервис для работы с временным конфигурационным файлом
type TemporaryConfigService struct {
	Config             *TemporaryConfig
	temporaryImageFile string
}

func NewTemporaryConfigService(temporaryImageFile string) *TemporaryConfigService {
	return &TemporaryConfigService{
		temporaryImageFile: temporaryImageFile,
	}
}

// syncYamlMutex защищает операции работы с файлом.
var syncYamlTemporaryMutex sync.Mutex

// LoadConfig загружает конфигурацию из файла и сохраняет в поле config.
func (s *TemporaryConfigService) LoadConfig() error {
	data, err := os.ReadFile(s.temporaryImageFile)
	if err != nil {
		if os.IsNotExist(err) {
			cfg, err := s.generateDefaultConfig()
			if err != nil {
				return err
			}
			s.Config = &cfg
			return s.SaveConfig()
		}
		return err
	}

	var cfg TemporaryConfig
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	s.Config = &cfg

	return nil
}

// SaveConfig сохраняет текущую конфигурацию сервиса в файл.
func (s *TemporaryConfigService) SaveConfig() error {
	if s.Config == nil {
		return errors.New(app.T_("Configuration not loaded"))
	}

	syncYamlTemporaryMutex.Lock()
	defer syncYamlTemporaryMutex.Unlock()

	data, err := yaml.Marshal(s.Config)
	if err != nil {
		return err
	}
	return os.WriteFile(s.temporaryImageFile, data, 0644)
}

// generateDefaultConfig генерирует конфигурацию по умолчанию, если файл не существует.
func (s *TemporaryConfigService) generateDefaultConfig() (TemporaryConfig, error) {
	var cfg TemporaryConfig
	cfg.Packages.Install = []string{}
	cfg.Packages.Remove = []string{}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return cfg, err
	}

	if err = os.WriteFile(s.temporaryImageFile, data, 0644); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// IsInstalled проверяет наличие пакета в списке для установки.
func (s *TemporaryConfigService) IsInstalled(pkg string) bool {
	return contains(s.Config.Packages.Install, pkg)
}

// IsRemoved проверяет наличие пакета в списке для удаления.
func (s *TemporaryConfigService) IsRemoved(pkg string) bool {
	return contains(s.Config.Packages.Remove, pkg)
}

// AddInstallPackage добавляет пакет в список для установки и сохраняет изменения в файл.
func (s *TemporaryConfigService) AddInstallPackage(pkg string) error {
	if contains(s.Config.Packages.Install, pkg) {
		return nil
	}
	if contains(s.Config.Packages.Remove, pkg) {
		s.Config.Packages.Remove = removeElement(s.Config.Packages.Remove, pkg)
	}
	s.Config.Packages.Install = append(s.Config.Packages.Install, pkg)
	return s.SaveConfig()
}

// AddRemovePackage добавляет пакет в список для удаления и сохраняет изменения в файл.
func (s *TemporaryConfigService) AddRemovePackage(pkg string) error {
	if contains(s.Config.Packages.Remove, pkg) {
		return nil
	}
	if contains(s.Config.Packages.Install, pkg) {
		s.Config.Packages.Install = removeElement(s.Config.Packages.Install, pkg)
	}
	s.Config.Packages.Remove = append(s.Config.Packages.Remove, pkg)
	return s.SaveConfig()
}

// DeleteFile удаляет временный конфигурационный файл.
func (s *TemporaryConfigService) DeleteFile() error {
	syncYamlTemporaryMutex.Lock()
	defer syncYamlTemporaryMutex.Unlock()

	if _, err := os.Stat(s.temporaryImageFile); os.IsNotExist(err) {
		return nil
	}

	return os.Remove(s.temporaryImageFile)
}
