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
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

const (
	TypeCopy     = "copy"
	TypeGit      = "git"
	TypeInclude  = "include"
	TypeLink     = "link"
	TypeMerge    = "merge"
	TypeMove     = "move"
	TypePackages = "packages"
	TypeRemove   = "remove"
	TypeShell    = "shell"
	TypeSystemd  = "systemd"
)

// NewConfigService — конструктор сервиса для сборки
func NewConfigService(appConfig *app.Config, aptActions *_package.Actions, hostConfig *HostConfigService) *ConfigService {
	return &ConfigService{
		appConfig:         appConfig,
		serviceAptActions: aptActions,
		serviceHostConfig: hostConfig,
	}
}

type ConfigService struct {
	appConfig         *app.Config
	serviceAptActions *_package.Actions
	serviceHostConfig *HostConfigService
}

func (cfgService *ConfigService) Build(ctx context.Context) error {
	if cfgService.serviceHostConfig.Config == nil {
		return errors.New(app.T_("Configuration not loaded. Load config first"))
	}

	var sourcesListD = "/etc/apt/sources.list.d"
	if cfgService.serviceHostConfig.Config.CleanRepos {
		app.Log.Info(fmt.Sprintf("Cleaining repos in %s", sourcesListD))
		err := filepath.Walk(sourcesListD, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if path != sourcesListD {
				err = os.RemoveAll(path)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			fmt.Println("Error:", err)
		}
	}

	var repos []string
	repos = append(repos, cfgService.serviceHostConfig.Config.Repos...)
	// Empty line divider
	if len(cfgService.serviceHostConfig.Config.TasksRepos()) != 0 {
		repos = append(repos, "")
		repos = append(repos, cfgService.serviceHostConfig.Config.TasksRepos()...)
	}

	if len(repos) != 0 {
		var sourcesPath = path.Join(
			sourcesListD,
			fmt.Sprintf("%s.list", strings.ReplaceAll(cfgService.serviceHostConfig.Config.Name, " ", "-")),
		)
		app.Log.Info(fmt.Sprintf("Setting repos to %s", sourcesPath))
		err := cfgService.serviceAptActions.Install(ctx, []string{"ca-certificates"})
		if err != nil {
			return err
		}

		err = os.WriteFile(
			sourcesPath,
			[]byte(strings.Join(repos, "\n")+"\n"), 0644,
		)
		if err != nil {
			return err
		}
	}

	app.Log.Info("Updating package cache")
	_, err := cfgService.serviceAptActions.Update(ctx)
	if err != nil {
		return err
	}

	// Upgrade packages
	app.Log.Info("Upgrading packages")
	err = cfgService.serviceAptActions.Upgrade(ctx)
	if err != nil {
		return err
	}

	// Kernel
	var kernel = cfgService.serviceHostConfig.Config.Kernel
	if kernel != "" {
		app.Log.Info(fmt.Sprintf("Installing kernel %s", kernel))
		// TODO: Add Kernel support
	}

	err = os.MkdirAll(cfgService.appConfig.ConfigManager.GetResourcesDir(), 0644)
	if err != nil {
		return err
	}

	err = os.Chdir(cfgService.appConfig.ConfigManager.GetResourcesDir())
	if err != nil {
		return err
	}

	for _, module := range cfgService.serviceHostConfig.Config.Modules {
		if err = cfgService.executeModule(ctx, module); err != nil {
			return err
		}
	}

	cleanup := func(paths ...string) error {
		for _, path := range paths {
			if err = os.RemoveAll(path); err != nil {
				return err
			}
		}
		return nil
	}

	return cleanup(
		cfgService.appConfig.ConfigManager.GetPathImageFile(),
		cfgService.appConfig.ConfigManager.GetResourcesDir(),
	)
}

type moduleHandler func(context.Context, *ConfigService, *Module) error

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
}

func (cfgService *ConfigService) executeModule(ctx context.Context, module Module) error {
	if module.Name != "" {
		app.Log.Info(fmt.Sprintf("-: %s", module.Name))
	}

	handler, ok := moduleHandlers[module.Type]
	if !ok {
		return fmt.Errorf(app.T_("Unknown module type: %s"), module.Type)
	}

	return handler(ctx, cfgService, &module)
}

func executeCopyModule(_ context.Context, _ *ConfigService, module *Module) error {
	b := &module.Body
	var withReplaceText string
	if b.Replace {
		withReplaceText = " with replacing"
	}
	app.Log.Info(fmt.Sprintf("Copying %s to %s%s", b.Target, b.Destination, withReplaceText))
	return osutils.Copy(b.Target, b.Destination, b.Replace)
}

func executeGitModule(ctx context.Context, cfgService *ConfigService, module *Module) error {
	b := &module.Body

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

	app.Log.Info(fmt.Sprintf("Cloning %s to %s", b.Url, tempDir))
	cmd := exec.CommandContext(ctx, "git", "clone", b.Url, tempDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	for _, cmdSh := range b.Commands {
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

func executeLinkModule(_ context.Context, _ *ConfigService, module *Module) error {
	b := &module.Body
	app.Log.Info(fmt.Sprintf("Linking %s to %s", b.Target, b.Destination))
	return os.Symlink(b.Target, b.Destination)
}

func executeMergeModule(_ context.Context, _ *ConfigService, module *Module) error {
	b := &module.Body
	app.Log.Info(fmt.Sprintf("Merging %s with %s", b.Target, b.Destination))
	return osutils.AppendFile(b.Target, b.Destination)
}

func executeMoveModule(_ context.Context, _ *ConfigService, module *Module) error {
	b := &module.Body
	var withText []string
	if b.CreateLink {
		withText = append(withText, "with linking")
	}
	if b.Replace {
		withText = append(withText, "with replacing")
	}
	app.Log.Info(fmt.Sprintf("Moving %s to %s%s", b.Target, b.Destination, " "+strings.Join(withText, " and ")))
	err := osutils.Move(b.Target, b.Destination, b.Replace)
	if err != nil {
		return err
	}

	if b.CreateLink {
		err = os.Symlink(b.Destination, b.Target)
		if err != nil {
			return err
		}
	}
	return nil
}

func executePackagesModule(ctx context.Context, cfgService *ConfigService, module *Module) error {
	b := &module.Body
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

func executeRemoveModule(_ context.Context, _ *ConfigService, module *Module) error {
	b := &module.Body
	app.Log.Info(fmt.Sprintf("Removing %s", strings.Join(b.GetTargets(), ", ")))
	for _, pathTarget := range b.GetTargets() {
		err := os.RemoveAll(pathTarget)
		if err != nil {
			return err
		}
	}
	return nil
}

func executeShellModule(ctx context.Context, cfgService *ConfigService, module *Module) error {
	b := &module.Body
	for _, cmdSh := range b.Commands {
		app.Log.Info(fmt.Sprintf("Executing `%s`", cmdSh))
		osutils.ExecSh(ctx, cmdSh, cfgService.appConfig.ConfigManager.GetResourcesDir(), true)
	}
	return nil
}

func executeSystemdModule(ctx context.Context, _ *ConfigService, module *Module) error {
	b := &module.Body
	for _, target := range b.GetTargets() {
		var text = fmt.Sprintf("Disabling %s", target)
		var action = "disable"
		if b.Enabled {
			text = fmt.Sprintf("Enabling %s", target)
			action = "enable"
		}
		app.Log.Info(text)
		cmd := exec.CommandContext(ctx, "systemctl", action, target)
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

	errInstall := cfgService.serviceAptActions.CombineInstallRemovePackages(ctx, packagesInstall, packagesRemove)
	if errInstall != nil {
		return errInstall
	}

	return nil
}
