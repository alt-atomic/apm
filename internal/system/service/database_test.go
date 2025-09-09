package service

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDBHistory_fromDBModel(t *testing.T) {
	testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	configJSON := `{
		"image": "test-image:latest",
		"packages": {
			"install": ["package1", "package2"],
			"remove": ["old-package"]
		},
		"commands": ["echo hello", "apt update"]
	}`

	dbHistory := DBHistory{
		ImageName:  "test-image",
		ImageDate:  testTime,
		ConfigJSON: configJSON,
	}

	imageHistory, err := dbHistory.fromDBModel()
	if err != nil {
		t.Fatalf("fromDBModel failed: %v", err)
	}

	if imageHistory.ImageName != "test-image" {
		t.Errorf("Expected ImageName 'test-image', got %s", imageHistory.ImageName)
	}

	expectedDate := testTime.Format(time.RFC3339)
	if imageHistory.ImageDate != expectedDate {
		t.Errorf("Expected ImageDate '%s', got %s", expectedDate, imageHistory.ImageDate)
	}

	if imageHistory.Config == nil {
		t.Error("Config should not be nil")
		return
	}

	if imageHistory.Config.Image != "test-image:latest" {
		t.Errorf("Expected Config.Image 'test-image:latest', got %s", imageHistory.Config.Image)
	}

	if len(imageHistory.Config.Packages.Install) != 2 {
		t.Errorf("Expected 2 install packages, got %d", len(imageHistory.Config.Packages.Install))
	}

	if len(imageHistory.Config.Commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(imageHistory.Config.Commands))
	}
}

func TestDBHistory_fromDBModel_InvalidJSON(t *testing.T) {
	dbHistory := DBHistory{
		ImageName:  "test-image",
		ImageDate:  time.Now(),
		ConfigJSON: `invalid json`,
	}

	_, err := dbHistory.fromDBModel()
	if err == nil {
		t.Error("fromDBModel should fail with invalid JSON")
	}
}

func TestImageHistory_JSONSerialization(t *testing.T) {
	config := &Config{
		Image: "test-image:latest",
		Packages: struct {
			Install []string `yaml:"install" json:"install"`
			Remove  []string `yaml:"remove" json:"remove"`
		}{
			Install: []string{"pkg1"},
			Remove:  []string{},
		},
		Commands: []string{"echo test"},
	}

	original := ImageHistory{
		ImageName: "test-image",
		Config:    config,
		ImageDate: "2023-01-01T00:00:00Z",
	}

	// Сериализуем в JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	// Десериализуем обратно
	var restored ImageHistory
	err = json.Unmarshal(jsonData, &restored)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Проверяем, что данные сохранились
	if restored.ImageName != original.ImageName {
		t.Errorf("ImageName mismatch: expected %s, got %s", original.ImageName, restored.ImageName)
	}

	if restored.ImageDate != original.ImageDate {
		t.Errorf("ImageDate mismatch: expected %s, got %s", original.ImageDate, restored.ImageDate)
	}

	if restored.Config.Image != original.Config.Image {
		t.Errorf("Config.Image mismatch: expected %s, got %s", original.Config.Image, restored.Config.Image)
	}
}
