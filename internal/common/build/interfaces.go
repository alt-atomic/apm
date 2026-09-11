package build

import (
	"context"

	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/filter"
	aptBinding "altlinux.space/alt-atomic/apm/pkg/apt"
	aptLib "altlinux.space/alt-atomic/apm/pkg/apt/lib"
	pkgbuild "altlinux.space/alt-atomic/apm/pkg/build"
)

// buildAptActionsService определяет методы APT операций для сборки образа.
type buildAptActionsService interface {
	SetAptConfigOverrides(overrides map[string]string)
	Apply(ctx context.Context, spec aptBinding.TransactionSpec, confirm aptBinding.Confirm) (*aptLib.PackageChanges, error)
	Update(ctx context.Context, noLock ...bool) ([]_package.Package, error)
	Upgrade(ctx context.Context, downloadOnly bool) error
	RpmIsPackageInstalled(packageName string) (bool, error)
}

// buildPackageDBService определяет методы для запросов к базе данных пакетов при сборке.
type buildPackageDBService interface {
	QueryHostImagePackages(ctx context.Context, filters []filter.FilterGroup, sortField, sortOrder string, limit, offset int) ([]_package.Package, error)
	GetPackageByName(ctx context.Context, packageName string) (_package.Package, error)
	PackageDatabaseExist(ctx context.Context) error
}

// buildHostConfigService определяет методы для работы с конфигурацией хоста при сборке.
type buildHostConfigService interface {
	GetConfig() *pkgbuild.Config
}
