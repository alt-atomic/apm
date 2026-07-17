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

package filter

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FieldApplier позволяет определить кастомную логику применения фильтра для поля
type FieldApplier func(query *gorm.DB, f Filter) (*gorm.DB, bool)

// GormApplier применяет фильтры к GORM-запросу
type GormApplier struct {
	// CustomAppliers кастомные обработчики для конкретных полей.
	CustomAppliers map[string]FieldApplier
}

// Apply применяет список фильтров (AND) к GORM-запросу.
func (a *GormApplier) Apply(query *gorm.DB, filters []Filter) *gorm.DB {
	for _, f := range filters {
		if len(SplitOrValues(f.Field)) == 1 && len(SplitOrValues(f.Value)) == 1 {
			query = a.applyOne(query, f)
			continue
		}
		query = query.Where(a.buildCondition(query, f))
	}
	return query
}

// ApplyGroups применяет группы фильтров: внутри группы — OR, между группами — AND.
func (a *GormApplier) ApplyGroups(query *gorm.DB, groups []FilterGroup) *gorm.DB {
	for _, g := range groups {
		switch len(g) {
		case 0:
		case 1:
			query = a.Apply(query, []Filter(g))
		default:
			sub := a.buildCondition(query, g[0])
			for _, f := range g[1:] {
				sub = sub.Or(a.buildCondition(query, f))
			}
			query = query.Where(sub)
		}
	}
	return query
}

// buildCondition собирает OR-скобку одного фильтра по комбинациям полей и значений ("|").
func (a *GormApplier) buildCondition(query *gorm.DB, f Filter) *gorm.DB {
	var sub *gorm.DB
	for _, fld := range SplitOrValues(f.Field) {
		for _, val := range SplitOrValues(f.Value) {
			part := query.Session(&gorm.Session{NewDB: true})
			part = a.applyOne(part, Filter{Field: fld, Op: f.Op, Value: val})
			if sub == nil {
				sub = part
			} else {
				sub = sub.Or(part)
			}
		}
	}
	return sub
}

// applyOne применяет один фильтр (без OR) через кастомный или стандартный applier
func (a *GormApplier) applyOne(query *gorm.DB, f Filter) *gorm.DB {
	if a.CustomAppliers != nil {
		if applier, ok := a.CustomAppliers[f.Field]; ok {
			q, handled := applier(query, f)
			if handled {
				return q
			}
		}
	}
	return applyDefault(query, f)
}

// escapeLike экранирует спецсимволы LIKE (%, _) в значении
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// ColOpToSQL возвращает SQL-выражение для произвольной колонки/выражения
func ColOpToSQL(col string, op Op, value string) (colExpr string, sqlOp string, sqlValue string) {
	switch op {
	case OpEq:
		return col, "= ?", value
	case OpNe:
		return col, "<> ?", value
	case OpLike:
		return col, "LIKE ? ESCAPE '\\'", "%" + escapeLike(value) + "%"
	case OpGt:
		return col, "> ?", value
	case OpGte:
		return col, ">= ?", value
	case OpLt:
		return col, "< ?", value
	case OpLte:
		return col, "<= ?", value
	case OpContains:
		return fmt.Sprintf("(',' || %s || ',')", col), "LIKE ? ESCAPE '\\'", "%," + escapeLike(value) + ",%"
	default:
		return col, "= ?", value
	}
}

// applyDefault применяет один фильтр через GORM clause
func applyDefault(query *gorm.DB, f Filter) *gorm.DB {
	col := clause.Column{Name: f.Field}
	switch f.Op {
	case OpEq:
		return query.Where(clause.Eq{Column: col, Value: f.Value})
	case OpNe:
		return query.Where(clause.Neq{Column: col, Value: f.Value})
	case OpGt:
		return query.Where(clause.Gt{Column: col, Value: f.Value})
	case OpGte:
		return query.Where(clause.Gte{Column: col, Value: f.Value})
	case OpLt:
		return query.Where(clause.Lt{Column: col, Value: f.Value})
	case OpLte:
		return query.Where(clause.Lte{Column: col, Value: f.Value})
	case OpLike:
		return query.Where(clause.Like{Column: col, Value: "%" + escapeLike(f.Value) + "%"})
	case OpContains:
		return query.Where(
			fmt.Sprintf("(',' || %s || ',') LIKE ? ESCAPE '\\'", f.Field),
			"%,"+escapeLike(f.Value)+",%",
		)
	default:
		return query.Where(clause.Eq{Column: col, Value: f.Value})
	}
}
