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

package appstream

import (
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	_package "apm/internal/common/apt/package"
	"apm/internal/common/reply"
	"apm/internal/common/swcat"
	"context"
	"errors"
	"fmt"
	"syscall"
)

// Actions объединяет методы для управления AppStream данными.
type Actions struct {
	appConfig    *app.Config
	swCatService *swcat.Service
	dbService    *swcat.DBService
	pkgDBService *_package.PackageDBService
}

// NewActions создаёт новый экземпляр Actions.
func NewActions(appConfig *app.Config) *Actions {
	return &Actions{
		appConfig:    appConfig,
		swCatService: swcat.NewSwCatService("/usr/share/swcatalog/xml"),
		dbService:    swcat.NewAppStreamDBService(appConfig.DatabaseManager),
		pkgDBService: _package.NewPackageDBService(appConfig.DatabaseManager),
	}
}

// Update загружает AppStream данные из XML и сохраняет в БД.
func (a *Actions) Update(ctx context.Context) (*UpdateResponse, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventApplicationUpdate))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventApplicationUpdate))

	pkgMap, err := a.swCatService.Load(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, fmt.Errorf(app.T_("Failed to load application data: %w"), err))
	}

	if err = a.dbService.SaveComponentsToDB(ctx, pkgMap); err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, fmt.Errorf(app.T_("Failed to save application data to database: %w"), err))
	}

	if err = a.pkgDBService.UpdateAppStreamLinks(ctx); err != nil {
		app.Log.Debugf("UpdateAppStreamLinks: %v", err)
	}

	return &UpdateResponse{
		Message: app.T_("Application data updated successfully"),
		Count:   len(pkgMap),
	}, nil
}

// validateDB проверяет наличие данных AppStream в БД
func (a *Actions) validateDB(ctx context.Context) error {
	if err := a.dbService.DatabaseExist(ctx); err != nil {
		if syscall.Geteuid() != 0 {
			return apmerr.New(apmerr.ErrorTypePermission, errors.New(app.T_("Elevated rights are required to perform this action. Please use sudo or su")))
		}
		if _, updateErr := a.Update(ctx); updateErr != nil {
			return updateErr
		}
	}
	return nil
}

// Info возвращает AppStream данные для конкретного пакета.
func (a *Actions) Info(ctx context.Context, pkgname string) (*InfoResponse, error) {
	if err := a.validateDB(ctx); err != nil {
		return nil, err
	}

	components, err := a.dbService.GetByPkgName(ctx, pkgname)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeNotFound, fmt.Errorf(app.T_("Application data not found for package: %s"), pkgname))
	}

	return &InfoResponse{
		Message:    fmt.Sprintf(app.T_("Application info for %s"), pkgname),
		PkgName:    pkgname,
		Components: components,
	}, nil
}

// List возвращает список AppStream компонентов с фильтрацией и пагинацией.
func (a *Actions) List(ctx context.Context, params ListParams) (*ListResponse, error) {
	if err := a.validateDB(ctx); err != nil {
		return nil, err
	}

	totalCount, err := a.dbService.CountComponents(ctx, params.Filters)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	components, err := a.dbService.QueryComponents(ctx, params.Filters, params.Sort, params.Order, params.Limit, params.Offset)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	msg := fmt.Sprintf(app.TN_("%d record found", "%d records found", len(components)), len(components))

	return &ListResponse{
		Message:    msg,
		Components: components,
		TotalCount: int(totalCount),
	}, nil
}

// Categories возвращает список всех уникальных категорий.
func (a *Actions) Categories(ctx context.Context) (*CategoriesResponse, error) {
	if err := a.validateDB(ctx); err != nil {
		return nil, err
	}

	categories, err := a.dbService.GetCategories(ctx)
	if err != nil {
		return nil, apmerr.New(apmerr.ErrorTypeDatabase, err)
	}

	return &CategoriesResponse{
		Message:    fmt.Sprintf(app.TN_("%d category found", "%d categories found", len(categories)), len(categories)),
		Categories: categories,
	}, nil
}

// GetFilterFields возвращает список полей для фильтрации.
func (a *Actions) GetFilterFields(_ context.Context) (FilterFieldsAppStreamResponse, error) {
	return swcat.FilterConfig.FieldsInfo(), nil
}
