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
	_package "apm/internal/common/apt/package"
	"apm/internal/common/binding/apt"
	"apm/internal/common/reply"
	"apm/internal/kernel/service"
	"apm/lib"
	"context"
	"errors"
	"fmt"
	"strings"
)

// Actions объединяет методы для выполнения системных действий.
type Actions struct {
	serviceAptActions  *_package.Actions
	serviceAptDatabase *_package.PackageDBService
	kernelManager      *service.Manager
}

// NewActionsWithDeps создаёт новый экземпляр Actions с ручными управлением зависимостями
func NewActionsWithDeps(
	aptDB *_package.PackageDBService,
	aptActions *_package.Actions,
	kernelManager *service.Manager,
) *Actions {
	return &Actions{
		serviceAptDatabase: aptDB,
		serviceAptActions:  aptActions,
		kernelManager:      kernelManager,
	}
}

// NewActions создаёт новый экземпляр Actions.
func NewActions() *Actions {
	hostPackageDBSvc, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		lib.Log.Fatal(err)
	}

	aptActions := apt.NewActions()
	aptPackageActions := _package.NewActions(hostPackageDBSvc)
	kernelManager := service.NewKernelManager(hostPackageDBSvc, aptActions)

	return &Actions{
		serviceAptDatabase: hostPackageDBSvc,
		serviceAptActions:  aptPackageActions,
		kernelManager:      kernelManager,
	}
}

// ListKernels возвращает список ядер
func (a *Actions) ListKernels(ctx context.Context, flavour string, installedOnly bool, full bool) (*reply.APIResponse, error) {
	kernels, err := a.kernelManager.ListKernels(flavour)
	if err != nil {
		return nil, fmt.Errorf("failed to list kernels: %w", err)
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
		return nil, errors.New(lib.T_("No kernels found"))
	}

	data := map[string]interface{}{
		"message": fmt.Sprintf(lib.TN_("%d kernel found", "%d kernels found", len(kernels)), len(kernels)),
		"kernels": a.formatKernelOutput(kernels, full),
	}

	return &reply.APIResponse{
		Data:  data,
		Error: false,
	}, nil
}

// GetCurrentKernel возвращает информацию о текущем ядре
func (a *Actions) GetCurrentKernel(ctx context.Context) (*reply.APIResponse, error) {
	kernel, err := a.kernelManager.GetCurrentKernel()
	if err != nil {
		return nil, fmt.Errorf("failed to get current kernel: %w", err)
	}

	data := map[string]interface{}{
		"message": lib.T_("Current kernel information"),
		"kernel":  a.formatKernelOutput([]*service.Info{kernel}, true)[0],
	}

	return &reply.APIResponse{
		Data:  data,
		Error: false,
	}, nil
}

// GetKernelInfo возвращает информацию о текущем ядре
func (a *Actions) GetKernelInfo(ctx context.Context) (*reply.APIResponse, error) {
	return a.GetCurrentKernel(ctx)
}

// InstallKernel устанавливает ядро с указанным flavour
func (a *Actions) InstallKernel(ctx context.Context, flavour string, modules []string, includeHeaders bool, dryRun bool) (*reply.APIResponse, error) {
	if err := a.checkAtomicSystemRestriction("install", dryRun); err != nil {
		return nil, err
	}

	err := a.serviceAptActions.AptUpdate(ctx)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(flavour) == "" {
		return nil, errors.New(lib.T_("Kernel flavour must be specified"))
	}
	latest, err := a.kernelManager.FindLatestKernel(flavour)
	if err != nil {
		return nil, fmt.Errorf("failed to find latest kernel for flavour %s: %w", flavour, err)
	}

	if len(modules) == 0 {
		currentKernel, _ := a.kernelManager.GetCurrentKernel()
		inheritedModules, _ := a.kernelManager.InheritModulesFromKernel(latest, currentKernel)
		if len(inheritedModules) > 0 {
			modules = inheritedModules
		}
	}

	// Автоматическое добавление headers и модулей как в bash скрипте
	additionalPackages, _ := a.kernelManager.AutoSelectHeadersAndFirmware(latest, includeHeaders)

	// Добавляем дополнительные пакеты к модулям
	for _, pkg := range additionalPackages {
		// Если это модуль ядра - извлекаем имя модуля
		if strings.HasPrefix(pkg, "kernel-modules-") && strings.HasSuffix(pkg, fmt.Sprintf("-%s", latest.Flavour)) {
			moduleName := strings.TrimPrefix(pkg, "kernel-modules-")
			moduleName = strings.TrimSuffix(moduleName, fmt.Sprintf("-%s", latest.Flavour))
			// Добавляем только если его еще нет в списке
			moduleExists := false
			for _, existingModule := range modules {
				if existingModule == moduleName {
					moduleExists = true
					break
				}
			}
			if !moduleExists {
				modules = append(modules, moduleName)
			}
		}
	}

	preview, err := a.kernelManager.SimulateUpgrade(latest, modules, includeHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to simulate kernel installation: %w", err)
	}

	if len(preview.MissingModules) > 0 {
		return nil, fmt.Errorf("some modules are not available: %s", strings.Join(preview.MissingModules, ", "))
	}

	if len(preview.Changes.NewInstalledPackages) == 0 && len(preview.Changes.UpgradedPackages) == 0 {
		message := fmt.Sprintf(lib.T_("Kernel %s is already installed"), latest.FullVersion)
		return &reply.APIResponse{
			Data: map[string]interface{}{
				"message": message,
				"kernel":  latest.ToMap(true, a.kernelManager),
			},
			Error: false,
		}, nil
	}

	if dryRun {
		data := map[string]interface{}{
			"message": lib.T_("Installation preview"),
			"kernel":  latest.ToMap(true, a.kernelManager),
			"preview": preview,
		}

		return &reply.APIResponse{
			Data:  data,
			Error: false,
		}, nil
	}

	err = a.kernelManager.InstallKernel(latest, modules, includeHeaders, false)
	if err != nil {
		return nil, fmt.Errorf("failed to install kernel: %w", err)
	}

	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"message": fmt.Sprintf(lib.T_("Kernel %s installed successfully"), latest.FullVersion),
		"kernel":  latest,
		"changes": preview.Changes,
	}

	return &reply.APIResponse{
		Data:  data,
		Error: false,
	}, nil
}

// UpdateKernel обновляет ядро до последней версии
func (a *Actions) UpdateKernel(ctx context.Context, flavour string, modules []string, includeHeaders bool, dryRun bool) (*reply.APIResponse, error) {
	var err error

	if err = a.checkAtomicSystemRestriction("update", dryRun); err != nil {
		return nil, err
	}

	_, err = a.serviceAptActions.Update(ctx)
	if err != nil {
		return nil, err
	}

	flavour, err = a.detectFlavourOrDefault(flavour)
	if err != nil {
		return nil, fmt.Errorf("failed to detect current flavour: %w", err)
	}

	latest, err := a.kernelManager.FindLatestKernel(flavour)
	if err != nil {
		return nil, fmt.Errorf("failed to find latest kernel: %w", err)
	}

	current, err := a.kernelManager.GetCurrentKernel()
	if err != nil {
		return nil, fmt.Errorf("failed to get current kernel: %w", err)
	}

	if latest.Version == current.Version && latest.Release == current.Release {
		return &reply.APIResponse{
			Data: map[string]interface{}{
				"message": lib.T_("Kernel is already up to date"),
				"kernel":  current.ToMap(true, a.kernelManager),
			},
			Error: false,
		}, nil
	}

	if len(modules) == 0 {
		allModules, err := a.kernelManager.FindAvailableModules(current)
		if err == nil {
			for _, module := range allModules {
				if module.IsInstalled {
					modules = append(modules, module.Name)
				}
			}
		}
	}

	return a.InstallKernel(ctx, flavour, modules, includeHeaders, dryRun)
}

// CheckKernelUpdate проверяет наличие обновлений ядра
func (a *Actions) CheckKernelUpdate(ctx context.Context, flavour string) (*reply.APIResponse, error) {
	var err error

	if err = a.checkAtomicSystemRestriction("update", false); err != nil {
		return nil, err
	}

	_, err = a.serviceAptActions.Update(ctx)
	if err != nil {
		return nil, err
	}

	flavour, err = a.detectFlavourOrDefault(flavour)
	if err != nil {
		return nil, fmt.Errorf("failed to detect current flavour: %w", err)
	}

	current, err := a.kernelManager.GetCurrentKernel()
	if err != nil {
		return nil, fmt.Errorf("failed to get current kernel: %w", err)
	}

	latest, err := a.kernelManager.FindLatestKernel(flavour)
	if err != nil {
		return nil, fmt.Errorf("failed to find latest kernel: %w", err)
	}

	updateAvailable := !(latest.Version == current.Version && latest.Release == current.Release)

	data := map[string]interface{}{
		"message":         lib.T_("Kernel update check"),
		"currentKernel":   current,
		"latestKernel":    latest,
		"updateAvailable": updateAvailable,
	}

	return &reply.APIResponse{
		Data:  data,
		Error: false,
	}, nil
}

// CleanOldKernels удаляет старые ядра
func (a *Actions) CleanOldKernels(ctx context.Context, keep int, dryRun bool) (*reply.APIResponse, error) {
	if keep < 1 {
		return nil, errors.New(lib.T_("Keep count must be at least 1"))
	}

	kernels, err := a.kernelManager.ListKernels("")
	if err != nil {
		return nil, fmt.Errorf("failed to list kernels: %w", err)
	}

	current, err := a.kernelManager.GetCurrentKernel()
	if err != nil {
		return nil, fmt.Errorf("failed to get current kernel: %w", err)
	}

	var installedKernels []*service.Info
	for _, kernel := range kernels {
		if kernel.IsInstalled && !kernel.IsRunning {
			installedKernels = append(installedKernels, kernel)
		}
	}

	if len(installedKernels) <= keep {
		return &reply.APIResponse{
			Data: map[string]interface{}{
				"message":       lib.T_("No old kernels to clean"),
				"currentKernel": current,
				"kept":          len(installedKernels) + 1, // +1 для текущего ядра
			},
			Error: false,
		}, nil
	}

	toRemove := installedKernels[keep:]

	if dryRun {
		data := map[string]interface{}{
			"message":     lib.T_("Kernels that would be removed"),
			"toRemove":    a.formatKernelOutput(toRemove, false),
			"removeCount": len(toRemove),
		}

		return &reply.APIResponse{
			Data:  data,
			Error: false,
		}, nil
	}

	var removed []*service.Info
	for _, kernel := range toRemove {
		err = a.kernelManager.RemoveKernel(kernel, false)
		if err != nil {
			return nil, fmt.Errorf("failed to remove kernel %s: %w", kernel.FullVersion, err)
		}
		removed = append(removed, kernel)
	}

	data := map[string]interface{}{
		"message":        fmt.Sprintf(lib.T_("Successfully removed %d old kernels"), len(removed)),
		"currentKernel":  current,
		"removedKernels": a.formatKernelOutput(removed, false),
		"kept":           len(installedKernels) - len(removed) + 1, // +1 для текущего ядра
	}

	return &reply.APIResponse{
		Data:  data,
		Error: false,
	}, nil
}

// ListKernelModules возвращает список модулей для ядра
func (a *Actions) ListKernelModules(ctx context.Context, flavour string) (*reply.APIResponse, error) {
	var err error
	flavour, err = a.detectFlavourOrDefault(flavour)
	if err != nil {
		return nil, fmt.Errorf("failed to detect current flavour: %w", err)
	}

	latest, err := a.kernelManager.FindLatestKernel(flavour)
	if err != nil {
		return nil, fmt.Errorf("failed to find kernel for flavour %s: %w", flavour, err)
	}

	modules, err := a.kernelManager.FindAvailableModules(latest)
	if err != nil {
		return nil, fmt.Errorf("failed to list modules: %w", err)
	}

	data := map[string]interface{}{
		"message": fmt.Sprintf(lib.TN_("%d module found", "%d modules found", len(modules)), len(modules)),
		"kernel":  latest.ToMap(false, a.kernelManager),
		"modules": modules,
	}

	return &reply.APIResponse{
		Data:  data,
		Error: false,
	}, nil
}

// InstallKernelModules устанавливает модули ядра
func (a *Actions) InstallKernelModules(ctx context.Context, flavour string, modules []string, dryRun bool) (*reply.APIResponse, error) {
	if err := a.checkAtomicSystemRestriction("install", dryRun); err != nil {
		return nil, err
	}
	if len(modules) == 0 {
		return nil, errors.New(lib.T_("At least one module must be specified"))
	}

	if flavour == "" {
		detected, err := a.kernelManager.DetectCurrentFlavour()
		if err != nil {
			return nil, fmt.Errorf("failed to detect current flavour: %w", err)
		}
		flavour = detected
	}

	latest, err := a.kernelManager.FindLatestKernel(flavour)
	if err != nil {
		return nil, fmt.Errorf("failed to find kernel for flavour %s: %w", flavour, err)
	}

	availableModules, err := a.kernelManager.FindAvailableModules(latest)
	if err != nil {
		return nil, fmt.Errorf("failed to get available modules: %w", err)
	}

	var missingModules []string
	var alreadyInstalledModules []string
	for _, module := range modules {
		found := false
		for _, available := range availableModules {
			if module == available.Name {
				found = true
				if available.IsInstalled {
					alreadyInstalledModules = append(alreadyInstalledModules, module)
				}
				break
			}
		}
		if !found {
			missingModules = append(missingModules, module)
		}
	}

	if len(missingModules) > 0 {
		return nil, fmt.Errorf("modules not available: %s", strings.Join(missingModules, ", "))
	}

	if len(alreadyInstalledModules) > 0 {
		return nil, fmt.Errorf("modules already installed: %s", strings.Join(alreadyInstalledModules, ", "))
	}

	// Подготавливаем список пакетов для установки
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
		preview, err := a.kernelManager.InstallModules(installPackages, true)
		if err != nil {
			return nil, fmt.Errorf("failed to simulate modules installation: %w", err)
		}

		data := map[string]interface{}{
			"message": lib.T_("Modules installation preview"),
			"kernel":  latest.ToMap(true, a.kernelManager),
			"preview": preview,
		}

		return &reply.APIResponse{
			Data:  data,
			Error: false,
		}, nil
	}

	_, err = a.kernelManager.InstallModules(installPackages, false)
	if err != nil {
		return nil, fmt.Errorf("failed to install modules: %w", err)
	}

	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return nil, err
	}

	// Получаем обновлённую информацию о ядре после установки модулей
	updatedKernel, err := a.kernelManager.GetCurrentKernel()
	if err != nil {
		return nil, fmt.Errorf("failed to get updated kernel info: %w", err)
	}

	data := map[string]interface{}{
		"message": fmt.Sprintf(lib.TN_("%d module installed successfully for kernel %s", "%d modules installed successfully for kernel %s", len(modules)), len(modules), latest.FullVersion),
		"kernel":  updatedKernel.ToMap(true, a.kernelManager),
	}

	return &reply.APIResponse{
		Data:  data,
		Error: false,
	}, nil
}

// RemoveKernelModules удаляет модули ядра
func (a *Actions) RemoveKernelModules(ctx context.Context, flavour string, modules []string, dryRun bool) (*reply.APIResponse, error) {
	if err := a.checkAtomicSystemRestriction("remove", dryRun); err != nil {
		return nil, err
	}
	if len(modules) == 0 {
		return nil, errors.New(lib.T_("At least one module must be specified"))
	}

	if flavour == "" {
		detected, err := a.kernelManager.DetectCurrentFlavour()
		if err != nil {
			return nil, fmt.Errorf("failed to detect current flavour: %w", err)
		}
		flavour = detected
	}

	latest, err := a.kernelManager.FindLatestKernel(flavour)
	if err != nil {
		return nil, fmt.Errorf("failed to find kernel for flavour %s: %w", flavour, err)
	}

	availableModules, err := a.kernelManager.FindAvailableModules(latest)
	if err != nil {
		return nil, fmt.Errorf("failed to get available modules: %w", err)
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
			return nil, fmt.Errorf("module not found: %s", module)
		}
	}

	if len(notInstalledModules) > 0 {
		return nil, fmt.Errorf("modules not installed: %s", strings.Join(notInstalledModules, ", "))
	}

	if len(modulesToRemove) == 0 {
		return &reply.APIResponse{
			Data: map[string]interface{}{
				"message": lib.T_("No modules to remove"),
				"kernel":  latest.ToMap(true, a.kernelManager),
			},
			Error: false,
		}, nil
	}

	// Подготавливаем список пакетов для удаления
	var removePackages []string
	for _, module := range modulesToRemove {
		for _, available := range availableModules {
			if module == available.Name && available.IsInstalled {
				fullPackageName := a.kernelManager.GetFullPackageNameForModule(available.PackageName)
				removePackages = append(removePackages, fullPackageName)
				break
			}
		}
	}

	if dryRun {
		preview, err := a.kernelManager.RemoveModules(removePackages, true)
		if err != nil {
			return nil, fmt.Errorf("failed to simulate modules removal: %w", err)
		}

		data := map[string]interface{}{
			"message": lib.T_("Modules removal preview"),
			"kernel":  latest.ToMap(true, a.kernelManager),
			"preview": preview,
		}

		return &reply.APIResponse{
			Data:  data,
			Error: false,
		}, nil
	}

	_, err = a.kernelManager.RemoveModules(removePackages, false)
	if err != nil {
		return nil, fmt.Errorf("failed to remove modules: %w", err)
	}

	err = a.updateAllPackagesDB(ctx)
	if err != nil {
		return nil, err
	}

	// Получаем обновлённую информацию о ядре после удаления модулей
	updatedKernel, err := a.kernelManager.GetCurrentKernel()
	if err != nil {
		return nil, fmt.Errorf("failed to get updated kernel info: %w", err)
	}

	data := map[string]interface{}{
		"message": fmt.Sprintf(lib.TN_("%d module removed successfully from kernel %s", "%d modules removed successfully from kernel %s", len(modulesToRemove)), len(modulesToRemove), latest.FullVersion),
		"kernel":  updatedKernel.ToMap(true, a.kernelManager),
	}

	return &reply.APIResponse{
		Data:  data,
		Error: false,
	}, nil
}

// formatKernelOutput форматирует вывод информации о ядрах
func (a *Actions) formatKernelOutput(kernels []*service.Info, full bool) []interface{} {
	var result []interface{}

	for _, kernel := range kernels {
		if full {
			result = append(result, kernel.ToMap(true, a.kernelManager))
		} else {
			result = append(result, kernel.ToMap(false, a.kernelManager))
		}
	}

	return result
}

// detectFlavourOrDefault возвращает указанный flavour или определяет автоматически
func (a *Actions) detectFlavourOrDefault(flavour string) (string, error) {
	if flavour != "" {
		return flavour, nil
	}
	return a.kernelManager.DetectCurrentFlavour()
}

// checkAtomicSystemRestriction проверяет ограничения для atomic систем
func (a *Actions) checkAtomicSystemRestriction(operation string, dryRun bool) error {
	if !lib.Env.IsAtomic {
		return nil
	}

	switch operation {
	case "install":
		return errors.New(lib.T_("Direct kernel installation is not supported on atomic systems. Use system image updates instead"))
	case "remove":
		return errors.New(lib.T_("Direct kernel removal is not supported on atomic systems. Use system image management instead"))
	case "update":
		return errors.New(lib.T_("Kernel updates are managed through system image updates on atomic systems"))
	}
	return nil
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
