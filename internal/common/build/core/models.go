package core

import (
	"apm/internal/common/app"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"slices"

	"gopkg.in/yaml.v3"
)

const (
	TypeCopy     = "copy"
	TypeGit      = "git"
	TypeInclude  = "include"
	TypeLink     = "link"
	TypeMerge    = "merge"
	TypeReplace  = "replace"
	TypeMove     = "move"
	TypePackages = "packages"
	TypeRemove   = "remove"
	TypeShell    = "shell"
	TypeSystemd  = "systemd"
	TypeMkdir    = "mkdir"
)

const (
	EtcHostname            = "/etc/hostname"
	EtcHosts               = "/etc/hosts"
	EtcOsRelease           = "/etc/os-release"
	UsrLibOsRelease        = "/usr/lib/os-release"
	AptSourcesList         = "/etc/apt/sources.list"
	AptSourcesListD        = "/etc/apt/sources.list.d"
	PlymouthThemesDir      = "/usr/share/plymouth/themes"
	PlymouthConfigFile     = "/etc/plymouth/plymouthd.conf"
	PlymouthKargsPath      = "/usr/lib/bootc/kargs.d/00-plymouth.toml"
	PlymouthDracutConfPath = "/usr/lib/dracut/dracut.conf.d/00-plymouth.conf"
	KernelDir              = "/usr/lib/modules"
	BootVmlinuzTemplate    = "/boot/vmlinuz-%s"
)

var (
	imageApplyModuleName = "image-apply-results"
	goodBranches         = []string{"sisyphus"}
	goodBTypes           = []string{"stable", "nightly"}
	requiredText         = app.T_("module '%s' required '%s'")
	requiredTextOr       = fmt.Sprintf(requiredText, "%s", app.T_("%s' or '%s"))
)

type Branding struct {
	Name          string `yaml:"name,omitempty" json:"name,omitempty"`
	PlymouthTheme string `yaml:"plymouth-theme,omitempty" json:"plymouth-theme,omitempty"`
}

type Kernel struct {
	Flavour        string   `yaml:"flavour,omitempty" json:"flavour,omitempty"`
	Modules        []string `yaml:"modules,omitempty" json:"modules,omitempty"`
	IncludeHeaders bool     `yaml:"include-headers,omitempty" json:"include-headers,omitempty"`
}

type Repos struct {
	Clean  bool     `yaml:"clean,omitempty" json:"clean,omitempty"`
	Custom []string `yaml:"custom,omitempty" json:"custom,omitempty"`
	Branch string   `yaml:"branch,omitempty" json:"branch,omitempty"`
	Date   string   `yaml:"date,omitempty" json:"date,omitempty"`
	Tasks  []string `yaml:"tasks,omitempty" json:"tasks,omitempty"`
}

func (r *Repos) AllRepos() []string {
	var repos []string
	repos = append(repos, r.Custom...)
	repos = append(repos, r.TasksRepos()...)
	repos = append(repos, r.BranchRepos()...)
	return repos
}

func (r *Repos) TasksRepos() []string {
	var repos []string
	var templates []string

	switch runtime.GOARCH {
	case "amd64":
		templates = append(templates, "rpm http://git.altlinux.org repo/%s/x86_64 task")
	case "arm64", "aarch64":
		templates = append(templates, "rpm http://git.altlinux.org repo/%s/aarch64 task")
	default:
		return []string{}
	}

	for _, task := range r.Tasks {
		for _, template := range templates {
			repos = append(repos, fmt.Sprintf(template, task))
		}
	}

	return repos
}

func (r *Repos) BranchRepos() []string {
	if r.Branch == "" {
		return []string{}
	}

	date := "latest"
	if r.Date != "" {
		date = fmt.Sprintf("date/%s", r.Date)
	}

	var repos []string
	var templates []string

	switch runtime.GOARCH {
	case "amd64":
		templates = append(
			templates,
			"rpm [alt] https://ftp.altlinux.org/pub/distributions/archive %s/%s/x86_64 classic",
			"rpm [alt] https://ftp.altlinux.org/pub/distributions/archive %s/%s/x86_64-i586 classic",
			"rpm [alt] https://ftp.altlinux.org/pub/distributions/archive %s/%s/noarch classic",
		)
	case "arm64", "aarch64":
		templates = append(
			templates,
			"rpm [alt] https://ftp.altlinux.org/pub/distributions/archive %s/%s/aarch64 classic",
			"rpm [alt] https://ftp.altlinux.org/pub/distributions/archive %s/%s/noarch classic",
		)
	default:
		return []string{}
	}

	for _, template := range templates {
		repos = append(repos, fmt.Sprintf(template, r.Branch, date))
	}

	return repos
}

type Config struct {
	Image     string   `yaml:"image" json:"image"`
	BuildType string   `yaml:"build-type,omitempty" json:"build-type,omitempty"`
	Name      string   `yaml:"name" json:"name"`
	Env       []string `yaml:"env,omitempty" json:"env,omitempty"`
	Branding  Branding `yaml:"branding,omitempty" json:"branding,omitempty"`
	Hostname  string   `yaml:"hostname,omitempty" json:"hostname,omitempty"`
	Repos     Repos    `yaml:"repos,omitempty" json:"repos,omitempty"`
	Kernel    Kernel   `yaml:"kernel,omitempty" json:"kernel,omitempty"`
	Modules   []Module `yaml:"modules,omitempty" json:"modules,omitempty"`
}

func (cfg *Config) getTotalInstall() []string {
	var totalInstall []string
	for _, module := range cfg.Modules {
		if module.Type == TypePackages {
			totalInstall = append(totalInstall, module.Body.Install...)
			for _, p := range module.Body.Remove {
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
			for _, p := range module.Body.Install {
				totalRemove = removeByValue(totalRemove, p)
			}
			totalRemove = append(totalRemove, module.Body.Remove...)
		}
	}
	return totalRemove
}

func (cfg *Config) getApplyPackagesModule() *Module {
	empty := Module{
		Name: imageApplyModuleName,
		Type: TypePackages,
		Body: Body{
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

	packagesModule := cfg.getApplyPackagesModule()
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

	packagesModule := cfg.getApplyPackagesModule()
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

func (cfg *Config) fix() {
	if os.Getenv("APM_BUILD_IMAGE") != "" {
		cfg.Image = os.Getenv("APM_BUILD_IMAGE")
	}
	if os.Getenv("APM_BUILD_BUILD_TYPE") != "" {
		cfg.BuildType = os.Getenv("APM_BUILD_BUILD_TYPE")
	}
	if os.Getenv("APM_BUILD_NAME") != "" {
		cfg.Name = os.Getenv("APM_BUILD_NAME")
	}
	if os.Getenv("APM_BUILD_HOSTNAME") != "" {
		cfg.Hostname = os.Getenv("APM_BUILD_HOSTNAME")
	}
	if os.Getenv("APM_BUILD_KERNEL_FLAVOUR") != "" {
		cfg.Kernel.Flavour = os.Getenv("APM_BUILD_KERNEL_FLAVOUR")
	}
	if os.Getenv("APM_BUILD_REPO_BRANCH") != "" {
		cfg.Repos.Branch = os.Getenv("APM_BUILD_REPO_BRANCH")
	}
	if os.Getenv("APM_BUILD_REPO_DATE") != "" {
		cfg.Repos.Date = os.Getenv("APM_BUILD_REPO_DATE")
	}

	if cfg.BuildType == "" {
		cfg.BuildType = "stable"
	}
	if cfg.Name == "" {
		cfg.Name = "local"
	}
}

func (cfg *Config) checkRoot() error {
	if cfg.Image == "" {
		return errors.New(app.T_("Image can not be empty"))
	}
	if cfg.Repos.Date != "" && cfg.Repos.Branch == "" {
		return errors.New(app.T_("Repos branch can not be empty"))
	}
	if cfg.Repos.Branch != "" {
		if !slices.Contains(goodBranches, cfg.Repos.Branch) {
			return fmt.Errorf(app.T_("Branch %s not allowed"), cfg.Repos.Branch)
		}
	}
	if cfg.BuildType != "" {
		if !slices.Contains(goodBTypes, cfg.BuildType) {
			return fmt.Errorf(app.T_("Build type %s not allowed"), cfg.Repos.Branch)
		}
	}

	return nil
}

func (cfg *Config) checkModules() error {
	return CheckModules(&cfg.Modules)
}

func (cfg *Config) CheckAndFix() error {
	cfg.fix()
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
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	Type string `yaml:"type" json:"type"`
	If   string `yaml:"if,omitempty" json:"if,omitempty"`
	Body Body   `yaml:"body" json:"body"`
}

type Body struct {
	Command     string   `yaml:"command,omitempty" json:"command,omitempty"`
	Commands    []string `yaml:"commands,omitempty" json:"commands,omitempty"`
	Deps        []string `yaml:"deps,omitempty" json:"deps,omitempty"`
	Target      string   `yaml:"target,omitempty" json:"target,omitempty"`
	Targets     []string `yaml:"targets,omitempty" json:"targets,omitempty"`
	Destination string   `yaml:"destination,omitempty" json:"destination,omitempty"`
	Install     []string `yaml:"install,omitempty" json:"install,omitempty"`
	Remove      []string `yaml:"remove,omitempty" json:"remove,omitempty"`
	Replace     bool     `yaml:"replace,omitempty" json:"replace,omitempty"`
	CreateLink  bool     `yaml:"create-link,omitempty" json:"create-link,omitempty"`
	Enabled     bool     `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Global      bool     `yaml:"global,omitempty" json:"global,omitempty"`
	Masked      bool     `yaml:"masked,omitempty" json:"masked,omitempty"`
	Pattern     string   `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	Repl        string   `yaml:"repl,omitempty" json:"repl,omitempty"`
	Ref         string   `yaml:"ref,omitempty" json:"ref,omitempty"`
	Perm        string   `yaml:"perm,omitempty" json:"perm,omitempty"`
	Inside      bool     `yaml:"inside,omitempty" json:"inside,omitempty"`
}

func (b *Body) GetTargets() []string {
	var targets []string
	if b.Target != "" {
		targets = append(targets, b.Target)
	}
	if len(b.Targets) != 0 {
		targets = append(targets, b.Targets...)
	}
	return targets
}

func (b *Body) GetCommands() []string {
	var commands []string
	if b.Command != "" {
		commands = append(commands, b.Command)
	}
	if len(b.Commands) != 0 {
		commands = append(commands, b.Commands...)
	}
	return commands
}

func CheckModules(modules *[]Module) error {
	for _, module := range *modules {
		if module.Type == "" {
			return errors.New(app.T_("Module type can not be empty"))
		}

		b := module.Body

		switch module.Type {
		case TypeGit:
			if len(b.GetCommands()) == 0 {
				return fmt.Errorf(requiredTextOr, TypeGit, "command", "commands")
			}
			if b.Target == "" {
				return fmt.Errorf(requiredText, TypeGit, "target")
			}
		case TypeShell:
			if len(b.GetCommands()) == 0 {
				return fmt.Errorf(requiredTextOr, TypeShell, "command", "commands")
			}
		case TypeMerge:
			if b.Target == "" {
				return fmt.Errorf(requiredText, TypeMerge, "target")
			}
			if b.Destination == "" {
				return fmt.Errorf(requiredText, TypeMerge, "destination")
			}
		case TypeCopy:
			if b.Target == "" {
				return fmt.Errorf(requiredText, TypeCopy, "target")
			}
			if b.Destination == "" {
				return fmt.Errorf(requiredText, TypeCopy, "destination")
			}
		case TypeMove:
			if b.Target == "" {
				return fmt.Errorf(requiredText, TypeMove, "target")
			}
			if b.Destination == "" {
				return fmt.Errorf(requiredText, TypeMove, "destination")
			}
		case TypeRemove:
			if len(b.GetTargets()) == 0 {
				return fmt.Errorf(requiredTextOr, TypeRemove, "target", "targets")
			}
		case TypeMkdir:
			if len(b.GetTargets()) == 0 {
				return fmt.Errorf(requiredTextOr, TypeMkdir, "target", "targets")
			}
			if b.Perm == "" {
				return fmt.Errorf(requiredText, TypeMkdir, "perm")
			}
		case TypeSystemd:
			if len(b.GetTargets()) == 0 {
				return fmt.Errorf(requiredTextOr, TypeSystemd, "target", "targets")
			}
			if b.Enabled && b.Masked {
				return fmt.Errorf("module %s can't have both 'enabled' and 'masked'", TypeSystemd)
			}
		case TypeLink:
			if b.Target == "" {
				return fmt.Errorf(requiredText, TypeLink, "target")
			}
			if b.Destination == "" {
				return fmt.Errorf(requiredText, TypeLink, "destination")
			}
		case TypePackages:
			if len(b.Install) == 0 && len(b.Remove) == 0 {
				return fmt.Errorf(requiredTextOr, TypePackages, "install", "remove")
			}
		case TypeInclude:
			if len(b.GetTargets()) == 0 {
				return fmt.Errorf(requiredTextOr, TypeInclude, "target", "targets")
			}
			for _, target := range b.GetTargets() {
				if _, err := ReadAndParseModulesYaml(target); err != nil {
					return err
				}
			}
		case TypeReplace:
			if b.Target == "" {
				return fmt.Errorf(requiredText, TypeReplace, "target")
			}
			if b.Pattern == "" {
				return fmt.Errorf(requiredText, TypeReplace, "pattern")
			}
			if b.Repl == "" {
				return fmt.Errorf(requiredText, TypeReplace, "repl")
			}
		default:
			return errors.New(app.T_("Unknown type: " + module.Type))
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
	return parseConfigData(data, true)
}

func ParseJsonConfigData(data []byte) (Config, error) {
	return parseConfigData(data, false)
}

func parseConfigData(data []byte, isYaml bool) (Config, error) {
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
	if err = cfg.CheckAndFix(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func ReadAndParseModulesYamlData(data []byte) (*[]Module, error) {
	cfg := Config{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.checkModules(); err != nil {
		return nil, err
	}
	return &cfg.Modules, nil
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
	return ReadAndParseModulesYamlData(data)
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

	return ReadAndParseModulesYamlData(data)
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
