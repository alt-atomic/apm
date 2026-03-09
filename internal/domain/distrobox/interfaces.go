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

package distrobox

import (
	"apm/internal/common/sandbox"
	"context"
)

// packageService определяет методы для работы с пакетами в контейнерах.
type packageService interface {
	UpdatePackages(ctx context.Context, osInfo sandbox.ContainerInfo) ([]sandbox.PackageInfo, error)
	GetInfoPackage(ctx context.Context, osInfo sandbox.ContainerInfo, packageName string) (sandbox.InfoPackageAnswer, error)
	GetPackageByName(ctx context.Context, osInfo sandbox.ContainerInfo, packageName string) (sandbox.PackageQueryResult, error)
	GetPackagesQuery(ctx context.Context, osInfo sandbox.ContainerInfo, builder sandbox.PackageQueryBuilder) (sandbox.PackageQueryResult, error)
	InstallPackage(ctx context.Context, osInfo sandbox.ContainerInfo, packageName string) error
	RemovePackage(ctx context.Context, osInfo sandbox.ContainerInfo, packageName string) error
}

// distroDBService определяет методы для работы с базой данных контейнеров.
type distroDBService interface {
	DatabaseExist(ctx context.Context) error
	ContainerDatabaseExist(ctx context.Context, containerName string) error
	DeletePackagesFromContainer(ctx context.Context, containerName string) error
	UpdatePackageField(ctx context.Context, containerName, name, fieldName string, value bool)
}

// distroAPIService определяет методы для взаимодействия с distrobox CLI.
type distroAPIService interface {
	GetContainerList(ctx context.Context, getFullInfo bool) ([]sandbox.ContainerInfo, error)
	GetContainerOsInfo(ctx context.Context, containerName string) (sandbox.ContainerInfo, error)
	CreateContainer(ctx context.Context, image, containerName string, addPkg string, hook string) (sandbox.ContainerInfo, error)
	RemoveContainer(ctx context.Context, containerName string) (sandbox.ContainerInfo, error)
	ExportingApp(ctx context.Context, containerInfo sandbox.ContainerInfo, packageName string, desktopPaths, consolePaths []string, deleteApp bool) error
}

// IconServiceProvider определяет методы для работы с иконками пакетов.
type IconServiceProvider interface {
	GetIcon(pkgName, container string) ([]byte, error)
	ReloadIcons(ctx context.Context) error
}
