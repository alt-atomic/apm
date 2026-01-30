package models

import (
	_package "apm/internal/common/apt/package"
	"apm/internal/kernel/service"
	_repo_service "apm/internal/repo/service"
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
	ExecuteInclude(ctx context.Context, target string) error
	RepoService() *_repo_service.RepoService
}

type Body interface {
	// Execute context.Context - app context
	// Service - build service
	//
	// returns
	// any as output struct
	Execute(context.Context, Service) (any, error)

	// Hash возвращает хеш содержимого модуля для кэширования
	// baseDir - базовая директория для резолва относительных путей
	// env - переменные окружения для раскрытия плейсхолдеров
	Hash(baseDir string, env map[string]string) string
}
