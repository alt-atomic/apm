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

package http_server

import (
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// backgroundTaskResponse структура ответа при запуске фоновой задачи
type backgroundTaskResponse struct {
	Message     string `json:"message"`
	Transaction string `json:"transaction"`
}

// BaseHTTPWrapper общая база для HTTP обёрток модулей
type BaseHTTPWrapper struct {
	Ctx       context.Context
	AppConfig *app.Config
}

// CtxWithTransaction создает контекст с transaction из запроса
func (b *BaseHTTPWrapper) CtxWithTransaction(r *http.Request) context.Context {
	tx := r.Header.Get("X-Transaction-ID")
	if tx == "" {
		tx = r.URL.Query().Get("transaction")
	}
	return context.WithValue(b.Ctx, helper.TransactionKey, tx)
}

// CtxWithTransactionOrGenerate создает контекст с transaction, генерируя его если не передан
func (b *BaseHTTPWrapper) CtxWithTransactionOrGenerate(r *http.Request) (context.Context, string) {
	tx := r.Header.Get("X-Transaction-ID")
	if tx == "" {
		tx = r.URL.Query().Get("transaction")
	}
	if tx == "" {
		tx = helper.GenerateTransactionID()
	}
	return context.WithValue(b.Ctx, helper.TransactionKey, tx), tx
}

// WriteJSON отправляет JSON ответ
func (b *BaseHTTPWrapper) WriteJSON(rw http.ResponseWriter, resp reply.APIResponse) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(rw).Encode(resp)
}

// ParseBodyParams парсит параметры из тела запроса
func (b *BaseHTTPWrapper) ParseBodyParams(r *http.Request) (map[string]json.RawMessage, error) {
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("request body is required")
		}
		return nil, err
	}
	return body, nil
}

// RunBackground запускает задачу в фоне
func (b *BaseHTTPWrapper) RunBackground(rw http.ResponseWriter, r *http.Request, event string, fn func(ctx context.Context) (interface{}, error)) bool {
	if r.URL.Query().Get("background") != "true" {
		return false
	}

	ctx, txID := b.CtxWithTransactionOrGenerate(r)
	go func() {
		resp, err := fn(ctx)
		reply.SendTaskResult(ctx, event, resp, err)
	}()

	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(rw).Encode(reply.OK(backgroundTaskResponse{
		Message:     app.T_("Task started in background"),
		Transaction: txID,
	}))
	return true
}
