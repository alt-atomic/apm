package build

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"

	"github.com/goccy/go-yaml"
)

type Envs struct {
	Env map[string]string `yaml:"env" json:"env"`
}

type Config struct {
	// Environment vars
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	// Base image to build from
	Image string `yaml:"image,omitempty" json:"image,omitempty"`

	// Module list
	Modules []Module `yaml:"modules,omitempty" json:"modules,omitempty"`
}

func (cfg *Config) CheckImage() error {
	if cfg.Image == "" {
		return errors.New(T_("Image can not be empty"))
	}

	return nil
}

func (cfg *Config) checkModules() error {
	return CheckModules(&cfg.Modules)
}

func (cfg *Config) Save(filename string) error {
	if err := cfg.checkModules(); err != nil {
		return err
	}

	data, err := yaml.MarshalWithOptions(cfg, yaml.UseLiteralStyleIfMultiline(true))
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

type Module struct {
	// Module name, for logging
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Module body type
	Type string `yaml:"type" json:"type"`

	// Module ID
	Id string `yaml:"id,omitempty" json:"id,omitempty"`

	// Environmant vars for module
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	// Condition in expr language
	If string `yaml:"if,omitempty" json:"if,omitempty"`

	// Module body
	Body Body `yaml:"body" json:"body"`

	// Output data
	Output map[string]string `yaml:"output,omitempty" json:"output,omitempty"`
}

func (m Module) GetLabel() any {
	if m.Name != "" {
		return m.Name
	} else if m.Id != "" {
		return fmt.Sprintf("id=%s", m.Id)
	}

	return fmt.Sprintf("type=%s", m.Type)
}

func CheckModules(modules *[]Module) error {
	moduleIds := []string{}

	re, err := regexp.Compile(`^[A-z][A-z0-9_]*$`)
	if err != nil {
		return err
	}

	for _, module := range *modules {
		if module.Id != "" {
			matched := re.MatchString(module.Id)

			if !matched {
				return fmt.Errorf(T_("Invalid id '%s'. Acceptable characters are letters, numbers, and underscores. The key must start with a letter."), module.Id)
			}
			if slices.Contains(moduleIds, module.Id) {
				return fmt.Errorf("found id collision")
			}
			moduleIds = append(moduleIds, module.Id)
		}
		if module.Body == nil {
			return fmt.Errorf("module %s has empty body", module.Type)
		}
		if err := CheckBody(module.Body); err != nil {
			return err
		}
	}

	return nil
}
