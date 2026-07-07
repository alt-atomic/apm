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

import (
	"context"

	"altlinux.space/alt-atomic/apm/pkg/command"
)

// RuntimeContext is the engine's neutral capability set available to every module.
type RuntimeContext interface {
	Runner() command.Runner
	CollectOutput(text string)
	EmitEvent(ctx context.Context, state, name, view string)
	ExecuteInclude(ctx context.Context, target string) (map[string]*MapModule, error)
}

// Body is a build module body; its returned value becomes the module output.
type Body interface {
	Execute(context.Context, RuntimeContext) (any, error)
}
