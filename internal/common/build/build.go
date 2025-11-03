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

	var repos []string
	repos = append(repos, cfgService.serviceHostConfig.Config.Repos...)
	repos = append(repos, cfgService.serviceHostConfig.Config.TasksRepos()...)
	if len(repos) != 0 {
		app.Log.Info("Setting repo")
		err := cfgService.serviceAptActions.Install(ctx, []string{"ca-certificates"})
		if err != nil {
			return err
		}
		err = os.RemoveAll("/etc/apt/sources.list.d")
		if err != nil {
			return err
		}
		err = os.WriteFile("/etc/apt/sources.list", []byte(strings.Join(repos, "\n")+"\n"), 0644)
		if err != nil {
			return err
		}

		app.Log.Info("Updating package cache")
		_, err = cfgService.serviceAptActions.Update(ctx)
		if err != nil {
			return err
		}
	}

	// Upgrade packages
	app.Log.Info("Upgrading packages")
	err := cfgService.serviceAptActions.Upgrade(ctx)
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
		if module.Name != "" {
			app.Log.Info(fmt.Sprintf("-: %s", module.Name))
		}
		var b = &module.Body
		switch module.Type {
		case TypeCopy:
			var withReplaceText string
			if b.Replace {
				withReplaceText = " with replacing"
			}
			app.Log.Info(fmt.Sprintf("Copying %s to %s%s", b.Target, b.Destination, withReplaceText))
			err = osutils.Copy(b.Target, b.Destination, b.Replace)
			if err != nil {
				return err
			}
		case TypeGit:
			if len(b.Deps) != 0 {
				errInstall := cfgService.CombineInstallRemovePackages(ctx, b.Deps, false, false)
				if errInstall != nil {
					return errInstall
				}
			}

			tempDir, errDir := os.MkdirTemp(os.TempDir(), "git-*")
			if errDir != nil {
				return errDir
			}

			cmd := exec.Command("git", "clone", b.Url, tempDir)
			_, err = cmd.Output()
			if err != nil {
				return err
			}

			for _, cmdSh := range b.Commands {
				app.Log.Info(fmt.Sprintf("Executing `%s`", cmdSh))
				errExec := osutils.ExecSh(cmdSh, tempDir)
				if errExec != nil {
					return errExec
				}
			}

			if len(b.Deps) != 0 {
				err = cfgService.CombineInstallRemovePackages(ctx, b.Deps, true, true)
				if err != nil {
					return err
				}
			}
		case TypeLink:
			app.Log.Info(fmt.Sprintf("Linking %s to %s", b.Target, b.Destination))
			err = os.Symlink(b.Target, b.Destination)
			if err != nil {
				return err
			}
		case TypeMerge:
			app.Log.Info(fmt.Sprintf("Merging %s with %s", b.Target, b.Destination))
			err = osutils.AppendFile(b.Target, b.Destination)
			if err != nil {
				return err
			}
		case TypeMove:
			var withText []string
			if b.CreateLink {
				withText = append(withText, "with linking")
			}
			if b.Replace {
				withText = append(withText, "with replacing")
			}
			app.Log.Info(fmt.Sprintf("Moving %s to %s%s", b.Target, b.Destination, " "+strings.Join(withText, " and ")))
			err = osutils.Move(b.Target, b.Destination, b.Replace)
			if err != nil {
				return err
			}

			if b.CreateLink {
				err = os.Symlink(b.Destination, b.Target)
				if err != nil {
					return err
				}
			}
		case TypePackages:
			var text []string
			if len(b.Install) != 0 {
				text = append(text, fmt.Sprintf("installing %s", strings.Join(b.Install, ", ")))
			}
			if b.Replace {
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
			var err = cfgService.CombineInstallRemovePackages(ctx, do, false, false)
			if err != nil {
				return err
			}
		case TypeRemove:
			app.Log.Info(fmt.Sprintf("Removing %s", strings.Join(b.GetTargets(), ", ")))
			for _, pathTarget := range b.GetTargets() {
				err = os.RemoveAll(pathTarget)
				if err != nil {
					return err
				}
			}
		case TypeShell:
			for _, cmdsh := range b.Commands {
				app.Log.Info(fmt.Sprintf("Executing `%s`", cmdsh))
				osutils.ExecSh(cmdsh, cfgService.appConfig.ConfigManager.GetResourcesDir())
			}
		case TypeSystemd:
			for _, target := range b.GetTargets() {
				var text = fmt.Sprintf("Disabling %s", target)
				var action = "disable"
				if b.Enabled {
					text = fmt.Sprintf("Enabling %s", target)
					action = "enable"
				}
				app.Log.Info(text)
				cmd := exec.Command("systemctl", action, target)
				_, err = cmd.Output()
				if err != nil {
					return err
				}
			}
		}
	}

	err = os.RemoveAll(cfgService.appConfig.ConfigManager.GetPathImageFile())
	if err != nil {
		return err
	}
	err = os.RemoveAll(cfgService.appConfig.ConfigManager.GetResourcesDir())
	if err != nil {
		return err
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
