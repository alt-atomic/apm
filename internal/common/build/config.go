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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"slices"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
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

var imageApplyModuleName = "Image apply result"

type Config struct {
	// Базовый образ для использования
	Image string `yaml:"image" json:"image"`
	// Имя образа. Используется в именах некоторых созданных файлов
	Name string `yaml:"name" json:"name"`
	// Осистить старые репозитории
	CleanRepos bool `yaml:"clean-repos,omitempty" json:"clean-repos,omitempty"`
	// Репозитории для sources.list. Если пусто, используются репозитории из образа
	Repos []string `yaml:"repos,omitempty" json:"repos,omitempty"`
	// Задачи для подключения в качестве репозиториев
	Tasks []string `yaml:"tasks,omitempty" json:"tasks,omitempty"`
	// Ядро для использования в образе. Если пусто, используется ядро из образа
	Kernel string `yaml:"kernel,omitempty" json:"kernel,omitempty"`
	// Список модулей
	Modules    []Module `yaml:"modules,omitempty" json:"modules,omitempty"`
	hasInclude bool
}

func (cfg *Config) TasksRepos() []string {
	var repos []string

	var templates []string
	switch runtime.GOARCH {
	case "amd64":
		templates = append(
			templates,
			"rpm http://git.altlinux.org repo/%s/x86_64 task",
			"rpm http://git.altlinux.org repo/%s/x86_64-i586 task",
		)
	case "arm64", "aarch64":
		templates = append(
			templates,
			"rpm http://git.altlinux.org repo/%s/aarch64 task",
		)
	default:
		return []string{}
	}

	if !aE(templates) {
		for _, task := range cfg.Tasks {
			for _, template := range templates {
				repos = append(repos, fmt.Sprintf(template, task))
			}
		}
	}

	return repos
}

func (cfg *Config) getTotalInstall() []string {
	var totalInstall []string

	for _, module := range cfg.Modules {
		if module.Type == TypePackages {
			totalInstall = append(totalInstall, module.Body.Install...)

			for _, p := range module.Body.Remove {
				removeByValue(totalInstall, p)
			}
		}
	}

	return totalInstall
}

func (cfg *Config) getTotalRemove() []string {
	var totalRemove []string

	for _, module := range cfg.Modules {
		if module.Type == TypePackages {
			for _, p := range module.Body.Install {
				removeByValue(totalRemove, p)
			}

			totalRemove = append(totalRemove, module.Body.Remove...)
		}
	}

	return totalRemove
}

func (cfg *Config) HasInclude() bool {
	return cfg.hasInclude
}

func (cfg *Config) IsInstalled(pkg string) bool {
	return slices.Contains(cfg.getTotalInstall(), pkg)
}

func (cfg *Config) IsRemoved(pkg string) bool {
	return slices.Contains(cfg.getTotalRemove(), pkg)
}

func (cfg *Config) AddInstallPackage(pkg string) {
	var totalInstall = cfg.getTotalInstall()

	if slices.Contains(totalInstall, pkg) {
		return
	} else {
		var packagesModule *Module
		if cfg.Modules[len(cfg.Modules)-1].Type != TypePackages || cfg.Modules[len(cfg.Modules)-1].Name != imageApplyModuleName {
			cfg.Modules = append(cfg.Modules, Module{
				Name: imageApplyModuleName,
				Type: TypePackages,
				Body: Body{
					Install: []string{},
					Remove:  []string{},
				},
			})
		}
		packagesModule = &cfg.Modules[len(cfg.Modules)-1]
		if slices.Contains(packagesModule.Body.Remove, pkg) {
			packagesModule.Body.Remove = removeByValue(packagesModule.Body.Remove, pkg)
		} else {
			packagesModule.Body.Install = append(packagesModule.Body.Install, pkg)
		}
	}
}

func (cfg *Config) AddRemovePackage(pkg string) {
	var totalRemove = cfg.getTotalRemove()

	if slices.Contains(totalRemove, pkg) {
		return
	} else {
		var packagesModule *Module
		if cfg.Modules[len(cfg.Modules)-1].Type != TypePackages || cfg.Modules[len(cfg.Modules)-1].Name != imageApplyModuleName {
			cfg.Modules = append(cfg.Modules, Module{
				Name: imageApplyModuleName,
				Type: TypePackages,
				Body: Body{
					Install: []string{},
					Remove:  []string{},
				},
			})
		}
		packagesModule = &cfg.Modules[len(cfg.Modules)-1]
		if slices.Contains(packagesModule.Body.Install, pkg) {
			packagesModule.Body.Install = removeByValue(packagesModule.Body.Install, pkg)
		} else {
			packagesModule.Body.Remove = append(packagesModule.Body.Remove, pkg)
		}
	}
}

func (cfg *Config) extendIncludes() error {
	var newModules []Module
	cfg.hasInclude = false

	for _, module := range cfg.Modules {
		if module.Type == TypeInclude {
			cfg.hasInclude = true
			for _, include := range module.Body.GetTargets() {
				data, err := os.ReadFile(include)
				if err != nil {
					return err
				}
				includeCfg, err := parseData(data, true, false)
				if err != nil {
					return err
				}
				err = includeCfg.extendIncludes()
				if err != nil {
					return err
				}

				newModules = append(newModules, includeCfg.Modules...)
			}
		} else {
			newModules = append(newModules, module)
		}
	}

	cfg.Modules = newModules

	return nil
}

func (cfg *Config) fix() error {
	if sE(cfg.Image) {
		return errors.New(app.T_("Image can not be empty"))
	}
	if sE(cfg.Name) {
		return errors.New(app.T_("Name can not be empty"))
	}

	var requiredText = app.T_("Module '%s' required '%s'")
	var requiredTextOr = fmt.Sprintf(requiredText, "%s", app.T_("%s or %s"))

	for _, module := range cfg.Modules {
		if sE(module.Type) {
			return errors.New(app.T_("Module type can not be empty"))
		}

		var b = module.Body

		switch module.Type {
		case TypeGit:
			if aE(b.Commands) {
				return fmt.Errorf(requiredText, TypeGit, "commands")
			}
			if sE(b.Url) {
				return fmt.Errorf(requiredText, TypeGit, "url")
			}
		case TypeShell:
			if aE(b.Commands) {
				return fmt.Errorf(requiredText, TypeShell, "commands")
			}
		case TypeMerge:
			if sE(b.Target) {
				return fmt.Errorf(requiredText, TypeMerge, "target")
			}
			if sE(b.Destination) {
				return fmt.Errorf(requiredText, TypeMerge, "destination")
			}
		case TypeCopy:
			if sE(b.Target) {
				return fmt.Errorf(requiredText, TypeCopy, "target")
			}
			if sE(b.Destination) {
				return fmt.Errorf(requiredText, TypeCopy, "destination")
			}
		case TypeMove:
			if sE(b.Target) {
				return fmt.Errorf(requiredText, TypeMove, "target")
			}
			if sE(b.Destination) {
				return fmt.Errorf(requiredText, TypeMove, "destination")
			}
		case TypeRemove:
			if aE(b.GetTargets()) {
				return fmt.Errorf(requiredTextOr, TypeRemove, "target", "targets")
			}
		case TypeSystemd:
			if aE(b.GetTargets()) {
				return fmt.Errorf(requiredTextOr, TypeSystemd, "target", "targets")
			}
		case TypeLink:
			if sE(b.Target) {
				return fmt.Errorf(requiredText, TypeLink, "target")
			}
			if sE(b.Destination) {
				return fmt.Errorf(requiredText, TypeLink, "destination")
			}
		case TypePackages:
		case TypeInclude:
			return errors.New(app.T_("Include should be extended"))
		default:
			return errors.New(app.T_("Unknown type: " + module.Type))
		}

		if !sE(b.Destination) {
			if !path.IsAbs(b.Destination) {
				return errors.New(app.T_(""))
			}
		}
	}

	return nil
}

// CheckAndFix проверяет и разворачивает include'ы
func (cfg *Config) CheckAndFix() error {
	if err := cfg.extendIncludes(); err != nil {
		return err
	}
	if err := cfg.fix(); err != nil {
		return err
	}

	return nil
}

// Save проверяет и разворачивает include'ы, затем сохраняет конфигурацию
func (cfg *Config) Save(filename string) error {
	var err error
	if err = cfg.CheckAndFix(); err != nil {
		return err
	}

	var data []byte
	if data, err = yaml.Marshal(cfg); err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

type Module struct {
	// Имя модуля для логирования
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Тип тела модуля
	Type string `yaml:"type" json:"type"`

	// Тело модуля
	Body Body `yaml:"body" json:"body"`
}

type Body struct {
	// Типы: git, shell
	// Команды для выполнения относительно директории ресурсов
	Commands []string `yaml:"commands,omitempty" json:"commands,omitempty"`

	// Типы: [git]
	// Зависимости для модуля. Они будут удалены после завершения модуля
	Deps []string `yaml:"deps,omitempty" json:"deps,omitempty"`

	// Типы: merge, include, copy, move, remove, systemd, link
	// Цель для использования в типе
	// Относительный путь к /var/apm/resources в merge, include, copy
	// Абсолютный путь в remove
	// Имя сервиса в systemd
	Target string `yaml:"target,omitempty" json:"target,omitempty"`

	// Типы: include, remove, systemd
	// Цели для использования в типе
	// Относительные пути к /var/apm/resources в include
	// Абсолютные пути в remove
	// Имена сервисов в systemd
	Targets []string `yaml:"targets,omitempty" json:"targets,omitempty"`

	// Типы: copy, move, merge, link
	// Директория назначения
	Destination string `yaml:"destination,omitempty" json:"destination,omitempty"`

	// Типы: packages
	// Пакеты для установки из repos/tasks
	Install []string `yaml:"install,omitempty" json:"install,omitempty"`

	// Типы: packages
	// Пакеты для удаления из образа
	Remove []string `yaml:"remove,omitempty" json:"remove,omitempty"`

	// Типы: [copy], [move]
	// Заменить назначение, если оно существует
	Replace bool `yaml:"replace,omitempty" json:"replace,omitempty"`

	// Типы: [move]
	// Создать ссылку из родительской директории цели на назначение
	CreateLink bool `yaml:"create-link,omitempty" json:"create-link,omitempty"`

	// Типы: systemd
	// Включить или отключить systemd сервис
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// Типы: git
	// URL git-репозитория
	Url string `yaml:"url,omitempty" json:"url,omitempty"`
}

// GetTargets возвращает все цели (target и targets)
func (b *Body) GetTargets() []string {
	var targets []string

	if !sE(b.Target) {
		targets = append(targets, b.Target)
	}
	if !aE(b.Targets) {
		targets = append(targets, b.Targets...)
	}

	return targets
}

// ReadAndParseYamlFile читает и парсит YAML файл, include'ы будут развернуты
func ReadAndParseYamlFile(name string) (cfg Config, err error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return cfg, err
	}
	return ParseYamlData(data)
}

// ParseYamlData парсит YAML данные, include'ы будут развернуты
func ParseYamlData(data []byte) (cfg Config, err error) {
	return parseData(data, true, true)
}

// ParseJsonData парсит JSON данные, include'ы вернут ошибку
func ParseJsonData(data []byte) (cfg Config, err error) {
	return parseData(data, false, true)
}

func parseData(data []byte, isYaml bool, fix bool) (cfg Config, err error) {
	if isYaml {
		err = yaml.Unmarshal(data, &cfg)
	} else {
		err = json.Unmarshal(data, &cfg)
	}

	if err != nil {
		return cfg, err
	}
	if fix {
		err = cfg.CheckAndFix()
		if err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

func removeByValue(arr []string, value string) []string {
	return slices.DeleteFunc(arr, func(s string) bool {
		return s == value
	})
}

// sE проверяет, пуста ли строка
func sE(s string) bool {
	return s == ""
}

// aE проверяет, пуст ли массив
func aE(a []string) bool {
	return len(a) == 0
}

// syncYamlMutex защищает операции работы с файлом.
var syncYamlMutex sync.Mutex

// LoadConfig загружает конфигурацию из файла и сохраняет в поле config.
func (s *HostConfigService) LoadConfig() error {
	var cfg Config
	var err error

	_, err = os.Stat(s.pathImageFile)
	if os.IsNotExist(err) {
		if cfg, err = s.hostImageService.GenerateDefaultConfig(); err != nil {
			return err
		}
		s.Config = &cfg
		return s.SaveConfig()
	} else {
		if cfg, err = ReadAndParseYamlFile(s.pathImageFile); err != nil {
			return err
		}
		s.Config = &cfg
		return nil
	}
}

// SaveConfig сохраняет текущую конфигурацию сервиса в файл.
func (s *HostConfigService) SaveConfig() error {
	if s.Config == nil {
		return errors.New(app.T_("Configuration not loaded"))
	}
	if s.Config.HasInclude() {
		return errors.New(app.T_("Saving config with 'include' module type not supported"))
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
