package aptrepo

import (
	"context"

	"altlinux.space/alt-atomic/apm/pkg/command"
)

// commandRunner определяет методы для выполнения системных команд.
type commandRunner interface {
	Run(ctx context.Context, args []string, opts ...command.Option) (string, string, error)
}

// HasPackageFunc проверяет, установлен ли пакет с указанным именем.
type HasPackageFunc func(ctx context.Context, name string) bool
