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

package build

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/apt"
	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/build/modules"
	"altlinux.space/alt-atomic/apm/internal/common/filter"
	"altlinux.space/alt-atomic/apm/internal/domain/kernel/service"
	reposervice "altlinux.space/alt-atomic/apm/pkg/aptrepo"
)

type domainService struct {
	aptActions    buildAptActionsService
	dbService     buildPackageDBService
	kernelManager *service.Manager
	repoService   *reposervice.RepoService
}

func (d *domainService) SetAptConfigOverrides(overrides map[string]string) {
	d.aptActions.SetAptConfigOverrides(overrides)
}

func (d *domainService) ValidateDB(ctx context.Context) error {
	if err := d.dbService.PackageDatabaseExist(ctx); err != nil {
		app.Log.Info("Package database is empty, running update")
		_, err = d.aptActions.Update(ctx)
		if err != nil {
			return fmt.Errorf("failed to update package database: %w", err)
		}
	}
	return nil
}

func (d *domainService) QueryHostImagePackages(ctx context.Context, filters []filter.FilterGroup, sortField, sortOrder string, limit, offset int) ([]_package.Package, error) {
	return d.dbService.QueryHostImagePackages(ctx, filters, sortField, sortOrder, limit, offset)
}

func (d *domainService) GetPackageByName(ctx context.Context, packageName string) (*_package.Package, error) {
	packageInfo, err := d.dbService.GetPackageByName(ctx, packageName)
	if err != nil {
		filters := []filter.Filter{
			{Field: "provides", Op: filter.OpContains, Value: packageName},
		}

		alternativePackages, errFind := d.dbService.QueryHostImagePackages(ctx, filter.AndGroups(filters), "", "", 5, 0)
		if errFind != nil {
			return nil, errFind
		}

		if len(alternativePackages) == 0 {
			errorFindPackage := fmt.Sprintf(app.T_("Failed to retrieve information about the package %s"), packageName)
			return nil, errors.New(errorFindPackage)
		} else if len(alternativePackages) == 1 {
			return &alternativePackages[0], nil
		}

		var altNames []string
		for _, altPkg := range alternativePackages {
			altNames = append(altNames, altPkg.Name)
		}

		message := err.Error() + app.T_(". Maybe you were looking for: ")

		errPackageNotFound := fmt.Errorf(message+"%s", strings.Join(altNames, " "))

		return nil, errPackageNotFound
	}

	return &packageInfo, nil
}

func (d *domainService) CombineInstallRemovePackages(ctx context.Context, packages []string, purge bool, depends bool, downloadOnly bool) error {
	if err := d.ValidateDB(ctx); err != nil {
		return err
	}

	packagesInstall, packagesRemove, errPrepare := d.aptActions.PrepareInstallPackages(ctx, packages)
	if errPrepare != nil {
		return errPrepare
	}

	packagesInstall, packagesRemove, _, aptPackageChanges, errFind := d.aptActions.FindPackage(
		ctx,
		packagesInstall,
		packagesRemove,
		false,
		false,
		false,
	)
	if errFind != nil {
		var matchedErr *apt.MatchedError
		if errors.As(errFind, &matchedErr) && matchedErr.Entry.Code == apt.ErrPackagesAlreadyInstalled {
			app.Log.Info("Skipping error:", errFind.Error())
			return nil
		}
		return errFind
	}

	if aptPackageChanges != nil {
		if len(aptPackageChanges.NewInstalledPackages) > 0 {
			app.Log.Info(fmt.Sprintf("Install plan: %s", strings.Join(aptPackageChanges.NewInstalledPackages, ", ")))
		}

		if len(aptPackageChanges.RemovedPackages) > 0 {
			app.Log.Info(fmt.Sprintf("Remove plan: %s", strings.Join(aptPackageChanges.RemovedPackages, ", ")))
		}
	}

	errInstall := d.aptActions.CombineInstallRemovePackages(
		ctx,
		packagesInstall,
		packagesRemove,
		purge,
		depends,
		downloadOnly,
	)
	if errInstall != nil {
		return errInstall
	}

	return nil
}

func (d *domainService) InstallPackages(ctx context.Context, packages []string) error {
	return d.aptActions.Install(ctx, packages, false)
}

func (d *domainService) UpdatePackages(ctx context.Context) error {
	_, err := d.aptActions.Update(ctx)
	return err
}

func (d *domainService) UpgradePackages(ctx context.Context) error {
	err := d.aptActions.Upgrade(ctx, false)
	return err
}

func (d *domainService) KernelManager() modules.KernelManager {
	return d.kernelManager
}

func (d *domainService) RepoService() modules.RepoManager {
	return d.repoService
}
