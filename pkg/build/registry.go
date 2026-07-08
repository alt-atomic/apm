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

package build

import "fmt"

// bodyRegistry is the open registry of module body factories.
var bodyRegistry = map[string]func() Body{}

// Register maps a module type name to its body factory.
func Register(typeName string, factory func() Body) {
	bodyRegistry[typeName] = factory
}

// NewBody creates an empty body for the given module type.
func NewBody(typeName string) (Body, error) {
	factory, ok := bodyRegistry[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown module type: %s", typeName)
	}
	return factory(), nil
}
