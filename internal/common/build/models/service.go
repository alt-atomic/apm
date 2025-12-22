package models

import (
	_package "apm/internal/common/apt/package"
	"apm/internal/common/build/common_types"
	"apm/internal/kernel/service"
	"context"
)

type Service interface {
	IsAtomic() bool
	CombineInstallRemovePackages(ctx context.Context, packages []string, purge, depends bool) error
	InstallPackages(ctx context.Context, packages []string) error
	QueryHostImagePackages(ctx context.Context, filters map[string]any, sortField, sortOrder string, limit, offset int) ([]_package.Package, error)
	GetPackageByName(ctx context.Context, packageName string) (*_package.Package, error)
	UpdatePackages(ctx context.Context) error
	UpgradePackages(ctx context.Context) error
	KernelManager() *service.Manager
	ResourcesDir() string
	ExecuteInclude(ctx context.Context, target string) (map[string]*common_types.MapModule, error)
}

type Body interface {
	// context.Context - app context
	// Service - build service
	//
	// returns
	// any as output struct
	Execute(context.Context, Service) (any, error)
}
