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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"altlinux.space/alt-atomic/apm/internal/common/app"
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

// Filter представляет один фильтр: поле (или несколько через "|"), оператор и значение.
type Filter struct {
	Field string `json:"field"`
	Op    Op     `json:"op"`
	Value string `json:"value"`
}

// FilterGroup группа фильтров-OR; группы объединяются через AND.
type FilterGroup []Filter

// ListBody тело запроса списка: filters — AND-условия, orFilters — одна OR-скобка.
type ListBody struct {
	Filters   []Filter `json:"filters"`
	OrFilters []Filter `json:"orFilters,omitempty"`
}

// ParseJSONBody разбирает JSON-объект фильтров в ListBody.
func ParseJSONBody(raw string) (ListBody, error) {
	var body ListBody
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return body, nil
	}
	return body, json.Unmarshal([]byte(raw), &body)
}

// MakeGroups собирает группы: filters — по одному в группе, orFilters — одной группой.
func MakeGroups(filters, orFilters []Filter) []FilterGroup {
	groups := make([]FilterGroup, 0, len(filters)+1)
	for _, f := range filters {
		groups = append(groups, FilterGroup{f})
	}
	if len(orFilters) > 0 {
		groups = append(groups, FilterGroup(orFilters))
	}
	return groups
}

// AndGroups оборачивает плоский AND-список фильтров в группы.
func AndGroups(filters []Filter) []FilterGroup {
	return MakeGroups(filters, nil)
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
}

// IsAllowedField проверяет, допустимо ли поле для фильтрации.
func (c *Config) IsAllowedField(field string) bool {
	_, ok := c.Fields[field]
	return ok
}

// AllowedFields возвращает список допустимых полей.
func (c *Config) AllowedFields() []string {
	fields := make([]string, 0, len(c.Fields))
	for f := range c.Fields {
		fields = append(fields, f)
	}
	return fields
}

// FieldsInfo возвращает описание всех полей для API-ответа.
func (c *Config) FieldsInfo() []FieldInfo {
	result := make([]FieldInfo, 0, len(c.Fields))
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

// getFieldConfig возвращает конфигурацию поля
func (c *Config) getFieldConfig(field string) (FieldConfig, bool) {
	fc, ok := c.Fields[field]
	return fc, ok
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
		fields := SplitOrValues(f.Field)
		op := f.Op
		if op == "" {
			for i, fld := range fields {
				fc, _ := c.getFieldConfig(fld)
				fldOp := fc.DefaultOp
				if fldOp == "" {
					fldOp = OpEq
				}
				if i == 0 {
					op = fldOp
				} else if op != fldOp {
					return nil, fmt.Errorf(app.T_("Operator must be set explicitly for multi-field filter: %s"), f.Field)
				}
			}
		}
		if !isValidOp(op) {
			return nil, fmt.Errorf(app.T_("Invalid filter operator: %s. Available operators: %s"),
				op, "eq, ne, like, gt, gte, lt, lte, contains")
		}
		for _, fld := range fields {
			if !IsSafeFieldName(fld) {
				return nil, fmt.Errorf(app.T_("Invalid filter field name: %s"), fld)
			}
			if !c.IsAllowedField(fld) {
				return nil, fmt.Errorf(app.T_("Invalid filter field: %s. Available fields: %s"),
					fld, strings.Join(c.AllowedFields(), ", "))
			}
			if !c.isAllowedOp(fld, op) {
				return nil, fmt.Errorf(app.T_("Operator %s is not allowed for field %s"), op, fld)
			}
		}
		result = append(result, Filter{Field: strings.Join(fields, OrSeparator), Op: op, Value: f.Value})
	}
	return result, nil
}

// ValidateBody валидирует тело запроса и собирает группы фильтров.
func (c *Config) ValidateBody(body ListBody) ([]FilterGroup, error) {
	filters, err := c.Validate(body.Filters)
	if err != nil {
		return nil, err
	}
	orFilters, err := c.Validate(body.OrFilters)
	if err != nil {
		return nil, err
	}
	return MakeGroups(filters, orFilters), nil
}

// ParseGroups разбирает CLI-флаги: filterRaw — AND-условия, orFilterRaw — одна OR-скобка.
func (c *Config) ParseGroups(filterRaw, orFilterRaw []string) ([]FilterGroup, error) {
	filters, err := c.Parse(filterRaw)
	if err != nil {
		return nil, err
	}
	orFilters, err := c.Parse(orFilterRaw)
	if err != nil {
		return nil, err
	}
	return MakeGroups(filters, orFilters), nil
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
// Пример: "name[like]=zip|rar" = name LIKE '%zip%' OR name LIKE '%rar%'
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
		if bracketEnd < 0 || bracketEnd < bracketStart || strings.TrimSpace(key[bracketEnd+1:]) != "" {
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
		"**Фильтры** передаются в JSON body, каждый элемент содержит:\n"+
		"- `field` — имя поля (например: %s)\n"+
		"- `op` — оператор: eq, ne, like, gt, gte, lt, lte, contains (если не указан — используется оператор по умолчанию для поля)\n"+
		"- `value` — значение для сравнения\n\n"+
		"**Поля body:**\n"+
		"- `filters` — массив условий, объединяемых через AND\n"+
		"- `orFilters` — массив условий, образующих одну OR-скобку; она добавляется к `filters` через AND\n\n"+
		"**OR-логика внутри одного условия:**\n"+
		"- несколько значений через `|` в value: `\"value\": \"Games|Education\"`\n"+
		"- несколько полей через `|` в field: `\"field\": \"name|description\"`\n\n"+
		"Пример: `{\"filters\":[{\"field\":\"installed\",\"value\":\"true\"}],"+
		"\"orFilters\":[{\"field\":\"name\",\"op\":\"like\",\"value\":\"x\"},{\"field\":\"size\",\"op\":\"gt\",\"value\":\"1000\"}]}` "+
		"→ `installed = true AND (name LIKE '%%x%%' OR size > 1000)`\n\n"+
		"Остальные параметры передаются через query string.\n\n"+
		"**Пример**:\n"+
		"```\n"+
		"%s\n"+
		"Body: %s\n"+
		"```\n\n"+
		"Доступные поля и операторы можно получить через GET %s",
		subject, fieldsExample, exampleURL, exampleBody, filterFieldsURL)
}
