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

package dbusv2

import (
	"fmt"
	"reflect"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

var (
	senderType    = reflect.TypeOf(dbus.Sender(""))
	dbusErrorType = reflect.TypeOf((*dbus.Error)(nil))
)

// VerifyIntrospection сверяет описание интерфейса с реальной сигнатурой реализации.
// Ловит рассинхрон "объявили одно, экспортируем другое" в тестах.
func VerifyIntrospection(iface introspect.Interface, impl any) error {
	tv := reflect.TypeOf(impl)

	declared := make(map[string]bool, len(iface.Methods))
	for _, m := range iface.Methods {
		declared[m.Name] = true
		method, ok := tv.MethodByName(m.Name)
		if !ok {
			return fmt.Errorf("%s.%s: declared but not implemented", iface.Name, m.Name)
		}
		if err := verifyMethod(m, method.Type); err != nil {
			return fmt.Errorf("%s.%s: %w", iface.Name, m.Name, err)
		}
	}

	for i := 0; i < tv.NumMethod(); i++ {
		name := tv.Method(i).Name
		if !declared[name] {
			return fmt.Errorf("%s: exported method %s missing from introspection", iface.Name, name)
		}
	}
	return nil
}

// verifyMethod сверяет in/out аргументы описания с Go-сигнатурой.
func verifyMethod(m introspect.Method, mt reflect.Type) error {
	var in, out []string
	for i := 1; i < mt.NumIn(); i++ {
		if t := mt.In(i); t != senderType {
			in = append(in, dbus.SignatureOfType(t).String())
		}
	}
	for i := 0; i < mt.NumOut(); i++ {
		if t := mt.Out(i); t != dbusErrorType {
			out = append(out, dbus.SignatureOfType(t).String())
		}
	}

	var declIn, declOut []string
	for _, a := range m.Args {
		if a.Direction == "in" {
			declIn = append(declIn, a.Type)
		} else {
			declOut = append(declOut, a.Type)
		}
	}

	if !reflect.DeepEqual(in, declIn) {
		return fmt.Errorf("in args: declared %v, implemented %v", declIn, in)
	}
	if !reflect.DeepEqual(out, declOut) {
		return fmt.Errorf("out args: declared %v, implemented %v", declOut, out)
	}
	return nil
}
