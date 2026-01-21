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
	"apm/internal/common/app"
	"apm/internal/common/build"
	"apm/internal/common/helper"
	"apm/internal/common/http_server"
	"apm/internal/common/reply"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HTTPWrapper – обёртка для системных действий, предназначенная для экспорта через HTTP.
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

// ctxWithTransactionOrGenerate создает контекст с transaction, генерируя его если не передан
func (w *HTTPWrapper) ctxWithTransactionOrGenerate(r *http.Request) (context.Context, string) {
	tx := r.Header.Get("X-Transaction-ID")
	if tx == "" {
		tx = r.URL.Query().Get("transaction")
	}
	if tx == "" {
		tx = generateTransactionID()
	}
	return context.WithValue(w.ctx, helper.TransactionKey, tx), tx
}

// generateTransactionID генерирует уникальный ID транзакции
func generateTransactionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
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
		return nil, err
	}
	return body, nil
}

// CheckRemove – Проверить пакеты перед удалением
func (w *HTTPWrapper) CheckRemove(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		w.writeError(rw, err, http.StatusBadRequest)
		return
	}

	var packages []string
	if raw, ok := body["packages"]; ok {
		_ = json.Unmarshal(raw, &packages)
	}

	var purge, depends, background bool
	if raw, ok := body["purge"]; ok {
		_ = json.Unmarshal(raw, &purge)
	}
	if raw, ok := body["depends"]; ok {
		_ = json.Unmarshal(raw, &depends)
	}
	if raw, ok := body["background"]; ok {
		_ = json.Unmarshal(raw, &background)
	}

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.CheckRemove(ctx, packages, purge, depends)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.CheckRemove", data, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		})
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.CheckRemove(ctx, packages, purge, depends)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// CheckInstall – Проверить пакеты перед установкой
func (w *HTTPWrapper) CheckInstall(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		w.writeError(rw, err, http.StatusBadRequest)
		return
	}

	var packages []string
	var background bool
	if raw, ok := body["packages"]; ok {
		_ = json.Unmarshal(raw, &packages)
	}
	if raw, ok := body["background"]; ok {
		_ = json.Unmarshal(raw, &background)
	}

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.CheckInstall(ctx, packages)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.CheckInstall", data, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		})
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.CheckInstall(ctx, packages)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// CheckUpgrade – Проверить пакеты перед обновлением системы
func (w *HTTPWrapper) CheckUpgrade(rw http.ResponseWriter, r *http.Request) {
	background := r.URL.Query().Get("background") == "true"

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.CheckUpgrade(ctx)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.CheckUpgrade", data, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		})
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.CheckUpgrade(ctx)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Remove – Удалить пакеты
func (w *HTTPWrapper) Remove(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		w.writeError(rw, err, http.StatusBadRequest)
		return
	}

	var packages []string
	if raw, ok := body["packages"]; ok {
		_ = json.Unmarshal(raw, &packages)
	}

	purge := false
	depends := false
	background := false

	if raw, ok := body["purge"]; ok {
		_ = json.Unmarshal(raw, &purge)
	}
	if raw, ok := body["depends"]; ok {
		_ = json.Unmarshal(raw, &depends)
	}
	if raw, ok := body["background"]; ok {
		_ = json.Unmarshal(raw, &background)
	}

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.Remove(ctx, packages, purge, depends, true)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.Remove", data, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		})
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Remove(ctx, packages, purge, depends, true)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Install – Установить пакеты
func (w *HTTPWrapper) Install(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		w.writeError(rw, err, http.StatusBadRequest)
		return
	}

	var packages []string
	var background bool
	if raw, ok := body["packages"]; ok {
		_ = json.Unmarshal(raw, &packages)
	}
	if raw, ok := body["background"]; ok {
		_ = json.Unmarshal(raw, &background)
	}

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.Install(ctx, packages, true)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.Install", data, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		})
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Install(ctx, packages, true)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Info – Получить информацию о пакете
func (w *HTTPWrapper) Info(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	full := r.URL.Query().Get("full") == "true"

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Info(ctx, name, full)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// List – Получить список пакетов
func (w *HTTPWrapper) List(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
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

	var filters []string
	filtersParam := query["filters"]
	for _, f := range filtersParam {
		if strings.Contains(f, ",") {
			filters = append(filters, strings.Split(f, ",")...)
		} else {
			filters = append(filters, f)
		}
	}

	params := ListParams{
		Sort:        query.Get("sort"),
		Order:       query.Get("order"),
		Limit:       limit,
		Offset:      offset,
		Filters:     filters,
		ForceUpdate: query.Get("forceUpdate") == "true",
	}

	full := query.Get("full") != "false"

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.List(ctx, params, full)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// GetFilterFields – Получить доступные поля для фильтрации
func (w *HTTPWrapper) GetFilterFields(rw http.ResponseWriter, r *http.Request) {
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.GetFilterFields(ctx)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Search – Поиск пакетов по названию
func (w *HTTPWrapper) Search(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	q := query.Get("q")
	installed := query.Get("installed") == "true"
	full := query.Get("full") == "true"

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Search(ctx, q, installed, full)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Update – Обновить базу данных пакетов
func (w *HTTPWrapper) Update(rw http.ResponseWriter, r *http.Request) {
	noLock := r.URL.Query().Get("noLock") == "true"
	background := r.URL.Query().Get("background") == "true"

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.Update(ctx, noLock)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.Update", data, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		})
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Update(ctx, noLock)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Upgrade – Обновить систему
func (w *HTTPWrapper) Upgrade(rw http.ResponseWriter, r *http.Request) {
	background := r.URL.Query().Get("background") == "true"

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.Upgrade(ctx)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.Upgrade", data, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		})
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Upgrade(ctx)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// --- Image (atomic only) ---

// ImageStatus – Получить статус образа
func (w *HTTPWrapper) ImageStatus(rw http.ResponseWriter, r *http.Request) {
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.ImageStatus(ctx)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// ImageUpdate – Обновить образ
func (w *HTTPWrapper) ImageUpdate(rw http.ResponseWriter, r *http.Request) {
	background := r.URL.Query().Get("background") == "true"

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.ImageUpdate(ctx)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.ImageUpdate", data, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		})
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.ImageUpdate(ctx)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// ImageApply – Применить изменения к образу
func (w *HTTPWrapper) ImageApply(rw http.ResponseWriter, r *http.Request) {
	background := r.URL.Query().Get("background") == "true"

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.ImageApply(ctx)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.ImageApply", data, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		})
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.ImageApply(ctx)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// ImageHistory – Получить историю изменений образа
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

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.ImageHistory(ctx, imageName, limit, offset)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// ImageGetConfig – Получить конфигурацию образа
func (w *HTTPWrapper) ImageGetConfig(rw http.ResponseWriter, r *http.Request) {
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.ImageGetConfig(ctx)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// ImageSaveConfig – Сохранить конфигурацию образа
func (w *HTTPWrapper) ImageSaveConfig(rw http.ResponseWriter, r *http.Request) {
	var config build.Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		if err.Error() == "EOF" {
			w.writeError(rw, errors.New("request body is required"), http.StatusBadRequest)
			return
		}
		w.writeError(rw, fmt.Errorf("invalid JSON: %w", err), http.StatusBadRequest)
		return
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.ImageSaveConfig(ctx, config)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// RegisterRoutes регистрирует все HTTP маршруты в mux
func (w *HTTPWrapper) RegisterRoutes(mux *http.ServeMux, isAtomic bool) {
	// Packages - проверки
	mux.HandleFunc("POST /api/v1/packages/check-remove", w.CheckRemove)
	mux.HandleFunc("POST /api/v1/packages/check-install", w.CheckInstall)
	mux.HandleFunc("GET /api/v1/system/check-upgrade", w.CheckUpgrade)

	// Packages - действия
	mux.HandleFunc("POST /api/v1/packages/remove", w.Remove)
	mux.HandleFunc("POST /api/v1/packages/install", w.Install)

	// Packages - информация
	mux.HandleFunc("GET /api/v1/packages/filter-fields", w.GetFilterFields)
	mux.HandleFunc("GET /api/v1/packages/search", w.Search)
	mux.HandleFunc("GET /api/v1/packages/{name}", w.Info)
	mux.HandleFunc("GET /api/v1/packages", w.List)

	// System
	mux.HandleFunc("POST /api/v1/system/update", w.Update)
	mux.HandleFunc("POST /api/v1/system/upgrade", w.Upgrade)

	// Image (только для atomic)
	if isAtomic {
		mux.HandleFunc("GET /api/v1/image/status", w.ImageStatus)
		mux.HandleFunc("POST /api/v1/image/update", w.ImageUpdate)
		mux.HandleFunc("POST /api/v1/image/apply", w.ImageApply)
		mux.HandleFunc("GET /api/v1/image/history", w.ImageHistory)
		mux.HandleFunc("GET /api/v1/image/config", w.ImageGetConfig)
		mux.HandleFunc("PUT /api/v1/image/config", w.ImageSaveConfig)
	}
}

// GetHTTPEndpoints возвращает описания endpoints для OpenAPI документации
func GetHTTPEndpoints() []http_server.Endpoint {
	return []http_server.Endpoint{
		{
			Method:       "CheckRemove",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/packages/check-remove",
			ResponseType: "CheckResponse",
			Permission:   "read",
			Summary:      "Проверить пакеты перед удалением",
			Tags:         []string{"packages"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "packages", Source: "body", Type: "[]string", ArgIndex: 1},
				{Name: "purge", Source: "body", Type: "bool", Default: "false", ArgIndex: 2},
				{Name: "depends", Source: "body", Type: "bool", Default: "false", ArgIndex: 3},
				{Name: "background", Source: "body", Type: "bool", Default: "false"},
			},
		},
		{
			Method:       "CheckInstall",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/packages/check-install",
			ResponseType: "CheckResponse",
			Permission:   "read",
			Summary:      "Проверить пакеты перед установкой",
			Tags:         []string{"packages"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "packages", Source: "body", Type: "[]string", ArgIndex: 1},
				{Name: "background", Source: "body", Type: "bool", Default: "false"},
			},
		},
		{
			Method:       "CheckUpgrade",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/system/check-upgrade",
			ResponseType: "CheckResponse",
			Permission:   "read",
			Summary:      "Проверить пакеты перед обновлением системы",
			Tags:         []string{"system"},
			QueryParams: []http_server.QueryParam{
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},

		// Packages - действия
		{
			Method:       "Remove",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/packages/remove",
			ResponseType: "InstallRemoveResponse",
			Permission:   "manage",
			Summary:      "Удалить пакеты",
			Tags:         []string{"packages"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "packages", Source: "body", Type: "[]string", ArgIndex: 1},
				{Name: "purge", Source: "body", Type: "bool", Default: "false", ArgIndex: 2},
				{Name: "depends", Source: "body", Type: "bool", Default: "false", ArgIndex: 3},
				{Name: "background", Source: "body", Type: "bool", Default: "false"},
			},
		},
		{
			Method:       "Install",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/packages/install",
			ResponseType: "InstallRemoveResponse",
			Permission:   "manage",
			Summary:      "Установить пакеты",
			Tags:         []string{"packages"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "packages", Source: "body", Type: "[]string", ArgIndex: 1},
				{Name: "background", Source: "body", Type: "bool", Default: "false"},
			},
		},

		// Packages - информация
		{
			Method:       "Info",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/packages/{name}",
			ResponseType: "InfoResponse",
			Permission:   "read",
			Summary:      "Получить информацию о пакете",
			Tags:         []string{"packages"},
			PathParams:   []string{"name"},
			QueryParams: []http_server.QueryParam{
				{Name: "full", Type: "boolean", Required: false, Description: "Полный формат вывода"},
			},
		},
		{
			Method:       "List",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/packages",
			ResponseType: "ListResponse",
			Permission:   "read",
			Summary:      "Получить список пакетов",
			Tags:         []string{"packages"},
			QueryParams: []http_server.QueryParam{
				{Name: "sort", Type: "string", Required: false, Description: "Поле сортировки"},
				{Name: "order", Type: "string", Required: false, Description: "Порядок сортировки (asc/desc)"},
				{Name: "limit", Type: "integer", Required: false, Description: "Лимит записей (по умолчанию 50)"},
				{Name: "offset", Type: "integer", Required: false, Description: "Смещение"},
				{Name: "filters", Type: "string", Required: false, Description: "Фильтры (можно несколько)"},
				{Name: "forceUpdate", Type: "boolean", Required: false, Description: "Принудительное обновление базы"},
				{Name: "full", Type: "boolean", Required: false, Description: "Полный формат вывода"},
			},
		},
		{
			Method:       "GetFilterFields",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/packages/filter-fields",
			ResponseType: "GetFilterFieldsResponse",
			Permission:   "read",
			Summary:      "Получить доступные поля для фильтрации",
			Tags:         []string{"packages"},
		},
		{
			Method:       "Search",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/packages/search",
			ResponseType: "SearchResponse",
			Permission:   "read",
			Summary:      "Поиск пакетов по названию",
			Tags:         []string{"packages"},
			QueryParams: []http_server.QueryParam{
				{Name: "q", Type: "string", Required: true, Description: "Поисковый запрос"},
				{Name: "installed", Type: "boolean", Required: false, Description: "Искать только установленные"},
				{Name: "full", Type: "boolean", Required: false, Description: "Полный формат вывода"},
			},
		},

		// System
		{
			Method:       "Update",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/system/update",
			ResponseType: "UpdateResponse",
			Permission:   "manage",
			Summary:      "Обновить базу данных пакетов",
			Tags:         []string{"system"},
			QueryParams: []http_server.QueryParam{
				{Name: "noLock", Type: "boolean", Required: false, Description: "Не блокировать базу"},
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},
		{
			Method:       "Upgrade",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/system/upgrade",
			ResponseType: "UpgradeResponse",
			Permission:   "manage",
			Summary:      "Обновить систему",
			Tags:         []string{"system"},
			QueryParams: []http_server.QueryParam{
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},

		// Image (atomic only)
		{
			Method:       "ImageStatus",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/image/status",
			ResponseType: "ImageStatusResponse",
			Permission:   "read",
			Summary:      "Получить статус образа",
			Tags:         []string{"image"},
		},
		{
			Method:       "ImageUpdate",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/image/update",
			ResponseType: "ImageUpdateResponse",
			Permission:   "manage",
			Summary:      "Обновить образ",
			Tags:         []string{"image"},
			QueryParams: []http_server.QueryParam{
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},
		{
			Method:       "ImageApply",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/image/apply",
			ResponseType: "ImageApplyResponse",
			Permission:   "manage",
			Summary:      "Применить изменения к образу",
			Tags:         []string{"image"},
			QueryParams: []http_server.QueryParam{
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},
		{
			Method:       "ImageHistory",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/image/history",
			ResponseType: "ImageHistoryResponse",
			Permission:   "read",
			Summary:      "Получить историю изменений образа",
			Tags:         []string{"image"},
			QueryParams: []http_server.QueryParam{
				{Name: "imageName", Type: "string", Required: false, Description: "Имя образа"},
				{Name: "limit", Type: "integer", Required: false, Description: "Лимит записей"},
				{Name: "offset", Type: "integer", Required: false, Description: "Смещение"},
			},
		},
		{
			Method:       "ImageGetConfig",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/image/config",
			ResponseType: "ImageConfigResponse",
			Permission:   "read",
			Summary:      "Получить конфигурацию образа",
			Tags:         []string{"image"},
		},
		{
			Method:       "ImageSaveConfig",
			HTTPMethod:   "PUT",
			HTTPPath:     "/api/v1/image/config",
			RequestType:  "ImageConfigRequest",
			ResponseType: "ImageConfigResponse",
			Permission:   "manage",
			Summary:      "Сохранить конфигурацию образа",
			Tags:         []string{"image"},
		},
	}
}
