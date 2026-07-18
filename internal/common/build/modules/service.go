package modules

import (
	"context"
	"errors"

	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/filter"
	"altlinux.space/alt-atomic/apm/internal/domain/kernel/service"
	reposervice "altlinux.space/alt-atomic/apm/pkg/aptrepo"
	pkgbuild "altlinux.space/alt-atomic/apm/pkg/build"
)

// DomainContext — доменные способности apm (пакеты/БД/ядро/репозитории) поверх нейтрального
// RuntimeContext движка. Доменные модули достают его из RuntimeContext через withDomain.
type DomainContext interface {
	pkgbuild.RuntimeContext
	IsAtomic() bool
	SetAptConfigOverrides(overrides map[string]string)
	ValidateDB(ctx context.Context) error
	CombineInstallRemovePackages(ctx context.Context, packages []string, purge, depends, downloadOnly bool) error
	InstallPackages(ctx context.Context, packages []string) error
	QueryHostImagePackages(ctx context.Context, filters []filter.FilterGroup, sortField, sortOrder string, limit, offset int) ([]_package.Package, error)
	GetPackageByName(ctx context.Context, packageName string) (*_package.Package, error)
	UpdatePackages(ctx context.Context) error
	UpgradePackages(ctx context.Context) error
	KernelManager() KernelManager
	RepoService() RepoManager
}

// RepoManager управления репозиториями.
type RepoManager interface {
	GetBranches() []string
	AddRepository(ctx context.Context, args []string, date string) ([]reposervice.Repository, error)
	RemoveRepository(ctx context.Context, args []string, date string, purge bool) ([]reposervice.Repository, error)
	SetBranch(ctx context.Context, branch, date string) (added, removed []reposervice.Repository, err error)
	CleanTemporary(ctx context.Context) ([]reposervice.Repository, error)
}

// KernelManager управления ядром.
type KernelManager interface {
	FindLatestKernel(ctx context.Context, flavour string) (*service.Info, error)
	AutoSelectHeadersAndFirmware(ctx context.Context, kernel *service.Info, currentKernel *service.Info, includeHeaders bool) ([]string, error)
	RemoveKernel(kernel *service.Info, purge bool) error
	InstallKernel(ctx context.Context, kernel *service.Info, modules []string, includeHeaders, dryRun bool) error
	ParseKernelPackageFromDB(pkg _package.Package) *service.Info
}

// withDomain поднимает нейтральный RuntimeContext до доменного и зовёт run.
// Единственная точка assert'а: тела модулей работают уже с DomainContext, без проверок.
func withDomain(ctx context.Context, bc pkgbuild.RuntimeContext,
	run func(context.Context, DomainContext) (any, error)) (any, error) {
	dc, ok := bc.(DomainContext)
	if !ok {
		return nil, errors.New("this module requires apm domain build context")
	}
	return run(ctx, dc)
}
