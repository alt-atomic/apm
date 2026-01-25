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

package app

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ilyakaznacheev/cleanenv"
)

// Manager управляет конфигурацией приложения
type Manager interface {
	GetConfig() *Configuration
	GetColors() Colors
	IsDevMode() bool
	SetFormat(format string)
	GetTemporaryImageFile() string
	GetPathImageContainerFile() string
	GetPathImageFile() string
	GetResourcesDir() string
}

// BuildInfo информация интегрированная через сборку meson
type BuildInfo struct {
	CommandPrefix     string
	Environment       string
	PathLocales       string
	PathDBSQLSystem   string
	PathContainerFile string
	PathImageFile     string
	PathResourcesDir  string
	Version           string
}

// Colors конфигурация цветовой схемы
type Colors struct {
	Enumerator     string `yaml:"enumerator"`
	Accent         string `yaml:"accent"`
	ItemLight      string `yaml:"itemLight"`
	ItemDark       string `yaml:"itemDark"`
	Success        string `yaml:"success"`
	Error          string `yaml:"error"`
	Delete         string `yaml:"delete"`
	Install        string `yaml:"install"`
	Shortcut       string `yaml:"shortcut"`
	ScrollBar      string `yaml:"scrollBar"`
	DialogKeyLight string `yaml:"dialogKeyLight"`
	DialogKeyDark  string `yaml:"dialogKeyDark"`
	ProgressStart  string `yaml:"progressStart"`
	ProgressEnd    string `yaml:"progressEnd"`
}

// Константы форматов вывода
const (
	FormatText = "text" // CLI текстовый вывод
	FormatJSON = "json" // CLI JSON вывод
	FormatDBus = "dbus" // D-Bus сервис
	FormatHTTP = "http" // HTTP сервер с WebSocket
)

// Configuration основная конфигурация приложения
type Configuration struct {
	CommandPrefix   string `yaml:"commandPrefix"`
	Environment     string `yaml:"environment"`
	PathDBSQLSystem string `yaml:"pathDBSQLSystem"`
	PathDBSQLUser   string `yaml:"pathDBSQLUser"`
	PathDBKV        string `yaml:"pathDBKV"`
	PathLocales     string `yaml:"pathLocales"`
	Colors          Colors `yaml:"colors"`

	PathContainerFile string `yaml:"-"`
	PathImageFile     string `yaml:"-"`
	PathResourcesDir  string `yaml:"-"`
	Version           string `yaml:"-"`

	// Runtime flags
	ExistStplr     bool   `yaml:"-"`
	ExistDistrobox bool   `yaml:"-"`
	Format         string `yaml:"-"`
	IsAtomic       bool   `yaml:"-"`
	DevMode        bool   `yaml:"-"`
}

// configManagerImpl реализация Manager
type configManagerImpl struct {
	config *Configuration
}

// NewConfigManager создает новый менеджер конфигурации
func NewConfigManager(buildInfo BuildInfo) (Manager, error) {
	cfg := &Configuration{
		Colors: getDefaultColors(),
	}

	cm := &configManagerImpl{
		config: cfg,
	}

	if err := cm.loadConfiguration(buildInfo); err != nil {
		return nil, err
	}

	return cm, nil
}

// loadConfiguration загружает конфигурацию из файлов и build-time переменных
func (cm *configManagerImpl) loadConfiguration(buildInfo BuildInfo) error {
	cm.applyBuildInfo(buildInfo)

	cm.config.PathDBSQLUser = "~/.cache/apm/apm.db"
	cm.config.PathDBKV = "~/.cache/apm/pogreb"

	// Устанавливаем дефолт для системной БД если не задан через build, тесты будут использовать этот путь
	if cm.config.PathDBSQLSystem == "" {
		cm.config.PathDBSQLSystem = filepath.Join(os.TempDir(), "apm-system.db")
	}
	if cm.config.PathResourcesDir == "" {
		cm.config.PathResourcesDir = filepath.Join(os.TempDir(), "apm-resources")
	}

	if err := cm.loadConfigFile(); err != nil {
		return err
	}

	// Определяем режим разработки
	cm.config.DevMode = cm.config.Environment != "prod"

	cm.expandPaths()

	if err := cm.ensureDirectories(); err != nil {
		return err
	}

	cm.detectSystemCapabilities()

	return nil
}

// applyBuildInfo применяет параметры времени сборки
func (cm *configManagerImpl) applyBuildInfo(buildInfo BuildInfo) {
	if buildInfo.CommandPrefix != "" {
		cm.config.CommandPrefix = buildInfo.CommandPrefix
	}
	if buildInfo.Environment != "" {
		cm.config.Environment = buildInfo.Environment
	}
	if buildInfo.PathLocales != "" {
		cm.config.PathLocales = buildInfo.PathLocales
	}
	if buildInfo.PathDBSQLSystem != "" {
		cm.config.PathDBSQLSystem = buildInfo.PathDBSQLSystem
	}
	if buildInfo.PathContainerFile != "" {
		cm.config.PathContainerFile = buildInfo.PathContainerFile
	}
	if buildInfo.PathImageFile != "" {
		cm.config.PathImageFile = buildInfo.PathImageFile
	}
	if buildInfo.PathResourcesDir != "" {
		cm.config.PathResourcesDir = buildInfo.PathResourcesDir
	}
	if buildInfo.Version != "" {
		cm.config.Version = buildInfo.Version
	}
}

// loadConfigFile загружает конфигурацию из YAML файлов
func (cm *configManagerImpl) loadConfigFile() error {
	var configPath string

	if _, err := os.Stat("config.yml"); err == nil {
		configPath = "config.yml"
	} else if _, err = os.Stat("/etc/apm/config.yml"); err == nil {
		configPath = "/etc/apm/config.yml"
	}

	if configPath != "" {
		if err := cleanenv.ReadConfig(configPath, cm.config); err != nil {
			Log.Warning("Failed to read config file: ", err)
		}
	}

	return nil
}

// expandPaths расширяет пути с ~ и переменными окружения
func (cm *configManagerImpl) expandPaths() {
	cm.config.PathDBSQLUser = filepath.Clean(expandUser(cm.config.PathDBSQLUser))
	cm.config.PathDBSQLSystem = filepath.Clean(expandUser(cm.config.PathDBSQLSystem))
	cm.config.PathDBKV = filepath.Clean(expandUser(cm.config.PathDBKV))
}

// ensureDirectories создает необходимые директории
func (cm *configManagerImpl) ensureDirectories() error {
	if syscall.Geteuid() != 0 {
		if err := EnsureDir(cm.config.PathDBKV); err != nil {
			return err
		}
		if err := EnsurePath(cm.config.PathDBSQLUser); err != nil {
			return err
		}
	} else {
		if err := EnsurePath(cm.config.PathDBSQLSystem); err != nil {
			return err
		}
		if err := EnsureDir(cm.config.PathResourcesDir); err != nil {
			return err
		}
	}

	return nil
}

// detectSystemCapabilities определяет доступные системные утилиты
func (cm *configManagerImpl) detectSystemCapabilities() {
	cm.config.IsAtomic = fileExists("/usr/bin/bootc")
	cm.config.ExistStplr = fileExists("/usr/bin/stplr")
	cm.config.ExistDistrobox = fileExists("/usr/bin/distrobox")
}

// GetConfig возвращает конфигурацию
func (cm *configManagerImpl) GetConfig() *Configuration {
	return cm.config
}

// GetColors возвращает цветовую схему
func (cm *configManagerImpl) GetColors() Colors {
	return cm.config.Colors
}

// IsDevMode возвращает флаг режима разработки
func (cm *configManagerImpl) IsDevMode() bool {
	return cm.config.DevMode
}

// GetTemporaryImageFile возвращает временный файл для сохранения действия с атомарным образом
func (cm *configManagerImpl) GetTemporaryImageFile() string {
	return filepath.Join(os.TempDir(), "apm.tmp")
}

// GetPathImageFile возвращает путь к файлу конфигурации образа
func (cm *configManagerImpl) GetPathImageFile() string {
	return cm.config.PathImageFile
}

// GetResourcesDir возвращает путь к файлу конфигурации образа
func (cm *configManagerImpl) GetResourcesDir() string {
	return cm.config.PathResourcesDir
}

// GetPathImageContainerFile возвращает путь к файлу для сборки контейнера
func (cm *configManagerImpl) GetPathImageContainerFile() string {
	return cm.config.PathContainerFile
}

// SetFormat устанавливает формат вывода
func (cm *configManagerImpl) SetFormat(format string) {
	cm.config.Format = format
}

// getDefaultColors возвращает цветовую схему по умолчанию
func getDefaultColors() Colors {
	return Colors{
		Enumerator:     "#c4c8c6",
		Accent:         "#a2734c",
		ItemLight:      "#171717",
		ItemDark:       "#c4c8c6",
		Success:        "2",
		Error:          "9",
		Delete:         "#a81c1f",
		Install:        "#26a269",
		Shortcut:       "#888888",
		ScrollBar:      "#ff0000",
		DialogKeyLight: "#234f55",
		DialogKeyDark:  "#82a0a3",
		ProgressStart:  "#c4c8c6",
		ProgressEnd:    "#26a269",
	}
}

// EnsurePath создает файл и его директорию при необходимости
func EnsurePath(path string) error {
	dir := filepath.Dir(path)

	if err := EnsureDir(dir); err != nil {
		return err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		return file.Close()
	}

	return nil
}

// EnsureDir создает директорию при необходимости
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0777)
}

// expandUser расширяет ~ в начале пути
func expandUser(s string) string {
	if strings.HasPrefix(s, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return s
		}
		return filepath.Join(homeDir, s[2:])
	}
	return s
}

// fileExists проверяет существование файла
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
