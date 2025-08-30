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
	"apm/internal/common/binding/apt"
	"apm/internal/common/reply"
	"apm/internal/kernel/service"
	_package "apm/internal/system/package"
	"apm/lib"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Actions объединяет методы для выполнения системных действий.
type Actions struct {
	serviceAptDatabase *_package.PackageDBService
	kernelManager      *service.Manager
}

// NewActionsWithDeps создаёт новый экземпляр Actions с ручными управлением зависимостями
func NewActionsWithDeps(
	aptDB *_package.PackageDBService,
	kernelManager *service.Manager,
) *Actions {
	return &Actions{
		serviceAptDatabase: aptDB,
		kernelManager:      kernelManager,
	}
}

// NewActions создаёт новый экземпляр Actions.
func NewActions() *Actions {
	hostPackageDBSvc, err := _package.NewPackageDBService(lib.GetDB(true))
	if err != nil {
		lib.Log.Fatal(err)
	}

	// Создаём APT Actions для операций установки/удаления
	aptActions := apt.NewActions()

	// Создаём KernelManager с необходимыми зависимостями
	kernelManager := service.NewKernelManager(hostPackageDBSvc, aptActions)

	return &Actions{
		serviceAptDatabase: hostPackageDBSvc,
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

// GetKernelInfo возвращает информацию о указанном ядре или текущем если не указано
func (a *Actions) GetKernelInfo(ctx context.Context, version string) (*reply.APIResponse, error) {
	if version == "" {
		return a.GetCurrentKernel(ctx)
	}

	kernels, err := a.kernelManager.ListKernels("")
	if err != nil {
		return nil, fmt.Errorf("failed to list kernels: %w", err)
	}

	kernel := a.findKernelByVersion(version, kernels)
	if kernel != nil {
		data := map[string]interface{}{
			"message": lib.T_("Kernel information"),
			"kernel":  a.formatKernelOutput([]*service.Info{kernel}, true)[0],
		}

		return &reply.APIResponse{
			Data:  data,
			Error: false,
		}, nil
	}

	return nil, fmt.Errorf("kernel %s not found", version)
}

// InstallKernel устанавливает ядро с указанным flavour
func (a *Actions) InstallKernel(ctx context.Context, flavour string, modules []string, includeHeaders bool, dryRun bool, force bool) (*reply.APIResponse, error) {
	if err := a.checkAtomicSystemRestriction("install", dryRun); err != nil {
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

	if latest.IsInstalled && !force {
		allModules, _ := a.kernelManager.FindAvailableModules(latest)
		var installedModulesInfo []map[string]interface{}
		for _, module := range allModules {
			if module.IsInstalled {
				installedModulesInfo = append(installedModulesInfo, map[string]interface{}{
					"name":        module.Name,
					"packageName": module.PackageName,
				})
			}
		}

		kernelInfo := map[string]interface{}{
			"packageName":      latest.PackageName,
			"flavour":          latest.Flavour,
			"version":          latest.Version,
			"release":          latest.Release,
			"fullVersion":      latest.FullVersion,
			"isInstalled":      latest.IsInstalled,
			"isRunning":        latest.IsRunning,
			"ageInDays":        latest.AgeInDays,
			"buildTime":        latest.BuildTime.Format(time.RFC3339),
			"InstalledModules": installedModulesInfo,
		}

		message := fmt.Sprintf(lib.T_("Kernel %s is already installed"), latest.FullVersion)
		if !dryRun {
			message += lib.T_(". Use --force to reinstall")
		}
		return &reply.APIResponse{
			Data: map[string]interface{}{
				"message": message,
				"kernel":  kernelInfo,
			},
			Error: false,
		}, nil
	}

	preview, err := a.kernelManager.SimulateUpgrade(latest, modules, includeHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to simulate kernel installation: %w", err)
	}

	if len(preview.MissingModules) > 0 {
		return nil, fmt.Errorf("some modules are not available: %s", strings.Join(preview.MissingModules, ", "))
	}

	if dryRun {
		data := map[string]interface{}{
			"message": lib.T_("Installation preview"),
			"kernel":  latest,
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

	// После установки обновляем статус в базе данных пакетов (как в system/actions.go)
	if a.serviceAptDatabase != nil {
		// TODO сделать
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

// RemoveKernel удаляет указанное ядро
func (a *Actions) RemoveKernel(ctx context.Context, version string) (*reply.APIResponse, error) {
	if err := a.checkAtomicSystemRestriction("remove", false); err != nil {
		return nil, err
	}

	current, err := a.kernelManager.GetCurrentKernel()
	if err != nil {
		return nil, fmt.Errorf("failed to get current kernel: %w", err)
	}

	if current.FullVersion == version || current.Version == version {
		return nil, errors.New(lib.T_("Cannot remove currently running kernel"))
	}

	kernels, err := a.kernelManager.ListKernels("")
	if err != nil {
		return nil, fmt.Errorf("failed to list kernels: %w", err)
	}

	targetKernel := a.findKernelByVersion(version, kernels)
	if targetKernel == nil {
		return nil, fmt.Errorf("kernel %s not found", version)
	}

	// Проверяем статус ядра
	if !targetKernel.IsInstalled {
		return nil, fmt.Errorf("kernel %s is not installed", version)
	}

	err = a.kernelManager.RemoveKernel(targetKernel, false)
	if err != nil {
		return nil, fmt.Errorf("failed to remove kernel: %w", err)
	}

	data := map[string]interface{}{
		"message": fmt.Sprintf(lib.T_("Kernel %s removed successfully"), targetKernel.FullVersion),
		"kernel":  targetKernel,
	}

	return &reply.APIResponse{
		Data:  data,
		Error: false,
	}, nil
}

// UpdateKernel обновляет ядро до последней версии
func (a *Actions) UpdateKernel(ctx context.Context, flavour string, modules []string, includeHeaders bool, dryRun bool) (*reply.APIResponse, error) {
	var err error
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

	if latest.FullVersion == current.FullVersion {
		return &reply.APIResponse{
			Data: map[string]interface{}{
				"message": lib.T_("Kernel is already up to date"),
				"kernel":  current,
			},
			Error: false,
		}, nil
	}

	return a.InstallKernel(ctx, flavour, modules, includeHeaders, dryRun, true) // force=true для обновления
}

// CheckKernelUpdate проверяет наличие обновлений ядра
func (a *Actions) CheckKernelUpdate(ctx context.Context, flavour string) (*reply.APIResponse, error) {
	if err := a.checkAtomicSystemRestriction("update", false); err != nil {
		return nil, err
	}

	var err error
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

	updateAvailable := latest.FullVersion != current.FullVersion

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
		err := a.kernelManager.RemoveKernel(kernel, false)
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

	kernelInfo := map[string]interface{}{
		"packageName": latest.PackageName,
		"flavour":     latest.Flavour,
		"version":     latest.Version,
		"release":     latest.Release,
		"fullVersion": latest.FullVersion,
		"isInstalled": latest.IsInstalled,
		"isRunning":   latest.IsRunning,
		"ageInDays":   latest.AgeInDays,
		"buildTime":   latest.BuildTime.Format(time.RFC3339),
	}

	data := map[string]interface{}{
		"message": fmt.Sprintf(lib.TN_("%d module found", "%d modules found", len(modules)), len(modules)),
		"kernel":  kernelInfo,
		"modules": modules,
	}

	return &reply.APIResponse{
		Data:  data,
		Error: false,
	}, nil
}

// InstallKernelModules устанавливает модули ядра
func (a *Actions) InstallKernelModules(ctx context.Context, flavour string, modules []string) (*reply.APIResponse, error) {
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
		return nil, fmt.Errorf("modules not available: %s", strings.Join(missingModules, ", "))
	}

	err = a.kernelManager.InstallModules(latest, modules)
	if err != nil {
		return nil, fmt.Errorf("failed to install modules: %w", err)
	}

	data := map[string]interface{}{
		"message": fmt.Sprintf(lib.T_("Successfully installed %d modules for kernel %s"), len(modules), latest.FullVersion),
		"kernel":  latest,
		"modules": modules,
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
			allModules, _ := a.kernelManager.FindAvailableModules(kernel)

			// Фильтруем только установленные модули
			installedModules := make([]map[string]interface{}, 0)
			for _, module := range allModules {
				if module.IsInstalled {
					installedModules = append(installedModules, map[string]interface{}{
						"name":        module.Name,
						"packageName": module.PackageName,
					})
				}
			}

			fullInfo := map[string]interface{}{
				"packageName":      kernel.PackageName,
				"flavour":          kernel.Flavour,
				"version":          kernel.Version,
				"release":          kernel.Release,
				"fullVersion":      kernel.FullVersion,
				"isInstalled":      kernel.IsInstalled,
				"isRunning":        kernel.IsRunning,
				"ageInDays":        kernel.AgeInDays,
				"buildTime":        kernel.BuildTime.Format(time.RFC3339),
				"InstalledModules": installedModules,
			}
			result = append(result, fullInfo)
		} else {
			shortInfo := map[string]interface{}{
				"version":     kernel.Version,
				"flavour":     kernel.Flavour,
				"fullVersion": kernel.FullVersion,
				"isInstalled": kernel.IsInstalled,
				"isRunning":   kernel.IsRunning,
			}
			result = append(result, shortInfo)
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
	if !lib.Env.IsAtomic || dryRun {
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
