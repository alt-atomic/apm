package service

import (
	_package "apm/internal/common/apt/package"
	apt "apm/internal/common/binding/apt"
	libApt "apm/internal/common/binding/apt/lib"
	"apm/internal/common/filter"
	"context"
)

// packageDBService определяет методы для запросов к базе данных пакетов.
type packageDBService interface {
	QueryHostImagePackages(ctx context.Context, filters []filter.Filter, sortField, sortOrder string, limit, offset int) ([]_package.Package, error)
	SearchPackagesByNameLike(ctx context.Context, likePattern string, installed bool) ([]_package.Package, error)
	GetPackageByName(ctx context.Context, packageName string) (_package.Package, error)
}

// aptBindingActions определяет методы для низкоуровневых APT операций.
type aptBindingActions interface {
	SimulateRemove(packageNames []string, purge bool, depends bool) (*libApt.PackageChanges, error)
	RemovePackages(packageNames []string, purge bool, depends bool, handler libApt.ProgressHandler) error
	SimulateInstall(packageNames []string) (*libApt.PackageChanges, error)
	InstallPackages(packageNames []string, handler libApt.ProgressHandler, downloadOnly bool) error
	RpmQueryKernelPackages(ctx context.Context) ([]apt.KernelRPMInfo, error)
	RpmIsPackageInstalled(packageName string) (bool, error)
	RpmIsAnyPackageInstalled(possibleNames []string) (bool, error)
}
