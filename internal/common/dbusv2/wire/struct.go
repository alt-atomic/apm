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
	"reflect"
	"strings"

	"github.com/godbus/dbus/v5"
)

// StructDict сериализует DTO-структуру в Dict по json-тегам, без JSON-раунда:
// строки — s, целые — x/t, float — d, вложенные структуры — a{sv}.
// DTO ответа и есть проводной контракт — тот же, что у HTTP-транспорта.
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
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// structDict собирает словарь из экспортируемых полей структуры.
func structDict(rv reflect.Value) (Dict, error) {
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

		if field.Anonymous && field.Type.Kind() == reflect.Struct && !hasJSONName(field) {
			if err := appendFields(d, value); err != nil {
				return err
			}
			continue
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

// isEmptyValue повторяет семантику omitempty из encoding/json:
// пустые срезы/карты/строки опускаются независимо от nil.
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	default:
		return v.IsZero()
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

// fieldVariant конвертирует значение поля в Variant; present=false — поле опускается.
func fieldVariant(rv reflect.Value) (dbus.Variant, bool, error) {
	if m, ok := asJSONMarshaler(rv); ok {
		return jsonVariant(m)
	}
	switch rv.Kind() {
	case reflect.String:
		return dbus.MakeVariant(rv.String()), true, nil
	case reflect.Bool:
		return dbus.MakeVariant(rv.Bool()), true, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return dbus.MakeVariant(rv.Int()), true, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return dbus.MakeVariant(rv.Uint()), true, nil
	case reflect.Float32, reflect.Float64:
		return dbus.MakeVariant(rv.Float()), true, nil
	case reflect.Struct:
		d, err := structDict(rv)
		if err != nil {
			return dbus.Variant{}, false, err
		}
		return dbus.MakeVariant(d), true, nil
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			return dbus.Variant{}, false, nil
		}
		return fieldVariant(rv.Elem())
	case reflect.Slice, reflect.Array:
		return sliceVariant(rv)
	case reflect.Map:
		return mapVariant(rv)
	default:
		return dbus.Variant{}, false, fmt.Errorf("unsupported kind %s", rv.Kind())
	}
}

// sliceVariant конвертирует срез в типизированный DBus-массив.
func sliceVariant(rv reflect.Value) (dbus.Variant, bool, error) {
	switch rv.Type().Elem().Kind() {
	case reflect.String:
		return scalarSlice(rv, reflect.Value.String), true, nil
	case reflect.Bool:
		return scalarSlice(rv, reflect.Value.Bool), true, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return scalarSlice(rv, reflect.Value.Int), true, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return scalarSlice(rv, reflect.Value.Uint), true, nil
	case reflect.Float32, reflect.Float64:
		return scalarSlice(rv, reflect.Value.Float), true, nil
	case reflect.Struct, reflect.Pointer:
		items := make([]Dict, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			el := rv.Index(i)
			for el.Kind() == reflect.Pointer && !el.IsNil() {
				el = el.Elem()
			}
			if el.Kind() != reflect.Struct {
				continue
			}
			d, err := structDict(el)
			if err != nil {
				return dbus.Variant{}, false, err
			}
			items = append(items, d)
		}
		return dbus.MakeVariant(items), true, nil
	default:
		items := make([]dbus.Variant, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			v, present, err := fieldVariant(rv.Index(i))
			if err != nil {
				return dbus.Variant{}, false, err
			}
			if present {
				items = append(items, v)
			}
		}
		return dbus.MakeVariant(items), true, nil
	}
}

// scalarSlice собирает типизированный срез скаляров из reflect-значений.
func scalarSlice[T any](rv reflect.Value, get func(reflect.Value) T) dbus.Variant {
	items := make([]T, rv.Len())
	for i := range items {
		items[i] = get(rv.Index(i))
	}
	return dbus.MakeVariant(items)
}

// mapVariant конвертирует map со строковым ключом в a{ss} или a{sv}.
func mapVariant(rv reflect.Value) (dbus.Variant, bool, error) {
	if rv.Type().Key().Kind() != reflect.String {
		return dbus.Variant{}, false, fmt.Errorf("unsupported map key %s", rv.Type().Key())
	}

	if rv.Type().Elem().Kind() == reflect.String {
		items := make(map[string]string, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			items[iter.Key().String()] = iter.Value().String()
		}
		return dbus.MakeVariant(items), true, nil
	}

	items := make(Dict, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		v, present, err := fieldVariant(iter.Value())
		if err != nil {
			return dbus.Variant{}, false, fmt.Errorf("map key %q: %w", iter.Key().String(), err)
		}
		if present {
			items[iter.Key().String()] = v
		}
	}
	return dbus.MakeVariant(items), true, nil
}
