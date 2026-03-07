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
	"apm/internal/common/app"
	"fmt"
	"regexp"
	"strings"
)

// safeFieldName проверяет, что имя поля содержит только допустимые символы (защита от SQL-инъекции).
var safeFieldRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*$`)

func IsSafeFieldName(field string) bool {
	return safeFieldRe.MatchString(field)
}

// Op определяет тип операции фильтра.
type Op string

const (
	OpEq       Op = "eq"       // точное совпадение (=)
	OpNe       Op = "ne"       // не равно (<>)
	OpLike     Op = "like"     // LIKE %value%
	OpGt       Op = "gt"       // больше (>)
	OpGte      Op = "gte"      // больше или равно (>=)
	OpLt       Op = "lt"       // меньше (<)
	OpLte      Op = "lte"      // меньше или равно (<=)
	OpContains Op = "contains" // поиск в comma-separated полях
)

// AllOps список всех поддерживаемых операторов.
var AllOps = []Op{OpEq, OpNe, OpLike, OpGt, OpGte, OpLt, OpLte, OpContains}

func isValidOp(op Op) bool {
	for _, o := range AllOps {
		if o == op {
			return true
		}
	}
	return false
}

// Filter представляет один фильтр с полем, оператором и значением.
type Filter struct {
	Field string `json:"field"`
	Op    Op     `json:"op"`
	Value string `json:"value"`
}

// FieldConfig описывает конфигурацию поля
type FieldConfig struct {
	DefaultOp  Op
	AllowedOps []Op
	Sortable   bool
	Extra      map[string]any
}

// FieldInfo описывает поле фильтрации для API-ответа.
type FieldInfo struct {
	Name       string         `json:"name"`
	DefaultOp  Op             `json:"defaultOp"`
	AllowedOps []Op           `json:"allowedOps"`
	Sortable   bool           `json:"sortable"`
	Extra      map[string]any `json:"extra,omitempty"`
}

// Config описывает конфигурацию фильтрации для конкретного модуля
type Config struct {
	Fields map[string]FieldConfig
	// Prefixes позволяет определить конфигурацию для полей с общим префиксом
	Prefixes map[string]FieldConfig
}

// IsAllowedField проверяет, допустимо ли поле для фильтрации.
func (c *Config) IsAllowedField(field string) bool {
	if _, ok := c.Fields[field]; ok {
		return true
	}
	for prefix := range c.Prefixes {
		if strings.HasPrefix(field, prefix) {
			return true
		}
	}
	return false
}

// AllowedFields возвращает список допустимых полей.
func (c *Config) AllowedFields() []string {
	fields := make([]string, 0, len(c.Fields)+len(c.Prefixes))
	for f := range c.Fields {
		fields = append(fields, f)
	}
	for p := range c.Prefixes {
		fields = append(fields, p+"*")
	}
	return fields
}

// FieldsInfo возвращает описание всех полей для API-ответа.
func (c *Config) FieldsInfo() []FieldInfo {
	result := make([]FieldInfo, 0, len(c.Fields)+len(c.Prefixes))
	for name, fc := range c.Fields {
		allowedOps := fc.AllowedOps
		if allowedOps == nil {
			allowedOps = AllOps
		}
		result = append(result, FieldInfo{
			Name:       name,
			DefaultOp:  fc.DefaultOp,
			AllowedOps: allowedOps,
			Sortable:   fc.Sortable,
			Extra:      fc.Extra,
		})
	}
	for prefix, fc := range c.Prefixes {
		allowedOps := fc.AllowedOps
		if allowedOps == nil {
			allowedOps = AllOps
		}
		result = append(result, FieldInfo{
			Name:       prefix + "*",
			DefaultOp:  fc.DefaultOp,
			AllowedOps: allowedOps,
			Sortable:   fc.Sortable,
			Extra:      fc.Extra,
		})
	}
	return result
}

// SortableFields возвращает список полей, по которым можно сортировать.
func (c *Config) SortableFields() []string {
	var fields []string
	for name, fc := range c.Fields {
		if fc.Sortable {
			fields = append(fields, name)
		}
	}
	return fields
}

// ValidateSortField проверяет, допустимо ли поле для сортировки.
func (c *Config) ValidateSortField(field string) error {
	if !IsSafeFieldName(field) {
		return fmt.Errorf(app.T_("Invalid sort field name: %s"), field)
	}
	if fc, ok := c.Fields[field]; ok && fc.Sortable {
		return nil
	}
	return fmt.Errorf(app.T_("Invalid sort field: %s. Available fields: %s"),
		field, strings.Join(c.SortableFields(), ", "))
}

// getFieldConfig возвращает конфигурацию поля (из Fields или Prefixes)
func (c *Config) getFieldConfig(field string) (FieldConfig, bool) {
	if fc, ok := c.Fields[field]; ok {
		return fc, true
	}
	for prefix, fc := range c.Prefixes {
		if strings.HasPrefix(field, prefix) {
			return fc, true
		}
	}
	return FieldConfig{}, false
}

// isAllowedOp проверяет, допустим ли оператор для данного поля
func (c *Config) isAllowedOp(field string, op Op) bool {
	fc, ok := c.getFieldConfig(field)
	if !ok {
		return false
	}
	if fc.AllowedOps == nil {
		return true
	}
	for _, allowed := range fc.AllowedOps {
		if allowed == op {
			return true
		}
	}
	return false
}

// Validate валидирует фильтры: проверяет поля, операторы, проставляет дефолты.
func (c *Config) Validate(filters []Filter) ([]Filter, error) {
	result := make([]Filter, 0, len(filters))
	for _, f := range filters {
		if f.Field == "" || f.Value == "" {
			continue
		}
		if !IsSafeFieldName(f.Field) {
			return nil, fmt.Errorf(app.T_("Invalid filter field name: %s"), f.Field)
		}
		if !c.IsAllowedField(f.Field) {
			return nil, fmt.Errorf(app.T_("Invalid filter field: %s. Available fields: %s"),
				f.Field, strings.Join(c.AllowedFields(), ", "))
		}
		op := f.Op
		if op == "" {
			fc, _ := c.getFieldConfig(f.Field)
			op = fc.DefaultOp
			if op == "" {
				op = OpEq
			}
		}
		if !isValidOp(op) {
			return nil, fmt.Errorf(app.T_("Invalid filter operator: %s. Available operators: %s"),
				op, "eq, ne, like, gt, gte, lt, lte, contains")
		}
		if !c.isAllowedOp(f.Field, op) {
			return nil, fmt.Errorf(app.T_("Operator %s is not allowed for field %s"), op, f.Field)
		}
		result = append(result, Filter{Field: f.Field, Op: op, Value: f.Value})
	}
	return result, nil
}

// Parse разбирает строки формата "field[op]=value" или "field=value" и валидирует результат.
func (c *Config) Parse(raw []string) ([]Filter, error) {
	var filters []Filter
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		field, op, value, err := splitFilterString(s)
		if err != nil {
			return nil, err
		}
		filters = append(filters, Filter{Field: field, Op: op, Value: value})
	}
	return c.Validate(filters)
}

// OrSeparator разделитель для OR-логики в значениях фильтра.
// Пример: "name[like]=zip|rar" → name LIKE '%zip%' OR name LIKE '%rar%'
const OrSeparator = "|"

// SplitOrValues разделяет значение
func SplitOrValues(value string) []string {
	if !strings.Contains(value, OrSeparator) {
		return []string{value}
	}
	parts := strings.Split(value, OrSeparator)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return []string{value}
	}
	return result
}

// splitFilterString разбирает строку вида "field[op]=value" или "field=value".
func splitFilterString(s string) (field string, op Op, value string, err error) {
	eqIdx := strings.Index(s, "=")
	if eqIdx < 0 {
		return "", "", "", fmt.Errorf(app.T_("Filter must contain '=': %s"), s)
	}

	key := s[:eqIdx]
	value = strings.TrimSpace(s[eqIdx+1:])

	if value == "" {
		return "", "", "", fmt.Errorf(app.T_("Empty filter value: %s"), s)
	}

	if bracketStart := strings.Index(key, "["); bracketStart >= 0 {
		bracketEnd := strings.Index(key, "]")
		if bracketEnd < 0 || bracketEnd < bracketStart {
			return "", "", "", fmt.Errorf(app.T_("Invalid filter operator format: %s"), s)
		}
		field = strings.TrimSpace(key[:bracketStart])
		op = Op(strings.TrimSpace(key[bracketStart+1 : bracketEnd]))
	} else {
		field = strings.TrimSpace(key)
		op = ""
	}

	if field == "" {
		return "", "", "", fmt.Errorf(app.T_("Empty filter field: %s"), s)
	}

	return field, op, value, nil
}

// ListEndpointDescription генерирует описание для списка с фильтрацией
func ListEndpointDescription(subject, fieldsExample, exampleURL, exampleBody, filterFieldsURL string) string {
	return fmt.Sprintf("%s с фильтрацией, сортировкой и пагинацией.\n\n"+
		"**Фильтры** передаются в JSON body в массиве `filters`, каждый элемент содержит:\n"+
		"- `field` — имя поля (например: %s)\n"+
		"- `op` — оператор: eq, ne, like, gt, gte, lt, lte, contains (если не указан — используется оператор по умолчанию для поля)\n"+
		"- `value` — значение для сравнения\n\n"+
		"**OR-логика**: для поиска по нескольким значениям используйте `|` в value: `\"value\": \"Games|Education\"`\n\n"+
		"Остальные параметры передаются через query string.\n\n"+
		"**Пример**:\n"+
		"```\n"+
		"%s\n"+
		"Body: %s\n"+
		"```\n\n"+
		"Доступные поля и операторы можно получить через GET %s",
		subject, fieldsExample, exampleURL, exampleBody, filterFieldsURL)
}
