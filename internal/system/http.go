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
	"io"
	"net/http"
	"reflect"
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
	_ = json.NewEncoder(rw).Encode(resp)
}

// parseBodyParams парсит параметры из тела запроса
func (w *HTTPWrapper) parseBodyParams(r *http.Request) (map[string]json.RawMessage, error) {
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("request body is required")
		}
		return nil, err
	}
	return body, nil
}

// CheckRemove – Проверить пакеты перед удалением
func (w *HTTPWrapper) CheckRemove(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
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

	background := r.URL.Query().Get("background") == "true"
	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.CheckRemove(ctx, packages, purge, depends)
			reply.SendTaskResult(ctx, reply.EventSystemCheckRemove, resp, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(reply.OK(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		}))
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.CheckRemove(ctx, packages, purge, depends)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
}

// CheckInstall – Проверить пакеты перед установкой
func (w *HTTPWrapper) CheckInstall(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var packages []string

	if err = reply.UnmarshalField(body, "packages", &packages); err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	background := r.URL.Query().Get("background") == "true"
	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.CheckInstall(ctx, packages)
			reply.SendTaskResult(ctx, reply.EventSystemCheckInstall, resp, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(reply.OK(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		}))
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.CheckInstall(ctx, packages)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
}

// CheckUpgrade – Проверить пакеты перед обновлением системы
func (w *HTTPWrapper) CheckUpgrade(rw http.ResponseWriter, r *http.Request) {
	background := r.URL.Query().Get("background") == "true"

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.CheckUpgrade(ctx)
			reply.SendTaskResult(ctx, reply.EventSystemCheckUpgrade, resp, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(reply.OK(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		}))
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.CheckUpgrade(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
}

// Remove – Удалить пакеты
func (w *HTTPWrapper) Remove(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
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

	background := r.URL.Query().Get("background") == "true"
	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.Remove(ctx, packages, purge, depends, true)
			reply.SendTaskResult(ctx, reply.EventSystemRemove, resp, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(reply.OK(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		}))
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Remove(ctx, packages, purge, depends, true)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
}

// Install – Установить пакеты
func (w *HTTPWrapper) Install(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var packages []string

	if err = reply.UnmarshalField(body, "packages", &packages); err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	background := r.URL.Query().Get("background") == "true"
	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.Install(ctx, packages, true)
			reply.SendTaskResult(ctx, reply.EventSystemInstall, resp, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(reply.OK(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		}))
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Install(ctx, packages, true)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
}

// Info – Получить информацию о пакете
func (w *HTTPWrapper) Info(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	full := r.URL.Query().Get("full") == "true"

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Info(ctx, name)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(map[string]interface{}{
		"message":     resp.Message,
		"packageInfo": w.actions.FormatPackageOutput(resp.PackageInfo, full),
	}))
}

// MultiInfo – Получить информацию о нескольких пакетах
func (w *HTTPWrapper) MultiInfo(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
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

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.MultiInfo(ctx, packages)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}

	w.writeJSON(rw, reply.OK(map[string]interface{}{
		"message":  resp.Message,
		"packages": w.actions.FormatPackageOutput(resp.Packages, full),
		"notFound": resp.NotFound,
	}))
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
	resp, err := w.actions.List(ctx, params)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(map[string]interface{}{
		"message":    resp.Message,
		"packages":   w.actions.FormatPackageOutput(resp.Packages, full),
		"totalCount": resp.TotalCount,
	}))
}

// GetFilterFields – Получить доступные поля для фильтрации
func (w *HTTPWrapper) GetFilterFields(rw http.ResponseWriter, r *http.Request) {
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.GetFilterFields(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
}

// Search – Поиск пакетов по названию
func (w *HTTPWrapper) Search(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	q := query.Get("q")
	installed := query.Get("installed") == "true"
	full := query.Get("full") == "true"

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Search(ctx, q, installed)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(map[string]interface{}{
		"message":  resp.Message,
		"packages": w.actions.FormatPackageOutput(resp.Packages, full),
	}))
}

// Update – Обновить базу данных пакетов
func (w *HTTPWrapper) Update(rw http.ResponseWriter, r *http.Request) {
	noLock := r.URL.Query().Get("noLock") == "true"
	background := r.URL.Query().Get("background") == "true"

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.Update(ctx, noLock)
			reply.SendTaskResult(ctx, reply.EventSystemUpdate, resp, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(reply.OK(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		}))
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Update(ctx, noLock)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
}

// Upgrade – Обновить систему
func (w *HTTPWrapper) Upgrade(rw http.ResponseWriter, r *http.Request) {
	background := r.URL.Query().Get("background") == "true"

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.Upgrade(ctx)
			reply.SendTaskResult(ctx, reply.EventSystemUpgrade, resp, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(reply.OK(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		}))
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Upgrade(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
}

// --- Image (atomic only) ---

// ImageStatus – Получить статус образа
func (w *HTTPWrapper) ImageStatus(rw http.ResponseWriter, r *http.Request) {
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.ImageStatus(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
}

// ImageUpdate – Обновить образ
func (w *HTTPWrapper) ImageUpdate(rw http.ResponseWriter, r *http.Request) {
	background := r.URL.Query().Get("background") == "true"

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.ImageUpdate(ctx)
			reply.SendTaskResult(ctx, reply.EventSystemImageUpdate, resp, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(reply.OK(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		}))
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.ImageUpdate(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
}

// ImageApply – Применить изменения к образу
func (w *HTTPWrapper) ImageApply(rw http.ResponseWriter, r *http.Request) {
	background := r.URL.Query().Get("background") == "true"

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.ImageApply(ctx)
			reply.SendTaskResult(ctx, reply.EventSystemImageApply, resp, err)
		}()

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(rw).Encode(reply.OK(BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: txID,
		}))
		return
	}

	// Синхронное выполнение
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.ImageApply(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
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
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
}

// ImageGetConfig – Получить конфигурацию образа
func (w *HTTPWrapper) ImageGetConfig(rw http.ResponseWriter, r *http.Request) {
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.ImageGetConfig(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
}

// ImageSaveConfig – Сохранить конфигурацию образа
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

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.ImageSaveConfig(ctx, config)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.writeJSON(rw, reply.OK(resp))
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
	mux.HandleFunc("POST /api/v1/packages/info", w.MultiInfo)
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
			ResponseType: reflect.TypeOf(CheckResponse{}),
			Permission:   "read",
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
			Method:       "CheckInstall",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/packages/check-install",
			ResponseType: reflect.TypeOf(CheckResponse{}),
			Permission:   "read",
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
			Method:       "CheckUpgrade",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/system/check-upgrade",
			ResponseType: reflect.TypeOf(CheckResponse{}),
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
			ResponseType: reflect.TypeOf(InstallRemoveResponse{}),
			Permission:   "manage",
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
			Method:       "Install",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/packages/install",
			ResponseType: reflect.TypeOf(InstallRemoveResponse{}),
			Permission:   "manage",
			Summary:      "Установить пакеты",
			Tags:         []string{"packages"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "packages", Source: "body", Type: "[]string", ArgIndex: 1},
			},
			QueryParams: []http_server.QueryParam{
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},

		// Packages - информация
		{
			Method:       "Info",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/packages/{name}",
			ResponseType: reflect.TypeOf(InfoResponse{}),
			Permission:   "read",
			Summary:      "Получить информацию о пакете",
			Tags:         []string{"packages"},
			PathParams:   []string{"name"},
			QueryParams: []http_server.QueryParam{
				{Name: "full", Type: "boolean", Required: false, Description: "Полный формат вывода"},
			},
		},
		{
			Method:       "MultiInfo",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/packages/info",
			ResponseType: reflect.TypeOf(MultiInfoResponse{}),
			Permission:   "read",
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
			Method:       "List",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/packages",
			ResponseType: reflect.TypeOf(ListResponse{}),
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
			ResponseType: reflect.TypeOf(GetFilterFieldsResponse{}),
			Permission:   "read",
			Summary:      "Получить доступные поля для фильтрации",
			Tags:         []string{"packages"},
		},
		{
			Method:       "Search",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/packages/search",
			ResponseType: reflect.TypeOf(SearchResponse{}),
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
			ResponseType: reflect.TypeOf(UpdateResponse{}),
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
			ResponseType: reflect.TypeOf(UpgradeResponse{}),
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
			ResponseType: reflect.TypeOf(ImageStatusResponse{}),
			Permission:   "read",
			Summary:      "Получить статус образа",
			Tags:         []string{"image"},
		},
		{
			Method:       "ImageUpdate",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/image/update",
			ResponseType: reflect.TypeOf(ImageUpdateResponse{}),
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
			ResponseType: reflect.TypeOf(ImageApplyResponse{}),
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
			ResponseType: reflect.TypeOf(ImageHistoryResponse{}),
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
			ResponseType: reflect.TypeOf(ImageConfigResponse{}),
			Permission:   "read",
			Summary:      "Получить конфигурацию образа",
			Tags:         []string{"image"},
		},
		{
			Method:       "ImageSaveConfig",
			HTTPMethod:   "PUT",
			HTTPPath:     "/api/v1/image/config",
			RequestType:  reflect.TypeOf(build.Config{}),
			ResponseType: reflect.TypeOf(ImageConfigResponse{}),
			Permission:   "manage",
			Summary:      "Сохранить конфигурацию образа",
			Tags:         []string{"image"},
		},
	}
}
