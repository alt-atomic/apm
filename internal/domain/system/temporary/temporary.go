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

package temporary

import (
	"apm/internal/common/app"
	"errors"
	"os"
	"slices"
	"sync"

	"github.com/goccy/go-yaml"
)

// Config описывает структуру временного конфигурационного файла
type Config struct {
	Packages struct {
		Install []string `yaml:"install" json:"install"`
		Remove  []string `yaml:"remove" json:"remove"`
	} `yaml:"packages" json:"packages"`
}

// Manager сервис для работы с временным конфигурационным файлом
type Manager struct {
	config             *Config
	temporaryImageFile string
}

func NewManager(temporaryImageFile string) *Manager {
	return &Manager{
		temporaryImageFile: temporaryImageFile,
	}
}

// syncYamlMutex защищает операции работы с файлом.
var syncYamlTemporaryMutex sync.Mutex

// LoadConfig загружает конфигурацию из файла и сохраняет в поле config.
func (s *Manager) LoadConfig() error {
	data, err := os.ReadFile(s.temporaryImageFile)
	if err != nil {
		if os.IsNotExist(err) {
			cfg, err := s.generateDefaultConfig()
			if err != nil {
				return err
			}
			s.config = &cfg
			return s.SaveConfig()
		}
		return err
	}

	var cfg Config
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	s.config = &cfg

	return nil
}

// SaveConfig сохраняет текущую конфигурацию сервиса в файл.
func (s *Manager) SaveConfig() error {
	if s.config == nil {
		return errors.New(app.T_("Configuration not loaded"))
	}

	syncYamlTemporaryMutex.Lock()
	defer syncYamlTemporaryMutex.Unlock()

	data, err := yaml.Marshal(s.config)
	if err != nil {
		return err
	}
	return os.WriteFile(s.temporaryImageFile, data, 0644)
}

// generateDefaultConfig генерирует конфигурацию по умолчанию, если файл не существует.
func (s *Manager) generateDefaultConfig() (Config, error) {
	var cfg Config
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
func (s *Manager) IsInstalled(pkg string) bool {
	return slices.Contains(s.config.Packages.Install, pkg)
}

// IsRemoved проверяет наличие пакета в списке для удаления.
func (s *Manager) IsRemoved(pkg string) bool {
	return slices.Contains(s.config.Packages.Remove, pkg)
}

// AddInstallPackage добавляет пакет в список для установки и сохраняет изменения в файл.
func (s *Manager) AddInstallPackage(pkg string) error {
	if slices.Contains(s.config.Packages.Install, pkg) {
		return nil
	}
	if slices.Contains(s.config.Packages.Remove, pkg) {
		s.config.Packages.Remove = removeElement(s.config.Packages.Remove, pkg)
	}
	s.config.Packages.Install = append(s.config.Packages.Install, pkg)
	return s.SaveConfig()
}

// AddRemovePackage добавляет пакет в список для удаления и сохраняет изменения в файл.
func (s *Manager) AddRemovePackage(pkg string) error {
	if slices.Contains(s.config.Packages.Remove, pkg) {
		return nil
	}
	if slices.Contains(s.config.Packages.Install, pkg) {
		s.config.Packages.Install = removeElement(s.config.Packages.Install, pkg)
	}
	s.config.Packages.Remove = append(s.config.Packages.Remove, pkg)
	return s.SaveConfig()
}

// DeleteFile удаляет временный конфигурационный файл.
func (s *Manager) DeleteFile() error {
	syncYamlTemporaryMutex.Lock()
	defer syncYamlTemporaryMutex.Unlock()

	if _, err := os.Stat(s.temporaryImageFile); os.IsNotExist(err) {
		return nil
	}

	return os.Remove(s.temporaryImageFile)
}

// GetConfig возвращает текущую конфигурацию.
func (s *Manager) GetConfig() *Config {
	return s.config
}

// removeElement удаляет элемент из среза строк.
func removeElement(slice []string, element string) []string {
	var newSlice []string
	for _, v := range slice {
		if v != element {
			newSlice = append(newSlice, v)
		}
	}
	return newSlice
}
