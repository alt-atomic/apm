package system

import (
	"testing"
)

func TestNewErrorResponse(t *testing.T) {
	message := "Test error message"

	response := newErrorResponse(message)

	// Проверяем, что ошибка установлена
	if !response.Error {
		t.Error("Expected Error to be true")
	}

	// Проверяем данные ответа
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Error("Expected Data to be map[string]interface{}")
		return
	}

	actualMessage, exists := data["message"]
	if !exists {
		t.Error("Expected 'message' key in Data")
		return
	}

	if actualMessage != message {
		t.Errorf("Expected message '%s', got '%s'", message, actualMessage)
	}
}

func TestNewErrorResponse_Structure(t *testing.T) {
	response := newErrorResponse("test")

	// Проверяем тип ответа
	if response.Error != true {
		t.Error("Error response should have Error=true")
	}

	// Проверяем структуру Data
	data := response.Data
	if data == nil {
		t.Error("Data should not be nil")
	}

	dataMap, ok := data.(map[string]interface{})
	if !ok {
		t.Error("Data should be a map")
	}

	if len(dataMap) == 0 {
		t.Error("Data map should not be empty")
	}
}
