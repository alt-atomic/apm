package service

import (
	"context"

	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/pkg/command"
)

// commandRunner определяет методы для выполнения системных команд.
type commandRunner interface {
	Run(ctx context.Context, args []string, opts ...command.Option) (string, string, error)
}

// packageDBService определяет методы для запросов к базе данных пакетов.
type packageDBService interface {
	GetPackageByName(ctx context.Context, packageName string) (_package.Package, error)
}
