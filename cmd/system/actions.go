package system

import (
	"apm/cmd/system/converter"
	"apm/lib"
	"fmt"
	"os"
	"syscall"

	"apm/cmd/common/reply"
)

// Actions объединяет методы для выполнения системных действий.
type Actions struct{}

// NewActions создаёт новый экземпляр Actions.
func NewActions() *Actions {
	return &Actions{}
}

// Install осуществляет установку системного пакета.
func (a *Actions) Install(packageName string) (reply.APIResponse, error) {
	// Пока пустая реализация, можно добавить реальную логику установки
	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Install action вызван для пакета '%s'", packageName),
		},
		Error: false,
	}, nil
}

// Update обновляет информацию или базу данных пакетов.
func (a *Actions) Update(packageName string) (reply.APIResponse, error) {
	// Пустая реализация
	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Update action вызван для пакета '%s'", packageName),
		},
		Error: false,
	}, nil
}

// Info возвращает информацию о системном пакете.
func (a *Actions) Info(packageName string) (reply.APIResponse, error) {
	// пустая реализация
	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Info action вызван для пакета '%s'", packageName),
		},
		Error: false,
	}, nil
}

// Search осуществляет поиск системного пакета по названию.
func (a *Actions) Search(packageName string) (reply.APIResponse, error) {
	// Пустая реализация
	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Search action вызван для пакета '%s'", packageName),
		},
		Error: false,
	}, nil
}

// Remove удаляет системный пакет.
func (a *Actions) Remove(packageName string) (reply.APIResponse, error) {
	// Пустая реализация
	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Remove action вызван для пакета '%s'", packageName),
		},
		Error: false,
	}, nil
}

// ImageGenerate осуществляет принудительную генерацию локального образа.
// Параметр switchFlag указывает, нужно ли переключиться на локальный образ.
func (a *Actions) ImageGenerate(switchFlag bool) (reply.APIResponse, error) {
	err := checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	config, err := converter.ParseConfig()
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	dockerStr, err := config.GenerateDockerfile()
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	err = os.WriteFile("test.txt", []byte(dockerStr), 0644)
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": "Конфигурация образа",
			"config":  config,
		},
		Error: false,
	}, nil
}

// ImageUpdate обновляет локальный образ.
func (a *Actions) ImageUpdate() (reply.APIResponse, error) {
	// Пустая реализация
	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": "ImageUpdate action вызван",
		},
		Error: false,
	}, nil
}

// ImageSwitch переключает образ.
func (a *Actions) ImageSwitch() (reply.APIResponse, error) {
	// Пустая реализация
	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": "ImageSwitch action вызван",
		},
		Error: false,
	}, nil
}

// checkRoot проверяет, запущен ли установщик от имени root
func (a *Actions) checkRoot() error {
	if syscall.Geteuid() != 0 {
		return fmt.Errorf("для выполнения необходимы права администратора, используйте sudo или su")
	}

	return nil
}

// newErrorResponse создаёт ответ с ошибкой.
func (a *Actions) newErrorResponse(message string) reply.APIResponse {
	lib.Log.Error(message)
	return reply.APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	}
}
