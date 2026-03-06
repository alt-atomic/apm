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
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	"apm/internal/common/http_server"
	"apm/internal/common/reply"
	"apm/internal/distrobox/service"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"
)

// HTTPWrapper предоставляет обёртку для действий с контейнерами через HTTP.
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

// Update обновляет список пакетов.
func (w *HTTPWrapper) Update(rw http.ResponseWriter, r *http.Request) {
	container := r.URL.Query().Get("container")
	if container == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("container is required")))
		return
	}

	if w.RunBackground(rw, r, reply.EventDistroUpdate, func(ctx context.Context) (interface{}, error) {
		return w.actions.Update(ctx, container)
	}) {
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Update(ctx, container)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Info возвращает информацию о пакете.
func (w *HTTPWrapper) Info(rw http.ResponseWriter, r *http.Request) {
	container := r.URL.Query().Get("container")
	if container == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("container is required")))
		return
	}

	name := r.PathValue("name")
	if name == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("package name is required")))
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Info(ctx, container, name)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Search выполняет поиск пакетов.
func (w *HTTPWrapper) Search(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	container := query.Get("container")
	q := query.Get("q")

	if q == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("search query (q) is required")))
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Search(ctx, container, q)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
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

	validated, err := service.DistroFilterConfig.Validate(body.Filters)
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
		Container:   query.Get("container"),
		Sort:        query.Get("sort"),
		Order:       query.Get("order"),
		Limit:       limit,
		Offset:      offset,
		Filters:     validated,
		ForceUpdate: query.Get("forceUpdate") == "true",
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.List(ctx, params)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Install устанавливает пакет.
func (w *HTTPWrapper) Install(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var container, packageName string
	var export bool

	for _, f := range []struct {
		key    string
		target interface{}
	}{
		{"container", &container},
		{"package", &packageName},
		{"export", &export},
	} {
		if err = reply.UnmarshalField(body, f.key, f.target); err != nil {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
			return
		}
	}

	if container == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("container is required")))
		return
	}
	if packageName == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("package is required")))
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Install(ctx, container, packageName, export)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// Remove удаляет пакет.
func (w *HTTPWrapper) Remove(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var container, packageName string
	var onlyExport bool

	for _, f := range []struct {
		key    string
		target interface{}
	}{
		{"container", &container},
		{"package", &packageName},
		{"onlyExport", &onlyExport},
	} {
		if err = reply.UnmarshalField(body, f.key, f.target); err != nil {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
			return
		}
	}

	if container == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("container is required")))
		return
	}
	if packageName == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("package is required")))
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.Remove(ctx, container, packageName, onlyExport)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
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

// GetIcon возвращает иконку пакета.
func (w *HTTPWrapper) GetIcon(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	container := r.URL.Query().Get("container")

	ctx := w.CtxWithTransaction(r)
	data, err := w.actions.GetIconByPackage(ctx, name, container)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}

	rw.Header().Set("Content-Type", http.DetectContentType(data))
	_, _ = rw.Write(data)
}

// ContainerList возвращает список контейнеров.
func (w *HTTPWrapper) ContainerList(rw http.ResponseWriter, r *http.Request) {
	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.ContainerList(ctx)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// ContainerAdd создаёт новый контейнер.
func (w *HTTPWrapper) ContainerAdd(rw http.ResponseWriter, r *http.Request) {
	body, err := w.ParseBodyParams(r)
	if err != nil {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
		return
	}

	var image, name, additionalPackages, initHooks string

	for _, f := range []struct {
		key    string
		target interface{}
	}{
		{"image", &image},
		{"name", &name},
		{"additionalPackages", &additionalPackages},
		{"initHooks", &initHooks},
	} {
		if err = reply.UnmarshalField(body, f.key, f.target); err != nil {
			reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, err))
			return
		}
	}

	if image == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("image is required")))
		return
	}
	if name == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("name is required")))
		return
	}

	if w.RunBackground(rw, r, reply.EventDistroContainerAdd, func(ctx context.Context) (interface{}, error) {
		return w.actions.ContainerAdd(ctx, image, name, additionalPackages, initHooks)
	}) {
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.ContainerAdd(ctx, image, name, additionalPackages, initHooks)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// ContainerRemove удаляет контейнер.
func (w *HTTPWrapper) ContainerRemove(rw http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		reply.WriteHTTPError(rw, apmerr.New(apmerr.ErrorTypeValidation, errors.New("container name is required")))
		return
	}

	ctx := w.CtxWithTransaction(r)
	resp, err := w.actions.ContainerRemove(ctx, name)
	if err != nil {
		reply.WriteHTTPError(rw, err)
		return
	}
	w.WriteJSON(rw, reply.OK(resp))
}

// GetEndpoints возвращает описания endpoints с handler
func (w *HTTPWrapper) GetEndpoints() []http_server.Endpoint {
	return []http_server.Endpoint{
		// Пакеты - информация
		{
			Handler:      w.List,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/distrobox/packages/list",
			RequestType:  reflect.TypeOf(ListFiltersBody{}),
			ResponseType: reflect.TypeOf(ListResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить список пакетов в контейнере",
			Description: "Поиск пакетов в контейнере с фильтрацией, сортировкой и пагинацией.\n\n" +
				"**Фильтры** передаются в JSON body в массиве `filters`, каждый элемент содержит:\n" +
				"- `field` — имя поля (например: name, section, installed)\n" +
				"- `op` — оператор: eq, ne, like, gt, gte, lt, lte, contains (если не указан — используется оператор по умолчанию для поля)\n" +
				"- `value` — значение для сравнения\n\n" +
				"**OR-логика**: для поиска по нескольким значениям используйте `|` в value: `\"value\": \"Games|Education\"`\n\n" +
				"Остальные параметры (container, sort, order, limit, offset, forceUpdate) передаются через query string.\n\n" +
				"**Пример**:\n" +
				"```\n" +
				"POST /api/v1/distrobox/packages/list?container=ubuntu&sort=name&limit=20\n" +
				"Body: {\"filters\": [{\"field\": \"name\", \"op\": \"like\", \"value\": \"fire\"}]}\n" +
				"```\n\n" +
				"Доступные поля и операторы можно получить через GET /api/v1/distrobox/packages/filter-fields",
			Tags: []string{"distrobox"},
			QueryParams: []http_server.QueryParam{
				{Name: "container", Type: "string", Required: false, Description: "Имя контейнера"},
				{Name: "sort", Type: "string", Required: false, Description: "Поле сортировки"},
				{Name: "order", Type: "string", Required: false, Description: "Порядок сортировки (ASC/DESC)"},
				{Name: "limit", Type: "integer", Required: false, Description: "Лимит записей (по умолчанию 50)"},
				{Name: "offset", Type: "integer", Required: false, Description: "Смещение"},
				{Name: "forceUpdate", Type: "boolean", Required: false, Description: "Принудительное обновление базы"},
			},
		},
		{
			Handler:      w.Info,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/distrobox/packages/{name}",
			ResponseType: reflect.TypeOf(InfoResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить информацию о пакете",
			Tags:         []string{"distrobox"},
			PathParams:   []string{"name"},
			QueryParams: []http_server.QueryParam{
				{Name: "container", Type: "string", Required: true, Description: "Имя контейнера"},
			},
		},
		{
			Handler:     w.GetIcon,
			HTTPMethod:  "GET",
			HTTPPath:    "/api/v1/distrobox/packages/{name}/icon",
			ContentType: "image/*",
			Permission:  http_server.PermRead,
			Summary:     "Получить иконку пакета",
			Tags:        []string{"distrobox"},
			PathParams:  []string{"name"},
			QueryParams: []http_server.QueryParam{
				{Name: "container", Type: "string", Required: false, Description: "Имя контейнера"},
			},
		},
		{
			Handler:      w.Search,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/distrobox/packages/search",
			ResponseType: reflect.TypeOf(SearchResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Поиск пакетов по названию",
			Tags:         []string{"distrobox"},
			QueryParams: []http_server.QueryParam{
				{Name: "q", Type: "string", Required: true, Description: "Поисковый запрос"},
				{Name: "container", Type: "string", Required: false, Description: "Имя контейнера"},
			},
		},
		{
			Handler:      w.GetFilterFields,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/distrobox/packages/filter-fields",
			ResponseType: reflect.TypeOf(GetFilterFieldsResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить доступные поля для фильтрации",
			Tags:         []string{"distrobox"},
		},

		// Пакеты - действия
		{
			Handler:      w.Update,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/distrobox/update",
			ResponseType: reflect.TypeOf(UpdateResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Обновить список пакетов в контейнере",
			Tags:         []string{"distrobox"},
			QueryParams: []http_server.QueryParam{
				{Name: "container", Type: "string", Required: true, Description: "Имя контейнера"},
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},
		{
			Handler:      w.Install,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/distrobox/packages/install",
			ResponseType: reflect.TypeOf(InstallResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Установить пакет в контейнер",
			Tags:         []string{"distrobox"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "container", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "package", Source: "body", Type: "string", ArgIndex: 2},
				{Name: "export", Source: "body", Type: "bool", Default: "false", ArgIndex: 3},
			},
		},
		{
			Handler:      w.Remove,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/distrobox/packages/remove",
			ResponseType: reflect.TypeOf(RemoveResponse{}),
			Permission:   http_server.PermManage,
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
			Handler:      w.ContainerList,
			HTTPMethod:   "GET",
			HTTPPath:     "/api/v1/distrobox/containers",
			ResponseType: reflect.TypeOf(ContainerListResponse{}),
			Permission:   http_server.PermRead,
			Summary:      "Получить список контейнеров",
			Tags:         []string{"distrobox"},
		},
		{
			Handler:      w.ContainerAdd,
			HTTPMethod:   "POST",
			HTTPPath:     "/api/v1/distrobox/containers",
			ResponseType: reflect.TypeOf(ContainerAddResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Создать новый контейнер",
			Tags:         []string{"distrobox"},
			ParamMappings: []http_server.ParamMapping{
				{Name: "image", Source: "body", Type: "string", ArgIndex: 1},
				{Name: "name", Source: "body", Type: "string", ArgIndex: 2},
				{Name: "additionalPackages", Source: "body", Type: "string", Default: "", ArgIndex: 3},
				{Name: "initHooks", Source: "body", Type: "string", Default: "", ArgIndex: 4},
			},
			QueryParams: []http_server.QueryParam{
				{Name: "background", Type: "boolean", Required: false, Description: "Выполнить в фоне (результат придёт через WebSocket)"},
			},
		},
		{
			Handler:      w.ContainerRemove,
			HTTPMethod:   "DELETE",
			HTTPPath:     "/api/v1/distrobox/containers/{name}",
			ResponseType: reflect.TypeOf(ContainerRemoveResponse{}),
			Permission:   http_server.PermManage,
			Summary:      "Удалить контейнер",
			Tags:         []string{"distrobox"},
			PathParams:   []string{"name"},
		},
	}
}
