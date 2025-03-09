package system

import (
	"apm/cmd/system/service"
	"apm/lib"
	"context"
	"fmt"
	"syscall"
	"time"

	"apm/cmd/common/reply"
)

// Actions объединяет методы для выполнения системных действий.
type Actions struct{}

// NewActions создаёт новый экземпляр Actions.
func NewActions() *Actions {
	return &Actions{}
}

type ImageStatus struct {
	Image  service.HostImage `json:"image"`
	Status string            `json:"status"`
	Config service.Config    `json:"config"`
}

func (a *Actions) getImageStatus() (ImageStatus, error) {
	hostImage, err := service.GetHostImage()
	if err != nil {
		return ImageStatus{}, err
	}

	config, err := service.ParseConfig()
	if err != nil {
		return ImageStatus{}, err
	}

	if hostImage.Status.Booted.Image.Image.Transport == "containers-storage" {
		return ImageStatus{
			Status: "Изменённый образ. Файл конфигурации: " + lib.Env.PathImageFile,
			Image:  hostImage,
			Config: config,
		}, nil
	}

	return ImageStatus{
		Status: "Облачный образ без изменений",
		Image:  hostImage,
		Config: config,
	}, nil
}

// Install осуществляет установку системного пакета.
func (a *Actions) Install(ctx context.Context, packageName string) (reply.APIResponse, error) {
	// Пока пустая реализация, можно добавить реальную логику установки
	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Install action вызван для пакета '%s'", packageName),
		},
		Error: false,
	}, nil
}

// Update обновляет информацию или базу данных пакетов.
func (a *Actions) Update(ctx context.Context, packageName string) (reply.APIResponse, error) {
	// Пустая реализация
	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Update action вызван для пакета '%s'", packageName),
		},
		Error: false,
	}, nil
}

// Info возвращает информацию о системном пакете.
func (a *Actions) Info(ctx context.Context, packageName string) (reply.APIResponse, error) {
	// пустая реализация
	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Info action вызван для пакета '%s'", packageName),
		},
		Error: false,
	}, nil
}

// Search осуществляет поиск системного пакета по названию.
func (a *Actions) Search(ctx context.Context, packageName string) (reply.APIResponse, error) {
	// Пустая реализация
	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Search action вызван для пакета '%s'", packageName),
		},
		Error: false,
	}, nil
}

// Remove удаляет системный пакет.
func (a *Actions) Remove(ctx context.Context, packageName string) (reply.APIResponse, error) {
	// Пустая реализация
	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Remove action вызван для пакета '%s'", packageName),
		},
		Error: false,
	}, nil
}

// ImageStatus возвращает статус актуального образа
func (a *Actions) ImageStatus(ctx context.Context) (reply.APIResponse, error) {
	err := checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	imageStatus, err := a.getImageStatus()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":     "Состояние образа",
			"bootedImage": imageStatus,
		},
		Error: false,
	}, nil
}

// ImageUpdate обновляет образ.
func (a *Actions) ImageUpdate(ctx context.Context) (reply.APIResponse, error) {
	err := checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	err = service.CheckAndUpdateBaseImage(ctx, true)
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	imageStatus, err := a.getImageStatus()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":     "Команда успешно выполнена",
			"bootedImage": imageStatus,
		},
		Error: false,
	}, nil
}

// ImageApply применить изменения к хосту
func (a *Actions) ImageApply(ctx context.Context) (reply.APIResponse, error) {
	err := checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	config, err := service.ParseConfig()
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	err = config.GenerateDockerfile()
	if err != nil {
		return newErrorResponse(err.Error()), nil
	}

	imageStatus, err := a.getImageStatus()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	err = service.BuildAndSwitch(ctx, true)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	history := service.ImageHistory{
		ImageName: config.Image,
		Config:    &config,
		ImageDate: time.Now().Format(time.RFC3339),
	}

	err = service.SaveImageToDB(ctx, history)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":     "Изменения успешно применены. Необходима перезагрузка",
			"bootedImage": imageStatus,
		},
		Error: false,
	}, nil
}

// ImageHistory история изменений образа
func (a *Actions) ImageHistory(ctx context.Context, imageName string, limit int64, offset int64) (reply.APIResponse, error) {
	err := checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	history, err := service.GetImageHistoriesFiltered(ctx, imageName, limit, offset)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}
	count, err := service.CountImageHistoriesFiltered(ctx, imageName)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message": "История изменений образа",
			"history": history,
			"count":   count,
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
