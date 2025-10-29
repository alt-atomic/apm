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
	"apm/internal/common/app"
	"encoding/json"
	"errors"
	"fmt"
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
	TypeLink     = "link"
)

type Config struct {
	// Image to use as base
	Image string `yaml:"image" json:"image"`

	// Repos to need put as sources.list. If empty, will be used repos from Image
	Repos []string `yaml:"repos,omitempty" json:"repos,omitempty"`

	// Tasks to connect as repos
	Tasks []string `yaml:"tasks,omitempty" json:"tasks,omitempty"`

	// Kernel to use in image. If empty, will be used kernel from Image
	Kernel string `yaml:"kernel,omitempty" json:"kernel,omitempty"`

	// Modules list
	Modules []Module `yaml:"modules,omitempty" json:"modules,omitempty"`

	hasInclude bool
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

func (cfg *Config) HasInclude() bool {
	return cfg.hasInclude
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
		var packagesModule *Module
		if cfg.Modules[len(cfg.Modules)-1].Type != TypePackages || cfg.Modules[len(cfg.Modules)-1].Name != imageApplyModuleName {
			cfg.Modules = append(cfg.Modules, Module{
				Name: imageApplyModuleName,
				Type: TypePackages,
				Body: Body{
					Install: []string{},
					Remove:  []string{},
				},
			})
		}
		packagesModule = &cfg.Modules[len(cfg.Modules)-1]
		if slices.Contains(packagesModule.Body.Remove, pkg) {
			packagesModule.Body.Remove = removeByValue(packagesModule.Body.Remove, pkg)
		} else {
			packagesModule.Body.Install = append(packagesModule.Body.Install, pkg)
		}
	}
}

func (cfg *Config) AddRemovePackage(pkg string) {
	var totalRemove = cfg.getTotalRemove()

	if slices.Contains(totalRemove, pkg) {
		return
	} else {
		var packagesModule *Module
		if cfg.Modules[len(cfg.Modules)-1].Type != TypePackages || cfg.Modules[len(cfg.Modules)-1].Name != imageApplyModuleName {
			cfg.Modules = append(cfg.Modules, Module{
				Name: imageApplyModuleName,
				Type: TypePackages,
				Body: Body{
					Install: []string{},
					Remove:  []string{},
				},
			})
		}
		packagesModule = &cfg.Modules[len(cfg.Modules)-1]
		if slices.Contains(packagesModule.Body.Install, pkg) {
			packagesModule.Body.Install = removeByValue(packagesModule.Body.Install, pkg)
		} else {
			packagesModule.Body.Remove = append(packagesModule.Body.Remove, pkg)
		}
	}
}

// Extend includes
func (cfg *Config) extendIncludes() error {
	var newModules = []Module{}
	cfg.hasInclude = false

	for _, module := range cfg.Modules {
		if module.Type == TypeInclude {
			cfg.hasInclude = true
			for _, include := range module.Body.GetTargets() {
				data, err := os.ReadFile(include)
				if err != nil {
					return err
				}
				include_cfg, err := parseData(data, true, false)
				if err != nil {
					return err
				}
				err = include_cfg.extendIncludes()
				if err != nil {
					return err
				}

				newModules = append(newModules, include_cfg.Modules...)
			}
		} else {
			newModules = append(newModules, module)
		}
	}

	cfg.Modules = newModules

	return nil
}

// Extend includes
func (cfg *Config) fix() error {
	if sE(cfg.Image) {
		return errors.New(app.T_("Image can not be empty"))
	}

	var reqiredText = app.T_("Module '%s' required '%s'")
	var reqiredTextOr = fmt.Sprintf(reqiredText, "%s", app.T_("%s or %s"))

	for _, module := range cfg.Modules {
		if sE(module.Type) {
			return errors.New(app.T_("Module type can not be empty"))
		}

		var b = module.Body

		switch module.Type {
		case TypeGit:
			if aE(b.Commands) {
				return fmt.Errorf(reqiredText, TypeGit, "commands")
			}
		case TypeShell:
			if aE(b.Commands) {
				return fmt.Errorf(reqiredText, TypeShell, "commands")
			}
		case TypeMerge:
			if sE(b.Target) {
				return fmt.Errorf(reqiredText, TypeMerge, "target")
			}
			if sE(b.Destination) {
				return fmt.Errorf(reqiredText, TypeMerge, "destination")
			}
		case TypeCopy:
			if aE(b.GetTargets()) {
				return fmt.Errorf(reqiredText, TypeCopy, "targets")
			}
			if sE(b.Destination) {
				return fmt.Errorf(reqiredText, TypeCopy, "destination")
			}
		case TypeMove:
			if aE(b.GetTargets()) {
				return fmt.Errorf(reqiredTextOr, TypeMove, "target", "targets")
			}
			if sE(b.Destination) {
				return fmt.Errorf(reqiredText, TypeMove, "destination")
			}
		case TypeRemove:
			if aE(b.GetTargets()) {
				return fmt.Errorf(reqiredTextOr, TypeRemove, "target", "targets")
			}
		case TypeSystemd:
			if aE(b.GetTargets()) {
				return fmt.Errorf(reqiredTextOr, TypeSystemd, "target", "targets")
			}
		case TypeLink:
			if aE(b.GetTargets()) {
				return fmt.Errorf(reqiredTextOr, TypeLink, "target", "targets")
			}
			if sE(b.Destination) {
				return fmt.Errorf(reqiredText, TypeLink, "destination")
			}
		case TypePackages:
		case TypeInclude:
			return errors.New(app.T_("Include should be extended"))
		default:
			return errors.New(app.T_("Unknown type: " + module.Type))
		}
	}

	return nil
}

// Check and extend includes
func (cfg *Config) CheckAndFix() error {
	if err := cfg.extendIncludes(); err != nil {
		return err
	}
	if err := cfg.fix(); err != nil {
		return err
	}

	return nil
}

// Check and extend includes
func (cfg *Config) Save(filename string) error {
	var err error
	if err = cfg.CheckAndFix(); err != nil {
		return err
	}

	var data []byte
	if data, err = yaml.Marshal(cfg); err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

type Module struct {
	// Name of module for logging
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Type of body
	Type string `yaml:"type" json:"type"`

	// Body of module
	Body Body `yaml:"body" json:"body"`
}

type Body struct {
	// Types: git, shell
	// Commands to execute relative to resorces dir
	Commands []string `yaml:"commands,omitempty" json:"commands,omitempty"`

	// Types: [git]
	// Deps for module. They will be removed at the module end
	Deps []string `yaml:"deps,omitempty" json:"deps,omitempty"`

	// Types: merge, include, copy, move, remove, systemd, link
	// Target what use in type
	// Relative path to /var/apm/resources in merge, include, copy
	// Absolute path in move, remove
	// Service name in systemd
	Target string `yaml:"target,omitempty" json:"target,omitempty"`

	// Types: include, copy, move, remove, systemd, link
	// Targets what use in type
	// Relative paths to /var/apm/resources in include, copy
	// Absolute paths in move, remove
	// Service names in systemd
	Targets []string `yaml:"targets,omitempty" json:"targets,omitempty"`

	// Types: copy, move, merge, link
	// Directory to use as destination
	Destination string `yaml:"destination,omitempty" json:"destination,omitempty"`

	// Types: packages
	// Packages to install from repos/tasks
	Install []string `yaml:"install,omitempty" json:"install,omitempty"`

	// Types: packages
	// Packages to remove from image
	Remove []string `yaml:"remove,omitempty" json:"remove,omitempty"`

	// Types: [copy], [move]
	// Replace destination if it exists
	Replace bool `yaml:"replace,omitempty" json:"replace,omitempty"`

	// Types: [move]
	// Make link from targets parent dir to destination
	CreateLink bool `yaml:"create-link,omitempty" json:"create-link,omitempty"`

	// Types: systemd
	// Enable or disable systemd service
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// Check and extend includes
func (b *Body) GetTargets() []string {
	var targets = []string{}

	if !sE(b.Target) {
		targets = append(targets, b.Target)
	}
	if !aE(b.Targets) {
		targets = append(targets, b.Targets...)
	}

	return targets
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
	return parseData(data, true, true)
}

// Includes will return error
func ParseJsonData(data []byte) (cfg Config, err error) {
	return parseData(data, false, true)
}

func parseData(data []byte, is_yaml bool, fix bool) (cfg Config, err error) {
	if is_yaml {
		err = yaml.Unmarshal(data, &cfg)
	} else {
		err = json.Unmarshal(data, &cfg)
	}

	if err != nil {
		return cfg, err
	}
	if fix {
		err = cfg.CheckAndFix()
		if err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

func removeByValue(arr []string, value string) []string {
	return slices.DeleteFunc(arr, func(s string) bool {
		return s == value
	})
}

// string is empty
func sE(s string) bool {
	return s == ""
}

// array is empty
func aE(a []string) bool {
	return len(a) == 0
}
