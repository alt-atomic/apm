package testutil

import (
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	"apm/internal/common/version"
	"errors"
	"testing"
)

// MockConfigManager реализует app.Manager для тестов.
type MockConfigManager struct {
	Config *app.Configuration
}

func (m *MockConfigManager) GetConfig() *app.Configuration         { return m.Config }
func (m *MockConfigManager) GetColors() app.Colors                 { return app.Colors{} }
func (m *MockConfigManager) SaveConfig(_ *app.Configuration) error { return nil }
func (m *MockConfigManager) GetConfigPath() string                 { return "" }
func (m *MockConfigManager) GetParsedVersion() *version.Version    { return nil }
func (m *MockConfigManager) IsDevMode() bool                       { return false }
func (m *MockConfigManager) SetFormat(_ string)                    {}
func (m *MockConfigManager) SetFormatType(_ string)                {}
func (m *MockConfigManager) SetFields(_ []string)                  {}
func (m *MockConfigManager) EnableVerbose()                        {}
func (m *MockConfigManager) GetTemporaryImageFile() string         { return "" }
func (m *MockConfigManager) GetPathImageContainerFile() string     { return "" }
func (m *MockConfigManager) GetPathImageFile() string              { return "" }
func (m *MockConfigManager) GetResourcesDir() string               { return "" }

// DefaultAppConfig возвращает app.Config с текстовым форматом вывода.
func DefaultAppConfig() *app.Config {
	return &app.Config{
		ConfigManager: &MockConfigManager{
			Config: &app.Configuration{Format: app.FormatText},
		},
	}
}

// JsonAppConfig возвращает app.Config с JSON форматом вывода.
func JsonAppConfig() *app.Config {
	return &app.Config{
		ConfigManager: &MockConfigManager{
			Config: &app.Configuration{Format: app.FormatJSON},
		},
	}
}

// AssertAPMError проверяет, что ошибка является APMError с ожидаемым типом.
func AssertAPMError(t *testing.T, err error, expectedType string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apmErr apmerr.APMError
	if !errors.As(err, &apmErr) {
		t.Fatalf("expected APMError, got %T: %v", err, err)
	}
	if apmErr.Type != expectedType {
		t.Errorf("expected error type %s, got %s: %v", expectedType, apmErr.Type, err)
	}
}
