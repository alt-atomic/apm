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
	"fmt"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/filter"
)

// FilterRule одно условие фильтра в aa(sss): группы CNF, внутри группы OR.
type FilterRule struct {
	Field string
	Op    string
	Value string
}

// FilterGroups конвертирует aa(sss)-фильтры и валидирует их конфигом модуля.
func FilterGroups(rows [][]FilterRule, cfg *filter.Config) ([]filter.FilterGroup, error) {
	groups := make([]filter.FilterGroup, 0, len(rows))
	for _, group := range rows {
		flat := make([]filter.Filter, 0, len(group))
		for _, r := range group {
			flat = append(flat, filter.Filter{Field: r.Field, Op: filter.Op(r.Op), Value: r.Value})
		}
		validated, err := cfg.Validate(flat)
		if err != nil {
			return nil, apmerr.New(apmerr.ErrorTypeValidation, err)
		}
		groups = append(groups, validated)
	}
	return groups, nil
}

// Лимиты страницы списковых методов: массив DBus ограничен 64 МиБ.
const (
	PageDefaultLimit = 100
	PageMaxLimit     = 1000
)

// PageLimit нормализует limit страницы: 0 — дефолт, больше потолка — ошибка.
func PageLimit(limit uint32) (int, error) {
	if limit == 0 {
		return PageDefaultLimit, nil
	}
	if limit > PageMaxLimit {
		return 0, apmerr.New(apmerr.ErrorTypeValidation,
			fmt.Errorf("limit %d exceeds maximum %d", limit, PageMaxLimit))
	}
	return int(limit), nil
}
