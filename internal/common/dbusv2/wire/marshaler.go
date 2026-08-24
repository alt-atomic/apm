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
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/godbus/dbus/v5"
)

var jsonMarshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()

// asJSONMarshaler возвращает json.Marshaler значения, если тип объявил собственную JSON-форму.
func asJSONMarshaler(rv reflect.Value) (json.Marshaler, bool) {
	if rv.Type().Implements(jsonMarshalerType) {
		if (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) && rv.IsNil() {
			return nil, false
		}
		return rv.Interface().(json.Marshaler), true
	}
	if rv.CanAddr() && reflect.PointerTo(rv.Type()).Implements(jsonMarshalerType) {
		return rv.Addr().Interface().(json.Marshaler), true
	}
	return nil, false
}

// jsonVariant сериализует значение через его собственный json.Marshaler.
func jsonVariant(m json.Marshaler) (dbus.Variant, bool, error) {
	data, err := m.MarshalJSON()
	if err != nil {
		return dbus.Variant{}, false, err
	}
	var v any
	if err = json.Unmarshal(data, &v); err != nil {
		return dbus.Variant{}, false, err
	}
	return jsonAnyVariant(v)
}

// jsonAnyVariant конвертирует распакованный JSON-узел в Variant.
func jsonAnyVariant(v any) (dbus.Variant, bool, error) {
	switch t := v.(type) {
	case nil:
		return dbus.Variant{}, false, nil
	case string, bool, float64:
		return dbus.MakeVariant(t), true, nil
	case map[string]any:
		d := make(Dict, len(t))
		for key, value := range t {
			item, present, err := jsonAnyVariant(value)
			if err != nil {
				return dbus.Variant{}, false, err
			}
			if present {
				d[key] = item
			}
		}
		return dbus.MakeVariant(d), true, nil
	case []any:
		return jsonArrayVariant(t)
	default:
		return dbus.Variant{}, false, fmt.Errorf("unsupported json value %T", v)
	}
}

// jsonArrayVariant отдаёт однородный строковый массив как as, остальное — av.
func jsonArrayVariant(items []any) (dbus.Variant, bool, error) {
	strs := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			strs = append(strs, s)
		}
	}
	if len(strs) == len(items) {
		return dbus.MakeVariant(strs), true, nil
	}

	variants := make([]dbus.Variant, 0, len(items))
	for _, item := range items {
		v, present, err := jsonAnyVariant(item)
		if err != nil {
			return dbus.Variant{}, false, err
		}
		if present {
			variants = append(variants, v)
		}
	}
	return dbus.MakeVariant(variants), true, nil
}
