package execute

import (
	_package "apm/internal/common/apt/package"
	"apm/internal/common/build/core"
	"apm/internal/kernel/service"
	"context"
)

type Service interface {
	Config() *core.Config
	QueryHostImagePackages(ctx context.Context, filters map[string]any, sortField, sortOrder string, limit, offset int) ([]_package.Package, error)
	CombineInstallRemovePackages(ctx context.Context, packages []string, purge, depends bool) error
	InstallPackages(ctx context.Context, packages []string) error
	UpdatePackages(ctx context.Context) error
	KernelManager() *service.Manager
	ResourcesDir() string
}
