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
	"context"
	"errors"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/build/modules"
	"altlinux.space/alt-atomic/apm/internal/common/reply"
	"altlinux.space/alt-atomic/apm/internal/domain/kernel/service"
	reposervice "altlinux.space/alt-atomic/apm/pkg/aptrepo"
	pkgbuild "altlinux.space/alt-atomic/apm/pkg/build"
	_ "altlinux.space/alt-atomic/apm/pkg/build/modules"
	"altlinux.space/alt-atomic/apm/pkg/command"
)

// ConfigService адаптер, оборачивает Engine и добавляет доменные способности.
var _ modules.DomainContext = (*ConfigService)(nil)

type ConfigService struct {
	*domainService

	appConfig         *app.Config
	reporter          *reply.Reporter
	serviceHostConfig buildHostConfigService
	runner            command.Runner

	engine *pkgbuild.Engine
}

func NewConfigService(appConfig *app.Config, reporter *reply.Reporter, aptActions buildAptActionsService, dBService buildPackageDBService,
	kernelManager *service.Manager, repoService *reposervice.RepoService, hostConfig buildHostConfigService,
	runner command.Runner) *ConfigService {
	svc := &ConfigService{
		domainService: &domainService{
			aptActions:    aptActions,
			dbService:     dBService,
			kernelManager: kernelManager,
			repoService:   repoService,
		},
		appConfig:         appConfig,
		reporter:          reporter,
		serviceHostConfig: hostConfig,
		runner:            runner,
	}

	svc.engine = pkgbuild.NewEngine(appConfig.ConfigManager.GetParsedVersion().ToBuildVersion(), reply.EventSystemImageModule)

	svc.engine.Hook(pkgbuild.PreModules, func(_ context.Context) error {
		if !svc.IsAtomic() {
			return nil
		}
		return svc.revertNssAltFiles()
	})
	svc.engine.Hook(pkgbuild.PostModules, func(ctx context.Context) error {
		if !svc.IsAtomic() {
			return nil
		}
		return svc.splitNssAltFiles(ctx)
	})
	svc.engine.Hook(pkgbuild.PostBuild, func(ctx context.Context) error {
		if !svc.IsAtomic() {
			return nil
		}
		return svc.fixTmpFiles(ctx)
	})

	return svc
}

// EmitEvent шлёт событие прогресса модуля через reporter.
func (cfgService *ConfigService) EmitEvent(ctx context.Context, state, name, view string) {
	opts := []reply.NotificationOption{reply.WithEventName(name)}
	if view != "" {
		opts = append(opts, reply.WithEventView(view))
	}
	cfgService.reporter.CreateEventNotification(ctx, state, opts...)
}

func (cfgService *ConfigService) IsAtomic() bool {
	return cfgService.appConfig.ConfigManager.GetConfig().IsAtomic
}

func (cfgService *ConfigService) Runner() command.Runner {
	return cfgService.runner
}

// CollectOutput копит вывод модулей для итогового ответа.
func (cfgService *ConfigService) CollectOutput(text string) {
	cfgService.engine.CollectOutput(text)
}

// CollectedOutput возвращает накопленный вывод модулей.
func (cfgService *ConfigService) CollectedOutput() string {
	return cfgService.engine.CollectedOutput()
}

func (cfgService *ConfigService) ExecuteInclude(ctx context.Context, target string) (map[string]*pkgbuild.MapModule, error) {
	return cfgService.engine.ExecuteInclude(ctx, cfgService, target)
}

// ExecuteModule выполняет один модуль (используется тестами).
func (cfgService *ConfigService) ExecuteModule(ctx context.Context, module pkgbuild.Module, modulesMap map[string]*pkgbuild.MapModule) (*pkgbuild.MapModule, error) {
	return cfgService.engine.ExecuteModule(ctx, cfgService, module, modulesMap)
}

// Build выполняет сборку по загруженному рецепту.
func (cfgService *ConfigService) Build(ctx context.Context) error {
	if cfgService.serviceHostConfig.GetConfig() == nil {
		return errors.New(app.T_("Configuration not loaded. Load config first"))
	}

	return cfgService.engine.Build(ctx, cfgService, cfgService.serviceHostConfig.GetConfig().Modules)
}
