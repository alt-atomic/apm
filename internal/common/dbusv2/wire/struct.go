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
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"
)

var textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()

// StructDict сериализует DTO-структуру в Dict по json-тегам, без полного JSON-раунда:
// строки — s, целые — x/t, float — d, вложенные структуры — a{sv}.
// Nil-поля опускаются: D-Bus не имеет nullable/maybe-типа.
func StructDict(v any) (Dict, error) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return Dict{}, nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("wire: StructDict wants a struct, got %s", rv.Kind())
	}
	// Адресуемая копия: json.Marshaler с pointer receiver должен находиться у полей.
	addr := reflect.New(rv.Type())
	addr.Elem().Set(rv)
	return structDict(addr.Elem())
}

// StructDicts сериализует срез DTO в aa{sv}.
func StructDicts[T any](items []T) ([]Dict, error) {
	out := make([]Dict, 0, len(items))
	for i := range items {
		d, err := StructDict(items[i])
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i, err)
		}
		out = append(out, d)
	}
	return out, nil
}

// structDict собирает словарь из экспортируемых полей структуры.
func structDict(rv reflect.Value) (Dict, error) {
	if !rv.CanAddr() {
		addr := reflect.New(rv.Type())
		addr.Elem().Set(rv)
		rv = addr.Elem()
	}
	d := Dict{}
	if err := appendFields(d, rv); err != nil {
		return nil, err
	}
	return d, nil
}

// appendFields добавляет поля структуры в словарь; анонимные структуры разворачиваются как в encoding/json.
func appendFields(d Dict, rv reflect.Value) error {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		name, omitEmpty, skip := jsonFieldName(field)
		if skip {
			continue
		}
		value := rv.Field(i)

		if field.Anonymous && !hasJSONName(field) {
			embedded, present := indirectValue(value)
			if present && embedded.Kind() == reflect.Struct {
				if err := appendFields(d, embedded); err != nil {
					return err
				}
				continue
			}
			if !present {
				continue
			}
		}
		if omitEmpty && isEmptyValue(value) {
			continue
		}

		variant, present, err := fieldVariant(value)
		if err != nil {
			return fmt.Errorf("field %s: %w", field.Name, err)
		}
		if present {
			d[name] = variant
		}
	}
	return nil
}

func indirectValue(rv reflect.Value) (reflect.Value, bool) {
	for rv.IsValid() && (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) {
		if rv.IsNil() {
			return reflect.Value{}, false
		}
		rv = rv.Elem()
	}
	return rv, rv.IsValid()
}

// isEmptyValue повторяет семантику omitempty из encoding/json.
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Pointer, reflect.Interface:
		return v.IsZero()
	default:
		return false
	}
}

// jsonFieldName возвращает проводное имя поля из json-тега.
func jsonFieldName(field reflect.StructField) (name string, omitEmpty, skip bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	name, opts, _ := strings.Cut(tag, ",")
	if name == "" {
		name = field.Name
	}
	for opts != "" {
		var opt string
		opt, opts, _ = strings.Cut(opts, ",")
		if opt == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}

// hasJSONName сообщает, задано ли явное имя в json-теге.
func hasJSONName(field reflect.StructField) bool {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	return name != "" && name != "-"
}

// fieldVariant конвертирует значение поля в Variant; present=false — nil-поле опускается.
func fieldVariant(rv reflect.Value) (dbus.Variant, bool, error) {
	for {
		if m, ok := asJSONMarshaler(rv); ok {
			return jsonVariant(m)
		}
		if rv.Kind() != reflect.Pointer && rv.Kind() != reflect.Interface {
			break
		}
		if rv.IsNil() {
			return dbus.Variant{}, false, nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		return dbus.MakeVariant(rv.String()), true, nil
	case reflect.Bool:
		return dbus.MakeVariant(rv.Bool()), true, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return dbus.MakeVariant(rv.Int()), true, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return dbus.MakeVariant(rv.Uint()), true, nil
	case reflect.Float32, reflect.Float64:
		return dbus.MakeVariant(rv.Float()), true, nil
	case reflect.Struct:
		d, err := structDict(rv)
		if err != nil {
			return dbus.Variant{}, false, err
		}
		return dbus.MakeVariant(d), true, nil
	case reflect.Slice, reflect.Array:
		return sliceVariant(rv)
	case reflect.Map:
		return mapVariant(rv)
	default:
		return dbus.Variant{}, false, fmt.Errorf("unsupported kind %s", rv.Kind())
	}
}

// sliceVariant конвертирует срез в однородный DBus-массив.
func sliceVariant(rv reflect.Value) (dbus.Variant, bool, error) {
	elemType := rv.Type().Elem()
	if elemType.Kind() == reflect.Uint8 {
		items := make([]byte, rv.Len())
		for i := range items {
			items[i] = byte(rv.Index(i).Uint())
		}
		return dbus.MakeVariant(items), true, nil
	}
	switch elemType.Kind() {
	case reflect.String:
		return scalarSlice(rv, reflect.Value.String), true, nil
	case reflect.Bool:
		return scalarSlice(rv, reflect.Value.Bool), true, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return scalarSlice(rv, reflect.Value.Int), true, nil
	case reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return scalarSlice(rv, reflect.Value.Uint), true, nil
	case reflect.Float32, reflect.Float64:
		return scalarSlice(rv, reflect.Value.Float), true, nil
	case reflect.Struct, reflect.Pointer:
		return structSliceVariant(rv)
	default:
		return variantSlice(rv)
	}
}

// structSliceVariant кодирует только структуры как aa{sv}; тип элемента не меняется от данных.
func structSliceVariant(rv reflect.Value) (dbus.Variant, bool, error) {
	elemType := rv.Type().Elem()
	if typeHasJSONMarshaler(elemType) {
		return dbus.Variant{}, false,
			fmt.Errorf("array element type %s has a custom JSON representation", elemType)
	}
	baseType := elemType
	for baseType.Kind() == reflect.Pointer {
		baseType = baseType.Elem()
	}
	if baseType.Kind() != reflect.Struct {
		return dbus.Variant{}, false, fmt.Errorf("array element type %s is not a struct", elemType)
	}

	items := make([]Dict, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		item, present := indirectValue(rv.Index(i))
		if !present {
			return dbus.Variant{}, false, fmt.Errorf("array item %d is nil; D-Bus has no nullable array elements", i)
		}
		dict, err := structDict(item)
		if err != nil {
			return dbus.Variant{}, false, fmt.Errorf("array item %d: %w", i, err)
		}
		items = append(items, dict)
	}
	return dbus.MakeVariant(items), true, nil
}

// variantSlice кодирует составные или интерфейсные элементы как av.
func variantSlice(rv reflect.Value) (dbus.Variant, bool, error) {
	items := make([]dbus.Variant, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		variant, present, err := fieldVariant(rv.Index(i))
		if err != nil {
			return dbus.Variant{}, false, fmt.Errorf("array item %d: %w", i, err)
		}
		if !present {
			return dbus.Variant{}, false, fmt.Errorf("array item %d is nil; D-Bus has no nullable array elements", i)
		}
		items = append(items, variant)
	}
	return dbus.MakeVariant(items), true, nil
}

// scalarSlice собирает типизированный срез скаляров из reflect-значений.
func scalarSlice[T any](rv reflect.Value, get func(reflect.Value) T) dbus.Variant {
	items := make([]T, rv.Len())
	for i := range items {
		items[i] = get(rv.Index(i))
	}
	return dbus.MakeVariant(items)
}

// mapVariant конвертирует map в a{ss} или a{sv}; ключи приводятся к строке.
func mapVariant(rv reflect.Value) (dbus.Variant, bool, error) {
	key, err := mapKeyFunc(rv.Type().Key())
	if err != nil {
		return dbus.Variant{}, false, err
	}

	if rv.Type().Elem().Kind() == reflect.String {
		items := make(map[string]string, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			name, err := key(iter.Key())
			if err != nil {
				return dbus.Variant{}, false, err
			}
			items[name] = iter.Value().String()
		}
		return dbus.MakeVariant(items), true, nil
	}

	items := make(Dict, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		name, err := key(iter.Key())
		if err != nil {
			return dbus.Variant{}, false, err
		}
		v, present, err := fieldVariant(iter.Value())
		if err != nil {
			return dbus.Variant{}, false, fmt.Errorf("map key %q: %w", name, err)
		}
		if present {
			items[name] = v
		}
	}
	return dbus.MakeVariant(items), true, nil
}

// mapKeyFunc возвращает конвертер ключа карты в строку.
func mapKeyFunc(t reflect.Type) (func(reflect.Value) (string, error), error) {
	if t.Implements(textMarshalerType) || reflect.PointerTo(t).Implements(textMarshalerType) {
		return func(v reflect.Value) (string, error) {
			if !v.Type().Implements(textMarshalerType) {
				addr := reflect.New(v.Type())
				addr.Elem().Set(v)
				v = addr
			}
			text, err := v.Interface().(encoding.TextMarshaler).MarshalText()
			return string(text), err
		}, nil
	}

	switch t.Kind() {
	case reflect.String:
		return func(v reflect.Value) (string, error) { return v.String(), nil }, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func(v reflect.Value) (string, error) { return strconv.FormatInt(v.Int(), 10), nil }, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return func(v reflect.Value) (string, error) { return strconv.FormatUint(v.Uint(), 10), nil }, nil
	default:
		return nil, fmt.Errorf("unsupported map key %s", t)
	}
}
