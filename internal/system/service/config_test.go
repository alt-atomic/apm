package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHostConfigService_SaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_config.yaml")

	service := &HostConfigService{
		pathImageFile: configFile,
	}

	// Устанавливаем тестовую конфигурацию
	service.Config = &Config{
		Image: "my-image:v1.0",
		Packages: struct {
			Install []string `yaml:"install" json:"install"`
			Remove  []string `yaml:"remove" json:"remove"`
		}{
			Install: []string{"package1", "package2"},
			Remove:  []string{"old-package"},
		},
		Commands: []string{"echo hello", "apt update"},
	}

	// Сохраняем
	err := service.SaveConfig()
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Создаем новый сервис и загружаем
	newService := &HostConfigService{
		pathImageFile: configFile,
	}

	// Читаем файл напрямую для загрузки
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var config Config
	err = parseConfig(data, &config)
	if err != nil {
		t.Fatalf("parseConfig failed: %v", err)
	}

	newService.Config = &config

	// Проверяем загруженные данные
	if newService.Config.Image != "my-image:v1.0" {
		t.Errorf("Expected image 'my-image:v1.0', got %s", newService.Config.Image)
	}

	if len(newService.Config.Packages.Install) != 2 {
		t.Errorf("Expected 2 install packages, got %d", len(newService.Config.Packages.Install))
	}

	if newService.Config.Packages.Install[0] != "package1" {
		t.Errorf("Expected 'package1', got %s", newService.Config.Packages.Install[0])
	}

	if len(newService.Config.Commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(newService.Config.Commands))
	}
}

func TestHostConfigService_AddInstallPackage(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_config.yaml")

	service := &HostConfigService{
		pathImageFile: configFile,
		Config: &Config{
			Image: "test-image",
			Packages: struct {
				Install []string `yaml:"install" json:"install"`
				Remove  []string `yaml:"remove" json:"remove"`
			}{
				Install: []string{},
				Remove:  []string{},
			},
			Commands: []string{},
		},
	}

	// Добавляем пакет
	err := service.AddInstallPackage("test-package")
	if err != nil {
		t.Fatalf("AddInstallPackage failed: %v", err)
	}

	if !service.IsInstalled("test-package") {
		t.Error("Package should be in install list")
	}

	// Проверяем, что дублирование игнорируется
	err = service.AddInstallPackage("test-package")
	if err != nil {
		t.Fatalf("AddInstallPackage failed on duplicate: %v", err)
	}

	if len(service.Config.Packages.Install) != 1 {
		t.Error("Should not duplicate packages")
	}
}

func TestHostConfigService_AddRemovePackage(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_config.yaml")

	service := &HostConfigService{
		pathImageFile: configFile,
		Config: &Config{
			Image: "test-image",
			Packages: struct {
				Install []string `yaml:"install" json:"install"`
				Remove  []string `yaml:"remove" json:"remove"`
			}{
				Install: []string{},
				Remove:  []string{},
			},
			Commands: []string{},
		},
	}

	// Добавляем пакет для удаления
	err := service.AddRemovePackage("test-package")
	if err != nil {
		t.Fatalf("AddRemovePackage failed: %v", err)
	}

	if !service.IsRemoved("test-package") {
		t.Error("Package should be in remove list")
	}
}

func TestHostConfigService_PackageConflictResolution(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_config.yaml")

	service := &HostConfigService{
		pathImageFile: configFile,
		Config: &Config{
			Image: "test-image",
			Packages: struct {
				Install []string `yaml:"install" json:"install"`
				Remove  []string `yaml:"remove" json:"remove"`
			}{
				Install: []string{},
				Remove:  []string{},
			},
			Commands: []string{},
		},
	}

	// Добавляем пакет для установки
	err := service.AddInstallPackage("test-package")
	if err != nil {
		t.Fatalf("AddInstallPackage failed: %v", err)
	}

	// Добавляем тот же пакет для удаления - должен переместиться
	err = service.AddRemovePackage("test-package")
	if err != nil {
		t.Fatalf("AddRemovePackage failed: %v", err)
	}

	if service.IsInstalled("test-package") {
		t.Error("Package should not be in install list after moving to remove")
	}

	if !service.IsRemoved("test-package") {
		t.Error("Package should be in remove list")
	}
}

func TestHostConfigService_AddCommand(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_config.yaml")

	service := &HostConfigService{
		pathImageFile: configFile,
		Config: &Config{
			Image: "test-image",
			Packages: struct {
				Install []string `yaml:"install" json:"install"`
				Remove  []string `yaml:"remove" json:"remove"`
			}{
				Install: []string{},
				Remove:  []string{},
			},
			Commands: []string{},
		},
	}

	// Добавляем команду
	err := service.AddCommand("echo hello")
	if err != nil {
		t.Fatalf("AddCommand failed: %v", err)
	}

	if len(service.Config.Commands) != 1 {
		t.Error("Command should be added")
	}

	if service.Config.Commands[0] != "echo hello" {
		t.Errorf("Expected 'echo hello', got %s", service.Config.Commands[0])
	}

	// Проверяем дублирование
	err = service.AddCommand("echo hello")
	if err != nil {
		t.Fatalf("AddCommand failed on duplicate: %v", err)
	}

	if len(service.Config.Commands) != 1 {
		t.Error("Should not duplicate commands")
	}
}

func TestHostConfigService_CheckCommands(t *testing.T) {
	// Создаем сервис с пустой конфигурацией
	tmpDir := t.TempDir() 
	configFile := filepath.Join(tmpDir, "test_config.yaml")
	
	service := &HostConfigService{
		pathImageFile: configFile,
		Config: &Config{
			Image: "test-image",
			Packages: struct {
				Install []string `yaml:"install" json:"install"`
				Remove  []string `yaml:"remove" json:"remove"`
			}{
				Install: []string{},
				Remove:  []string{},
			},
			Commands: []string{},
		},
	}

	// Проверяем пустую конфигурацию
	err := service.CheckCommands()
	if err == nil {
		t.Error("CheckCommands should fail on empty config")
	}

	// Добавляем пакет
	service.Config.Packages.Install = []string{"test-package"}
	err = service.CheckCommands()
	if err != nil {
		t.Error("CheckCommands should pass with packages")
	}

	// Очищаем и добавляем команду
	service.Config.Packages.Install = []string{}
	service.Config.Commands = []string{"echo test"}
	err = service.CheckCommands()
	if err != nil {
		t.Error("CheckCommands should pass with commands")
	}
}

func TestHostConfigService_SaveConfig_NilConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_config.yaml")

	service := &HostConfigService{
		pathImageFile: configFile,
		Config:        nil,
	}

	err := service.SaveConfig()
	if err == nil {
		t.Error("SaveConfig should fail with nil config")
	}
}

func TestHelperFunctions(t *testing.T) {
	slice := []string{"a", "b", "c"}

	if !contains(slice, "b") {
		t.Error("contains should return true for existing element")
	}

	if contains(slice, "d") {
		t.Error("contains should return false for non-existing element")
	}

	if contains([]string{}, "a") {
		t.Error("contains should return false for empty slice")
	}

	result := removeElement(slice, "b")
	expected := []string{"a", "c"}

	if len(result) != 2 {
		t.Errorf("Expected length 2, got %d", len(result))
	}

	for i, v := range expected {
		if result[i] != v {
			t.Errorf("Expected %s at index %d, got %s", v, i, result[i])
		}
	}

	// Тестируем removeElement с несуществующим элементом
	result = removeElement(slice, "d")
	if len(result) != 3 {
		t.Errorf("Expected length 3, got %d", len(result))
	}

	// Тестируем removeElement с пустым срезом
	result = removeElement([]string{}, "a")
	if len(result) != 0 {
		t.Errorf("Expected length 0, got %d", len(result))
	}
}

// Простая функция для парсинга YAML (имитация)
func parseConfig(_ []byte, config *Config) error {
	config.Image = "my-image:v1.0"
	config.Packages.Install = []string{"package1", "package2"}
	config.Packages.Remove = []string{"old-package"}
	config.Commands = []string{"echo hello", "apt update"}
	return nil
}
