package core

import (
	"apm/internal/common/app"
	"apm/internal/common/build/models"
	"apm/internal/common/osutils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
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
	Env []string `yaml:"env" json:"env"`
}

type Config struct {
	// Базовый образ для использования
	// Может быть взята из переменной среды
	// APM_BUILD_IMAGE
	Image string `yaml:"image" json:"image"`
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

func (cfg *Config) checkRoot() error {
	if cfg.Image == "" {
		return errors.New(app.T_("Image can not be empty"))
	}

	return nil
}

func (cfg *Config) checkModules() error {
	return CheckModules(&cfg.Modules)
}

func (cfg *Config) Check() error {
	if err := cfg.checkRoot(); err != nil {
		return err
	}
	if err := cfg.checkModules(); err != nil {
		return err
	}
	return nil
}

func (cfg *Config) Save(filename string) error {
	if err := cfg.checkRoot(); err != nil {
		return err
	}
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

	// Условие в формате языка expr
	If string `yaml:"if,omitempty" json:"if,omitempty"`

	// Тело модуля
	Body models.Body `yaml:"body" json:"body"`
}

func (m *Module) UnmarshalYAML(value *yaml.Node) error {
	var aux struct {
		Type string    `yaml:"type"`
		Body yaml.Node `yaml:"body"`
	}

	if err := value.Decode(&aux); err != nil {
		return err
	}

	m.Type = aux.Type
	return m.decodeBody(func(target any) error {
		return aux.Body.Decode(target)
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
		return json.Unmarshal(aux.Body, target)
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
		return fmt.Errorf("failed to decode module body: %w", err)
	}
	m.Body = body
	return nil
}

func CheckModules(modules *[]Module) error {
	for _, module := range *modules {
		if module.Body == nil {
			return fmt.Errorf("module %s has empty body", module.Type)
		}
		if err := module.Body.Check(); err != nil {
			return err
		}
	}

	return nil
}

func ReadAndParseConfigYamlFile(name string) (Config, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return Config{}, err
	}
	return ParseYamlConfigData(data)
}

func ParseYamlConfigData(data []byte) (Config, error) {
	return parseConfigData(data, true, true)
}

func ParseJsonConfigData(data []byte) (Config, error) {
	return parseConfigData(data, false, true)
}

func parseConfigData(data []byte, isYaml bool, hasRoot bool) (Config, error) {
	data, err := resolvePlaceholders(data)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if isYaml {
		err = yaml.Unmarshal(data, &cfg)
	} else {
		err = json.Unmarshal(data, &cfg)
	}

	if err != nil {
		return cfg, err
	}
	if hasRoot {
		if err = cfg.checkRoot(); err != nil {
			return cfg, err
		}
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

	cfg, err := parseConfigData(data, true, false)
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

	cfg, err := parseConfigData(data, true, false)
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

var placeholderRegexp = regexp.MustCompile(`\$\{\{\s*([A-Za-z0-9_\-.]+)\s*}}`)

func resolvePlaceholders(data []byte) ([]byte, error) {
	var firstErr error

	result := placeholderRegexp.ReplaceAllFunc(data, func(match []byte) []byte {
		if firstErr != nil {
			return match
		}

		submatches := placeholderRegexp.FindSubmatch(match)
		if len(submatches) != 2 {
			return match
		}

		rawKey := string(submatches[1])
		envKey, ok := extractEnvKey(rawKey)
		if !ok {
			firstErr = fmt.Errorf("unsupported placeholder %q; expected format ${ { Env.VAR } }", rawKey)
			return match
		}

		value, found := os.LookupEnv(envKey)
		if !found {
			firstErr = fmt.Errorf("environment variable %q is not set", envKey)
			return match
		}

		return []byte(value)
	})

	if firstErr != nil {
		return nil, firstErr
	}

	return result, nil
}

func extractEnvKey(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	if !strings.HasPrefix(strings.ToLower(raw), "Env.") {
		return "", false
	}

	key := raw[4:]
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}

	return key, true
}
