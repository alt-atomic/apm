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
	"apm/internal/common/app"
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

// HTTPWrapper – обёртка для действий с distrobox, предназначенная для экспорта через HTTP.
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
		if err.Error() == "EOF" {
			return nil, errors.New("request body is required")
		}
		return nil, err
	}
	return body, nil
}

// Update – Обновить список пакетов в контейнере
func (w *HTTPWrapper) Update(rw http.ResponseWriter, r *http.Request) {
	container := r.URL.Query().Get("container")
	if container == "" {
		w.writeError(rw, errors.New("container is required"), http.StatusBadRequest)
		return
	}

	background := r.URL.Query().Get("background") == "true"

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.Update(ctx, container)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "distrobox.Update", data, err)
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
	resp, err := w.actions.Update(ctx, container)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Info – Получить информацию о пакете
func (w *HTTPWrapper) Info(rw http.ResponseWriter, r *http.Request) {
	container := r.URL.Query().Get("container")
	if container == "" {
		w.writeError(rw, errors.New("container is required"), http.StatusBadRequest)
		return
	}

	name := r.PathValue("name")
	if name == "" {
		w.writeError(rw, errors.New("package name is required"), http.StatusBadRequest)
		return
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Info(ctx, container, name)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Search – Поиск пакетов по названию
func (w *HTTPWrapper) Search(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	container := query.Get("container")
	q := query.Get("q")

	if q == "" {
		w.writeError(rw, errors.New("search query (q) is required"), http.StatusBadRequest)
		return
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Search(ctx, container, q)
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
		Container:   query.Get("container"),
		Sort:        query.Get("sort"),
		Order:       query.Get("order"),
		Limit:       limit,
		Offset:      offset,
		Filters:     filters,
		ForceUpdate: query.Get("forceUpdate") == "true",
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.List(ctx, params)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Install – Установить пакет
func (w *HTTPWrapper) Install(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		w.writeError(rw, err, http.StatusBadRequest)
		return
	}

	var container, packageName string
	var export bool

	if raw, ok := body["container"]; ok {
		_ = json.Unmarshal(raw, &container)
	}
	if raw, ok := body["package"]; ok {
		_ = json.Unmarshal(raw, &packageName)
	}
	if raw, ok := body["export"]; ok {
		_ = json.Unmarshal(raw, &export)
	}

	if container == "" {
		w.writeError(rw, errors.New("container is required"), http.StatusBadRequest)
		return
	}
	if packageName == "" {
		w.writeError(rw, errors.New("package is required"), http.StatusBadRequest)
		return
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Install(ctx, container, packageName, export)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// Remove – Удалить пакет
func (w *HTTPWrapper) Remove(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		w.writeError(rw, err, http.StatusBadRequest)
		return
	}

	var container, packageName string
	var onlyExport bool

	if raw, ok := body["container"]; ok {
		_ = json.Unmarshal(raw, &container)
	}
	if raw, ok := body["package"]; ok {
		_ = json.Unmarshal(raw, &packageName)
	}
	if raw, ok := body["onlyExport"]; ok {
		_ = json.Unmarshal(raw, &onlyExport)
	}

	if container == "" {
		w.writeError(rw, errors.New("container is required"), http.StatusBadRequest)
		return
	}
	if packageName == "" {
		w.writeError(rw, errors.New("package is required"), http.StatusBadRequest)
		return
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.Remove(ctx, container, packageName, onlyExport)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// GetFilterFields – Получить доступные поля для фильтрации
func (w *HTTPWrapper) GetFilterFields(rw http.ResponseWriter, r *http.Request) {
	container := r.URL.Query().Get("container")
	if container == "" {
		w.writeError(rw, errors.New("container is required"), http.StatusBadRequest)
		return
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.GetFilterFields(ctx, container)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// ContainerList – Получить список контейнеров
func (w *HTTPWrapper) ContainerList(rw http.ResponseWriter, r *http.Request) {
	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.ContainerList(ctx)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// ContainerAdd – Создать новый контейнер
func (w *HTTPWrapper) ContainerAdd(rw http.ResponseWriter, r *http.Request) {
	body, err := w.parseBodyParams(r)
	if err != nil {
		w.writeError(rw, err, http.StatusBadRequest)
		return
	}

	var image, name, additionalPackages, initHooks string
	var background bool

	if raw, ok := body["image"]; ok {
		_ = json.Unmarshal(raw, &image)
	}
	if raw, ok := body["name"]; ok {
		_ = json.Unmarshal(raw, &name)
	}
	if raw, ok := body["additionalPackages"]; ok {
		_ = json.Unmarshal(raw, &additionalPackages)
	}
	if raw, ok := body["initHooks"]; ok {
		_ = json.Unmarshal(raw, &initHooks)
	}
	if raw, ok := body["background"]; ok {
		_ = json.Unmarshal(raw, &background)
	}

	if image == "" {
		w.writeError(rw, errors.New("image is required"), http.StatusBadRequest)
		return
	}
	if name == "" {
		w.writeError(rw, errors.New("name is required"), http.StatusBadRequest)
		return
	}

	if background {
		ctx, txID := w.ctxWithTransactionOrGenerate(r)
		go func() {
			resp, err := w.actions.ContainerAdd(ctx, image, name, additionalPackages, initHooks)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "distrobox.ContainerAdd", data, err)
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
	resp, err := w.actions.ContainerAdd(ctx, image, name, additionalPackages, initHooks)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// ContainerRemove – Удалить контейнер
func (w *HTTPWrapper) ContainerRemove(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		w.writeError(rw, errors.New("container name is required"), http.StatusBadRequest)
		return
	}

	ctx := w.ctxWithTransaction(r)
	resp, err := w.actions.ContainerRemove(ctx, name)
	if err != nil {
		w.writeError(rw, err, http.StatusInternalServerError)
		return
	}
	w.writeJSON(rw, *resp)
}

// RegisterRoutes регистрирует все HTTP маршруты в mux
func (w *HTTPWrapper) RegisterRoutes(mux *http.ServeMux) {
	// Пакеты - информация
	mux.HandleFunc("GET /api/v1/distrobox/packages/filter-fields", w.GetFilterFields)
	mux.HandleFunc("GET /api/v1/distrobox/packages/search", w.Search)
	mux.HandleFunc("GET /api/v1/distrobox/packages/{name}", w.Info)
	mux.HandleFunc("GET /api/v1/distrobox/packages", w.List)

	// Пакеты - действия
	mux.HandleFunc("POST /api/v1/distrobox/update", w.Update)
	mux.HandleFunc("POST /api/v1/distrobox/packages/install", w.Install)
	mux.HandleFunc("POST /api/v1/distrobox/packages/remove", w.Remove)

	// Контейнеры
	mux.HandleFunc("GET /api/v1/distrobox/containers", w.ContainerList)
	mux.HandleFunc("POST /api/v1/distrobox/containers", w.ContainerAdd)
	mux.HandleFunc("DELETE /api/v1/distrobox/containers/{name}", w.ContainerRemove)
}

// GetHTTPEndpoints возвращает описания endpoints для OpenAPI документации
func GetHTTPEndpoints() []http_server.Endpoint {
	return []http_server.Endpoint{
		// Пакеты - информация
		{
			Method:       "List",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/distrobox/packages",
			ResponseType: "DistroboxListResponse",
			Permission:   "read",
			Summary:      "Получить список пакетов в контейнере",
			Tags:         []string{"distrobox"},
			QueryParams: []http_server.QueryParam{
				{Name: "container", Type: "string", Required: false, Description: "Имя контейнера"},
				{Name: "sort", Type: "string", Required: false, Description: "Поле сортировки"},
				{Name: "order", Type: "string", Required: false, Description: "Порядок сортировки (asc/desc)"},
				{Name: "limit", Type: "integer", Required: false, Description: "Лимит записей (по умолчанию 50)"},
				{Name: "offset", Type: "integer", Required: false, Description: "Смещение"},
				{Name: "filters", Type: "string", Required: false, Description: "Фильтры (можно несколько)"},
				{Name: "forceUpdate", Type: "boolean", Required: false, Description: "Принудительное обновление базы"},
			},
		},
		{
			Method:       "Info",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/distrobox/packages/{name}",
			ResponseType: "DistroboxInfoResponse",
			Permission:   "read",
			Summary:      "Получить информацию о пакете",
			Tags:         []string{"distrobox"},
			PathParams:   []string{"name"},
			QueryParams: []http_server.QueryParam{
				{Name: "container", Type: "string", Required: true, Description: "Имя контейнера"},
			},
		},
		{
			Method:       "Search",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/distrobox/packages/search",
			ResponseType: "DistroboxSearchResponse",
			Permission:   "read",
			Summary:      "Поиск пакетов по названию",
			Tags:         []string{"distrobox"},
			QueryParams: []http_server.QueryParam{
				{Name: "q", Type: "string", Required: true, Description: "Поисковый запрос"},
				{Name: "container", Type: "string", Required: false, Description: "Имя контейнера"},
			},
		},
		{
			Method:       "GetFilterFields",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/distrobox/packages/filter-fields",
			ResponseType: "DistroboxGetFilterFieldsResponse",
			Permission:   "read",
			Summary:      "Получить доступные поля для фильтрации",
			Tags:         []string{"distrobox"},
			QueryParams: []http_server.QueryParam{
				{Name: "container", Type: "string", Required: true, Description: "Имя контейнера"},
			},
		},

		// Пакеты - действия
		{
			Method:       "Update",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/distrobox/update",
			ResponseType: "DistroboxUpdateResponse",
			Permission:   "manage",
			Summary:      "Обновить список пакетов в контейнере",
			Tags:         []string{"distrobox"},
			QueryParams: []http_server.QueryParam{
				{Name: "container", Type: "string", Required: true, Description: "Имя контейнера"},
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},
		{
			Method:       "Install",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/distrobox/packages/install",
			ResponseType: "DistroboxInstallResponse",
			Permission:   "manage",
			Summary:      "Установить пакет в контейнер",
			Tags:         []string{"distrobox"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "container", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "package", Source: "body", Type: "string", ArgIndex: 2},
				{Name: "export", Source: "body", Type: "bool", Default: "false", ArgIndex: 3},
			},
		},
		{
			Method:       "Remove",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/distrobox/packages/remove",
			ResponseType: "DistroboxRemoveResponse",
			Permission:   "manage",
			Summary:      "Удалить пакет из контейнера",
			Tags:         []string{"distrobox"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "container", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "package", Source: "body", Type: "string", ArgIndex: 2},
				{Name: "onlyExport", Source: "body", Type: "bool", Default: "false", ArgIndex: 3},
			},
		},

		// Контейнеры
		{
			Method:       "ContainerList",
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/distrobox/containers",
			ResponseType: "DistroboxContainerListResponse",
			Permission:   "read",
			Summary:      "Получить список контейнеров",
			Tags:         []string{"distrobox"},
		},
		{
			Method:       "ContainerAdd",
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/distrobox/containers",
			ResponseType: "DistroboxContainerAddResponse",
			Permission:   "manage",
			Summary:      "Создать новый контейнер",
			Tags:         []string{"distrobox"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "image", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "name", Source: "body", Type: "string", ArgIndex: 2},
				{Name: "additionalPackages", Source: "body", Type: "string", Default: "", ArgIndex: 3},
				{Name: "initHooks", Source: "body", Type: "string", Default: "", ArgIndex: 4},
				{Name: "background", Source: "body", Type: "bool", Default: "false"},
			},
		},
		{
			Method:       "ContainerRemove",
			HTTPMethod:   "DELETE",
			HTTPPath:     "/api/v1/distrobox/containers/{name}",
			ResponseType: "DistroboxContainerRemoveResponse",
			Permission:   "manage",
			Summary:      "Удалить контейнер",
			Tags:         []string{"distrobox"},
			PathParams:   []string{"name"},
		},
	}
}
