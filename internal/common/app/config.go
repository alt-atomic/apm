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
	"apm/internal/common/version"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	goyaml "github.com/goccy/go-yaml"
	"github.com/ilyakaznacheev/cleanenv"
)

// Manager управляет конфигурацией приложения
type Manager interface {
	GetConfig() *Configuration
	GetColors() Colors
	SaveConfig(config *Configuration) error
	GetConfigPath() string
	GetParsedVersion() *version.Version
	IsDevMode() bool
	SetFormat(format string)
	SetFormatType(formatType string)
	SetFields(fields []string)
	EnableVerbose()
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
	Accent    string `yaml:"accent"`
	TextLight string `yaml:"textLight"`
	TextDark  string `yaml:"textDark"`

	TreeBranch    string `yaml:"treeBranch"`
	ResultSuccess string `yaml:"resultSuccess"`
	ResultError   string `yaml:"resultError"`

	DialogAction     string `yaml:"dialogAction"`
	DialogDanger     string `yaml:"dialogDanger"`
	DialogHint       string `yaml:"dialogHint"`
	DialogScroll     string `yaml:"dialogScroll"`
	DialogLabelLight string `yaml:"dialogLabelLight"`
	DialogLabelDark  string `yaml:"dialogLabelDark"`

	ProgressEmpty  string `yaml:"progressEmpty"`
	ProgressFilled string `yaml:"progressFilled"`
}

const defaultConfigPath = "/etc/apm/config.yml"

// Константы форматов вывода
const (
	FormatText = "text"
	FormatJSON = "json"
	FormatDBus = "dbus"
	FormatHTTP = "http"
)

// Константы типов отображения (в рамках FormatText)
const (
	FormatTypeTree  = "tree"
	FormatTypePlain = "plain"
)

// Configuration основная конфигурация приложения
type Configuration struct {
	CommandPrefix   string `yaml:"commandPrefix"`
	Environment     string `yaml:"environment"`
	PathDBSQLSystem string `yaml:"pathDBSQLSystem"`
	PathDBSQLUser   string `yaml:"pathDBSQLUser"`
	PathLocales     string `yaml:"pathLocales"`
	Colors          Colors `yaml:"colors"`
	FormatType      string `yaml:"formatType"`

	PathContainerFile string `yaml:"-"`
	PathImageFile     string `yaml:"-"`
	PathResourcesDir  string `yaml:"-"`
	Version           string `yaml:"-"`

	ParsedVersion *version.Version `yaml:"-"`

	// Runtime flags
	ExistStplr     bool     `yaml:"-"`
	ExistDistrobox bool     `yaml:"-"`
	Format         string   `yaml:"-"`
	Fields         []string `yaml:"-"`
	IsAtomic       bool     `yaml:"-"`
	DevMode        bool     `yaml:"-"`
	Verbose        bool     `yaml:"-"`
}

// configManagerImpl реализация Manager
type configManagerImpl struct {
	config     *Configuration
	configPath string
}

// NewConfigManager создает новый менеджер конфигурации
func NewConfigManager(buildInfo BuildInfo) (Manager, error) {
	cfg := &Configuration{
		Colors:     GetDefaultColors(),
		FormatType: FormatTypeTree,
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
	cm.parseVersion()

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
	} else if _, err = os.Stat(defaultConfigPath); err == nil {
		configPath = defaultConfigPath
	}

	if configPath != "" {
		if err := cleanenv.ReadConfig(configPath, cm.config); err != nil {
			Log.Warning("Failed to read config file: ", err)
		}
	}

	cm.configPath = configPath
	return nil
}

// expandPaths расширяет пути с ~ и переменными окружения
func (cm *configManagerImpl) expandPaths() {
	cm.config.PathDBSQLUser = filepath.Clean(expandUser(cm.config.PathDBSQLUser))
	cm.config.PathDBSQLSystem = filepath.Clean(expandUser(cm.config.PathDBSQLSystem))
}

// ensureDirectories создает необходимые директории
func (cm *configManagerImpl) ensureDirectories() error {
	if syscall.Geteuid() != 0 {
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

// parseVersion парсит версию из конфигурации, при ошибке использует "unknown"
func (cm *configManagerImpl) parseVersion() {
	ver, err := version.ParseVersion(cm.config.Version)
	if err != nil {
		Log.Warning("Failed to parse version: ", err)
		cm.config.ParsedVersion = &version.Version{Value: "unknown"}
	} else {
		cm.config.ParsedVersion = ver
	}
}

// GetParsedVersion возвращает версию
func (cm *configManagerImpl) GetParsedVersion() *version.Version {
	return cm.config.ParsedVersion
}

// GetConfig возвращает конфигурацию
func (cm *configManagerImpl) GetConfig() *Configuration {
	return cm.config
}

// GetColors возвращает цветовую схему
func (cm *configManagerImpl) GetColors() Colors {
	return cm.config.Colors
}

// SaveConfig сохраняет конфигурацию в файл и обновляет текущий конфиг
func (cm *configManagerImpl) SaveConfig(config *Configuration) error {
	configPath := cm.configPath
	if configPath == "" {
		configPath = defaultConfigPath
	}

	existing := make(map[string]interface{})
	data, err := os.ReadFile(configPath)
	if err == nil {
		_ = goyaml.Unmarshal(data, &existing)
	}

	configData, err := goyaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	var configMap map[string]interface{}
	if err = goyaml.Unmarshal(configData, &configMap); err != nil {
		return fmt.Errorf("failed to unmarshal config map: %w", err)
	}

	for k, v := range configMap {
		existing[k] = v
	}

	out, err := goyaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err = EnsurePath(configPath); err != nil {
		return fmt.Errorf("failed to ensure config path: %w", err)
	}

	if err = os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write config %s: %w", configPath, err)
	}

	cm.config.Colors = config.Colors
	cm.config.CommandPrefix = config.CommandPrefix
	cm.config.FormatType = config.FormatType
	cm.configPath = configPath
	return nil
}

// GetConfigPath возвращает путь к файлу конфигурации
func (cm *configManagerImpl) GetConfigPath() string {
	return cm.configPath
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

func (cm *configManagerImpl) SetFormatType(formatType string) {
	switch formatType {
	case FormatTypePlain:
		cm.config.FormatType = formatType
	default:
		cm.config.FormatType = FormatTypeTree
	}
}

func (cm *configManagerImpl) SetFields(fields []string) {
	cm.config.Fields = fields
}

func (cm *configManagerImpl) EnableVerbose() {
	cm.config.Verbose = true
	Log.EnableStdoutLogging()
}

// GetDefaultColors возвращает цветовую схему по умолчанию
func GetDefaultColors() Colors {
	return Colors{
		Accent:        "#a2734c",
		TextLight:     "#171717",
		TextDark:      "#c4c8c6",
		TreeBranch:    "#c4c8c6",
		ResultSuccess: "2",
		ResultError:   "9",

		DialogAction:     "#26a269",
		DialogDanger:     "#a81c1f",
		DialogHint:       "#888888",
		DialogScroll:     "#ff0000",
		DialogLabelLight: "#234f55",
		DialogLabelDark:  "#82a0a3",

		ProgressEmpty:  "#c4c8c6",
		ProgressFilled: "#26a269",
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
