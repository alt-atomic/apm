package system

import (
	"errors"
	"testing"
)

func TestNewErrorResponseFromError(t *testing.T) {
	message := "Test error message"

	response := newErrorResponseFromError(errors.New(message))

	// Проверяем, что ошибка установлена
	if response.Error == nil {
		t.Error("Expected Error to be non-nil")
	}

	// Проверяем сообщение ошибки
	if response.Error.Message != message {
		t.Errorf("Expected message '%s', got '%s'", message, response.Error.Message)
	}

	// Проверяем что Data == nil при ошибке
	if response.Data != nil {
		t.Error("Expected Data to be nil for error response")
	}
}

func TestNewErrorResponseFromError_Structure(t *testing.T) {
	response := newErrorResponseFromError(errors.New("test"))

	// Проверяем тип ответа
	if response.Error == nil {
		t.Error("Error response should have Error != nil")
	}

	// Проверяем структуру Error
	if response.Error.Message != "test" {
		t.Errorf("Expected Error.Message 'test', got '%s'", response.Error.Message)
	}
}
