// Atomic Package Manager
// Copyright (C) 2025 Vladimir Romanov <rirusha@altlinux.org>
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

type Config struct {
	// Image to use as base
	Image string `yaml:"image" json:"image"`

	// Repos to need put as sources.list
	Repos []string `yaml:"repos" json:"repos"`

	// Tasks to connect as repos
	Tasks []string `yaml:"tasks" json:"tasks"`

	// Kernel to use in image
	Kernel []string `yaml:"kernel" json:"kernel"`

	// Modules list
	Modules []Module `yaml:"modules" json:"modules"`
}

type Module struct {
	// Name of module for logging
	Name string `yaml:"name" json:"name"`

	// Type of body
	Type string `yaml:"type" json:"type"`

	// Body of module
	Body Body `yaml:"body" json:"body"`
}

type Body struct {
	// Types: git, shell
	// Commands to execute as script
	Commands string `yaml:"commands" json:"commands"`

	// Types: [git]
	// Deps for module. They will be removed at the module end
	Deps []string `yaml:"deps" json:"deps"`

	// Types: merge, include, copy, move, remove, systemd
	// Target what use in type
	// Relative path to /var/apm/resources in merge, include, copy
	// Absolute path in move, remove
	// Service name in systemd
	Target string `yaml:"target" json:"target"`

	// Types: include, copy, move, remove
	// Targets what use in type
	// Relative paths to /var/apm/resources in include, copy
	// Absolute paths in move, remove
	Targets []string `yaml:"targets" json:"targets"`

	// Types: copy, move, merge
	// Directory to use as destination
	Destination string `yaml:"destination" json:"destination"`

	// Types: packages
	// Packages to install from repos/tasks
	Install []string `yaml:"install" json:"install"`

	// Types: packages
	// Packages to remove from image
	Remove []string `yaml:"remove" json:"remove"`

	// Types: [copy], [move]
	// Replace destination if it exists
	Replace bool `yaml:"replace" json:"replace"`

	// Types: [move]
	// Make link from targets parent dir to destination
	CreateLink bool `yaml:"create-link" json:"create-link"`
}
