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
	QueryHostImagePackages(ctx context.Context, filters []filter.Filter, sortField, sortOrder string, limit, offset int) ([]_package.Package, error)
	GetPackageByName(ctx context.Context, packageName string) (*_package.Package, error)
	UpdatePackages(ctx context.Context) error
	UpgradePackages(ctx context.Context) error
	KernelManager() *service.Manager
	RepoService() *reposervice.RepoService
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
