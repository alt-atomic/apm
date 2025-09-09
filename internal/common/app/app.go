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
	"context"
	"database/sql"
	"fmt"

	"github.com/akrylysov/pogreb"
)

// Глобальные переменные для быстрого доступа к переводу и log
var (
	Log LoggerImpl
	T_  func(string) string
	TN_ func(string, string, int) string
)

// Инициализируем функции переводов и логирования автоматически при импорте модуля для тестов
func init() {
	if T_ == nil {
		T_ = func(s string) string { return s }
	}
	if TN_ == nil {
		TN_ = func(single string, plural string, count int) string {
			if count == 1 {
				return single
			}
			return plural
		}
	}
	if Log == nil {
		Log = &testLogger{}
	}
}

// testLogger простая реализация LoggerImpl для тестов
type testLogger struct{}

func (l *testLogger) Debug(...interface{})          {}
func (l *testLogger) Debugf(string, ...interface{}) {}
func (l *testLogger) Info(...interface{})           {}
func (l *testLogger) Warn(...interface{})           {}
func (l *testLogger) Error(...interface{})          {}
func (l *testLogger) Errorf(string, ...interface{}) {}
func (l *testLogger) Fatal(...interface{})          {}
func (l *testLogger) Warning(...interface{})        {}

// LoggerImpl интерфейс для логирования
type LoggerImpl interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
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
func GetAppConfig(ctx context.Context) *Config {
	if cfg, ok := ctx.Value(AppConfigKey).(*Config); ok {
		return cfg
	}
	panic("AppConfig not found in context")
}

// Config централизованный конфиг приложение
type Config struct {
	DatabaseManager DatabaseManager
	ConfigManager   Manager
	DBusManager     DBusManager
}

// NewAppConfig создает новое приложение
func NewAppConfig(dbManager DatabaseManager, configManager Manager, dbusManager DBusManager) *Config {
	return &Config{
		DatabaseManager: dbManager,
		ConfigManager:   configManager,
		DBusManager:     dbusManager,
	}
}

// InitializeAppDefault инициализация приложения с автоопределением
func InitializeAppDefault() (*Config, error) {
	buildInfo := GetBuildInfo()

	return InitializeApp(buildInfo)
}

// InitializeApp инициализирует полное приложение с конфигурацией
func InitializeApp(buildInfo BuildInfo) (*Config, error) {
	configManager, err := NewConfigManager(buildInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	logger := NewLogger(buildInfo.Environment != "prod")
	Log = logger

	config := configManager.GetConfig()
	translator := NewTranslator(config.PathLocales)
	T_ = translator.T_
	TN_ = translator.TN_

	dbManager := NewDatabaseManager(
		config.PathDBSQLSystem,
		config.PathDBSQLUser,
		config.PathDBKV,
	)

	dbusManager := NewDBusManager()
	appConfig := NewAppConfig(dbManager, configManager, dbusManager)

	return appConfig, nil
}
