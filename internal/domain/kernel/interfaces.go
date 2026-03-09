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

package kernel

import (
	_package "apm/internal/common/apt/package"
	aptlib "apm/internal/common/binding/apt/lib"
	"apm/internal/domain/kernel/service"
	"context"
)

// aptActionsService определяет методы APT операций, используемых в домене kernel.
type aptActionsService interface {
	Update(ctx context.Context, noLock ...bool) ([]_package.Package, error)
	AptUpdate(ctx context.Context, noLock ...bool) error
	GetInstalledPackages(ctx context.Context, noLock ...bool) (map[string]string, error)
}

// aptDatabaseService определяет методы для запросов к базе данных пакетов.
type aptDatabaseService interface {
	PackageDatabaseExist(ctx context.Context) error
	SyncPackageInstallationInfo(ctx context.Context, installedPackages map[string]string) error
}

// kernelManagerService определяет методы для управления ядрами системы.
type kernelManagerService interface {
	ListKernels(ctx context.Context, flavour string) ([]*service.Info, error)
	GetCurrentKernel(ctx context.Context) (*service.Info, error)
	FindLatestKernel(ctx context.Context, flavour string) (*service.Info, error)
	InheritModulesFromKernel(targetKernel *service.Info, sourceKernel *service.Info) ([]string, error)
	AutoSelectHeadersAndFirmware(ctx context.Context, kernel *service.Info, includeHeaders bool) ([]string, error)
	SimulateUpgrade(kernel *service.Info, modules []string, includeHeaders bool) (*service.UpgradePreview, error)
	InstallKernel(ctx context.Context, kernel *service.Info, modules []string, includeHeaders bool, dryRun bool) error
	FindNextFlavours(minVersion string) ([]string, error)
	ListInstalledKernelsFromRPM(ctx context.Context) ([]*service.Info, error)
	GetBackupKernel(ctx context.Context) (*service.Info, error)
	GroupKernelsByFlavour(kernels []*service.Info) map[string][]*service.Info
	RemovePackages(ctx context.Context, removePackages []string, dryRun bool) (*aptlib.PackageChanges, error)
	DetectCurrentFlavour(ctx context.Context) (string, error)
	FindAvailableModules(kernel *service.Info) ([]service.ModuleInfo, error)
	GetFullPackageNameForModule(packageName string) string
	InstallModules(ctx context.Context, installPackages []string, dryRun bool) (*aptlib.PackageChanges, error)
	GetSimplePackageNameForModule(packageName string) string
	BuildFullKernelInfo(info *service.Info) service.FullKernelInfo
}
