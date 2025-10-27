package service

import (
	"apm/internal/common/build"
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
	service.Config = &build.Config{
		Image: "my-image:v1.0",
		Modules: []build.Module{
			build.Module{
				Type: build.TypePackages,
				Body: build.Body{
					Install: []string{"package1", "package2"},
					Remove:  []string{"old-package"},
				},
			},
			build.Module{
				Type: build.TypeShell,
				Body: build.Body{
					Commands: "echo hello\napt update",
				},
			},
		},
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

	var config build.Config
	config, err = build.ReadAndParseYamlFile(configFile)
	if err != nil {
		t.Fatalf("ReadAndParseYamlFile failed: %v", err)
	}

	newService.Config = &config

	// Проверяем загруженные данные
	if newService.Config.Image != "my-image:v1.0" {
		t.Errorf("Expected image 'my-image:v1.0', got %s", newService.Config.Image)
	}

	if len(newService.Config.Modules[0].Body.Install) != 2 {
		t.Errorf("Expected 2 install packages, got %d", len(newService.Config.Modules[0].Body.Install))
	}

	if newService.Config.Modules[0].Body.Install[0] != "package1" {
		t.Errorf("Expected 'package1', got %s", newService.Config.Modules[0].Body.Install[0])
	}

	if len(newService.Config.Modules[1].Body.Commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(newService.Config.Modules[1].Body.Commands))
	}
}

func TestHostConfigService_AddInstallPackage(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_config.yaml")

	service := &HostConfigService{
		pathImageFile: configFile,
		Config: &build.Config{
			Image: "test-image",
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

	if len(service.Config.Modules[0].Body.Install) != 1 {
		t.Error("Should not duplicate packages")
	}
}

func TestHostConfigService_AddRemovePackage(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_config.yaml")

	service := &HostConfigService{
		pathImageFile: configFile,
		Config: &build.Config{
			Image: "test-image",
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
		Config: &build.Config{
			Image: "test-image",
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
