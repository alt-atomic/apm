package build

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"altlinux.space/alt-atomic/apm/pkg/osutils"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
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

	data, err := yaml.Marshal(cfg)
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

func (m *Module) UnmarshalYAML(n ast.Node) error {
	var aux struct {
		Name   string            `yaml:"name"`
		Env    map[string]string `yaml:"env"`
		If     string            `yaml:"if"`
		Id     string            `yaml:"id"`
		Type   string            `yaml:"type"`
		Body   ast.MappingNode   `yaml:"body"`
		Output map[string]string `yaml:"output"`
	}

	decoder := yaml.NewDecoder(n, yaml.DisallowUnknownField())
	if err := decoder.Decode(&aux); err != nil {
		return err
	}

	m.Name = aux.Name
	m.Env = aux.Env
	m.If = aux.If
	m.Id = aux.Id
	m.Type = aux.Type
	m.Output = aux.Output
	return m.decodeBody(func(target any) error {
		decoder := yaml.NewDecoder(
			&aux.Body,
			yaml.DisallowUnknownField(),
		)
		if err := decoder.Decode(target); err != nil {
			return err
		}
		return nil
	})
}

func (m *Module) UnmarshalJSON(data []byte) error {
	type alias Module
	aux := &struct {
		Body json.RawMessage `json:"body"`
		*alias
	}{
		alias: (*alias)(m),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	return m.decodeBody(func(target any) error {
		dec := json.NewDecoder(bytes.NewReader(aux.Body))
		dec.DisallowUnknownFields()
		return dec.Decode(target)
	})
}

// decodeBody selects and decodes the body by module type.
func (m *Module) decodeBody(decode func(any) error) error {
	if m.Type == "" {
		return fmt.Errorf("module type is required")
	}

	body, err := NewBody(m.Type)
	if err != nil {
		return err
	}

	if err := decode(body); err != nil {
		return fmt.Errorf("failed to decode module %s: %w", m.GetLabel(), err)
	}
	m.Body = body
	return nil
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

func ReadAndParseConfigEnvYamlFile(name string, version Version) (Envs, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return Envs{}, err
	}
	return ParseYamlConfigEnvData(data, true, version)
}

func ParseYamlConfigEnvData(data []byte, isYaml bool, version Version) (Envs, error) {
	var envs Envs
	var err error
	if isYaml {
		err = yaml.Unmarshal(data, &envs)
	} else {
		err = json.Unmarshal(data, &envs)
	}

	if err != nil {
		return envs, err
	}

	resolved, err := ResolveExprMap(envs.Env, ExprData{
		Env:     osutils.GetEnvMap(),
		Version: version,
	})
	if err != nil {
		return Envs{}, err
	}
	envs.Env = resolved

	return envs, nil
}

func ReadAndParseConfigYamlFile(name string) (Config, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return Config{}, err
	}
	return ParseYamlConfigData(data)
}

func ParseYamlConfigData(data []byte) (Config, error) {
	return parseConfigData(data, true)
}

func ParseJsonConfigData(data []byte) (Config, error) {
	return parseConfigData(data, false)
}

func parseConfigData(data []byte, isYaml bool) (Config, error) {
	var cfg Config
	var err error
	if isYaml {
		decoder := yaml.NewDecoder(
			bytes.NewReader(data),
			yaml.DisallowUnknownField(),
		)
		err = decoder.Decode(&cfg)
	} else {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&cfg)
	}

	if err != nil {
		return cfg, err
	}
	if err = cfg.checkModules(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func ReadAndParseModulesYaml(target string) (*[]Module, error) {
	if osutils.IsURL(target) {
		return ReadAndParseModulesYamlUrl(target)
	}
	return ReadAndParseModulesYamlFile(target)
}

func ReadAndParseModulesYamlFile(name string) (*[]Module, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}

	cfg, err := parseConfigData(data, true)
	if err != nil {
		return nil, err
	}

	return &cfg.Modules, nil
}

func ReadAndParseModulesYamlUrl(url string) (*[]Module, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	cfg, err := parseConfigData(data, true)
	if err != nil {
		return nil, err
	}

	return &cfg.Modules, nil
}

// ReadAndParseModules reads and parses modules from a file or URL.
func ReadAndParseModules(target string) (*[]Module, error) {
	if osutils.IsURL(target) {
		return ReadAndParseModulesYaml(target)
	}

	ext := filepath.Ext(target)
	isYaml := ext == ".yaml" || ext == ".yml"

	data, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}

	cfg, err := parseConfigData(data, isYaml)
	if err != nil {
		return nil, err
	}

	return &cfg.Modules, nil
}
