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
	"os"
	"path/filepath"
	"strings"
)

type ConfigService struct {
	appConfig         *app.Config
	modulesMap        map[string]core.MapModule
	serviceAptActions *_package.Actions
	serviceDBService  *_package.PackageDBService
	kernelManager     *service.Manager
	serviceHostConfig *HostConfigService
}

func NewConfigService(appConfig *app.Config, aptActions *_package.Actions, dBService *_package.PackageDBService, kernelManager *service.Manager, hostConfig *HostConfigService) *ConfigService {
	return &ConfigService{
		appConfig:         appConfig,
		modulesMap:        map[string]core.MapModule{},
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

	for _, module := range cfgService.serviceHostConfig.Config.Modules {
		if err := cfgService.ExecuteModule(ctx, module); err != nil {
			return err
		}
	}

	return nil
}

func (cfgService *ConfigService) ExecuteModule(ctx context.Context, module core.Module) error {
	if module.Name != "" {
		app.Log.Info(fmt.Sprintf("-: %s", module.Name))
	}

	exprData := core.ExprData{
		Modules: cfgService.modulesMap,
		Env:     osutils.GetEnvMap(),
		Version: version.ParseVersion(cfgService.appConfig.ConfigManager.GetConfig().Version),
	}

	moduleResolvedEnvMap, err := core.ResolveExprMap(module.Env, exprData)
	if err != nil {
		return err
	}

	shouldExecute := true
	if module.If != "" {
		shouldExecute, err = core.ExtractExprResultBool(module.If, exprData)
		if err != nil {
			return err
		}
	}

	existedEnvMap := map[string]string{}
	for key, value := range moduleResolvedEnvMap {
		if currentValue, ok := os.LookupEnv(key); ok {
			existedEnvMap[key] = currentValue
		}

		if err = os.Setenv(key, value); err != nil {
			return err
		}
	}

	if !shouldExecute {
		if module.Id != "" {
			cfgService.modulesMap[module.Id] = core.MapModule{
				Name:   module.Name,
				Type:   module.Type,
				Id:     module.Id,
				If:     false,
				Output: map[string]string{},
			}
		}

		return nil
	}

	body := module.Body
	if body == nil {
		return fmt.Errorf("module %s has no body", module.Type)
	}

	exprData.Env = osutils.GetEnvMap()

	// Резолвим env переменные в структуре модуля через рефлексию
	if err = core.ResolveStruct(body, exprData); err != nil {
		return fmt.Errorf("failed to resolve env variables: %w", err)
	}

	output := map[string]string{}

	if out, err := body.Execute(ctx, cfgService); err != nil {
		return fmt.Errorf("module '%s': %w", module.GetLabel(), err)
	} else {
		if out == nil && len(module.Output) > 0 {
			app.Log.Warn(fmt.Sprintf(app.T_("'%s' type doesn't support output"), module.Type))
		} else if out != nil {
			output, err = core.ResolveExprMap(module.Output, out)
			if err != nil {
				return err
			}
		}
	}

	for key := range moduleResolvedEnvMap {
		if oldValue, ok := existedEnvMap[key]; ok {
			if err = os.Setenv(key, oldValue); err != nil {
				return err
			}
		} else {
			if err = os.Unsetenv(key); err != nil {
				return err
			}
		}
	}

	if module.Id != "" {
		cfgService.modulesMap[module.Id] = core.MapModule{
			Name:   module.Name,
			Type:   module.Type,
			Id:     module.Id,
			If:     true,
			Output: output,
		}
	}

	return nil
}

func (cfgService *ConfigService) QueryHostImagePackages(ctx context.Context, filters map[string]any, sortField, sortOrder string, limit, offset int) ([]_package.Package, error) {
	return cfgService.serviceDBService.QueryHostImagePackages(ctx, filters, sortField, sortOrder, limit, offset)
}

func (cfgService *ConfigService) GetPackageByName(ctx context.Context, packageName string) (*_package.Package, error) {
	packageInfo, err := cfgService.serviceDBService.GetPackageByName(ctx, packageName)
	if err != nil {
		return nil, err
	}

	return &packageInfo, nil
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

func (cfgService *ConfigService) ExecuteInclude(ctx context.Context, target string) error {
	if osutils.IsURL(target) {
		return cfgService.executeIncludeFile(ctx, target)
	}

	info, err := os.Stat(target)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return cfgService.executeIncludeDir(ctx, target)
	}

	return cfgService.executeIncludeFileWithCD(ctx, target)
}

// executeIncludeDir обрабатывает все файлы в директории
func (cfgService *ConfigService) executeIncludeDir(ctx context.Context, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml") {
			return cfgService.executeIncludeFileWithCD(ctx, path)
		}

		return nil
	})
}

// executeIncludeFileWithCD меняет директорию перед выполнением файла
func (cfgService *ConfigService) executeIncludeFileWithCD(ctx context.Context, filePath string) error {
	originalWd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Гарантированно возвращаемся в исходную директорию
	defer func() {
		if chErr := os.Chdir(originalWd); chErr != nil {
			app.Log.Error(fmt.Sprintf("failed to restore working directory: %v", chErr))
		}
	}()

	includeDir := filepath.Dir(filePath)
	includeName := filepath.Base(filePath)

	if includeDir != originalWd {
		if err = os.Chdir(includeDir); err != nil {
			return fmt.Errorf("failed to change directory to %s: %w", includeDir, err)
		}
	}

	return cfgService.executeIncludeFile(ctx, includeName)
}

// executeIncludeFile читает и выполняет файл с модулями (YAML или JSON)
func (cfgService *ConfigService) executeIncludeFile(ctx context.Context, path string) error {
	modules, err := core.ReadAndParseModules(path)
	if err != nil {
		return fmt.Errorf("failed to parse modules from %s: %w", path, err)
	}

	for _, module := range *modules {
		if err = cfgService.ExecuteModule(ctx, module); err != nil {
			return err
		}
	}

	return nil
}
