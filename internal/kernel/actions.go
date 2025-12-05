// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
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

package kernel

import (
	"apm/internal/common/app"
	_package "apm/internal/common/apt/package"
	"apm/internal/common/binding/apt"
	"apm/internal/common/reply"
	"apm/internal/kernel/service"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"syscall"
)

// Actions объединяет методы для выполнения системных действий.
type Actions struct {
	appConfig          *app.Config
	serviceAptActions  *_package.Actions
	serviceAptDatabase *_package.PackageDBService
	kernelManager      *service.Manager
}

// NewActionsWithDeps создаёт новый экземпляр Actions с ручными управлением зависимостями
func NewActionsWithDeps(
	appConfig *app.Config,
	aptDB *_package.PackageDBService,
	aptActions *_package.Actions,
	kernelManager *service.Manager,
) *Actions {
	return &Actions{
		appConfig:          appConfig,
		serviceAptDatabase: aptDB,
		serviceAptActions:  aptActions,
		kernelManager:      kernelManager,
	}
}

// NewActions создаёт новый экземпляр Actions.
func NewActions(appConfig *app.Config) *Actions {
	hostPackageDBSvc := _package.NewPackageDBService(appConfig.DatabaseManager)

	aptActions := apt.NewActions()
	aptPackageActions := _package.NewActions(hostPackageDBSvc, appConfig)
	kernelManager := service.NewKernelManager(hostPackageDBSvc, aptActions)

	return &Actions{
		appConfig:          appConfig,
		serviceAptDatabase: hostPackageDBSvc,
		serviceAptActions:  aptPackageActions,
		kernelManager:      kernelManager,
	}
}

// ListKernels возвращает список ядер
func (a *Actions) ListKernels(ctx context.Context, flavour string, installedOnly bool) (*reply.APIResponse, error) {
	err := a.validateDB(ctx)
	if err != nil {
		return nil, err
	}
	kernels, err := a.kernelManager.ListKernels(ctx, flavour)
	if err != nil {
		return nil, err
	}

	if installedOnly {
		var installedKernels []*service.Info
		for _, kernel := range kernels {
			if kernel.IsInstalled {
				installedKernels = append(installedKernels, kernel)
			}
		}
		kernels = installedKernels
	}

	if len(kernels) == 0 {
		return nil, errors.New(app.T_("No kernels found"))
	}

	return &reply.APIResponse{
		Data: ListKernelsResponse{
			Message: fmt.Sprintf(app.TN_("%d kernel found", "%d kernels found", len(kernels)), len(kernels)),
			Kernels: a.formatKernelOutput(kernels),
		},
		Error: false,
	}, nil
}

// GetCurrentKernel возвращает информацию о текущем ядре
func (a *Actions) GetCurrentKernel(ctx context.Context) (*reply.APIResponse, error) {
	err := a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	kernel, err := a.kernelManager.GetCurrentKernel(ctx)
	if err != nil {
		return nil, err
	}

	return &reply.APIResponse{
		Data: GetCurrentKernelResponse{
			Message: app.T_("Current kernel information"),
			Kernel:  kernel.ToFull(a.kernelManager),
		},
		Error: false,
	}, nil
}

// InstallKernel устанавливает ядро с указанным flavour
func (a *Actions) InstallKernel(ctx context.Context, flavour string, modules []string, includeHeaders bool, dryRun bool) (*reply.APIResponse, error) {
	err := a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	err = a.serviceAptActions.AptUpdate(ctx)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(flavour) == "" {
		return nil, errors.New(app.T_("Kernel flavour must be specified"))
	}
	latest, err := a.kernelManager.FindLatestKernel(ctx, flavour)
	if err != nil {
		return nil, err
	}

	if len(modules) == 0 {
		currentKernel, _ := a.kernelManager.GetCurrentKernel(ctx)
		inheritedModules, _ := a.kernelManager.InheritModulesFromKernel(latest, currentKernel)
		if len(inheritedModules) > 0 {
			modules = inheritedModules
		}
	}

	// Автоматическое добавление headers и модулей
	additionalPackages, _ := a.kernelManager.AutoSelectHeadersAndFirmware(ctx, latest, includeHeaders)

	// Добавляем дополнительные пакеты к модулям
	for _, pkg := range additionalPackages {
		// Если это модуль ядра - извлекаем имя модуля
		if strings.HasPrefix(pkg, "kernel-modules-") && strings.HasSuffix(pkg, fmt.Sprintf("-%s", latest.Flavour)) {
			moduleName := strings.TrimPrefix(pkg, "kernel-modules-")
			moduleName = strings.TrimSuffix(moduleName, fmt.Sprintf("-%s", latest.Flavour))
			// Добавляем только если его еще нет в списке
			moduleExists := slices.Contains(modules, moduleName)
			if !moduleExists {
				modules = append(modules, moduleName)
			}
		}
	}

	preview, err := a.kernelManager.SimulateUpgrade(latest, modules, includeHeaders)
	if err != nil {
		return nil, err
	}

	if len(preview.MissingModules) > 0 {
		return nil, fmt.Errorf(app.T_("some modules are not available: %s"), strings.Join(preview.MissingModules, ", "))
	}

	if len(preview.Changes.NewInstalledPackages) == 0 && len(preview.Changes.UpgradedPackages) == 0 {
		return &reply.APIResponse{
			Data: InstallUpdateKernelResponse{
				Message: fmt.Sprintf(app.T_("Kernel %s is already installed"), latest.FullVersion),
				Kernel:  latest.ToFull(a.kernelManager),
				Preview: nil,
			},
			Error: false,
		}, nil
	}

	if dryRun {
		return &reply.APIResponse{
			Data: InstallUpdateKernelResponse{
				Message: app.T_("Installation preview"),
				Kernel:  latest.ToFull(a.kernelManager),
				Preview: preview,
			},
			Error: false,
		}, nil
	}

	err = a.kernelManager.InstallKernel(ctx, latest, modules, includeHeaders, false)
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to install kernel: %s"), err.Error())
	}

	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return nil, err
	}

	return &reply.APIResponse{
		Data: InstallUpdateKernelResponse{
			Message: fmt.Sprintf(app.T_("Kernel %s installed successfully"), latest.FullVersion),
			Kernel:  latest.ToFull(a.kernelManager),
			Preview: preview,
		},
		Error: false,
	}, nil
}

// UpdateKernel обновляет ядро до последней версии
func (a *Actions) UpdateKernel(ctx context.Context, flavour string, modules []string, includeHeaders bool, dryRun bool) (*reply.APIResponse, error) {
	var err error
	err = a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	_, err = a.serviceAptActions.Update(ctx)
	if err != nil {
		return nil, err
	}

	userSpecifiedFlavour := flavour != ""

	flavour, err = a.detectFlavourOrDefault(ctx, flavour)
	if err != nil {
		return nil, err
	}

	latest, err := a.kernelManager.FindLatestKernel(ctx, flavour)

	// Если ядро не найдено и пользователь НЕ указал flavour явно - ищем ближайшую новую версию
	if err != nil && !userSpecifiedFlavour {
		current, currentErr := a.kernelManager.GetCurrentKernel(ctx)
		if currentErr == nil {
			nextFlavours, flavourErr := a.kernelManager.FindNextFlavours(current.Version)
			if flavourErr == nil && len(nextFlavours) > 0 {
				// Берем ПЕРВЫЙ (ближайший новый) flavour
				newFlavour := nextFlavours[0]

				newLatest, newErr := a.kernelManager.FindLatestKernel(ctx, newFlavour)
				if newErr == nil {
					latest = newLatest
					flavour = newFlavour
					err = nil
				}
			}
		}
	}

	if err != nil {
		if userSpecifiedFlavour {
			return nil, fmt.Errorf(app.T_("no kernels found for flavour: %s. Remove --flavour option to attempt an automatic upgrade"), flavour)
		}
		return nil, fmt.Errorf(app.T_("no kernels found for flavour: %s"), flavour)
	}

	current, err := a.kernelManager.GetCurrentKernel(ctx)
	if err != nil {
		return nil, err
	}

	// Сравниваем установленную версию с доступной версией
	if latest.Version == current.VersionInstalled && latest.Release == current.Release {
		return &reply.APIResponse{
			Data: InstallUpdateKernelResponse{
				Message: app.T_("Kernel is already up to date"),
				Kernel:  latest.ToFull(a.kernelManager),
				Preview: nil,
			},
			Error: false,
		}, nil
	}

	if len(modules) == 0 {
		inheritedModules, errModuleKernel := a.kernelManager.InheritModulesFromKernel(latest, current)
		if errModuleKernel == nil && len(inheritedModules) > 0 {
			modules = inheritedModules
		}
	}

	return a.InstallKernel(ctx, flavour, modules, includeHeaders, dryRun)
}

// CleanOldKernels удаляет старые ядра
func (a *Actions) CleanOldKernels(ctx context.Context, noBackup bool, dryRun bool) (*reply.APIResponse, error) {
	err := a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	// Получаем все установленные ядра через RPM
	allKernels, err := a.kernelManager.ListInstalledKernelsFromRPM(ctx)
	if err != nil {
		return nil, err
	}

	if len(allKernels) == 0 {
		return nil, errors.New(app.T_("no kernels found"))
	}

	// Определяем текущее ядро
	currentKernel, err := a.kernelManager.GetCurrentKernel(ctx)
	if err != nil {
		return nil, err
	}

	// Определяем backup ядро (с uptime >= 1 день)
	var backupKernel *service.Info
	if !noBackup {
		backupKernel, _ = a.kernelManager.GetBackupKernel(ctx)
	}

	// Группируем ядра по flavour
	flavourGroups := a.kernelManager.GroupKernelsByFlavour(allKernels)

	var targetFlavours []string
	for fl := range flavourGroups {
		targetFlavours = append(targetFlavours, fl)
	}

	var toRemove []service.Info
	var keptKernels []WithReasons

	for _, fl := range targetFlavours {
		kernelsInFlavour := flavourGroups[fl]
		if len(kernelsInFlavour) == 0 {
			continue
		}

		newestKernel := kernelsInFlavour[0]

		for _, kernel := range kernelsInFlavour {
			var reasons []string

			// 1. Сохраняем новейшее ядро только для текущего загруженного flavour'а
			if kernel.FullVersion == newestKernel.FullVersion && currentKernel != nil && fl == currentKernel.Flavour {
				reasons = append(reasons, fmt.Sprintf(app.T_("latest for %s"), fl))
			}

			// 2. Сохраняем текущее запущенное ядро
			if currentKernel != nil && kernel.Version == currentKernel.Version &&
				kernel.Release == currentKernel.Release && kernel.Flavour == currentKernel.Flavour {
				reasons = append(reasons, app.T_("Currently booted"))
				kernel.IsRunning = true
			}

			// 3. Сохраняем backup ядро (с uptime >= 1 день)
			if backupKernel != nil && kernel.FullVersion == backupKernel.FullVersion {
				reasons = append(reasons, app.T_("backup kernel"))
			}

			// Если есть причины для сохранения - добавляем в список сохраненных
			if len(reasons) > 0 {
				keptKernels = append(keptKernels, WithReasons{
					Kernel:  *kernel,
					Reasons: reasons,
				})
			} else {
				// Если нет причин для сохранения - помечаем на удаление
				toRemove = append(toRemove, *kernel)
			}
		}
	}

	if len(toRemove) == 0 {
		return nil, errors.New(app.T_("no old kernels to clean"))
	}

	if dryRun {
		var removePackages []string
		for _, kernel := range toRemove {
			removePackages = append(removePackages, kernel.FullVersion)
		}

		combinedPreview, errRemove := a.kernelManager.RemovePackages(ctx, removePackages, true)
		if errRemove != nil {
			return nil, fmt.Errorf(app.T_("failed to simulate kernels removal: %s"), errRemove.Error())
		}

		return &reply.APIResponse{
			Data: CleanOldKernelsResponse{
				Message:       fmt.Sprintf(app.TN_("Would remove %d old kernel", "Would remove %d old kernels", len(toRemove)), len(toRemove)),
				RemoveKernels: toRemove,
				KeptKernels:   keptKernels,
				Preview:       combinedPreview,
			},
			Error: false,
		}, nil
	}

	var removePackages []string
	for _, kernel := range toRemove {
		removePackages = append(removePackages, kernel.FullVersion)
	}

	combinedPreview, err := a.kernelManager.RemovePackages(ctx, removePackages, false)
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to remove kernels: %s"), err.Error())
	}

	return &reply.APIResponse{
		Data: CleanOldKernelsResponse{
			Message:       fmt.Sprintf(app.TN_("Successfully removed %d old kernel", "Successfully removed %d old kernels", len(toRemove)), len(toRemove)),
			RemoveKernels: toRemove,
			KeptKernels:   keptKernels,
			Preview:       combinedPreview,
		},
		Error: false,
	}, nil
}

// ListKernelModules возвращает список модулей для ядра
func (a *Actions) ListKernelModules(ctx context.Context, flavour string) (*reply.APIResponse, error) {
	var err error
	err = a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	if flavour == "" {
		detected, err := a.kernelManager.DetectCurrentFlavour(ctx)
		if err != nil {
			return nil, err
		}
		flavour = detected
	}

	latest, err := a.kernelManager.FindLatestKernel(ctx, flavour)
	if err != nil {
		return nil, err
	}

	modules, err := a.kernelManager.FindAvailableModules(latest)
	if err != nil {
		return nil, err
	}

	return &reply.APIResponse{
		Data: ListKernelModulesResponse{
			Message: fmt.Sprintf(app.TN_("%d module found", "%d modules found", len(modules)), len(modules)),
			Kernel:  latest.ToFull(a.kernelManager),
			Modules: modules,
		},
		Error: false,
	}, nil
}

// InstallKernelModules устанавливает модули ядра
func (a *Actions) InstallKernelModules(ctx context.Context, flavour string,
	modules []string, dryRun bool) (*reply.APIResponse, error) {
	err := a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	if len(modules) == 0 {
		return nil, errors.New(app.T_("At least one module must be specified"))
	}

	if flavour == "" {
		detected, err := a.kernelManager.DetectCurrentFlavour(ctx)
		if err != nil {
			return nil, err
		}
		flavour = detected
	}

	latest, err := a.kernelManager.FindLatestKernel(ctx, flavour)
	if err != nil {
		return nil, err
	}

	availableModules, err := a.kernelManager.FindAvailableModules(latest)
	if err != nil {
		return nil, err
	}

	var missingModules []string
	for _, module := range modules {
		found := false
		for _, available := range availableModules {
			if module == available.Name {
				found = true
				break
			}
		}
		if !found {
			missingModules = append(missingModules, module)
		}
	}

	if len(missingModules) > 0 {
		return nil, fmt.Errorf(app.T_("modules not available: %s"), strings.Join(missingModules, ", "))
	}

	// Проверяем уже установленные модули только для текущего ядра
	currentKernel, err := a.kernelManager.GetCurrentKernel(ctx)
	if err == nil && currentKernel.Flavour == latest.Flavour {
		var alreadyInstalledModules []string
		for _, module := range modules {
			for _, available := range availableModules {
				if module == available.Name && available.IsInstalled {
					alreadyInstalledModules = append(alreadyInstalledModules, module)
					break
				}
			}
		}
		if len(alreadyInstalledModules) > 0 {
			return nil, fmt.Errorf(app.T_("modules already installed: %s"), strings.Join(alreadyInstalledModules, ", "))
		}
	}

	var installPackages []string
	for _, module := range modules {
		for _, available := range availableModules {
			if module == available.Name {
				fullPackageName := a.kernelManager.GetFullPackageNameForModule(available.PackageName)
				installPackages = append(installPackages, fullPackageName)
				break
			}
		}
	}

	if dryRun {
		preview, err := a.kernelManager.InstallModules(ctx, installPackages, true)
		if err != nil {
			return nil, fmt.Errorf(app.T_("failed to simulate modules installation: %s"), err.Error())
		}

		return &reply.APIResponse{
			Data: InstallKernelModulesResponse{
				Message: app.T_("Modules installation preview"),
				Kernel:  latest.ToFull(a.kernelManager),
				Preview: preview,
			},
			Error: false,
		}, nil
	}

	_, err = a.kernelManager.InstallModules(ctx, installPackages, false)
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to install modules: %s"), err.Error())
	}

	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return nil, err
	}

	updatedKernel, err := a.kernelManager.FindLatestKernel(ctx, flavour)
	if err != nil {
		return nil, err
	}

	return &reply.APIResponse{
		Data: InstallKernelModulesResponse{
			Message: fmt.Sprintf(app.TN_("%d module installed successfully for kernel %s", "%d modules installed successfully for kernel %s", len(modules)), len(modules), updatedKernel.FullVersion),
			Kernel:  updatedKernel.ToFull(a.kernelManager),
			Preview: nil,
		},
		Error: false,
	}, nil
}

// RemoveKernelModules удаляет модули ядра
func (a *Actions) RemoveKernelModules(ctx context.Context, flavour string,
	modules []string, dryRun bool) (*reply.APIResponse, error) {
	err := a.validateDB(ctx)
	if err != nil {
		return nil, err
	}

	if len(modules) == 0 {
		return nil, errors.New(app.T_("At least one module must be specified"))
	}

	if flavour == "" {
		detected, err := a.kernelManager.DetectCurrentFlavour(ctx)
		if err != nil {
			return nil, err
		}
		flavour = detected
	}

	latest, err := a.kernelManager.FindLatestKernel(ctx, flavour)
	if err != nil {
		return nil, err
	}

	availableModules, err := a.kernelManager.FindAvailableModules(latest)
	if err != nil {
		return nil, err
	}

	var notInstalledModules []string
	var modulesToRemove []string
	for _, module := range modules {
		found := false
		for _, available := range availableModules {
			if module == available.Name {
				found = true
				if available.IsInstalled {
					modulesToRemove = append(modulesToRemove, module)
				} else {
					notInstalledModules = append(notInstalledModules, module)
				}
				break
			}
		}
		if !found {
			return nil, fmt.Errorf(app.T_("module not found: %s"), module)
		}
	}

	if len(notInstalledModules) > 0 {
		return nil, fmt.Errorf(app.T_("modules not installed: %s"), strings.Join(notInstalledModules, ", "))
	}

	if len(modulesToRemove) == 0 {
		return &reply.APIResponse{
			Data: RemoveKernelModulesResponse{
				Message: app.T_("No modules to remove"),
				Kernel:  latest.ToFull(a.kernelManager),
				Preview: nil,
			},
			Error: false,
		}, nil
	}

	var removePackages []string
	for _, module := range modulesToRemove {
		for _, available := range availableModules {
			if module == available.Name && available.IsInstalled {
				simplePackageName := a.kernelManager.GetSimplePackageNameForModule(available.PackageName)
				removePackages = append(removePackages, simplePackageName)
				break
			}
		}
	}

	if dryRun {
		preview, err := a.kernelManager.RemovePackages(ctx, removePackages, true)
		if err != nil {
			return nil, fmt.Errorf(app.T_("failed to simulate modules removal: %s"), err.Error())
		}

		return &reply.APIResponse{
			Data: RemoveKernelModulesResponse{
				Message: app.T_("Modules removal preview"),
				Kernel:  latest.ToFull(a.kernelManager),
				Preview: preview,
			},
			Error: false,
		}, nil
	}

	_, err = a.kernelManager.RemovePackages(ctx, removePackages, false)
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to remove modules: %s"), err.Error())
	}

	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return nil, err
	}

	// Получаем обновлённую информацию о ядре после удаления модулей
	updatedKernel, err := a.kernelManager.GetCurrentKernel(ctx)
	if err != nil {
		return nil, err
	}

	return &reply.APIResponse{
		Data: RemoveKernelModulesResponse{
			Message: fmt.Sprintf(app.TN_("%d module removed successfully from kernel %s", "%d modules removed successfully from kernel %s", len(modulesToRemove)), len(modulesToRemove), latest.FullVersion),
			Kernel:  updatedKernel.ToFull(a.kernelManager),
			Preview: nil,
		},
		Error: false,
	}, nil
}

// formatKernelOutput форматирует вывод информации о ядрах
func (a *Actions) formatKernelOutput(kernels []*service.Info) []service.FullKernelInfo {
	var result []service.FullKernelInfo
	for _, kernel := range kernels {
		result = append(result, kernel.ToFull(a.kernelManager))
	}
	return result
}

// detectFlavourOrDefault возвращает указанный flavour или определяет автоматически
func (a *Actions) detectFlavourOrDefault(ctx context.Context, flavour string) (string, error) {
	if flavour != "" {
		return flavour, nil
	}
	return a.kernelManager.DetectCurrentFlavour(ctx)
}

// findKernelByVersion находит ядро по версии из списка
func (a *Actions) findKernelByVersion(version string, kernels []*service.Info) *service.Info {
	for _, kernel := range kernels {
		if kernel.FullVersion == version || kernel.Version == version || kernel.Flavour == version {
			return kernel
		}
	}
	return nil
}

// updateAllPackagesDB обновляет состояние всех пакетов в базе данных
func (a *Actions) updateAllPackagesDB(ctx context.Context) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.updateAllPackagesDB"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.updateAllPackagesDB"))

	installedPackages, err := a.serviceAptActions.GetInstalledPackages(ctx)
	if err != nil {
		return err
	}

	err = a.serviceAptDatabase.SyncPackageInstallationInfo(ctx, installedPackages)
	if err != nil {
		return err
	}

	return nil
}

// validateDB проверяет, существует ли база данных
func (a *Actions) validateDB(ctx context.Context) error {
	if err := a.serviceAptDatabase.PackageDatabaseExist(ctx); err != nil {
		if syscall.Geteuid() != 0 {
			return reply.CliResponse(ctx, newErrorResponse(app.T_("Elevated rights are required to perform this action. Please use sudo or su")))
		}

		_, err = a.serviceAptActions.Update(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// GenerateOnlineDoc запускает веб-сервер с HTML документацией для DBus API
func (a *Actions) GenerateOnlineDoc(ctx context.Context) error {
	return startDocServer(ctx)
}
