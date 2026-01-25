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

package repo

import (
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"apm/internal/common/http_server"
	"apm/internal/common/reply"
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// HTTPWrapper – обёртка для действий с репозиториями, предназначенная для экспорта через HTTP.
type HTTPWrapper struct {
	actions   *Actions
	ctx       context.Context
	appConfig *app.Config
}

// NewHTTPWrapper создаёт новую обёртку над actions
func NewHTTPWrapper(a *Actions, appConfig *app.Config, ctx context.Context) *HTTPWrapper {
	return &HTTPWrapper{actions: a, appConfig: appConfig, ctx: ctx}
}

// ctxWithTransaction создает контекст с transaction из запроса
func (w *HTTPWrapper) ctxWithTransaction(r *http.Request) context.Context {
	tx := r.Header.Get("X-Transaction-ID")
	if tx == "" {
		tx = r.URL.Query().Get("transaction")
	}
	return context.WithValue(w.ctx, helper.TransactionKey, tx)
}

// writeJSON отправляет JSON ответ
func (w *HTTPWrapper) writeJSON(rw http.ResponseWriter, resp reply.APIResponse) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	if resp.Error {
		rw.WriteHeader(http.StatusBadRequest)
	}
	_ = json.NewEncoder(rw).Encode(resp)
}

// writeError отправляет ошибку
func (w *HTTPWrapper) writeError(rw http.ResponseWriter, err error, code int) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.WriteHeader(code)
	_ = json.NewEncoder(rw).Encode(reply.APIResponse{
		Data:  map[string]interface{}{"message": err.Error()},
		Error: true,
	})
}

// parseBodyParams парсит параметры из тела запроса
func (w *HTTPWrapper) parseBodyParams(r *http.Request) (map[string]json.RawMessage, error) {
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if err.Error() == "EOF" {
			return nil, errors.New("request body is required")
		}
		return nil, err
	}
	return body, nil
}

// --- Репозитории ---

// List – Получить список репозиториев
func (w *HTTPWrapper) List(rw http.ResponseWriter, r *http.Request) {
	all := r.URL.Query().Get("all") == "true"

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.List(ctx, all)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Add – Добавить репозиторий
func (w *HTTPWrapper) Add(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		w.writeError(rw, err, http.StatusBadRequest)
		return
	}

	var source string
	var date string

	if raw, ok := body["source"]; ok {
		_ = json.Unmarshal(raw, &source)
	}
	if raw, ok := body["date"]; ok {
		_ = json.Unmarshal(raw, &date)
	}

	if source == "" {
		w.writeError(rw, errors.New("source is required"), http.StatusBadRequest)
		return
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Add(ctx, []string{source}, date)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// CheckAdd – Симулировать добавление репозитория
func (w *HTTPWrapper) CheckAdd(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		w.writeError(rw, err, http.StatusBadRequest)
		return
	}

	var source string
	var date string

	if raw, ok := body["source"]; ok {
		_ = json.Unmarshal(raw, &source)
	}
	if raw, ok := body["date"]; ok {
		_ = json.Unmarshal(raw, &date)
	}

	if source == "" {
		w.writeError(rw, errors.New("source is required"), http.StatusBadRequest)
		return
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.CheckAdd(ctx, []string{source}, date)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Remove – Удалить репозиторий
func (w *HTTPWrapper) Remove(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		w.writeError(rw, err, http.StatusBadRequest)
		return
	}

	var source string
	var date string

	if raw, ok := body["source"]; ok {
		_ = json.Unmarshal(raw, &source)
	}
	if raw, ok := body["date"]; ok {
		_ = json.Unmarshal(raw, &date)
	}

	if source == "" {
		w.writeError(rw, errors.New("source is required"), http.StatusBadRequest)
		return
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Remove(ctx, []string{source}, date)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// CheckRemove – Симулировать удаление репозитория
func (w *HTTPWrapper) CheckRemove(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		w.writeError(rw, err, http.StatusBadRequest)
		return
	}

	var source string
	var date string

	if raw, ok := body["source"]; ok {
		_ = json.Unmarshal(raw, &source)
	}
	if raw, ok := body["date"]; ok {
		_ = json.Unmarshal(raw, &date)
	}

	if source == "" {
		w.writeError(rw, errors.New("source is required"), http.StatusBadRequest)
		return
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.CheckRemove(ctx, []string{source}, date)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Set – Установить ветку
func (w *HTTPWrapper) Set(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		w.writeError(rw, err, http.StatusBadRequest)
		return
	}

	var branch string
	var date string

	if raw, ok := body["branch"]; ok {
		_ = json.Unmarshal(raw, &branch)
	}
	if raw, ok := body["date"]; ok {
		_ = json.Unmarshal(raw, &date)
	}

	if branch == "" {
		w.writeError(rw, errors.New("branch is required"), http.StatusBadRequest)
		return
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Set(ctx, branch, date)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// CheckSet – Симулировать установку ветки
func (w *HTTPWrapper) CheckSet(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		w.writeError(rw, err, http.StatusBadRequest)
		return
	}

	var branch string
	var date string

	if raw, ok := body["branch"]; ok {
		_ = json.Unmarshal(raw, &branch)
	}
	if raw, ok := body["date"]; ok {
		_ = json.Unmarshal(raw, &date)
	}

	if branch == "" {
		w.writeError(rw, errors.New("branch is required"), http.StatusBadRequest)
		return
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.CheckSet(ctx, branch, date)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Clean – Удалить временные репозитории (cdrom, task)
func (w *HTTPWrapper) Clean(rw http.ResponseWriter, r *http.Request) {
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Clean(ctx)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// CheckClean – Симулировать удаление временных репозиториев
func (w *HTTPWrapper) CheckClean(rw http.ResponseWriter, r *http.Request) {
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.CheckClean(ctx)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// GetBranches – Получить список доступных веток
func (w *HTTPWrapper) GetBranches(rw http.ResponseWriter, r *http.Request) {
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.GetBranches(ctx)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// GetTaskPackages – Получить список пакетов из задачи
func (w *HTTPWrapper) GetTaskPackages(rw http.ResponseWriter, r *http.Request) {
	taskNum := r.PathValue("taskNum")

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.GetTaskPackages(ctx, taskNum)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// TestTask – Тестировать пакеты из задачи
func (w *HTTPWrapper) TestTask(rw http.ResponseWriter, r *http.Request) {
	taskNum := r.PathValue("taskNum")

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.TestTask(ctx, taskNum)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// RegisterRoutes регистрирует все HTTP маршруты в mux
func (w *HTTPWrapper) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/repo", w.List)
	mux.HandleFunc("POST /api/v1/repo", w.Add)
	mux.HandleFunc("POST /api/v1/repo/check", w.CheckAdd)
	mux.HandleFunc("DELETE /api/v1/repo", w.Remove)
	mux.HandleFunc("DELETE /api/v1/repo/check", w.CheckRemove)
	mux.HandleFunc("POST /api/v1/repo/set", w.Set)
	mux.HandleFunc("POST /api/v1/repo/set/check", w.CheckSet)
	mux.HandleFunc("POST /api/v1/repo/clean", w.Clean)
	mux.HandleFunc("POST /api/v1/repo/clean/check", w.CheckClean)
	mux.HandleFunc("GET /api/v1/repo/branches", w.GetBranches)
	mux.HandleFunc("GET /api/v1/repo/task/{taskNum}", w.GetTaskPackages)
	mux.HandleFunc("POST /api/v1/repo/task/{taskNum}/test", w.TestTask)
}

// GetHTTPEndpoints возвращает описания endpoints для OpenAPI документации
func GetHTTPEndpoints() []http_server.Endpoint {
	return []http_server.Endpoint{
		{
			Method:       "List",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/repo",
			ResponseType: "RepoListResponse",
			Permission:   "read",
			Summary:      "Получить список репозиториев",
			Tags:         []string{"repo"},
			QueryParams: []http_server.QueryParam{
				{Name: "all", Type: "boolean", Required: false, Description: "Показать все репозитории (включая неактивные)"},
			},
		},
		{
			Method:       "Add",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo",
			ResponseType: "RepoAddRemoveResponse",
			Permission:   "manage",
			Summary:      "Добавить репозиторий",
			Tags:         []string{"repo"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "source", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "date", Source: "body", Type: "string", Default: "", ArgIndex: 2},
			},
		},
		{
			Method:       "CheckAdd",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo/check",
			ResponseType: "RepoSimulateResponse",
			Permission:   "read",
			Summary:      "Симулировать добавление репозитория",
			Tags:         []string{"repo"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "source", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "date", Source: "body", Type: "string", Default: "", ArgIndex: 2},
			},
		},
		{
			Method:       "Remove",
			HTTPMethod:   "DELETE",
			HTTPPath:     "/api/v1/repo",
			ResponseType: "RepoAddRemoveResponse",
			Permission:   "manage",
			Summary:      "Удалить репозиторий",
			Tags:         []string{"repo"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "source", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "date", Source: "body", Type: "string", Default: "", ArgIndex: 2},
			},
		},
		{
			Method:       "CheckRemove",
			HTTPMethod:   "DELETE",
			HTTPPath:     "/api/v1/repo/check",
			ResponseType: "RepoSimulateResponse",
			Permission:   "read",
			Summary:      "Симулировать удаление репозитория",
			Tags:         []string{"repo"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "source", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "date", Source: "body", Type: "string", Default: "", ArgIndex: 2},
			},
		},
		{
			Method:       "Set",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo/set",
			ResponseType: "RepoSetResponse",
			Permission:   "manage",
			Summary:      "Установить ветку (удалить все и добавить указанную)",
			Tags:         []string{"repo"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "branch", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "date", Source: "body", Type: "string", Default: "", ArgIndex: 2},
			},
		},
		{
			Method:       "CheckSet",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo/set/check",
			ResponseType: "RepoSimulateResponse",
			Permission:   "read",
			Summary:      "Симулировать установку ветки",
			Tags:         []string{"repo"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "branch", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "date", Source: "body", Type: "string", Default: "", ArgIndex: 2},
			},
		},
		{
			Method:       "Clean",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo/clean",
			ResponseType: "RepoAddRemoveResponse",
			Permission:   "manage",
			Summary:      "Удалить временные репозитории (cdrom, task)",
			Tags:         []string{"repo"},
		},
		{
			Method:       "CheckClean",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo/clean/check",
			ResponseType: "RepoSimulateResponse",
			Permission:   "read",
			Summary:      "Симулировать удаление временных репозиториев",
			Tags:         []string{"repo"},
		},
		{
			Method:       "GetBranches",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/repo/branches",
			ResponseType: "BranchesResponse",
			Permission:   "read",
			Summary:      "Получить список доступных веток",
			Tags:         []string{"repo"},
		},
		{
			Method:       "GetTaskPackages",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/repo/task/{taskNum}",
			ResponseType: "TaskPackagesResponse",
			Permission:   "read",
			Summary:      "Получить список пакетов из задачи",
			Tags:         []string{"repo"},
			PathParams:   []string{"taskNum"},
		},
		{
			Method:       "TestTask",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/repo/task/{taskNum}/test",
			ResponseType: "TestTaskResponse",
			Permission:   "manage",
			Summary:      "Тестировать пакеты из задачи",
			Tags:         []string{"repo"},
			PathParams:   []string{"taskNum"},
		},
	}
}
