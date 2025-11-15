package core

import (
	"apm/internal/common/app"
	"apm/internal/common/build/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"

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

var modelMap = map[string]func() Body{
	TypeBranding: func() Body { return &models.BrandingBody{} },
	TypeCopy:     func() Body { return &models.CopyBody{} },
	TypeGit:      func() Body { return &models.GitBody{} },
	TypeInclude:  func() Body { return &models.IncludeBody{} },
	TypeKernel:   func() Body { return &models.KernelBody{} },
	TypeLink:     func() Body { return &models.LinkBody{} },
	TypeMerge:    func() Body { return &models.MergeBody{} },
	TypeMkdir:    func() Body { return &models.MkdirBody{} },
	TypeMove:     func() Body { return &models.MoveBody{} },
	TypePackages: func() Body { return &models.PackagesBody{} },
	TypeRemove:   func() Body { return &models.RemoveBody{} },
	TypeReplace:  func() Body { return &models.ReplaceBody{} },
	TypeRepos:    func() Body { return &models.ReposBody{} },
	TypeShell:    func() Body { return &models.ShellBody{} },
	TypeSystemd:  func() Body { return &models.SystemdBody{} },
}

const (
	EtcHostname = "/etc/hostname"
	EtcHosts    = "/etc/hosts"
)

var (
	imageApplyModuleName = "image-apply-results"
	goodBranches         = []string{"sisyphus"}
	goodBTypes           = []string{"stable", "nightly"}
	requiredText         = app.T_("module '%s' required '%s'")
	requiredTextOr       = fmt.Sprintf(requiredText, "%s", app.T_("%s' or '%s"))
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
			body := &models.PackagesBody(module.Body)
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
			body := &models.PackagesBody(module.Body)
			for _, p := range body.Install {
				totalRemove = removeByValue(totalRemove, p)
			}
			totalRemove = append(totalRemove, body.Remove...)
		}
	}
	return totalRemove
}

func (cfg *Config) getApplyPackagesModule() *Module {
	empty := Module{
		Name: imageApplyModuleName,
		Type: TypePackages,
		Body: models.PackagesBody{
			Install: []string{},
			Remove:  []string{},
		},
	}

	if len(cfg.Modules) == 0 {
		cfg.Modules = append(cfg.Modules, empty)
	} else if cfg.Modules[len(cfg.Modules)-1].Type != TypePackages || cfg.Modules[len(cfg.Modules)-1].Name != imageApplyModuleName {
		cfg.Modules = append(cfg.Modules, empty)
	}

	return &cfg.Modules[len(cfg.Modules)-1]
}

func (cfg *Config) AddInstallPackage(pkg string) {
	totalInstall := cfg.getTotalInstall()
	if slices.Contains(totalInstall, pkg) {
		return
	}

	packagesModule := &models.PackagesBody(cfg.getApplyPackagesModule().Body)
	if slices.Contains(packagesModule.Body.Remove, pkg) {
		packagesModule.Body.Remove = removeByValue(packagesModule.Body.Remove, pkg)
	} else {
		packagesModule.Body.Install = append(packagesModule.Body.Install, pkg)
	}
}

func (cfg *Config) AddRemovePackage(pkg string) {
	totalRemove := cfg.getTotalRemove()
	if slices.Contains(totalRemove, pkg) {
		return
	}

	packagesModule := &models.PackagesBody(cfg.getApplyPackagesModule().Body)
	if slices.Contains(packagesModule.Body.Install, pkg) {
		packagesModule.Body.Install = removeByValue(packagesModule.Body.Install, pkg)
	} else {
		packagesModule.Body.Remove = append(packagesModule.Body.Remove, pkg)
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
	Body Body `yaml:"body" json:"body"`
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

// Для JSON
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

// Общая логика выбора типа
func (m *Module) decodeBody(decode func(any) error) error {
	if m.Type == "" {
		return fmt.Errorf("module type is required")
	}

	keys := make([]string, 0, len(modelMap))
	for k := range modelMap {
		keys = append(keys, k)
	}
	if slices.Contains(keys, m.Type) {
		return fmt.Errorf("unknown module type: %s", m.Type)
	}

	var body = modelMap[m.Type]()
	if err := decode(&body); err != nil {
		return fmt.Errorf("failed to decode system body: %w", err)
	}
	m.Body = body
	return nil
}

type Body interface {
	Execute(context.Context, Service) error
	Check() error
}

func CheckModules(modules *[]Module) error {
	for _, module := range *modules {
		module.Body.Check()

		// switch module.Type {
		// case TypeGit:
		// 	if len(b.GetCommands()) == 0 {
		// 		return fmt.Errorf(requiredTextOr, TypeGit, "command", "commands")
		// 	}
		// 	if b.Target == "" {
		// 		return fmt.Errorf(requiredText, TypeGit, "target")
		// 	}
		// case TypeShell:
		// 	if len(b.GetCommands()) == 0 {
		// 		return fmt.Errorf(requiredTextOr, TypeShell, "command", "commands")
		// 	}
		// case TypeMerge:
		// 	if b.Target == "" {
		// 		return fmt.Errorf(requiredText, TypeMerge, "target")
		// 	}
		// 	if b.Destination == "" {
		// 		return fmt.Errorf(requiredText, TypeMerge, "destination")
		// 	}
		// case TypeCopy:
		// 	if b.Target == "" {
		// 		return fmt.Errorf(requiredText, TypeCopy, "target")
		// 	}
		// 	if b.Destination == "" {
		// 		return fmt.Errorf(requiredText, TypeCopy, "destination")
		// 	}
		// case TypeMove:
		// 	if b.Target == "" {
		// 		return fmt.Errorf(requiredText, TypeMove, "target")
		// 	}
		// 	if b.Destination == "" {
		// 		return fmt.Errorf(requiredText, TypeMove, "destination")
		// 	}
		// case TypeRemove:
		// 	if len(b.GetTargets()) == 0 {
		// 		return fmt.Errorf(requiredTextOr, TypeRemove, "target", "targets")
		// 	}
		// case TypeMkdir:
		// 	if len(b.GetTargets()) == 0 {
		// 		return fmt.Errorf(requiredTextOr, TypeMkdir, "target", "targets")
		// 	}
		// 	if b.Perm == "" {
		// 		return fmt.Errorf(requiredText, TypeMkdir, "perm")
		// 	}
		// case TypeSystemd:
		// 	if len(b.GetTargets()) == 0 {
		// 		return fmt.Errorf(requiredTextOr, TypeSystemd, "target", "targets")
		// 	}
		// 	if b.Enabled && b.Masked {
		// 		return fmt.Errorf("module %s can't have both 'enabled' and 'masked'", TypeSystemd)
		// 	}
		// case TypeLink:
		// 	if b.Target == "" {
		// 		return fmt.Errorf(requiredText, TypeLink, "target")
		// 	}
		// 	if b.Destination == "" {
		// 		return fmt.Errorf(requiredText, TypeLink, "destination")
		// 	}
		// case TypePackages:
		// 	if len(b.Install) == 0 && len(b.Remove) == 0 {
		// 		return fmt.Errorf(requiredTextOr, TypePackages, "install", "remove")
		// 	}
		// case TypeInclude:
		// 	if len(b.GetTargets()) == 0 {
		// 		return fmt.Errorf(requiredTextOr, TypeInclude, "target", "targets")
		// 	}
		// 	for _, target := range b.GetTargets() {
		// 		if _, err := ReadAndParseModulesYaml(target); err != nil {
		// 			return err
		// 		}
		// 	}
		// case TypeReplace:
		// 	if b.Target == "" {
		// 		return fmt.Errorf(requiredText, TypeReplace, "target")
		// 	}
		// 	if b.Pattern == "" {
		// 		return fmt.Errorf(requiredText, TypeReplace, "pattern")
		// 	}
		// 	if b.Repl == "" {
		// 		return fmt.Errorf(requiredText, TypeReplace, "repl")
		// 	}
		// default:
		// 	return errors.New(app.T_("Unknown type: " + module.Type))
		// }
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
	var cfg Config
	var err error
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
	if isURL(target) {
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

func isURL(str string) bool {
	u, err := url.Parse(str)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}

func removeByValue(arr []string, value string) []string {
	return slices.DeleteFunc(arr, func(s string) bool {
		return s == value
	})
}
