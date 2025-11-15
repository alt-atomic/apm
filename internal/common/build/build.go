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
	"apm/internal/common/build/core"
	"apm/internal/common/osutils"
	"apm/internal/common/version"
	"apm/internal/kernel/service"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/expr-lang/expr"
)

type ExprData struct {
	Config  *Config
	Env     map[string]string
	Version version.Version
}

type ConfigService struct {
	appConfig         *app.Config
	serviceAptActions *_package.Actions
	serviceDBService  *_package.PackageDBService
	kernelManager     *service.Manager
	serviceHostConfig *HostConfigService
}

func NewConfigService(appConfig *app.Config, aptActions *_package.Actions, dBService *_package.PackageDBService, kernelManager *service.Manager, hostConfig *HostConfigService) *ConfigService {
	return &ConfigService{
		appConfig:         appConfig,
		serviceAptActions: aptActions,
		serviceDBService:  dBService,
		kernelManager:     kernelManager,
		serviceHostConfig: hostConfig,
	}
}

func (cfgService *ConfigService) Build(ctx context.Context) error {
	if cfgService.serviceHostConfig.Config == nil {
		return errors.New(app.T_("Configuration not loaded. Load config first"))
	}

	// TODO: Перенести в работу с Env
	// if len(cfgService.serviceHostConfig.Config.Env) != 0 {
	// 	for _, e := range cfgService.serviceHostConfig.Config.Env {
	// 		parts := strings.SplitN(e, "=", 2)
	// 		if len(parts) != 2 {
	// 			return fmt.Errorf("error in %s env", e)
	// 		}
	// 		if err := os.Setenv(parts[0], parts[1]); err != nil {
	// 			return err
	// 		}
	// 	}
	// }

	// TODO: Исправить hostname
	// if cfgService.serviceHostConfig.Config.Hostname != "" {
	// 	if err := os.WriteFile(core.EtcHostname, []byte(fmt.Sprintf("%s\n", cfgService.serviceHostConfig.Config.Hostname)), 0644); err != nil {
	// 		return err
	// 	}
	// 	hosts := fmt.Sprintf(
	// 		"127.0.0.1  localhost.localdomain localhost %s\n::1  localhost6.localdomain localhost6 %s6\n",
	// 		cfgService.serviceHostConfig.Config.Hostname,
	// 		cfgService.serviceHostConfig.Config.Hostname,
	// 	)
	// 	if err := os.WriteFile(core.EtcHosts, []byte(hosts), 0644); err != nil {
	// 		return err
	// 	}
	// }

	for _, module := range cfgService.serviceHostConfig.Config.Modules {
		if err := cfgService.ExecuteModule(ctx, module); err != nil {
			return err
		}
	}

	return nil
}

type moduleHandler func(context.Context, core.Service, *core.Body) error

func (cfgService *ConfigService) ExecuteModule(ctx context.Context, module core.Module) error {
	if module.Name != "" {
		app.Log.Info(fmt.Sprintf("-: %s", module.Name))
	}

	shouldExecute := true
	if module.If != "" {
		data := ExprData{
			Config:  cfgService.serviceHostConfig.Config,
			Env:     osutils.GetEnvMap(),
			Version: version.ParseVersion(cfgService.appConfig.ConfigManager.GetConfig().Version),
		}

		program, err := expr.Compile(module.If, expr.Env(data))
		if err != nil {
			return err
		}

		output, err := expr.Run(program, data)
		if err != nil {
			return err
		}
		boolResult, ok := output.(bool)
		if !ok {
			return fmt.Errorf("module expression must return bool")
		}
		shouldExecute = boolResult
	}

	if !shouldExecute {
		return nil
	}

	if err := module.Body.Execute(ctx, cfgService); err != nil {
		label := module.Name
		if label == "" {
			label = fmt.Sprintf("type=%s", module.Type)
		}
		return fmt.Errorf("module %s: %w", label, err)
	}

	return nil
}

func (cfgService *ConfigService) Config() *core.Config {
	return cfgService.serviceHostConfig.Config
}

func (cfgService *ConfigService) QueryHostImagePackages(ctx context.Context, filters map[string]any, sortField, sortOrder string, limit, offset int) ([]_package.Package, error) {
	return cfgService.serviceDBService.QueryHostImagePackages(ctx, filters, sortField, sortOrder, limit, offset)
}

func (cfgService *ConfigService) CombineInstallRemovePackages(ctx context.Context, packages []string, purge bool, depends bool) error {
	packagesInstall, packagesRemove, errPrepare := cfgService.serviceAptActions.PrepareInstallPackages(ctx, packages)
	if errPrepare != nil {
		return errPrepare
	}

	packagesInstall, packagesRemove, _, aptPackageChanges, errFind := cfgService.serviceAptActions.FindPackage(
		ctx,
		packagesInstall,
		packagesRemove,
		false,
		false,
	)
	if errFind != nil {
		return errFind
	}

	if aptPackageChanges != nil {
		if len(aptPackageChanges.NewInstalledPackages) > 0 {
			app.Log.Info(fmt.Sprintf("Install plan: %s", strings.Join(aptPackageChanges.NewInstalledPackages, ", ")))
		}

		if len(aptPackageChanges.RemovedPackages) > 0 {
			app.Log.Info(fmt.Sprintf("Remove plan: %s", strings.Join(aptPackageChanges.RemovedPackages, ", ")))
		}
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

func (cfgService *ConfigService) InstallPackages(ctx context.Context, packages []string) error {
	return cfgService.serviceAptActions.Install(ctx, packages)
}

func (cfgService *ConfigService) UpdatePackages(ctx context.Context) error {
	_, err := cfgService.serviceAptActions.Update(ctx)
	return err
}

func (cfgService *ConfigService) UpgradePackages(ctx context.Context) error {
	err := cfgService.serviceAptActions.Upgrade(ctx)
	return err
}

func (cfgService *ConfigService) KernelManager() *service.Manager {
	return cfgService.kernelManager
}

func (cfgService *ConfigService) ResourcesDir() string {
	return cfgService.appConfig.ConfigManager.GetResourcesDir()
}
