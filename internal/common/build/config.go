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

import (
	"encoding/json"
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

var imageApplyModuleName = "Image apply result"

const (
	TypeGit      = "git"
	TypeShell    = "shell"
	TypeMerge    = "merge"
	TypeInclude  = "include"
	TypeCopy     = "copy"
	TypeMove     = "move"
	TypeRemove   = "remove"
	TypeSystemd  = "systemd"
	TypePackages = "packages"
)

type Config struct {
	// Image to use as base
	Image string `yaml:"image" json:"image"`

	// Repos to need put as sources.list. If empty, will be used repos from Image
	Repos []string `yaml:"repos" json:"repos"`

	// Tasks to connect as repos
	Tasks []string `yaml:"tasks" json:"tasks"`

	// Kernel to use in image. If empty, will be used kernel from Image
	Kernel string `yaml:"kernel" json:"kernel"`

	// Modules list
	Modules []Module `yaml:"modules" json:"modules"`
}

func (cfg *Config) getTotalInstall() []string {
	var totalInstall = []string{}

	for _, module := range cfg.Modules {
		if module.Type == TypePackages {
			totalInstall = append(totalInstall, module.Body.Install...)

			for _, p := range module.Body.Remove {
				removeByValue(totalInstall, p)
			}
		}
	}

	return totalInstall
}

func (cfg *Config) getTotalRemove() []string {
	var totalRemove = []string{}

	for _, module := range cfg.Modules {
		if module.Type == TypePackages {
			for _, p := range module.Body.Install {
				removeByValue(totalRemove, p)
			}

			totalRemove = append(totalRemove, module.Body.Remove...)
		}
	}

	return totalRemove
}

func (cfg *Config) IsInstalled(pkg string) bool {
	return slices.Contains(cfg.getTotalInstall(), pkg)
}

func (cfg *Config) IsRemoved(pkg string) bool {
	return slices.Contains(cfg.getTotalRemove(), pkg)
}

func (cfg *Config) AddInstallPackage(pkg string) {
	var totalInstall = cfg.getTotalInstall()

	if slices.Contains(totalInstall, pkg) {
		return
	} else {
		var packagesModule Module
		if cfg.Modules[len(cfg.Modules)-1].Type == TypePackages && cfg.Modules[len(cfg.Modules)-1].Name == imageApplyModuleName {
			packagesModule = cfg.Modules[len(cfg.Modules)-1]
		} else {
			packagesModule = Module{
				Name: imageApplyModuleName,
				Type: TypePackages,
				Body: Body{
					Install: []string{},
					Remove:  []string{},
				},
			}
			cfg.Modules = append(cfg.Modules, packagesModule)
		}
		packagesModule.Body.Install = append(packagesModule.Body.Install, pkg)
	}
}

func (cfg *Config) AddRemovePackage(pkg string) {
	var totalRemove = cfg.getTotalRemove()

	if slices.Contains(totalRemove, pkg) {
		return
	} else {
		var packagesModule Module
		if cfg.Modules[len(cfg.Modules)-1].Type == TypePackages && cfg.Modules[len(cfg.Modules)-1].Name == imageApplyModuleName {
			packagesModule = cfg.Modules[len(cfg.Modules)-1]
		} else {
			packagesModule = Module{
				Name: imageApplyModuleName,
				Type: TypePackages,
				Body: Body{
					Install: []string{},
					Remove:  []string{},
				},
			}
			cfg.Modules = append(cfg.Modules, packagesModule)
		}
		packagesModule.Body.Remove = append(packagesModule.Body.Remove, pkg)
	}
}

// Check and extend includes
func (cfg *Config) HasInclude() bool {
	for _, module := range cfg.Modules {
		if module.Type == TypeInclude {
			return true
		}
	}

	return false
}

// Check and extend includes
func (cfg *Config) finalize() {

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

// Includes will be extended
func ReadAndParseYamlFile(name string) (cfg Config, err error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return cfg, err
	}
	return ParseYamlData(data)
}

// Includes will be extended
func ParseYamlData(data []byte) (cfg Config, err error) {
	return parseData(data, true)
}

// Includes will return error
func ParseJsonData(data []byte) (cfg Config, err error) {
	return parseData(data, false)
}

func parseData(data []byte, is_yaml bool) (cfg Config, err error) {
	if is_yaml {
		err = yaml.Unmarshal(data, &cfg)
	} else {
		err = json.Unmarshal(data, &cfg)
	}

	if err != nil {
		return cfg, err
	}

	cfg.finalize()

	return cfg, nil
}

func removeByValue(arr []string, value string) []string {
	return slices.DeleteFunc(arr, func(s string) bool {
		return s == value
	})
}
