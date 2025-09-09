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

package config

import (
	"apm/internal/common/binding/apt"
	aptLib "apm/internal/common/binding/apt/lib"
	"context"
	"database/sql"
	"fmt"

	"github.com/akrylysov/pogreb"
)

// Logger интерфейс для логирования
type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Warning(args ...interface{})
}

// DatabaseManager управляет подключениями к базам данных
type DatabaseManager interface {
	GetSystemDB() *sql.DB
	GetUserDB() *sql.DB
	GetKeyValueDB() *pogreb.DB
	Close() error
}

// Translator интерфейс для переводов
type Translator interface {
	T_(messageID string) string
	TN_(messageID string, pluralMessageID string, count int) string
}
type contextKey string

const AppConfigKey contextKey = "appConfig"

// GetAppConfig достать конфиг приложения из контекста
func GetAppConfig(ctx context.Context) *AppConfig {
	if cfg, ok := ctx.Value(AppConfigKey).(*AppConfig); ok {
		return cfg
	}
	panic("AppConfig not found in context")
}

// AppConfig централизованное приложение
type AppConfig struct {
	Logger          Logger
	DatabaseManager DatabaseManager
	Translator      Translator
	ConfigManager   Manager
	DBusManager     DBusManager
}

// NewAppConfig создает новое приложение
func NewAppConfig(logger Logger, dbManager DatabaseManager, translator Translator, configManager Manager, dbusManager DBusManager) *AppConfig {
	return &AppConfig{
		Logger:          logger,
		DatabaseManager: dbManager,
		Translator:      translator,
		ConfigManager:   configManager,
		DBusManager:     dbusManager,
	}
}

// InitializeAppDefault инициализация приложения с автоопределением
func InitializeAppDefault() (*AppConfig, error) {
	buildInfo := GetBuildInfo()

	return InitializeApp(buildInfo)
}

// InitializeApp инициализирует полное приложение с конфигурацией
func InitializeApp(buildInfo BuildInfo) (*AppConfig, error) {
	configManager, err := NewConfigManager(buildInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	config := configManager.GetConfig()
	logger := configManager.GetLogger()
	translator := NewTranslator(config.PathLocales, logger)
	dbManager := NewDatabaseManager(
		config.PathDBSQLSystem,
		config.PathDBSQLUser,
		config.PathDBKV,
		logger,
		translator,
	)

	// Создаем DBus менеджер
	dbusManager := NewDBusManager(logger, translator)

	appConfig := NewAppConfig(logger, dbManager, translator, configManager, dbusManager)

	return appConfig, nil
}

// CleanupApp корректно закрывает все ресурсы
func CleanupApp(config *AppConfig) error {
	if config == nil {
		return nil
	}

	aptLib.WaitIdle()
	defer apt.Close()

	var errors []error

	// Закрываем DBus соединение
	if config.DBusManager != nil {
		if err := config.DBusManager.Close(); err != nil {
			errors = append(errors, fmt.Errorf(config.Translator.T_("failed to close DBus: %w"), err))
		}
	}

	// Закрываем базы данных
	if config.DatabaseManager != nil {
		if err := config.DatabaseManager.Close(); err != nil {
			errors = append(errors, fmt.Errorf(config.Translator.T_("failed to close databases: %w"), err))
		}
	}

	// Возвращаем первую ошибку, если есть
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}
