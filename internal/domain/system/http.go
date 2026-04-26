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
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	_package "apm/internal/common/apt/package"
	"apm/internal/common/build"
	"apm/internal/common/filter"
	"apm/internal/common/http_server"
	"apm/internal/common/reply"
	"apm/internal/common/swcat"
	"apm/internal/domain/system/appstream"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
)

// HTTPWrapper предоставляет обёртку для системных действий через HTTP.
type HTTPWrapper struct {
	http_server.BaseHTTPWrapper
	actions          *Actions
	appstreamActions *appstream.Actions
}

// NewHTTPWrapper создаёт новую обёртку над actions
func NewHTTPWrapper(a *Actions, appConfig *app.Config, ctx context.Context) *HTTPWrapper {
	return &HTTPWrapper{
		BaseHTTPWrapper:  http_server.BaseHTTPWrapper{Ctx: ctx, AppConfig: appConfig},
		actions:          a,
		appstreamActions: appstream.NewActions(appConfig),
	}
}

// CheckRemove проверяет возможность удаления пакетов.
func (w *HTTPWrapper) CheckRemove(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var packages []string
	var purge, depends bool

	for _, f := range []struct {
		key    string
		target interface{}
	}{
		{"packages", &packages},
		{"purge", &purge},
		{"depends", &depends},
	} {
		if err = reply.UnmarshalField(body, f.key, f.target); err != nil {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
			return
		}
	}

	if w.RunBackground(rw, r, reply.EventSystemCheckRemove, func(ctx context.Context) (interface{}, error) {
		return w.actions.CheckRemove(ctx, packages, purge, depends)
	}) {
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.CheckRemove(ctx, packages, purge, depends)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// CheckInstall проверяет возможность установки пакетов.
func (w *HTTPWrapper) CheckInstall(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var packages []string

	if err = reply.UnmarshalField(body, "packages", &packages); err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	if w.RunBackground(rw, r, reply.EventSystemCheckInstall, func(ctx context.Context) (interface{}, error) {
		return w.actions.CheckInstall(ctx, packages)
	}) {
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.CheckInstall(ctx, packages)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// CheckUpgrade проверяет возможность обновления системы.
func (w *HTTPWrapper) CheckUpgrade(rw http.ResponseWriter, r *http.Request) {
	if w.RunBackground(rw, r, reply.EventSystemCheckUpgrade, func(ctx context.Context) (interface{}, error) {
		return w.actions.CheckUpgrade(ctx)
	}) {
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.CheckUpgrade(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Remove удаляет пакеты.
func (w *HTTPWrapper) Remove(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var packages []string
	var purge, depends bool

	for _, f := range []struct {
		key    string
		target interface{}
	}{
		{"packages", &packages},
		{"purge", &purge},
		{"depends", &depends},
	} {
		if err = reply.UnmarshalField(body, f.key, f.target); err != nil {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
			return
		}
	}

	if w.RunBackground(rw, r, reply.EventSystemRemove, func(ctx context.Context) (interface{}, error) {
		return w.actions.Remove(ctx, packages, purge, depends, true)
	}) {
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Remove(ctx, packages, purge, depends, true)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Install устанавливает пакеты.
func (w *HTTPWrapper) Install(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var packages []string

	if err = reply.UnmarshalField(body, "packages", &packages); err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	downloadOnly := r.URL.Query().Get("download_only") == "true"

	if w.RunBackground(rw, r, reply.EventSystemInstall, func(ctx context.Context) (interface{}, error) {
		return w.actions.Install(ctx, packages, true, downloadOnly)
	}) {
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Install(ctx, packages, true, downloadOnly)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Info возвращает информацию о пакете.
func (w *HTTPWrapper) Info(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	full := r.URL.Query().Get("full") == "true"

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Info(ctx, name)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(map[string]interface{}{
		"message":     resp.Message,
		"packageInfo": w.actions.FormatPackageOutput(resp.PackageInfo, full),
	}))
}

// MultiInfo возвращает информацию о нескольких пакетах.
func (w *HTTPWrapper) MultiInfo(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var packages []string
	if err = reply.UnmarshalField(body, "packages", &packages); err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	full := r.URL.Query().Get("full") == "true"

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.MultiInfo(ctx, packages)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}

	w.WriteJSON(rw, reply.OK(map[string]interface{}{
		"message":  resp.Message,
		"packages": w.actions.FormatPackageOutput(resp.Packages, full),
		"notFound": resp.NotFound,
	}))
}

// List возвращает список пакетов с фильтрацией.
func (w *HTTPWrapper) List(rw http.ResponseWriter, r *http.Request) {
	var body ListFiltersBody
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
			return
		}
	}

	validated, err := _package.SystemFilterConfig.Validate(body.Filters)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	query := r.URL.Query()
	limit := 50
	if v := query.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	offset := 0
	if v := query.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}

	params := ListParams{
		Sort:        query.Get("sort"),
		Order:       query.Get("order"),
		Limit:       limit,
		Offset:      offset,
		Filters:     validated,
		ForceUpdate: query.Get("forceUpdate") == "true",
		Full:        query.Get("full") != "false",
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.List(ctx, params)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(map[string]interface{}{
		"message":    resp.Message,
		"packages":   w.actions.FormatPackageOutput(resp.Packages, params.Full),
		"totalCount": resp.TotalCount,
	}))
}

// GetFilterFields возвращает доступные поля фильтрации.
func (w *HTTPWrapper) GetFilterFields(rw http.ResponseWriter, r *http.Request) {
	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.GetFilterFields(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Search выполняет поиск пакетов.
func (w *HTTPWrapper) Search(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	q := query.Get("q")
	installed := query.Get("installed") == "true"
	full := query.Get("full") == "true"

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Search(ctx, q, installed)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(map[string]interface{}{
		"message":  resp.Message,
		"packages": w.actions.FormatPackageOutput(resp.Packages, full),
	}))
}

// Update обновляет базу данных пакетов.
func (w *HTTPWrapper) Update(rw http.ResponseWriter, r *http.Request) {
	noLock := r.URL.Query().Get("noLock") == "true"

	if w.RunBackground(rw, r, reply.EventSystemUpdate, func(ctx context.Context) (interface{}, error) {
		return w.actions.Update(ctx, noLock, false)
	}) {
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Update(ctx, noLock, false)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Upgrade обновляет систему.
func (w *HTTPWrapper) Upgrade(rw http.ResponseWriter, r *http.Request) {
	downloadOnly := r.URL.Query().Get("download_only") == "true"

	if w.RunBackground(rw, r, reply.EventSystemUpgrade, func(ctx context.Context) (interface{}, error) {
		return w.actions.Upgrade(ctx, downloadOnly)
	}) {
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Upgrade(ctx, downloadOnly)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Image (atomic only)

// ImageStatus возвращает статус образа.
func (w *HTTPWrapper) ImageStatus(rw http.ResponseWriter, r *http.Request) {
	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.ImageStatus(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// ImageUpdate обновляет образ системы.
func (w *HTTPWrapper) ImageUpdate(rw http.ResponseWriter, r *http.Request) {
	hostCache := r.URL.Query().Get("no_cache") != "true"

	if w.RunBackground(rw, r, reply.EventSystemImageUpdate, func(ctx context.Context) (interface{}, error) {
		return w.actions.ImageUpdate(ctx, hostCache)
	}) {
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.ImageUpdate(ctx, hostCache)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// ImageApply применяет изменения к образу.
func (w *HTTPWrapper) ImageApply(rw http.ResponseWriter, r *http.Request) {
	pullImage := r.URL.Query().Get("pull") == "true"
	hostCache := r.URL.Query().Get("no_cache") != "true"
	configPath := r.URL.Query().Get("config")
	workdir := r.URL.Query().Get("workdir")

	if w.RunBackground(rw, r, reply.EventSystemImageApply, func(ctx context.Context) (interface{}, error) {
		return w.actions.ImageApply(ctx, pullImage, hostCache, configPath, workdir)
	}) {
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.ImageApply(ctx, pullImage, hostCache, configPath, workdir)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// ImageHistory возвращает историю обновлений образа.
func (w *HTTPWrapper) ImageHistory(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	imageName := query.Get("imageName")

	limit := 50
	if l := query.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}

	offset := 0
	if o := query.Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.ImageHistory(ctx, imageName, limit, offset)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// ImageGetConfig возвращает конфигурацию образа.
func (w *HTTPWrapper) ImageGetConfig(rw http.ResponseWriter, r *http.Request) {
	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.ImageGetConfig(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// ImageSaveConfig сохраняет конфигурацию образа.
func (w *HTTPWrapper) ImageSaveConfig(rw http.ResponseWriter, r *http.Request) {
	var config build.Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		if errors.Is(err, io.EOF) {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("request body is required")))
			return
		}
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf("invalid JSON: %w", err)))
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.ImageSaveConfig(ctx, config)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// SetAptConfig устанавливает переопределения конфигурации APT.
func (w *HTTPWrapper) SetAptConfig(rw http.ResponseWriter, r *http.Request) {
	var body AptConfigResponse
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf("invalid JSON: %w", err)))
		return
	}
	resp, err := w.actions.SetAptConfigOverrides(body.Options)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// GetAptConfig возвращает текущие переопределения конфигурации APT.
func (w *HTTPWrapper) GetAptConfig(rw http.ResponseWriter, r *http.Request) {
	resp, err := w.actions.GetAptConfigOverrides()
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// ApplicationUpdate загружает и сохраняет данные приложений.
func (w *HTTPWrapper) ApplicationUpdate(rw http.ResponseWriter, r *http.Request) {
	if w.RunBackground(rw, r, reply.EventApplicationUpdate, func(ctx context.Context) (interface{}, error) {
		return w.appstreamActions.Update(ctx)
	}) {
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.appstreamActions.Update(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// ApplicationInfo возвращает данные приложения для конкретного пакета.
func (w *HTTPWrapper) ApplicationInfo(rw http.ResponseWriter, r *http.Request) {
	pkgname := r.PathValue("pkgname")
	ctx := w.CtxWithTransaction(r)
	resp, err := w.appstreamActions.Info(ctx, pkgname)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// ApplicationList возвращает список приложений.
func (w *HTTPWrapper) ApplicationList(rw http.ResponseWriter, r *http.Request) {
	var body struct {
		Filters []filter.Filter `json:"filters"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
			return
		}
	}

	validated, err := swcat.FilterConfig.Validate(body.Filters)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	query := r.URL.Query()
	limit := 50
	if v := query.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	offset := 0
	if v := query.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}

	params := appstream.ListParams{
		Sort:    query.Get("sort"),
		Order:   query.Get("order"),
		Limit:   limit,
		Offset:  offset,
		Filters: validated,
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.appstreamActions.List(ctx, params)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// ApplicationGetFilterFields возвращает доступные поля фильтрации приложений.
func (w *HTTPWrapper) ApplicationGetFilterFields(rw http.ResponseWriter, r *http.Request) {
	ctx := w.CtxWithTransaction(r)
	resp, err := w.appstreamActions.GetFilterFields(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Sections возвращает список уникальных секций пакетов.
func (w *HTTPWrapper) Sections(rw http.ResponseWriter, r *http.Request) {
	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Sections(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// ApplicationCategories возвращает список уникальных категорий приложений.
func (w *HTTPWrapper) ApplicationCategories(rw http.ResponseWriter, r *http.Request) {
	ctx := w.CtxWithTransaction(r)
	resp, err := w.appstreamActions.Categories(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// GetEndpoints возвращает описания endpoints с handler
func (w *HTTPWrapper) GetEndpoints(isAtomic bool) []http_server.Endpoint {
	endpoints := []http_server.Endpoint{
		{
			Handler:      w.CheckRemove,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/packages/check-remove",
			ResponseType: reflect.TypeOf(CheckResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Проверить пакеты перед удалением",
			Tags:         []string{"packages"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "packages", Source: "body", Type: "[]string", ArgIndex: 1},
				{Name: "purge", Source: "body", Type: "bool", Default: "false", ArgIndex: 2},
				{Name: "depends", Source: "body", Type: "bool", Default: "false", ArgIndex: 3},
			},
			QueryParams: []http_server.QueryParam{
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},
		{
			Handler:      w.CheckInstall,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/packages/check-install",
			ResponseType: reflect.TypeOf(CheckResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Проверить пакеты перед установкой",
			Tags:         []string{"packages"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "packages", Source: "body", Type: "[]string", ArgIndex: 1},
			},
			QueryParams: []http_server.QueryParam{
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},
		{
			Handler:      w.CheckUpgrade,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/system/check-upgrade",
			ResponseType: reflect.TypeOf(CheckResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Проверить пакеты перед обновлением системы",
			Tags:         []string{"system"},
			QueryParams: []http_server.QueryParam{
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},

		// Packages - действия
		{
			Handler:      w.Remove,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/packages/remove",
			ResponseType: reflect.TypeOf(InstallRemoveResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Удалить пакеты",
			Tags:         []string{"packages"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "packages", Source: "body", Type: "[]string", ArgIndex: 1},
				{Name: "purge", Source: "body", Type: "bool", Default: "false", ArgIndex: 2},
				{Name: "depends", Source: "body", Type: "bool", Default: "false", ArgIndex: 3},
			},
			QueryParams: []http_server.QueryParam{
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},
		{
			Handler:      w.Install,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/packages/install",
			ResponseType: reflect.TypeOf(InstallRemoveResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Установить пакеты",
			Tags:         []string{"packages"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "packages", Source: "body", Type: "[]string", ArgIndex: 1},
			},
			QueryParams: []http_server.QueryParam{
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
				{Name: "download_only", Type: "boolean", Required: false, Description: "Только скачать пакеты без установки"},
			},
		},

		// Packages - информация
		{
			Handler:      w.Info,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/packages/{name}",
			ResponseType: reflect.TypeOf(InfoResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить информацию о пакете",
			Tags:         []string{"packages"},
			PathParams:   []string{"name"},
			QueryParams: []http_server.QueryParam{
				{Name: "full", Type: "boolean", Required: false, Description: "Полный формат вывода"},
			},
		},
		{
			Handler:      w.MultiInfo,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/packages/info",
			ResponseType: reflect.TypeOf(MultiInfoResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить информацию о нескольких пакетах",
			Tags:         []string{"packages"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "packages", Source: "body", Type: "[]string", ArgIndex: 1},
			},
			QueryParams: []http_server.QueryParam{
				{Name: "full", Type: "boolean", Required: false, Description: "Полный формат вывода"},
			},
		},
		{
			Handler:      w.List,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/packages/list",
			RequestType:  reflect.TypeOf(ListFiltersBody{}),
			ResponseType: reflect.TypeOf(ListResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить список пакетов",
			Description: filter.ListEndpointDescription(
				"Поиск пакетов",
				"name, section, installed, app.categories",
				"POST /api/v1/packages/list?sort=name&limit=20",
				`{"filters": [{"field": "name", "op": "like", "value": "hello"}]}`,
				"/api/v1/packages/filter-fields",
			),
			Tags: []string{"packages"},
			QueryParams: []http_server.QueryParam{
				{Name: "sort", Type: "string", Required: false, Description: "Поле сортировки"},
				{Name: "order", Type: "string", Required: false, Description: "Порядок сортировки (ASC/DESC)"},
				{Name: "limit", Type: "integer", Required: false, Description: "Лимит записей (по умолчанию 50)"},
				{Name: "offset", Type: "integer", Required: false, Description: "Смещение"},
				{Name: "forceUpdate", Type: "boolean", Required: false, Description: "Принудительное обновление базы"},
				{Name: "full", Type: "boolean", Required: false, Description: "Полный формат вывода"},
			},
		},
		{
			Handler:      w.GetFilterFields,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/packages/filter-fields",
			ResponseType: reflect.TypeOf(GetFilterFieldsResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить доступные поля для фильтрации",
			Tags:         []string{"packages"},
		},
		{
			Handler:      w.Sections,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/packages/sections",
			ResponseType: reflect.TypeOf(SectionsResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить список секций пакетов",
			Tags:         []string{"packages"},
		},
		{
			Handler:      w.Search,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/packages/search",
			ResponseType: reflect.TypeOf(SearchResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Поиск пакетов по названию",
			Tags:         []string{"packages"},
			QueryParams: []http_server.QueryParam{
				{Name: "q", Type: "string", Required: true, Description: "Поисковый запрос"},
				{Name: "installed", Type: "boolean", Required: false, Description: "Искать только установленные"},
				{Name: "full", Type: "boolean", Required: false, Description: "Полный формат вывода"},
			},
		},

		// APT Config
		{
			Handler:      w.SetAptConfig,
			HTTPMethod:   "PUT",
			HTTPPath:     "/api/v1/system/apt-config",
			RequestType:  reflect.TypeOf(AptConfigResponse{}),
			ResponseType: reflect.TypeOf(AptConfigResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Установить переопределения конфигурации APT",
			Tags:         []string{"system"},
		},
		{
			Handler:      w.GetAptConfig,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/system/apt-config",
			ResponseType: reflect.TypeOf(AptConfigResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить текущие переопределения конфигурации APT",
			Tags:         []string{"system"},
		},

		// System
		{
			Handler:      w.Update,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/system/update",
			ResponseType: reflect.TypeOf(UpdateResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Обновить базу данных пакетов",
			Tags:         []string{"system"},
			QueryParams: []http_server.QueryParam{
				{Name: "noLock", Type: "boolean", Required: false, Description: "Не блокировать базу"},
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},
		{
			Handler:      w.Upgrade,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/system/upgrade",
			ResponseType: reflect.TypeOf(UpgradeResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Обновить систему",
			Tags:         []string{"system"},
			QueryParams: []http_server.QueryParam{
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
				{Name: "download_only", Type: "boolean", Required: false, Description: "Только скачать пакеты без установки"},
			},
		},
	}
	endpoints = append(endpoints,
		http_server.Endpoint{
			Handler:      w.ApplicationUpdate,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/applications/update",
			ResponseType: reflect.TypeOf(appstream.UpdateResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Обновить данные приложений",
			Tags:         []string{"applications"},
			QueryParams: []http_server.QueryParam{
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},
		http_server.Endpoint{
			Handler:      w.ApplicationInfo,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/applications/{pkgname}",
			ResponseType: reflect.TypeOf(appstream.InfoResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить данные приложения для пакета",
			Tags:         []string{"applications"},
			PathParams:   []string{"pkgname"},
		},
		http_server.Endpoint{
			Handler:      w.ApplicationList,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/applications/list",
			RequestType:  reflect.TypeOf(appstream.ListFiltersBody{}),
			ResponseType: reflect.TypeOf(appstream.ListResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить список приложений",
			Description: filter.ListEndpointDescription(
				"Поиск приложений",
				"pkgname, components.name, components.categories",
				"POST /api/v1/applications/list?sort=pkgname&limit=20",
				`{"filters": [{"field": "components.categories", "op": "eq", "value": "Game"}]}`,
				"/api/v1/applications/filter-fields",
			),
			Tags: []string{"applications"},
			QueryParams: []http_server.QueryParam{
				{Name: "sort", Type: "string", Required: false, Description: "Поле сортировки"},
				{Name: "order", Type: "string", Required: false, Description: "Порядок сортировки (ASC/DESC)"},
				{Name: "limit", Type: "integer", Required: false, Description: "Лимит записей (по умолчанию 50)"},
				{Name: "offset", Type: "integer", Required: false, Description: "Смещение"},
			},
		},
		http_server.Endpoint{
			Handler:      w.ApplicationGetFilterFields,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/applications/filter-fields",
			ResponseType: reflect.TypeOf(appstream.FilterFieldsAppStreamResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить доступные поля для фильтрации приложений",
			Tags:         []string{"applications"},
		},
		http_server.Endpoint{
			Handler:      w.ApplicationCategories,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/applications/categories",
			ResponseType: reflect.TypeOf(appstream.CategoriesResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить список категорий приложений",
			Tags:         []string{"applications"},
		},
	)

	// Image (только для atomic)
	if isAtomic {
		endpoints = append(endpoints,
			http_server.Endpoint{
				Handler:      w.ImageStatus,
				HTTPMethod:   "GET",
				HTTPPath:     "/api/v1/image/status",
				ResponseType: reflect.TypeOf(ImageStatusResponse{}),
				Permission:   http_server.PermRead,
				Summary:      "Получить статус образа",
				Tags:         []string{"image"},
			},
			http_server.Endpoint{
				Handler:      w.ImageUpdate,
				HTTPMethod:   "POST",
				HTTPPath:     "/api/v1/image/update",
				ResponseType: reflect.TypeOf(ImageUpdateResponse{}),
				Permission:   http_server.PermManage,
				Summary:      "Обновить образ",
				Tags:         []string{"image"},
				QueryParams: []http_server.QueryParam{
					{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
					{Name: "no_cache", Type: "boolean", Required: false, Description: "Отключить кэш APT-пакетов при сборке образа"},
				},
			},
			http_server.Endpoint{
				Handler:      w.ImageApply,
				HTTPMethod:   "POST",
				HTTPPath:     "/api/v1/image/apply",
				ResponseType: reflect.TypeOf(ImageApplyResponse{}),
				Permission:   http_server.PermManage,
				Summary:      "Применить изменения к образу",
				Tags:         []string{"image"},
				QueryParams: []http_server.QueryParam{
					{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
					{Name: "pull", Type: "boolean", Required: false, Description: "Всегда загружать базовый образ из реестра"},
					{Name: "no_cache", Type: "boolean", Required: false, Description: "Отключить кэш APT-пакетов при сборке образа"},
					{Name: "config", Type: "string", Required: false, Description: "Путь к файлу конфигурации образа"},
					{Name: "workdir", Type: "string", Required: false, Description: "Рабочая директория сборки"},
				},
			},
			http_server.Endpoint{
				Handler:      w.ImageHistory,
				HTTPMethod:   "GET",
				HTTPPath:     "/api/v1/image/history",
				ResponseType: reflect.TypeOf(ImageHistoryResponse{}),
				Permission:   http_server.PermRead,
				Summary:      "Получить историю изменений образа",
				Tags:         []string{"image"},
				QueryParams: []http_server.QueryParam{
					{Name: "imageName", Type: "string", Required: false, Description: "Имя образа"},
					{Name: "limit", Type: "integer", Required: false, Description: "Лимит записей"},
					{Name: "offset", Type: "integer", Required: false, Description: "Смещение"},
				},
			},
			http_server.Endpoint{
				Handler:      w.ImageGetConfig,
				HTTPMethod:   "GET",
				HTTPPath:     "/api/v1/image/config",
				ResponseType: reflect.TypeOf(ImageConfigResponse{}),
				Permission:   http_server.PermRead,
				Summary:      "Получить конфигурацию образа",
				Tags:         []string{"image"},
			},
			http_server.Endpoint{
				Handler:      w.ImageSaveConfig,
				HTTPMethod:   "PUT",
				HTTPPath:     "/api/v1/image/config",
				RequestType:  reflect.TypeOf(build.Config{}),
				ResponseType: reflect.TypeOf(ImageConfigResponse{}),
				Permission:   http_server.PermManage,
				Summary:      "Сохранить конфигурацию образа",
				Tags:         []string{"image"},
			},
		)
	}

	return endpoints
}
