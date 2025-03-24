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
	"apm/lib"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config описывает структуру конфигурационного файла.
type Config struct {
	Image    string `yaml:"image" json:"image"`
	Packages struct {
		Install []string `yaml:"install" json:"install"`
		Remove  []string `yaml:"remove" json:"remove"`
	} `yaml:"packages" json:"packages"`
	Commands []string `yaml:"commands" json:"commands"`
}

// HostConfigService — сервис для работы с конфигурацией хоста.
type HostConfigService struct {
	Config              *Config
	configPath          string
	serviceHostDatabase *HostDBService
}

func NewHostConfigService(configPath string, hostDBService *HostDBService) *HostConfigService {
	return &HostConfigService{
		configPath:          configPath,
		serviceHostDatabase: hostDBService,
	}
}

// syncYamlMutex защищает операции работы с файлом.
var syncYamlMutex sync.Mutex

// LoadConfig загружает конфигурацию из файла и сохраняет в поле config.
func (s *HostConfigService) LoadConfig() error {
	data, err := os.ReadFile(s.configPath)
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

	var cfg Config
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	if cfg.Image == "" {
		return fmt.Errorf(lib.T_("Image must be specified in the configuration file"))
	}
	s.Config = &cfg

	return nil
}

// SaveConfig сохраняет текущую конфигурацию сервиса в файл.
func (s *HostConfigService) SaveConfig() error {
	if s.Config == nil {
		return fmt.Errorf(lib.T_("Configuration not loaded"))
	}

	syncYamlMutex.Lock()
	defer syncYamlMutex.Unlock()

	data, err := yaml.Marshal(s.Config)
	if err != nil {
		return err
	}
	return os.WriteFile(s.configPath, data, 0644)
}

// generateDefaultConfig генерирует конфигурацию по умолчанию, если файл не существует.
func (s *HostConfigService) generateDefaultConfig() (Config, error) {
	var cfg Config
	hostImageService := NewHostImageService(s)
	imageName, err := hostImageService.GetImageFromDocker()
	if err != nil {
		return cfg, err
	}

	cfg.Image = imageName
	cfg.Packages.Install = []string{}
	cfg.Packages.Remove = []string{}
	cfg.Commands = []string{}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return cfg, err
	}

	if err = os.WriteFile(s.configPath, data, 0644); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// GenerateDockerfile генерирует содержимое Dockerfile, формируя apt-get команды с модификаторами для пакетов.
func (s *HostConfigService) GenerateDockerfile() error {
	if err := s.CheckCommands(); err != nil {
		return err
	}

	// Формирование базовой apt-get команды.
	aptCmd := "apt-get update"

	// Формирование списка пакетов с суффиксами: + для установки и - для удаления.
	var pkgs []string
	uniqueInstall := uniqueStrings(s.Config.Packages.Install)
	uniqueRemove := uniqueStrings(s.Config.Packages.Remove)

	for _, pkg := range uniqueInstall {
		pkgs = append(pkgs, pkg+"+")
	}
	for _, pkg := range uniqueRemove {
		pkgs = append(pkgs, pkg+"-")
	}
	if len(pkgs) > 0 {
		aptCmd += " && apt-get -y install " + strings.Join(pkgs, " ")
	}

	// Формирование Dockerfile.
	var dockerfileLines []string
	dockerfileLines = append(dockerfileLines, fmt.Sprintf("FROM \"%s\"", s.Config.Image))
	// Разбиваем apt-get команду по строкам.
	aptLines := splitCommand("RUN ", aptCmd)
	dockerfileLines = append(dockerfileLines, strings.Join(aptLines, "\n"))

	// Формирование RUN блока для пользовательских команд, если они заданы.
	if len(s.Config.Commands) > 0 {
		cmdCombined := strings.Join(s.Config.Commands, " && ")
		cmdLines := splitCommand("RUN ", cmdCombined)
		dockerfileLines = append(dockerfileLines, strings.Join(cmdLines, "\n"))
	}

	dockerStr := strings.Join(dockerfileLines, "\n") + "\n"
	err := os.WriteFile(ContainerFile, []byte(dockerStr), 0644)
	if err != nil {
		return err
	}

	return nil
}

func (s *HostConfigService) CheckCommands() error {
	if len(s.Config.Packages.Install) == 0 && len(s.Config.Packages.Remove) == 0 && len(s.Config.Commands) == 0 {
		return fmt.Errorf(lib.T_("Local image configuration file has no changes"))
	}
	return nil
}

// ConfigIsChanged проверяет, изменился ли новый конфиг, используя сервис для работы с базой.
func (s *HostConfigService) ConfigIsChanged(ctx context.Context) (bool, error) {
	statusSame, err := s.serviceHostDatabase.IsLatestConfigSame(ctx, *s.Config)
	if err != nil {
		return false, err
	}

	// Если конфиг не совпадает с последним сохранённым, значит он изменился.
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

// AddCommand добавляет команду в список Commands и сохраняет изменения в файл.
func (s *HostConfigService) AddCommand(cmd string) error {
	if contains(s.Config.Commands, cmd) {
		return nil
	}
	s.Config.Commands = append(s.Config.Commands, cmd)
	return s.SaveConfig()
}

// IsInstalled проверяет наличие пакета в списке для установки.
func (s *HostConfigService) IsInstalled(pkg string) bool {
	return contains(s.Config.Packages.Install, pkg)
}

// IsRemoved проверяет наличие пакета в списке для удаления.
func (s *HostConfigService) IsRemoved(pkg string) bool {
	return contains(s.Config.Packages.Remove, pkg)
}

// AddInstallPackage добавляет пакет в список для установки и сохраняет изменения в файл.
func (s *HostConfigService) AddInstallPackage(pkg string) error {
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
func (s *HostConfigService) AddRemovePackage(pkg string) error {
	if contains(s.Config.Packages.Remove, pkg) {
		return nil
	}
	if contains(s.Config.Packages.Install, pkg) {
		s.Config.Packages.Install = removeElement(s.Config.Packages.Install, pkg)
	}
	s.Config.Packages.Remove = append(s.Config.Packages.Remove, pkg)
	return s.SaveConfig()
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

// contains проверяет, содержит ли срез slice значение s.
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
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
