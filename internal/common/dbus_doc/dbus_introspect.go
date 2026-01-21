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

package dbus_doc

import (
	"strings"

	"github.com/godbus/dbus/v5/introspect"
)

// GenerateIntrospectXML генерирует XML интроспекции из переданных интерфейсов
// interfaces - map[interfaceName]wrapperStruct
func GenerateIntrospectXML(interfaces map[string]any) string {
	var sb strings.Builder

	sb.WriteString(`<node>
  <interface name="org.altlinux.APM">
    <signal name="Notification">
      <arg type="s" name="message" direction="out"/>
    </signal>
  </interface>
`)

	for ifaceName, wrapper := range interfaces {
		methods := introspect.Methods(wrapper)
		iface := introspect.Interface{
			Name:    ifaceName,
			Methods: methods,
		}

		sb.WriteString("\n")
		sb.WriteString(generateInterfaceXML(iface))
	}

	sb.WriteString("\n")
	sb.WriteString(introspect.IntrospectDataString)
	sb.WriteString("</node>")

	return sb.String()
}

// generateInterfaceXML генерирует XML для одного интерфейса
func generateInterfaceXML(iface introspect.Interface) string {
	var sb strings.Builder

	sb.WriteString(`  <interface name="`)
	sb.WriteString(iface.Name)
	sb.WriteString("\">\n")

	for _, method := range iface.Methods {
		sb.WriteString(`    <method name="`)
		sb.WriteString(method.Name)
		sb.WriteString("\">\n")

		for _, arg := range method.Args {
			sb.WriteString(`      <arg direction="`)
			sb.WriteString(arg.Direction)
			sb.WriteString(`" type="`)
			sb.WriteString(arg.Type)
			sb.WriteString(`" name="`)
			sb.WriteString(arg.Name)
			sb.WriteString("\"/>\n")
		}

		sb.WriteString("    </method>\n")
	}

	sb.WriteString("  </interface>\n")

	return sb.String()
}
