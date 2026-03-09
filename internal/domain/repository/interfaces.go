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

package repository

import (
	_package "apm/internal/common/apt/package"
	aptLib "apm/internal/common/binding/apt/lib"
	"apm/internal/domain/repository/service"
	"context"
)

// repoService определяет методы для работы с репозиториями.
type repoService interface {
	GetRepositories(ctx context.Context, all bool) ([]service.Repository, error)
	AddRepository(ctx context.Context, args []string, date string) ([]string, error)
	RemoveRepository(ctx context.Context, args []string, date string, purge bool) ([]string, error)
	SetBranch(ctx context.Context, branch, date string) (added []string, removed []string, err error)
	CleanTemporary(ctx context.Context) ([]string, error)
	GetBranches() []string
	GetTaskPackages(ctx context.Context, taskNum string) ([]string, error)
	SimulateAdd(ctx context.Context, args []string, date string, force bool) ([]string, error)
	SimulateRemove(ctx context.Context, args []string, date string, purge bool) ([]string, error)
}

// aptActionsService определяет методы APT операций, используемых в TestTask.
type aptActionsService interface {
	Update(ctx context.Context, noLock ...bool) ([]_package.Package, error)
	FindPackage(ctx context.Context, installed []string, removed []string, purge bool, depends bool, reinstall bool) ([]string, []string, []_package.Package, *aptLib.PackageChanges, error)
	CombineInstallRemovePackages(ctx context.Context, install []string, remove []string, purge bool, depends bool, downloadOnly bool) error
}
