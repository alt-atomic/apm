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
	"context"
	"errors"
	"fmt"
	"strings"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/app"
	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/filter"
)

// Info возвращает информацию о системном пакете.
func (a *Actions) Info(ctx context.Context, packageName string) (*InfoResponse, error) {
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Package name must be specified, for example info package")))
	}

	err := a.validateDB(ctx, false)
	if err != nil {
		return nil, err
	}

	packageInfo, err := a.serviceAptDatabase.GetPackageByName(ctx, packageName)
	if err != nil {
		filters := []filter.Filter{
			{Field: "provides", Op: filter.OpContains, Value: packageName},
		}

		alternativePackages, errFind := a.serviceAptDatabase.QueryHostImagePackages(ctx, filter.AndGroups(filters), "", "", 5, 0)
		if errFind != nil {
			return nil, apmerr.New(apmerr.ErrorTypeDatabase, errFind)
		}

		if len(alternativePackages) == 0 {
			errorFindPackage := fmt.Sprintf(app.T_("Failed to retrieve information about the package %s"), packageName)
			return nil, apmerr.New(apmerr.ErrorTypeNotFound, errors.New(errorFindPackage))
		} else if len(alternativePackages) == 1 {
			packageInfo = alternativePackages[0]
		} else {
			var altNames []string
			for _, altPkg := range alternativePackages {
				altNames = append(altNames, altPkg.Name)
			}

			message := err.Error() + app.T_(". Maybe you were looking for: ")

			return nil, apmerr.New(apmerr.ErrorTypeNotFound, fmt.Errorf(message+"%s", strings.Join(altNames, " ")))
		}
	}

	pkgs := []_package.Package{packageInfo}
	a.enrichWithAppStream(ctx, pkgs)
	packageInfo = pkgs[0]

	return &InfoResponse{
		Message:     app.T_("Package found"),
		PackageInfo: packageInfo,
	}, nil
}

// MultiInfo возвращает информацию о нескольких пакетах одним запросом.
func (a *Actions) MultiInfo(ctx context.Context, packageNames []string) (*MultiInfoResponse, error) {
	if len(packageNames) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Package list must not be empty")))
	}

	err := a.validateDB(ctx, false)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(packageNames))
	for _, name := range packageNames {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}

	if len(names) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New(app.T_("Package list must not be empty")))
	}

	packages, err := a.serviceAptDatabase.GetPackagesByNames(ctx, names)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	foundNames := make(map[string]bool, len(packages))
	for _, pkg := range packages {
		foundNames[pkg.Name] = true
	}

	var missing []string
	for _, name := range names {
		if !foundNames[name] {
			missing = append(missing, name)
		}
	}

	var notFound []string
	for _, name := range missing {
		providesPackages, err := a.serviceAptDatabase.QueryHostImagePackages(ctx, filter.AndGroups([]filter.Filter{
			{Field: "provides", Op: filter.OpContains, Value: name},
		}), "", "", 1, 0)
		if err != nil || len(providesPackages) == 0 {
			notFound = append(notFound, name)
			continue
		}
		packages = append(packages, providesPackages[0])
	}

	a.enrichWithAppStream(ctx, packages)

	return &MultiInfoResponse{
		Message:  fmt.Sprintf(app.T_("Found %d out of %d packages"), len(packages), len(names)),
		Packages: packages,
		NotFound: notFound,
	}, nil
}

// ListParams задаёт параметры для запроса списка пакетов.
type ListParams struct {
	Sort        string               `json:"sort"`
	Order       string               `json:"order"`
	Limit       int                  `json:"limit"`
	Offset      int                  `json:"offset"`
	Filters     []filter.FilterGroup `json:"filters"`
	ForceUpdate bool                 `json:"forceUpdate"`
	Full        bool                 `json:"full"`
}

// List возвращает список пакетов
func (a *Actions) List(ctx context.Context, params ListParams) (*ListResponse, error) {
	if params.ForceUpdate {
		_, err := a.serviceAptActions.Update(ctx)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeApt, err)
		}
	}
	err := a.validateDB(ctx, false)
	if err != nil {
		return nil, err
	}

	totalCount, err := a.serviceAptDatabase.CountHostImagePackages(ctx, params.Filters)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	packages, err := a.serviceAptDatabase.QueryHostImagePackages(ctx, params.Filters, params.Sort, params.Order, params.Limit, params.Offset)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	if len(packages) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNotFound, errors.New(app.T_("Nothing found")))
	}

	a.enrichWithAppStream(ctx, packages)

	msg := fmt.Sprintf(app.TN_("%d record found", "%d records found", len(packages)), len(packages))

	return &ListResponse{
		Message:    msg,
		Packages:   packages,
		TotalCount: int(totalCount),
	}, nil
}

// GetFilterFields возвращает список свойств для фильтрации
func (a *Actions) GetFilterFields(ctx context.Context) (GetFilterFieldsResponse, error) {
	if err := a.validateDB(ctx, false); err != nil {
		return nil, err
	}

	return _package.SystemFilterConfig.FieldsInfo(), nil
}

// Sections возвращает список всех уникальных секций пакетов.
func (a *Actions) Sections(ctx context.Context) (*SectionsResponse, error) {
	if err := a.validateDB(ctx, false); err != nil {
		return nil, err
	}

	sections, err := a.serviceAptDatabase.GetSections(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	return &SectionsResponse{
		Message:  fmt.Sprintf(app.TN_("%d section found", "%d sections found", len(sections)), len(sections)),
		Sections: sections,
	}, nil
}

// Search осуществляет поиск системного пакета по названию
func (a *Actions) Search(ctx context.Context, packageName string, installed bool) (*SearchResponse, error) {
	err := a.validateDB(ctx, false)
	if err != nil {
		return nil, err
	}

	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf(app.T_("You must specify the package name, for example `%s package`"), "search"))
	}

	packages, err := a.serviceAptDatabase.SearchPackagesByNameLike(ctx, "%"+packageName+"%", installed)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	if len(packages) == 0 {
		return nil, apmerr.New(apmerr.ErrorTypeNotFound, errors.New(app.T_("Nothing found")))
	}

	a.enrichWithAppStream(ctx, packages)

	msg := fmt.Sprintf(app.TN_("%d record found", "%d records found", len(packages)), len(packages))

	return &SearchResponse{
		Message:  msg,
		Packages: packages,
	}, nil
}

// enrichWithAppStream подтягивает AppStream данные из отдельной таблицы в пакеты
func (a *Actions) enrichWithAppStream(ctx context.Context, packages []_package.Package) {
	format := a.appConfig.ConfigManager.GetConfig().Format
	if format == app.FormatText {
		return
	}

	names := make([]string, 0, len(packages))
	for i := range packages {
		if packages[i].HasAppStream {
			names = append(names, packages[i].Name)
		}
	}
	if len(names) == 0 {
		return
	}
	compMap, err := a.serviceAppStreamDB.GetByPkgNames(ctx, names)
	if err != nil {
		app.Log.Debugf("enrichWithAppStream: %v", err)
		return
	}
	for i := range packages {
		if comps, ok := compMap[packages[i].Name]; ok {
			packages[i].AppStream = comps
		}
	}
}

// ShortPackageResponse Определяем структуру для короткого представления пакета
type ShortPackageResponse struct {
	Name       string `json:"name"`
	Summary    string `json:"summary"`
	Installed  bool   `json:"installed"`
	Version    string `json:"version"`
	Maintainer string `json:"maintainer"`
}

// FormatPackageOutput принимает данные (один пакет или срез пакетов) и флаг full.
// Если full == true, то возвращается полный вывод, иначе – сокращённый.
func (a *Actions) FormatPackageOutput(data interface{}, full bool) interface{} {
	switch v := data.(type) {
	case _package.Package:
		if full {
			return v
		}
		return ShortPackageResponse{
			Name:       v.Name,
			Summary:    v.Summary,
			Version:    v.Version,
			Installed:  v.Installed,
			Maintainer: v.Maintainer,
		}
	case []_package.Package:
		if full {
			return v
		}
		shortList := make([]ShortPackageResponse, 0, len(v))
		for _, pkg := range v {
			shortList = append(shortList, ShortPackageResponse{
				Name:       pkg.Name,
				Summary:    pkg.Summary,
				Version:    pkg.Version,
				Installed:  pkg.Installed,
				Maintainer: pkg.Maintainer,
			})
		}
		return shortList
	default:
		return nil
	}
}
