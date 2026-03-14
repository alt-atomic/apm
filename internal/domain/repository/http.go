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
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	"apm/internal/common/http_server"
	"apm/internal/common/reply"
	"context"
	"errors"
	"net/http"
	"reflect"
)

// HTTPWrapper предоставляет обёртку для действий с репозиториями через HTTP.
type HTTPWrapper struct {
	http_server.BaseHTTPWrapper
	actions *Actions
}

// NewHTTPWrapper создаёт новую обёртку над actions
func NewHTTPWrapper(a *Actions, appConfig *app.Config, ctx context.Context) *HTTPWrapper {
	return &HTTPWrapper{
		BaseHTTPWrapper: http_server.BaseHTTPWrapper{Ctx: ctx, AppConfig: appConfig},
		actions:         a,
	}
}

// List возвращает список репозиториев.
func (w *HTTPWrapper) List(rw http.ResponseWriter, r *http.Request) {
	all := r.URL.Query().Get("all") == "true"

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.List(ctx, all)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Add добавляет репозиторий.
func (w *HTTPWrapper) Add(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var source, date string

	for _, f := range []struct {
		key    string
		target interface{}
	}{
		{"source", &source},
		{"date", &date},
	} {
		if err = reply.UnmarshalField(body, f.key, f.target); err != nil {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
			return
		}
	}

	if source == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("source is required")))
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Add(ctx, []string{source}, date)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// CheckAdd симулирует добавление репозитория.
func (w *HTTPWrapper) CheckAdd(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var source, date string

	for _, f := range []struct {
		key    string
		target interface{}
	}{
		{"source", &source},
		{"date", &date},
	} {
		if err = reply.UnmarshalField(body, f.key, f.target); err != nil {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
			return
		}
	}

	if source == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("source is required")))
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.CheckAdd(ctx, []string{source}, date)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Remove удаляет репозиторий.
func (w *HTTPWrapper) Remove(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var source, date string

	for _, f := range []struct {
		key    string
		target interface{}
	}{
		{"source", &source},
		{"date", &date},
	} {
		if err = reply.UnmarshalField(body, f.key, f.target); err != nil {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
			return
		}
	}

	if source == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("source is required")))
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Remove(ctx, []string{source}, date)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// CheckRemove симулирует удаление репозитория.
func (w *HTTPWrapper) CheckRemove(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var source, date string

	for _, f := range []struct {
		key    string
		target interface{}
	}{
		{"source", &source},
		{"date", &date},
	} {
		if err = reply.UnmarshalField(body, f.key, f.target); err != nil {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
			return
		}
	}

	if source == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("source is required")))
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.CheckRemove(ctx, []string{source}, date)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Set устанавливает ветку репозитория.
func (w *HTTPWrapper) Set(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var branch, date string

	for _, f := range []struct {
		key    string
		target interface{}
	}{
		{"branch", &branch},
		{"date", &date},
	} {
		if err = reply.UnmarshalField(body, f.key, f.target); err != nil {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
			return
		}
	}

	if branch == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("branch is required")))
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Set(ctx, branch, date)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// CheckSet симулирует установку ветки репозитория.
func (w *HTTPWrapper) CheckSet(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var branch, date string

	for _, f := range []struct {
		key    string
		target interface{}
	}{
		{"branch", &branch},
		{"date", &date},
	} {
		if err = reply.UnmarshalField(body, f.key, f.target); err != nil {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
			return
		}
	}

	if branch == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("branch is required")))
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.CheckSet(ctx, branch, date)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Clean удаляет временные cdrom-источники.
func (w *HTTPWrapper) Clean(rw http.ResponseWriter, r *http.Request) {
	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Clean(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// CheckClean симулирует очистку cdrom-источников.
func (w *HTTPWrapper) CheckClean(rw http.ResponseWriter, r *http.Request) {
	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.CheckClean(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// GetBranches возвращает список доступных веток.
func (w *HTTPWrapper) GetBranches(rw http.ResponseWriter, r *http.Request) {
	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.GetBranches(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// GetTaskPackages возвращает список пакетов задачи.
func (w *HTTPWrapper) GetTaskPackages(rw http.ResponseWriter, r *http.Request) {
	taskNum := r.PathValue("taskNum")

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.GetTaskPackages(ctx, taskNum)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// TestTask тестирует пакеты задачи.
func (w *HTTPWrapper) TestTask(rw http.ResponseWriter, r *http.Request) {
	taskNum := r.PathValue("taskNum")

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.TestTask(ctx, taskNum)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// GetEndpoints возвращает описания endpoints с handler
func (w *HTTPWrapper) GetEndpoints() []http_server.Endpoint {
	return []http_server.Endpoint{
		{
			Handler:      w.List,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/repo",
			ResponseType: reflect.TypeOf(RepoListResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить список репозиториев",
			Tags:         []string{"repo"},
			QueryParams: []http_server.QueryParam{
				{Name: "all", Type: "boolean", Required: false, Description: "Показать все репозитории (включая неактивные)"},
			},
		},
		{
			Handler:      w.Add,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo",
			ResponseType: reflect.TypeOf(RepoAddRemoveResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Добавить репозиторий",
			Tags:         []string{"repo"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "source", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "date", Source: "body", Type: "string", Default: "", ArgIndex: 2},
			},
		},
		{
			Handler:      w.CheckAdd,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo/check",
			ResponseType: reflect.TypeOf(RepoSimulateResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Симулировать добавление репозитория",
			Tags:         []string{"repo"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "source", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "date", Source: "body", Type: "string", Default: "", ArgIndex: 2},
			},
		},
		{
			Handler:      w.Remove,
			HTTPMethod:   "DELETE",
			HTTPPath:     "/api/v1/repo",
			ResponseType: reflect.TypeOf(RepoAddRemoveResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Удалить репозиторий",
			Tags:         []string{"repo"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "source", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "date", Source: "body", Type: "string", Default: "", ArgIndex: 2},
			},
		},
		{
			Handler:      w.CheckRemove,
			HTTPMethod:   "DELETE",
			HTTPPath:     "/api/v1/repo/check",
			ResponseType: reflect.TypeOf(RepoSimulateResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Симулировать удаление репозитория",
			Tags:         []string{"repo"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "source", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "date", Source: "body", Type: "string", Default: "", ArgIndex: 2},
			},
		},
		{
			Handler:      w.Set,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo/set",
			ResponseType: reflect.TypeOf(RepoSetResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Установить ветку (удалить все и добавить указанную)",
			Tags:         []string{"repo"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "branch", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "date", Source: "body", Type: "string", Default: "", ArgIndex: 2},
			},
		},
		{
			Handler:      w.CheckSet,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo/set/check",
			ResponseType: reflect.TypeOf(RepoSimulateResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Симулировать установку ветки",
			Tags:         []string{"repo"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "branch", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "date", Source: "body", Type: "string", Default: "", ArgIndex: 2},
			},
		},
		{
			Handler:      w.Clean,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo/clean",
			ResponseType: reflect.TypeOf(RepoAddRemoveResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Удалить временные репозитории (cdrom, task)",
			Tags:         []string{"repo"},
		},
		{
			Handler:      w.CheckClean,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo/clean/check",
			ResponseType: reflect.TypeOf(RepoSimulateResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Симулировать удаление временных репозиториев",
			Tags:         []string{"repo"},
		},
		{
			Handler:      w.GetBranches,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/repo/branches",
			ResponseType: reflect.TypeOf(BranchesResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить список доступных веток",
			Tags:         []string{"repo"},
		},
		{
			Handler:      w.GetTaskPackages,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/repo/task/{taskNum}",
			ResponseType: reflect.TypeOf(TaskPackagesResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить список пакетов из задачи",
			Tags:         []string{"repo"},
			PathParams:   []string{"taskNum"},
		},
		{
			Handler:      w.TestTask,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo/task/{taskNum}/test",
			ResponseType: reflect.TypeOf(TestTaskResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Тестировать пакеты из задачи",
			Tags:         []string{"repo"},
			PathParams:   []string{"taskNum"},
		},
	}
}
