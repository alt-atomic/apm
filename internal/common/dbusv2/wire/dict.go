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

// Package wire задаёт проводные формы данных API v2: словари a{sv} и ошибки.
package wire

import (
	"fmt"
	"reflect"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"

	"github.com/godbus/dbus/v5"
)

// Dict расширяемая форма a{sv}, в которой возвращаются все сущности.
type Dict = map[string]dbus.Variant

// V оборачивает значение в Variant.
func V(v any) dbus.Variant {
	return dbus.MakeVariant(v)
}

// ParseOptions разбирает входные a{sv}-опции по карте "ключ -> указатель на приёмник".
// Неизвестный ключ или неподходящий тип — ошибка валидации.
func ParseOptions(opts Dict, spec map[string]any) error {
	for key, value := range opts {
		dst, ok := spec[key]
		if !ok {
			return apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf("unknown option %q", key))
		}
		dstType, err := optionDestinationType(key, dst)
		if err != nil {
			return err
		}
		if err := value.Store(dst); err != nil {
			want, sigErr := optionSignature(dstType)
			if sigErr != nil {
				return sigErr
			}
			return apmerr.New(apmerr.ErrorTypeValidation,
				fmt.Errorf("option %q: got %s, want %s", key, value.Signature(), want))
		}
	}
	return nil
}

func optionDestinationType(key string, dst any) (reflect.Type, error) {
	if dst == nil {
		return nil, fmt.Errorf("wire: option %q destination is nil", key)
	}
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return nil, fmt.Errorf("wire: option %q destination must be a non-nil pointer, got %T", key, dst)
	}
	return rv.Type().Elem(), nil
}

func optionSignature(t reflect.Type) (sig dbus.Signature, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("wire: unsupported option destination type %s: %v", t, recovered)
		}
	}()
	return dbus.SignatureOfType(t), nil
}
