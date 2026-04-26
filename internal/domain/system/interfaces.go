// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package system

import (
	_package "apm/internal/common/apt/package"
	aptLib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/build"
	"apm/internal/common/filter"
	"apm/internal/common/swcat"
	"apm/internal/domain/system/service"
	"context"
)

// aptActionsService определяет методы для APT операций с пакетами.
type aptActionsService interface {
	SetAptConfigOverrides(overrides map[string]string)
	GetAptConfigOverrides() map[string]string
	CheckRemove(ctx context.Context, packages []string, purge bool, depends bool) (*aptLib.PackageChanges, error)
	CheckUpgrade(ctx context.Context) (*aptLib.PackageChanges, error)
	PrepareInstallPackages(ctx context.Context, packages []string) ([]string, []string, error)
	FindPackage(ctx context.Context, installed []string, removed []string, purge bool, depends bool, reinstall bool) ([]string, []string, []_package.Package, *aptLib.PackageChanges, error)
	Remove(ctx context.Context, packages []string, purge bool, depends bool) error
	CombineInstallRemovePackages(ctx context.Context, install []string, remove []string, purge bool, depends bool, downloadOnly bool) error
	Update(ctx context.Context, noLock ...bool) ([]_package.Package, error)
	UpdateDBOnly(ctx context.Context, noLock ...bool) ([]_package.Package, error)
	AptUpdate(ctx context.Context, noLock ...bool) error
	GetInstalledPackages(ctx context.Context, noLock ...bool) (map[string]string, error)
	Upgrade(ctx context.Context, downloadOnly bool) error
	ReinstallPackages(ctx context.Context, packages []string) error
	Install(ctx context.Context, packages []string, downloadOnly bool) error
}

// aptDatabaseService определяет методы для запросов к базе данных пакетов.
type aptDatabaseService interface {
	PackageDatabaseExist(ctx context.Context) error
	GetPackageByName(ctx context.Context, packageName string) (_package.Package, error)
	GetPackagesByNames(ctx context.Context, names []string) ([]_package.Package, error)
	QueryHostImagePackages(ctx context.Context, filters []filter.Filter, sortField, sortOrder string, limit, offset int) ([]_package.Package, error)
	CountHostImagePackages(ctx context.Context, filters []filter.Filter) (int64, error)
	SearchPackagesByNameLike(ctx context.Context, likePattern string, installed bool) ([]_package.Package, error)
	SearchPackagesMultiLimit(ctx context.Context, likePattern string, limit int, installed bool) ([]_package.Package, error)
	SyncPackageInstallationInfo(ctx context.Context, installedPackages map[string]string) error
	UpdateAppStreamLinks(ctx context.Context) error
	GetSections(ctx context.Context) ([]string, error)
}

// hostDatabaseService определяет методы для работы с базой данных образов.
type hostDatabaseService interface {
	GetImageHistoriesFiltered(ctx context.Context, imageNameFilter string, limit, offset int) ([]build.ImageHistory, error)
	CountImageHistoriesFiltered(ctx context.Context, imageNameFilter string) (int, error)
}

// hostImageService определяет методы для работы с образами хоста.
type hostImageService interface {
	EnableOverlay() error
	GetHostImage() (build.HostImage, error)
	CheckAndUpdateBaseImage(ctx context.Context, pullImage bool, hostCache bool, config build.Config) error
	SwitchImage(ctx context.Context, podmanImageID string, isLocal bool) error
	BuildAndSwitch(ctx context.Context, pullImage bool, checkSame bool, hostConfigService build.SwitchableConfig) error
}

// hostConfigService определяет методы для работы с конфигурацией хоста.
type hostConfigService interface {
	LoadConfig() error
	GetConfigEnvVars() (map[string]string, error)
	SaveConfig() error
	GenerateDockerfile(hostCache bool) error
	AddInstallPackage(pkg string) error
	AddRemovePackage(pkg string) error
	GetConfig() *build.Config
	SetConfig(config *build.Config)
	ConfigIsChanged(ctx context.Context) (bool, error)
	SaveConfigToDB(ctx context.Context) error
	ApplyPathOverrides(configPath, workdir string) error
}

// temporaryConfigService определяет методы для работы с временной конфигурацией.
type temporaryConfigService interface {
	LoadConfig() error
	SaveConfig() error
	AddInstallPackage(pkg string) error
	AddRemovePackage(pkg string) error
	DeleteFile() error
	GetConfig() *service.TemporaryConfig
}

// appStreamService определяет методы для работы с AppStream данными.
type appStreamService interface {
	GetByPkgNames(ctx context.Context, names []string) (map[string][]swcat.Component, error)
}
