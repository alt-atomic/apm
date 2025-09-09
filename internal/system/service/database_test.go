package service

import (
	"encoding/json"
	"testing"
	"time"
)

func TestImageHistory_Structure(t *testing.T) {
	config := &Config{
		Image: "test-image:latest",
		Packages: struct {
			Install []string `yaml:"install" json:"install"`
			Remove  []string `yaml:"remove" json:"remove"`
		}{
			Install: []string{"package1"},
			Remove:  []string{"package2"},
		},
		Commands: []string{"echo test"},
	}
	
	history := ImageHistory{
		ImageName: "test-image",
		Config:    config,
		ImageDate: "2023-01-01T00:00:00Z",
	}
	
	if history.ImageName != "test-image" {
		t.Errorf("Expected ImageName 'test-image', got %s", history.ImageName)
	}
	
	if history.Config == nil {
		t.Error("Config should not be nil")
		return
	}
	
	if history.Config.Image != "test-image:latest" {
		t.Errorf("Expected Config.Image 'test-image:latest', got %s", history.Config.Image)
	}
	
	if len(history.Config.Packages.Install) != 1 {
		t.Errorf("Expected 1 install package, got %d", len(history.Config.Packages.Install))
	}
	
	if history.ImageDate != "2023-01-01T00:00:00Z" {
		t.Errorf("Expected ImageDate '2023-01-01T00:00:00Z', got %s", history.ImageDate)
	}
}

func TestDBHistory_Structure(t *testing.T) {
	testTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	
	dbHistory := DBHistory{
		ImageName:  "test-image",
		ImageDate:  testTime,
		ConfigJSON: `{"image":"test-image:latest","packages":{"install":["pkg1"],"remove":[]},"commands":[]}`,
	}
	
	if dbHistory.ImageName != "test-image" {
		t.Errorf("Expected ImageName 'test-image', got %s", dbHistory.ImageName)
	}
	
	if !dbHistory.ImageDate.Equal(testTime) {
		t.Errorf("Expected ImageDate %v, got %v", testTime, dbHistory.ImageDate)
	}
	
	if dbHistory.ConfigJSON == "" {
		t.Error("ConfigJSON should not be empty")
	}
}

func TestDBHistory_TableName(t *testing.T) {
	dbHistory := DBHistory{}
	tableName := dbHistory.TableName()
	
	expectedTableName := "host_image_history"
	if tableName != expectedTableName {
		t.Errorf("Expected table name '%s', got '%s'", expectedTableName, tableName)
	}
}

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