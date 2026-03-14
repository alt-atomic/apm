package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTemporaryConfigService_generateDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	tempFile := filepath.Join(tmpDir, "test_temp_config.yaml")

	service := NewTemporaryConfigService(tempFile)

	config, err := service.generateDefaultConfig()
	if err != nil {
		t.Fatalf("generateDefaultConfig failed: %v", err)
	}

	if config.Packages.Install == nil {
		t.Error("Install packages should not be nil")
	}

	if config.Packages.Remove == nil {
		t.Error("Remove packages should not be nil")
	}

	if len(config.Packages.Install) != 0 {
		t.Error("Install packages should be empty by default")
	}

	if len(config.Packages.Remove) != 0 {
		t.Error("Remove packages should be empty by default")
	}

	if _, err = os.Stat(tempFile); os.IsNotExist(err) {
		t.Error("Config file should be created")
	}
}

func TestTemporaryConfigService_LoadConfig_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	tempFile := filepath.Join(tmpDir, "test_temp_config.yaml")

	service := NewTemporaryConfigService(tempFile)

	err := service.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if service.Config == nil {
		t.Error("Config should not be nil after LoadConfig")
	}

	if len(service.Config.Packages.Install) != 0 {
		t.Error("Install packages should be empty by default")
	}
}

func TestTemporaryConfigService_SaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	tempFile := filepath.Join(tmpDir, "test_temp_config.yaml")

	service := NewTemporaryConfigService(tempFile)
	service.Config = &TemporaryConfig{}
	service.Config.Packages.Install = []string{"test-package"}
	service.Config.Packages.Remove = []string{"remove-package"}

	err := service.SaveConfig()
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Создаем новый сервис и загружаем конфиг
	newService := NewTemporaryConfigService(tempFile)
	err = newService.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(newService.Config.Packages.Install) != 1 || newService.Config.Packages.Install[0] != "test-package" {
		t.Error("Install packages not saved correctly")
	}

	if len(newService.Config.Packages.Remove) != 1 || newService.Config.Packages.Remove[0] != "remove-package" {
		t.Error("Remove packages not saved correctly")
	}
}

func TestTemporaryConfigService_AddInstallPackage(t *testing.T) {
	tmpDir := t.TempDir()
	tempFile := filepath.Join(tmpDir, "test_temp_config.yaml")

	service := NewTemporaryConfigService(tempFile)
	err := service.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Добавляем пакет для установки
	err = service.AddInstallPackage("test-package")
	if err != nil {
		t.Fatalf("AddInstallPackage failed: %v", err)
	}

	if !service.IsInstalled("test-package") {
		t.Error("Package should be in install list")
	}

	// Проверяем дублирование
	err = service.AddInstallPackage("test-package")
	if err != nil {
		t.Fatalf("AddInstallPackage failed on duplicate: %v", err)
	}

	if len(service.Config.Packages.Install) != 1 {
		t.Error("Should not duplicate packages")
	}
}

func TestTemporaryConfigService_AddRemovePackage(t *testing.T) {
	tmpDir := t.TempDir()
	tempFile := filepath.Join(tmpDir, "test_temp_config.yaml")

	service := NewTemporaryConfigService(tempFile)
	err := service.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Добавляем пакет для удаления
	err = service.AddRemovePackage("test-package")
	if err != nil {
		t.Fatalf("AddRemovePackage failed: %v", err)
	}

	if !service.IsRemoved("test-package") {
		t.Error("Package should be in remove list")
	}

	// Проверяем дублирование
	err = service.AddRemovePackage("test-package")
	if err != nil {
		t.Fatalf("AddRemovePackage failed on duplicate: %v", err)
	}

	if len(service.Config.Packages.Remove) != 1 {
		t.Error("Should not duplicate packages")
	}
}

func TestTemporaryConfigService_PackageConflictResolution(t *testing.T) {
	tmpDir := t.TempDir()
	tempFile := filepath.Join(tmpDir, "test_temp_config.yaml")

	service := NewTemporaryConfigService(tempFile)
	err := service.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Добавляем пакет для установки
	err = service.AddInstallPackage("test-package")
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

	// Проверяем обратную операцию
	err = service.AddInstallPackage("test-package")
	if err != nil {
		t.Fatalf("AddInstallPackage failed: %v", err)
	}

	if service.IsRemoved("test-package") {
		t.Error("Package should not be in remove list after moving to install")
	}

	if !service.IsInstalled("test-package") {
		t.Error("Package should be in install list")
	}
}

func TestTemporaryConfigService_DeleteFile(t *testing.T) {
	tmpDir := t.TempDir()
	tempFile := filepath.Join(tmpDir, "test_temp_config.yaml")

	service := NewTemporaryConfigService(tempFile)
	err := service.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Проверяем, что файл существует
	if _, err = os.Stat(tempFile); os.IsNotExist(err) {
		t.Error("Config file should exist")
	}

	// Удаляем файл
	err = service.DeleteFile()
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	// Проверяем, что файл удален
	if _, err = os.Stat(tempFile); !os.IsNotExist(err) {
		t.Error("Config file should be deleted")
	}

	// Повторное удаление должно работать без ошибок
	err = service.DeleteFile()
	if err != nil {
		t.Fatalf("DeleteFile should not fail on non-existent file: %v", err)
	}
}

func TestTemporaryConfigService_SaveConfig_NilConfig(t *testing.T) {
	tmpDir := t.TempDir()
	tempFile := filepath.Join(tmpDir, "test_temp_config.yaml")

	service := NewTemporaryConfigService(tempFile)
	service.Config = nil

	err := service.SaveConfig()
	if err == nil {
		t.Error("SaveConfig should fail with nil config")
	}
}
