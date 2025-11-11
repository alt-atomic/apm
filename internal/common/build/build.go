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
	_package "apm/internal/common/apt/package"
	"apm/internal/common/osutils"
	"apm/internal/common/reply"
	"apm/internal/common/version"
	"apm/internal/kernel/service"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/thediveo/osrelease"
)

const (
	etcHostname         = "/etc/hostname"
	etcHosts            = "/etc/hosts"
	etcOsRelease        = "/etc/os-release"
	usrLibOsRelease     = "/usr/lib/os-release"
	aptSourcesList      = "/etc/apt/sources.list"
	aptSourcesListD     = "/etc/apt/sources.list.d"
	plymouthThemesDir   = "/usr/share/plymouth/themes"
	plymouthConfigFile  = "/etc/plymouth/plymouthd.conf"
	kernelDir           = "/usr/lib/modules"
	bootVmlinuzTemplate = "/boot/vmlinuz-%s"

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

type ExprData struct {
	Config  *Config
	Env     map[string]string
	Version version.Version
}

// NewConfigService — конструктор сервиса для сборки
func NewConfigService(appConfig *app.Config, aptActions *_package.Actions, dBService *_package.PackageDBService, kernelManager *service.Manager, hostConfig *HostConfigService) *ConfigService {
	return &ConfigService{
		appConfig:         appConfig,
		serviceAptActions: aptActions,
		serviceDBService:  dBService,
		kernelManager:     kernelManager,
		serviceHostConfig: hostConfig,
	}
}

type ConfigService struct {
	appConfig         *app.Config
	serviceAptActions *_package.Actions
	serviceDBService  *_package.PackageDBService
	kernelManager     *service.Manager
	serviceHostConfig *HostConfigService
}

func (cfgService *ConfigService) Build(ctx context.Context) error {
	if cfgService.serviceHostConfig.Config == nil {
		return errors.New(app.T_("Configuration not loaded. Load config first"))
	}

	if len(cfgService.serviceHostConfig.Config.Env) != 0 {
		for _, e := range cfgService.serviceHostConfig.Config.Env {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("error in %s env", e)
			}
			err := os.Setenv(parts[0], parts[1])
			if err != nil {
				return err
			}
		}
	}

	if cfgService.serviceHostConfig.Config.Hostname != "" {
		err := os.WriteFile(
			etcHostname,
			fmt.Appendf(
				nil,
				"%s\n",
				cfgService.serviceHostConfig.Config.Hostname,
			),
			0644,
		)
		if err != nil {
			return err
		}
		err = os.WriteFile(
			etcHosts,
			fmt.Appendf(
				nil,
				"127.0.0.1  localhost.localdomain localhost %s\n::1  localhost6.localdomain localhost6 %s6\n",
				cfgService.serviceHostConfig.Config.Hostname,
				cfgService.serviceHostConfig.Config.Hostname,
			),
			0644,
		)
		if err != nil {
			return err
		}
	}

	if err := cfgService.executeRepos(ctx); err != nil {
		return err
	}

	app.Log.Info("Updating package cache")
	_, err := cfgService.serviceAptActions.Update(ctx)
	if err != nil {
		return err
	}

	app.Log.Info("Upgrading packages")
	err = cfgService.serviceAptActions.Upgrade(ctx)
	if err != nil {
		return err
	}

	if err = cfgService.executeBranding(ctx); err != nil {
		return err
	}

	if err = cfgService.executeKernel(ctx); err != nil {
		return err
	}

	for _, module := range cfgService.serviceHostConfig.Config.Modules {
		if err = cfgService.executeModule(ctx, module); err != nil {
			return err
		}
	}

	app.Log.Info("Final updating package cache")
	_, err = cfgService.serviceAptActions.Update(ctx)
	if err != nil {
		return err
	}

	app.Log.Info("Rebuild initramfs via dracut")
	err = rebuildInitramfs(ctx)
	if err != nil {
		return err
	}

	return nil
}

type moduleHandler func(context.Context, *ConfigService, *Body) error

var cpeNameRegex = regexp.MustCompile(`:\d+$`)

func (cfgService *ConfigService) executeBranding(ctx context.Context) error {
	var branding = cfgService.serviceHostConfig.Config.Branding

	if branding.Name != "" {
		filters := map[string]any{
			"name": fmt.Sprintf("branding-%s-", branding.Name),
		}
		packages, err := cfgService.serviceDBService.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
		if err != nil {
			return err
		}
		if len(packages) == 0 {
			return fmt.Errorf("no branding packages found for %s", branding.Name)
		}

		var pkgsNames = []string{}
		for _, pkg := range packages {
			pkgsNames = append(pkgsNames, pkg.Name)
		}
		err = executePackagesModule(ctx, cfgService, &Body{
			Install: pkgsNames,
		})
		if err != nil {
			return err
		}

		info, err := os.Stat(usrLibOsRelease)
		if err != nil {
			return err
		}
		vars := osrelease.NewFromName(usrLibOsRelease)

		now := time.Now()
		curVer := now.Format("20060102")
		prettyCurVer := now.Format("02.01.2006")

		prettyType := ""
		prettyNamePostfix := ""
		releaseType := ""

		bType := cfgService.serviceHostConfig.Config.BuildType

		switch bType {
		case "stable":
			prettyType = osutils.Capitalize(bType)
			releaseType = bType
		case "nightly":
			prettyType = osutils.Capitalize(bType)
			prettyNamePostfix = fmt.Sprintf(" %s", prettyType)
			releaseType = "development"
		}

		for name, value := range vars {
			switch name {
			case "VERSION":
				vars[name] = fmt.Sprintf("%s %s", prettyCurVer, prettyType)
			case "VERSION_ID":
				vars[name] = fmt.Sprintf("%s-%s", curVer, bType)
			case "RELEASE_TYPE":
				vars[name] = releaseType
			case "PRETTY_NAME":
				vars[name] = value + prettyNamePostfix
			case "CPE_NAME":
				vars[name] = cpeNameRegex.ReplaceAllString(value, fmt.Sprintf(":%s:%s", bType, curVer))
			}
		}

		vars["IMAGE_ID"] = vars["ID"]
		vars["IMAGE_VERSION"] = vars["VERSION_ID"]

		var newLines []string
		for name, value := range vars {
			newLines = append(newLines, fmt.Sprintf("%s=\"%s\"", name, value))
		}

		var newOsReleaseContent = strings.Join(newLines, "\n") + "\n"
		err = os.WriteFile(usrLibOsRelease, []byte(newOsReleaseContent), info.Mode().Perm())
		if err != nil {
			return err
		}

		err = executeLinkModule(ctx, cfgService, &Body{
			Target:      etcOsRelease,
			Destination: usrLibOsRelease,
			Replace:     true,
		})
		if err != nil {
			return err
		}
	}

	if branding.PlymouthTheme != "" {
		var themes []string
		if osutils.IsExists(plymouthThemesDir) {
			files, err := os.ReadDir(plymouthThemesDir)
			if err != nil {
				return err
			}

			for _, file := range files {
				themes = append(themes, file.Name())
			}
		}

		if !slices.Contains(themes, branding.PlymouthTheme) {
			filters := map[string]any{
				"name": fmt.Sprintf("plymouth-theme-%s", branding.Name),
			}
			packages, err := cfgService.serviceDBService.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
			if err != nil {
				return err
			}
			if len(packages) == 0 {
				return fmt.Errorf("no plymouth theme found for %s", branding.Name)
			}

			var pkgsNames []string
			for _, pkg := range packages {
				pkgsNames = append(pkgsNames, pkg.Name)
			}
			err = executePackagesModule(ctx, cfgService, &Body{
				Install: pkgsNames,
			})
			if err != nil {
				return err
			}
		}

		err := os.WriteFile(
			plymouthConfigFile,
			[]byte(strings.Join([]string{
				"[Daemon]",
				fmt.Sprintf("Theme=%s", branding.PlymouthTheme),
				"ShowDelay=0",
				"DeviceTimeout=10",
			}, "\n")+"\n"),
			0644,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cfgService *ConfigService) executeRepos(ctx context.Context) error {
	var repos = cfgService.serviceHostConfig.Config.Repos
	if repos.Clean {
		app.Log.Info(fmt.Sprintf("Cleaning repos in %s", aptSourcesListD))
		err := filepath.Walk(aptSourcesListD, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if path != aptSourcesListD {
				err = os.RemoveAll(path)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

		err = os.WriteFile(aptSourcesList, []byte(""), 0644)
		if err != nil {
			return err
		}
	}

	var allRepos = repos.AllRepos()

	if len(allRepos) != 0 {
		var sourcesPath = path.Join(
			aptSourcesListD,
			fmt.Sprintf("%s.list", strings.ReplaceAll(cfgService.serviceHostConfig.Config.Name, " ", "-")),
		)
		app.Log.Info(fmt.Sprintf("Setting repos to %s", sourcesPath))
		err := cfgService.serviceAptActions.Install(ctx, []string{"ca-certificates"})
		if err != nil {
			return err
		}

		err = os.WriteFile(
			sourcesPath,
			[]byte(strings.Join(allRepos, "\n")+"\n"), 0644,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cfgService *ConfigService) executeKernel(ctx context.Context) error {
	var kernel = cfgService.serviceHostConfig.Config.Kernel
	var kModules = kernel.Modules
	if kernel.Flavour != "" {
		latest, err := cfgService.kernelManager.FindLatestKernel(ctx, kernel.Flavour)
		if err != nil {
			return err
		}

		var currentKernel *service.Info

		if len(kModules) == 0 {
			currentKernel, _ = cfgService.getCurrentKernel(ctx)
			if currentKernel != nil {
				inheritedModules, _ := cfgService.kernelManager.InheritModulesFromKernel(latest, currentKernel)
				if len(inheritedModules) > 0 {
					kModules = inheritedModules
				}
			}
		}
		additionalPackages, _ := cfgService.kernelManager.AutoSelectHeadersAndFirmware(ctx, latest, kernel.IncludeHeaders)

		for _, pkg := range additionalPackages {
			// Если это модуль ядра - извлекаем имя модуля
			if strings.HasPrefix(pkg, "kernel-modules-") && strings.HasSuffix(pkg, fmt.Sprintf("-%s", latest.Flavour)) {
				moduleName := strings.TrimPrefix(pkg, "kernel-modules-")
				moduleName = strings.TrimSuffix(moduleName, fmt.Sprintf("-%s", latest.Flavour))
				// Добавляем только если его еще нет в списке
				moduleExists := slices.Contains(kModules, moduleName)
				if !moduleExists {
					kModules = append(kModules, moduleName)
				}
			}
		}

		if currentKernel != nil {
			app.Log.Info(fmt.Sprintf("Removing current kernel %s", currentKernel.Flavour))
			err = cfgService.kernelManager.RemoveKernel(currentKernel, true)
			if err != nil {
				return err
			}

			entries, err := os.ReadDir(kernelDir)
			if err != nil {
				return err
			}

			for _, entry := range entries {
				entryPath := filepath.Join(kernelDir, entry.Name())
				err = os.RemoveAll(entryPath)
				if err != nil {
					return err
				}
			}
		}

		app.Log.Info(fmt.Sprintf("Installing kernel %s", latest.Flavour))
		err = cfgService.kernelManager.InstallKernel(ctx, latest, kModules, kernel.IncludeHeaders, false)
		if err != nil {
			return err
		}

		app.Log.Info("Updating packages DB")
		_, err = cfgService.serviceAptActions.Update(ctx)
		if err != nil {
			return err
		}

		latestInstalledKernelVersion, err := getLatestInstalledKernelVersion()
		if err != nil {
			return err
		}

		app.Log.Info("Copy vmlinuz")
		err = osutils.Copy(
			fmt.Sprintf(bootVmlinuzTemplate, latestInstalledKernelVersion),
			fmt.Sprintf("%s/%s/vmlinuz", kernelDir, latestInstalledKernelVersion),
			true,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cfgService *ConfigService) executeModule(ctx context.Context, module Module) error {
	if module.Name != "" {
		app.Log.Info(fmt.Sprintf("-: %s", module.Name))
	}

	var moduleHandlers = map[string]moduleHandler{
		TypeCopy:     executeCopyModule,
		TypeGit:      executeGitModule,
		TypeLink:     executeLinkModule,
		TypeMerge:    executeMergeModule,
		TypeMove:     executeMoveModule,
		TypePackages: executePackagesModule,
		TypeRemove:   executeRemoveModule,
		TypeShell:    executeShellModule,
		TypeSystemd:  executeSystemdModule,
		TypeMkdir:    executeMkdirModule,
		TypeReplace:  executeReplaceModule,
		TypeInclude:  executeIncludeModule,
	}

	handler, ok := moduleHandlers[module.Type]
	if !ok {
		return fmt.Errorf(app.T_("Unknown module type: %s"), module.Type)
	}

	if module.If != "" {
		data := ExprData{
			Config:  cfgService.serviceHostConfig.Config,
			Env:     osutils.GetEnvMap(),
			Version: version.ParseVersion(cfgService.appConfig.ConfigManager.GetConfig().Version),
		}

		env := expr.Env(data)

		program, err := expr.Compile(module.If, env)
		if err != nil {
			return err
		}

		output, err := expr.Run(program, data)
		if err != nil {
			return err
		}
		if output.(bool) {
			return handler(ctx, cfgService, &module.Body)
		}
	} else {
		return handler(ctx, cfgService, &module.Body)
	}

	return nil
}

// ExecuteModule - публичная обертка для тестирования модулей
func (cfgService *ConfigService) ExecuteModule(ctx context.Context, module Module) error {
	return cfgService.executeModule(ctx, module)
}

func executeIncludeModule(ctx context.Context, cfgService *ConfigService, b *Body) error {
	for _, target := range b.GetTargets() {
		modules, err := ReadAndParseModulesYaml(target)
		if err != nil {
			return err
		}

		for _, module := range *modules {
			err = cfgService.executeModule(ctx, module)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func executeCopyModule(_ context.Context, _ *ConfigService, b *Body) error {
	var withReplaceText string
	if b.Replace {
		withReplaceText = " with replacing"
	}
	app.Log.Info(fmt.Sprintf("Copying %s to %s%s", b.Target, b.Destination, withReplaceText))

	if !filepath.IsAbs(b.Destination) {
		return fmt.Errorf("destination in move type must be absolute path")
	}

	return osutils.Copy(b.Target, b.Destination, b.Replace)
}

func executeReplaceModule(_ context.Context, _ *ConfigService, b *Body) error {
	app.Log.Info(fmt.Sprintf("Replacing %s to %s in %s", b.Pattern, b.Repl, b.Target))

	if !filepath.IsAbs(b.Target) {
		return fmt.Errorf("target in replace type must be absolute path")
	}

	var info, err = os.Stat(b.Target)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(b.Target)
	if err != nil {
		return err
	}
	re, err := regexp.Compile(b.Pattern)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		lines[i] = re.ReplaceAllString(line, b.Repl)
	}

	return os.WriteFile(b.Target, []byte(strings.Join(lines, "\n")), info.Mode().Perm())
}

func executeGitModule(ctx context.Context, cfgService *ConfigService, b *Body) error {
	if len(b.Deps) != 0 {
		var doInstall []string
		for _, p := range b.Deps {
			doInstall = append(doInstall, p+"+")
		}
		app.Log.Info(fmt.Sprintf("Installing %s", strings.Join(b.Deps, ", ")))
		errInstall := cfgService.CombineInstallRemovePackages(ctx, doInstall, false, false)
		if errInstall != nil {
			return errInstall
		}
	}

	tempDir, errDir := os.MkdirTemp(os.TempDir(), "git-*")
	if errDir != nil {
		return errDir
	}
	defer os.RemoveAll(tempDir)

	app.Log.Info(fmt.Sprintf("Cloning %s to %s", b.Target, tempDir))

	args := []string{}
	args = append(args, "clone")
	if b.Ref != "" {
		args = append(args, "-b", b.Ref)
	}
	args = append(args, b.Target, tempDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	for _, cmdSh := range b.GetCommands() {
		app.Log.Info(fmt.Sprintf("Executing `%s`", cmdSh))
		errExec := osutils.ExecSh(ctx, cmdSh, tempDir, true)
		if errExec != nil {
			return errExec
		}
	}

	if len(b.Deps) != 0 {
		var doRemove []string
		for _, p := range b.Deps {
			doRemove = append(doRemove, p+"-")
		}
		app.Log.Info(fmt.Sprintf("Removing %s", strings.Join(b.Deps, ", ")))
		err := cfgService.CombineInstallRemovePackages(ctx, doRemove, true, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func executeLinkModule(_ context.Context, _ *ConfigService, b *Body) error {
	if !filepath.IsAbs(b.Target) {
		return fmt.Errorf("target in link type must be absolute path")
	}

	app.Log.Info(fmt.Sprintf("Linking %s to %s", b.Target, b.Destination))
	if b.Replace {
		err := os.RemoveAll(b.Target)
		if err != nil {
			return err
		}
	}

	if filepath.IsAbs(b.Destination) {
		relativePath, err := filepath.Rel(path.Dir(b.Target), b.Destination)
		if err != nil {
			relativePath = b.Destination
		}

		return os.Symlink(relativePath, b.Target)
	} else {
		return os.Symlink(b.Destination, b.Target)
	}
}

func executeMergeModule(_ context.Context, _ *ConfigService, b *Body) error {
	app.Log.Info(fmt.Sprintf("Merging %s with %s", b.Target, b.Destination))

	if !filepath.IsAbs(b.Target) {
		return fmt.Errorf("target in merge type must be absolute path")
	}

	mode, err := osutils.StringToFileMode(b.Perm)
	if err != nil {
		return err
	}
	return osutils.AppendFile(b.Target, b.Destination, mode)
}

func executeMoveModule(ctx context.Context, cfgService *ConfigService, b *Body) error {
	var withText []string
	if b.CreateLink {
		withText = append(withText, "with linking")
	}
	if b.Replace {
		withText = append(withText, "with replacing")
	}
	app.Log.Info(fmt.Sprintf("Moving %s to %s%s", b.Target, b.Destination, " "+strings.Join(withText, " and ")))

	if !filepath.IsAbs(b.Target) {
		return fmt.Errorf("target in move type must be absolute path")
	}
	if !filepath.IsAbs(b.Destination) {
		return fmt.Errorf("destination in move type must be absolute path")
	}

	err := osutils.Move(b.Target, b.Destination, b.Replace)
	if err != nil {
		return err
	}

	if b.CreateLink {
		err = executeLinkModule(ctx, cfgService, &Body{
			Target:      b.Target,
			Destination: b.Destination,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func executePackagesModule(ctx context.Context, cfgService *ConfigService, b *Body) error {
	var text []string
	if len(b.Install) != 0 {
		text = append(text, fmt.Sprintf("installing %s", strings.Join(b.Install, ", ")))
	}
	if len(b.Remove) != 0 {
		text = append(text, fmt.Sprintf("removing %s", strings.Join(b.Remove, ", ")))
	}
	app.Log.Info(osutils.Capitalize(strings.Join(text, " and ")))
	var do []string
	for _, p := range b.Install {
		do = append(do, p+"+")
	}
	for _, p := range b.Remove {
		do = append(do, p+"-")
	}
	return cfgService.CombineInstallRemovePackages(ctx, do, false, false)
}

func executeRemoveModule(_ context.Context, _ *ConfigService, b *Body) error {
	app.Log.Info(fmt.Sprintf("Removing %s", strings.Join(b.GetTargets(), ", ")))

	for _, pathTarget := range b.GetTargets() {
		if !filepath.IsAbs(pathTarget) {
			return fmt.Errorf("target in remove type must be absolute path")
		}

		if b.Inside {
			err := osutils.Clean(pathTarget)
			if err != nil {
				return err
			}
		} else {
			err := os.RemoveAll(pathTarget)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func executeMkdirModule(_ context.Context, _ *ConfigService, b *Body) error {
	app.Log.Info(fmt.Sprintf("Creating dirs at %s", strings.Join(b.GetTargets(), ", ")))
	for _, pathTarget := range b.GetTargets() {
		if !filepath.IsAbs(pathTarget) {
			return fmt.Errorf("target in mkdir type must be absolute path")
		}

		mode, err := osutils.StringToFileMode(b.Perm)
		if err != nil {
			return err
		}
		err = os.MkdirAll(pathTarget, mode)
		if err != nil {
			return err
		}
	}
	return nil
}

func executeShellModule(ctx context.Context, cfgService *ConfigService, b *Body) error {
	for _, cmdSh := range b.GetCommands() {
		app.Log.Info(fmt.Sprintf("Executing `%s`", cmdSh))
		err := osutils.ExecSh(ctx, cmdSh, cfgService.appConfig.ConfigManager.GetResourcesDir(), true)
		if err != nil {
			return err
		}
	}
	return nil
}

func executeSystemdModule(ctx context.Context, _ *ConfigService, b *Body) error {
	for _, target := range b.GetTargets() {
		var text = fmt.Sprintf("Disabling %s", target)
		var action = "disable"
		if b.Enabled {
			text = fmt.Sprintf("Enabling %s", target)
			action = "enable"
		}
		app.Log.Info(text)

		var args []string
		if b.Global {
			args = append(args, "--global")
		}
		args = append(args, action, target)

		cmd := exec.CommandContext(ctx, "systemctl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (cfgService *ConfigService) CombineInstallRemovePackages(ctx context.Context, packages []string, purge bool, depends bool) error {
	packagesInstall, packagesRemove, errPrepare := cfgService.serviceAptActions.PrepareInstallPackages(ctx, packages)
	if errPrepare != nil {
		return errPrepare
	}

	packagesInstall, packagesRemove, _, _, errFind := cfgService.serviceAptActions.FindPackage(
		ctx,
		packagesInstall,
		packagesRemove,
		false,
		false,
	)
	if errFind != nil {
		return errFind
	}

	errInstall := cfgService.serviceAptActions.CombineInstallRemovePackages(
		ctx,
		packagesInstall,
		packagesRemove,
		purge,
		depends,
	)
	if errInstall != nil {
		return errInstall
	}

	return nil
}

func rebuildInitramfs(ctx context.Context) error {
	var kernelVersion, err = getLatestInstalledKernelVersion()
	if err != nil {
		return err
	}

	err = osutils.ExecSh(ctx, fmt.Sprintf(
		"depmod -a -v '%s'",
		kernelVersion,
	), "", true)
	if err != nil {
		return err
	}

	err = osutils.ExecSh(ctx, fmt.Sprintf(
		"dracut --force '%s/%s/initramfs.img' %s",
		kernelDir,
		kernelVersion,
		kernelVersion,
	), "", true)
	if err != nil {
		return err
	}

	return nil
}

func (cfgService *ConfigService) getCurrentKernel(ctx context.Context) (*service.Info, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("kernel.CurrentKernel"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("kernel.CurrentKernel"))

	filters := map[string]interface{}{
		"typePackage": int(_package.PackageTypeSystem),
		"name":        "kernel-image-",
		"installed":   true,
	}
	packages, err := cfgService.serviceDBService.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
	if len(packages) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	kernel := cfgService.kernelManager.ParseKernelPackageFromDB(packages[0])
	if kernel == nil {
		return nil, errors.New(app.T_("failed to parse kernel package from database"))
	}

	kernel.IsRunning = true
	kernel.IsInstalled = true

	return kernel, nil
}

func getLatestInstalledKernelVersion() (string, error) {
	files, err := os.ReadDir(kernelDir)
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no kernel versions found in %s", kernelDir)
	} else if len(files) > 1 {
		return "", fmt.Errorf("too many kernel versions found in %s", kernelDir)
	}

	return files[0].Name(), nil
}
