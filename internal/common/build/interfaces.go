package build

import (
	_package "apm/internal/common/apt/package"
	aptLib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/filter"
	"context"
)

// buildAptActionsService определяет методы APT операций для сборки образа.
type buildAptActionsService interface {
	PrepareInstallPackages(ctx context.Context, packages []string) ([]string, []string, error)
	FindPackage(ctx context.Context, installed []string, removed []string, purge bool, depends bool, reinstall bool) ([]string, []string, []_package.Package, *aptLib.PackageChanges, error)
	CombineInstallRemovePackages(ctx context.Context, install []string, remove []string, purge bool, depends bool, downloadOnly bool) error
	Install(ctx context.Context, packages []string, downloadOnly bool) error
	Update(ctx context.Context, noLock ...bool) ([]_package.Package, error)
	Upgrade(ctx context.Context, downloadOnly bool) error
}

// buildPackageDBService определяет методы для запросов к базе данных пакетов при сборке.
type buildPackageDBService interface {
	QueryHostImagePackages(ctx context.Context, filters []filter.Filter, sortField, sortOrder string, limit, offset int) ([]_package.Package, error)
	GetPackageByName(ctx context.Context, packageName string) (_package.Package, error)
}

// buildHostConfigService определяет методы для работы с конфигурацией хоста при сборке.
type buildHostConfigService interface {
	GetConfig() *Config
}

// SwitchableConfig определяет методы конфигурации для BuildAndSwitch.
type SwitchableConfig interface {
	ConfigIsChanged(ctx context.Context) (bool, error)
	SaveConfigToDB(ctx context.Context) error
}
