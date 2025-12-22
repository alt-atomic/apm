package core

import (
	"apm/internal/common/app"
	"apm/internal/common/build/common_types"
	"apm/internal/common/build/models"
	"apm/internal/common/osutils"
	"apm/internal/common/version"
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

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

const (
	TypeBranding = "branding"
	TypeCopy     = "copy"
	TypeGit      = "git"
	TypeInclude  = "include"
	TypeKernel   = "kernel"
	TypeLink     = "link"
	TypeMerge    = "merge"
	TypeMkdir    = "mkdir"
	TypeMove     = "move"
	TypeNetwork  = "network"
	TypePackages = "packages"
	TypeRemove   = "remove"
	TypeReplace  = "replace"
	TypeRepos    = "repos"
	TypeShell    = "shell"
	TypeSystemd  = "systemd"
)

var modelMap = map[string]func() models.Body{
	TypeBranding: func() models.Body { return &models.BrandingBody{} },
	TypeCopy:     func() models.Body { return &models.CopyBody{} },
	TypeGit:      func() models.Body { return &models.GitBody{} },
	TypeInclude:  func() models.Body { return &models.IncludeBody{} },
	TypeKernel:   func() models.Body { return &models.KernelBody{} },
	TypeLink:     func() models.Body { return &models.LinkBody{} },
	TypeMerge:    func() models.Body { return &models.MergeBody{} },
	TypeMkdir:    func() models.Body { return &models.MkdirBody{} },
	TypeMove:     func() models.Body { return &models.MoveBody{} },
	TypeNetwork:  func() models.Body { return &models.NetworkBody{} },
	TypePackages: func() models.Body { return &models.PackagesBody{} },
	TypeRemove:   func() models.Body { return &models.RemoveBody{} },
	TypeReplace:  func() models.Body { return &models.ReplaceBody{} },
	TypeRepos:    func() models.Body { return &models.ReposBody{} },
	TypeShell:    func() models.Body { return &models.ShellBody{} },
	TypeSystemd:  func() models.Body { return &models.SystemdBody{} },
}

var (
	imageApplyModuleName = "image-apply-results"
)

type Envs struct {
	Env map[string]string `yaml:"env" json:"env"`
}

type Config struct {
	// Environment vars
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	// Базовый образ для использования
	Image string `yaml:"image,omitempty" json:"image,omitempty"`

	// Список модулей
	Modules []Module `yaml:"modules,omitempty" json:"modules,omitempty"`
}

func (cfg *Config) getTotalInstall() []string {
	var totalInstall []string
	for _, module := range cfg.Modules {
		if module.Type == TypePackages {
			body, ok := module.Body.(*models.PackagesBody)
			if !ok {
				continue
			}
			totalInstall = append(totalInstall, body.Install...)
			for _, p := range body.Remove {
				totalInstall = removeByValue(totalInstall, p)
			}
		}
	}
	return totalInstall
}

func (cfg *Config) getTotalRemove() []string {
	var totalRemove []string
	for _, module := range cfg.Modules {
		if module.Type == TypePackages {
			body, ok := module.Body.(*models.PackagesBody)
			if !ok {
				continue
			}
			for _, p := range body.Install {
				totalRemove = removeByValue(totalRemove, p)
			}
			totalRemove = append(totalRemove, body.Remove...)
		}
	}
	return totalRemove
}

func (cfg *Config) getApplyPackagesModule() *Module {
	if len(cfg.Modules) == 0 || cfg.Modules[len(cfg.Modules)-1].Type != TypePackages || cfg.Modules[len(cfg.Modules)-1].Name != imageApplyModuleName {
		cfg.Modules = append(cfg.Modules, Module{
			Name: imageApplyModuleName,
			Type: TypePackages,
			Body: &models.PackagesBody{
				Install: []string{},
				Remove:  []string{},
			},
		})
	}

	return &cfg.Modules[len(cfg.Modules)-1]
}

func (cfg *Config) AddInstallPackage(pkg string) {
	totalInstall := cfg.getTotalInstall()
	if slices.Contains(totalInstall, pkg) {
		return
	}

	module := cfg.getApplyPackagesModule()
	body, ok := module.Body.(*models.PackagesBody)
	if !ok {
		body = &models.PackagesBody{}
		module.Body = body
	}

	if slices.Contains(body.Remove, pkg) {
		body.Remove = removeByValue(body.Remove, pkg)
	} else {
		body.Install = append(body.Install, pkg)
	}
}

func (cfg *Config) AddRemovePackage(pkg string) {
	totalRemove := cfg.getTotalRemove()
	if slices.Contains(totalRemove, pkg) {
		return
	}

	module := cfg.getApplyPackagesModule()
	body, ok := module.Body.(*models.PackagesBody)
	if !ok {
		body = &models.PackagesBody{}
		module.Body = body
	}

	if slices.Contains(body.Install, pkg) {
		body.Install = removeByValue(body.Install, pkg)
	} else {
		body.Remove = append(body.Remove, pkg)
	}
}

func (cfg *Config) IsInstalled(pkg string) bool {
	return slices.Contains(cfg.getTotalInstall(), pkg)
}

func (cfg *Config) IsRemoved(pkg string) bool {
	return slices.Contains(cfg.getTotalRemove(), pkg)
}

func (cfg *Config) CheckImage() error {
	if cfg.Image == "" {
		return errors.New(app.T_("Image can not be empty"))
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
	// Имя модуля для логирования
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Тип тела модуля
	Type string `yaml:"type" json:"type"`

	// Module ID
	Id string `yaml:"id,omitempty" json:"id,omitempty"`

	// Environmant vars for module
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	// Условие в формате языка expr
	If string `yaml:"if,omitempty" json:"if,omitempty"`

	// Тело модуля
	Body models.Body `yaml:"body" json:"body"`

	// Данные для вывода
	Output map[string]string `yaml:"output,omitempty" json:"output,omitempty"`
}

func (m Module) GetLabel() any {
	if m.Name != "" {
		return m.Name
	} else if m.Id != "" {
		return fmt.Sprintf("id=%s", m.Id)
	} else {
		return fmt.Sprintf("type=%s", m.Type)
	}
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

// decodeBody Общая логика выбора типа
func (m *Module) decodeBody(decode func(any) error) error {
	if m.Type == "" {
		return fmt.Errorf("module type is required")
	}

	factory, ok := modelMap[m.Type]
	if !ok {
		return fmt.Errorf("unknown module type: %s", m.Type)
	}

	body := factory()
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
				return fmt.Errorf(app.T_("Invalid id '%s'. Acceptable characters are letters, numbers, and underscores. The key must start with a letter."), module.Id)
			}
			if slices.Contains(moduleIds, module.Id) {
				return fmt.Errorf("found id collision")
			}
			moduleIds = append(moduleIds, module.Id)
		}
		if module.Body == nil {
			return fmt.Errorf("module %s has empty body", module.Type)
		}
		if err := models.CheckBody(module.Body); err != nil {
			return err
		}
	}

	return nil
}

func ReadAndParseConfigEnvYamlFile(name string, version version.Version) (Envs, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return Envs{}, err
	}
	return ParseYamlConfigEnvData(data, true, version)
}

func ParseYamlConfigEnvData(data []byte, isYaml bool, version version.Version) (Envs, error) {
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

	resolved, err := ResolveExprMap(envs.Env, common_types.ExprData{
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
	// НЕ резолвим плейсхолдеры здесь - они будут резолвиться при выполнении
	// Это позволяет сохранять конфиг с плейсхолдерами
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

// ReadAndParseModules чтение и парсинг из файла или URL
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

func removeByValue(arr []string, value string) []string {
	return slices.DeleteFunc(arr, func(s string) bool {
		return s == value
	})
}
