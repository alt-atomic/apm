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

package wire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/filter"

	"github.com/godbus/dbus/v5"
)

const (
	PageDefaultLimit = 100
	PageMaxLimit     = 1000
)

// PageRequest — общая пагинация JSON-запросов страничных методов.
type PageRequest struct {
	Limit  uint32 `json:"limit"`
	Offset uint32 `json:"offset"`
}

// ListRequest — общий JSON-запрос списковых методов.
// ForceUpdate читают не все домены: каталог AppStream его игнорирует.
type ListRequest struct {
	Sort        string `json:"sort"`
	Order       string `json:"order"`
	ForceUpdate bool   `json:"forceUpdate"`
	PageRequest
	filter.ListBody
}

// Page — проверенные параметры страницы.
type Page struct {
	Limit  int
	Offset int
}

// ListPage — проверенная страница вместе с фильтрами.
type ListPage struct {
	Page
	Filters []filter.FilterGroup
}

// DecodeJSON разбирает сложный D-Bus-запрос, переданный JSON-строкой.
// Для расширяемых запросов списка неизвестные поля допустимы.
func DecodeJSON(raw string, destination any) error {
	if strings.TrimSpace(raw) == "" {
		raw = "{}"
	}
	if err := json.Unmarshal([]byte(raw), destination); err != nil {
		return apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf("invalid JSON request: %w", err))
	}
	return nil
}

// DecodeOptions строго разбирает JSON-параметры метода.
func DecodeOptions(raw string, destination any) error {
	if strings.TrimSpace(raw) == "" {
		raw = "{}"
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf("invalid JSON options: %w", err))
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf("invalid JSON options: %w", err))
	}
	return nil
}

// JSON сериализует ответ API без изменения существующего JSON-контракта.
func JSON(response any) (string, error) {
	data, err := json.Marshal(response)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// JSONReply сериализует успешный ответ или преобразует ошибку в D-Bus.
func JSONReply(response any, err error) (string, *dbus.Error) {
	if err != nil {
		return "", Error(err)
	}
	encoded, err := JSON(response)
	return encoded, Error(err)
}

// JSONTask адаптирует действие, возвращающее структуру ответа, к фоновой задаче.
func JSONTask[T any](fn func(context.Context) (T, error)) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		response, err := fn(ctx)
		if err != nil {
			return "", err
		}
		return JSON(response)
	}
}

// pageLimit нормализует размер страницы JSON-запроса.
func pageLimit(limit uint32) (int, error) {
	if limit == 0 {
		return PageDefaultLimit, nil
	}
	if limit > PageMaxLimit {
		return 0, apmerr.New(apmerr.ErrorTypeValidation,
			fmt.Errorf("limit %d exceeds maximum %d", limit, PageMaxLimit))
	}
	return int(limit), nil
}

// pageOffset проверяет, что смещение представимо типом int текущей платформы.
func pageOffset(offset uint32) (int, error) {
	if uint64(offset) > uint64(math.MaxInt) {
		return 0, apmerr.New(apmerr.ErrorTypeValidation,
			fmt.Errorf("offset %d exceeds platform maximum %d", offset, math.MaxInt))
	}
	return int(offset), nil
}

// Validate проверяет пагинацию запроса.
func (r PageRequest) Validate() (Page, error) {
	limit, err := pageLimit(r.Limit)
	if err != nil {
		return Page{}, err
	}
	offset, err := pageOffset(r.Offset)
	if err != nil {
		return Page{}, err
	}
	return Page{Limit: limit, Offset: offset}, nil
}

// Validate проверяет пагинацию и фильтры спискового запроса.
func (r ListRequest) Validate(config *filter.Config) (ListPage, error) {
	page, err := r.PageRequest.Validate()
	if err != nil {
		return ListPage{}, err
	}
	groups, err := config.ValidateBody(r.ListBody)
	if err != nil {
		return ListPage{}, apmerr.New(apmerr.ErrorTypeValidation, err)
	}
	return ListPage{Page: page, Filters: groups}, nil
}
