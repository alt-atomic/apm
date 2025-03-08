package system

import (
	"apm/cmd/system/converter"
	"apm/cmd/system/service"
	"apm/lib"
	"fmt"
	"syscall"

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
	Config converter.Config  `json:"config"`
}

func (a *Actions) getImageStatus() (ImageStatus, error) {
	hostImage, err := service.GetHostImage()
	if err != nil {
		return ImageStatus{}, err
	}

	config, err := converter.ParseConfig()
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

// ImageStatus возвращает статус актуального образа
func (a *Actions) ImageStatus() (reply.APIResponse, error) {
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
func (a *Actions) ImageUpdate() (reply.APIResponse, error) {
	err := checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	err = service.CheckAndUpdateBaseImage(true)
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

// ImageSwitchLocal переключает образ на локальное хранилище
func (a *Actions) ImageSwitchLocal() (reply.APIResponse, error) {
	err := checkRoot()
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	config, err := converter.ParseConfig()
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

	if imageStatus.Image.Status.Booted.Image.Image.Transport == "containers-storage" {
		return reply.APIResponse{
			Data: map[string]interface{}{
				"message":     "Образ уже переключен на локальное хранилище, для применения изменений воспользуйтесь командой update",
				"bootedImage": imageStatus,
			},
			Error: false,
		}, nil
	}

	err = service.BuildAndSwitch(true)
	if err != nil {
		return newErrorResponse(err.Error()), err
	}

	return reply.APIResponse{
		Data: map[string]interface{}{
			"message":     "Переключение на локальный образ выполнено",
			"bootedImage": imageStatus,
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
