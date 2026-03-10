package service

import (
	_package "apm/internal/common/apt/package"
	"apm/internal/common/command"
	"context"
)

// commandRunner определяет методы для выполнения системных команд.
type commandRunner interface {
	Run(ctx context.Context, args []string, opts ...command.Option) (string, string, error)
}

// packageDBService определяет методы для запросов к базе данных пакетов.
type packageDBService interface {
	GetPackageByName(ctx context.Context, packageName string) (_package.Package, error)
}
