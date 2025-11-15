package models

import (
	_package "apm/internal/common/apt/package"
	"apm/internal/kernel/service"
	"context"
)

type Service interface {
	CombineInstallRemovePackages(ctx context.Context, packages []string, purge, depends bool) error
	InstallPackages(ctx context.Context, packages []string) error
	QueryHostImagePackages(ctx context.Context, filters map[string]any, sortField, sortOrder string, limit, offset int) ([]_package.Package, error)
	UpdatePackages(ctx context.Context) error
	KernelManager() *service.Manager
	ResourcesDir() string
	ExecuteInclude(ctx context.Context, target string) error
}

type Body interface {
	Check() error
	Execute(context.Context, Service) error
}
