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

package service

import (
	"apm/internal/common/app"
	_package "apm/internal/common/apt/package"
	"apm/internal/common/binding/apt"
	libApt "apm/internal/common/binding/apt/lib"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Info KernelInfo представляет информацию о ядре
type Info struct {
	PackageName      string    `json:"packageName"`
	Flavour          string    `json:"flavour"`
	Version          string    `json:"version"`
	VersionInstalled string    `json:"versionInstalled"`
	Release          string    `json:"release"`
	BuildTime        time.Time `json:"buildTime"`
	IsInstalled      bool      `json:"isInstalled"`
	IsRunning        bool      `json:"isRunning"`
	FullVersion      string    `json:"fullVersion"`
	AgeInDays        int       `json:"ageInDays"`
}

// FullKernelInfo полная информация о ядре с модулями
type FullKernelInfo struct {
	PackageName      string                `json:"packageName"`
	Flavour          string                `json:"flavour"`
	Version          string                `json:"version"`
	VersionInstalled string                `json:"versionInstalled"`
	Release          string                `json:"release"`
	FullVersion      string                `json:"fullVersion"`
	IsInstalled      bool                  `json:"isInstalled"`
	IsRunning        bool                  `json:"isRunning"`
	AgeInDays        int                   `json:"ageInDays"`
	BuildTime        string                `json:"buildTime"`
	InstalledModules []InstalledModuleInfo `json:"installedModules,omitempty"`
}

// ShortKernelInfo сокращенная информация о ядре
type ShortKernelInfo struct {
	Version          string `json:"version"`
	VersionInstalled string `json:"versionInstalled"`
	Flavour          string `json:"flavour"`
	FullVersion      string `json:"fullVersion"`
	IsInstalled      bool   `json:"isInstalled"`
	IsRunning        bool   `json:"isRunning"`
}

// InstalledModuleInfo информация об установленном модуле
type InstalledModuleInfo struct {
	Name        string `json:"name"`
	PackageName string `json:"packageName"`
}

// ToFull конвертирует Info в FullKernelInfo
func (info *Info) ToFull(manager *Manager) FullKernelInfo {
	result := FullKernelInfo{
		PackageName:      info.PackageName,
		Flavour:          info.Flavour,
		Version:          info.Version,
		VersionInstalled: info.VersionInstalled,
		Release:          info.Release,
		FullVersion:      info.FullVersion,
		IsInstalled:      info.IsInstalled,
		IsRunning:        info.IsRunning,
		AgeInDays:        info.AgeInDays,
		BuildTime:        info.BuildTime.Format(time.RFC3339),
	}

	if manager != nil {
		allModules, _ := manager.FindAvailableModules(info)
		var installedModules []InstalledModuleInfo
		for _, module := range allModules {
			if module.IsInstalled {
				installedModules = append(installedModules, InstalledModuleInfo{
					Name:        module.Name,
					PackageName: module.PackageName,
				})
			}
		}
		result.InstalledModules = installedModules
	}

	return result
}

// ToShort конвертирует Info в ShortKernelInfo
func (info *Info) ToShort() ShortKernelInfo {
	return ShortKernelInfo{
		Version:          info.Version,
		VersionInstalled: info.VersionInstalled,
		Flavour:          info.Flavour,
		FullVersion:      info.FullVersion,
		IsInstalled:      info.IsInstalled,
		IsRunning:        info.IsRunning,
	}
}

// ModuleInfo информация о модуле ядра
type ModuleInfo struct {
	Name        string `json:"name"`
	IsInstalled bool   `json:"isInstalled"`
	PackageName string `json:"packageName"`
}

// UpgradePreview показывает что будет происходить при обновлении ядра
type UpgradePreview struct {
	Changes         *libApt.PackageChanges `json:"changes"`
	SelectedModules []string               `json:"selectedModules"`
	MissingModules  []string               `json:"missingModules"`
}

// Manager KernelManager управляет операциями с ядрами
type Manager struct {
	dbService  *_package.PackageDBService
	aptActions *apt.Actions
}

// NewKernelManager создает новый KernelManager
func NewKernelManager(dbService *_package.PackageDBService, aptActions *apt.Actions) *Manager {
	return &Manager{
		dbService:  dbService,
		aptActions: aptActions,
	}
}

// SimulateRemoveKernel симулирует удаление указанного ядра
func (km *Manager) SimulateRemoveKernel(kernel *Info) (*libApt.PackageChanges, error) {
	var packagesToRemove []string

	// Добавляем само ядро
	packagesToRemove = append(packagesToRemove, kernel.PackageName)

	// Находим все установленные модули для данного ядра
	availableModules, err := km.FindAvailableModules(kernel)
	if err == nil {
		for _, moduleInfo := range availableModules {
			if moduleInfo.IsInstalled {
				packagesToRemove = append(packagesToRemove, moduleInfo.PackageName)
			}
		}
	}

	return km.aptActions.SimulateRemove(packagesToRemove, false, false)
}

// RemoveKernel удаляет указанное ядро
func (km *Manager) RemoveKernel(kernel *Info, purge bool) error {
	var packagesToRemove []string

	// Добавляем само ядро
	packagesToRemove = append(packagesToRemove, kernel.PackageName)

	// Находим все установленные модули для данного ядра
	availableModules, err := km.FindAvailableModules(kernel)
	if err == nil {
		for _, moduleInfo := range availableModules {
			if moduleInfo.IsInstalled {
				packagesToRemove = append(packagesToRemove, moduleInfo.PackageName)
			}
		}
	}

	return km.aptActions.RemovePackages(packagesToRemove, purge, false, nil)
}

// GetCurrentKernel возвращает информацию о текущем запущенном ядре
func (km *Manager) GetCurrentKernel(ctx context.Context) (*Info, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventKernelCurrent))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventKernelCurrent))

	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to get current kernel: %s"), err.Error())
	}

	release := strings.TrimSpace(string(output))

	tempKernel := parseKernelRelease(release)
	if tempKernel == nil {
		return nil, fmt.Errorf(app.T_("failed to parse kernel release: %s"), release)
	}

	// Получаем все установленные ядра через RPM
	installedKernels, err := km.ListInstalledKernelsFromRPM(ctx)
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to get installed kernels: %s"), err.Error())
	}

	// Ищем ядро с точной версией из uname
	for _, kernel := range installedKernels {
		if kernel.Flavour == tempKernel.Flavour &&
			kernel.Version == tempKernel.Version &&
			kernel.Release == tempKernel.Release {
			kernel.IsRunning = true
			kernel.IsInstalled = true

			km.enrichKernelInfoFromDB(kernel)

			return kernel, nil
		}
	}

	return nil, fmt.Errorf(app.T_("running kernel %s not found in installed packages"), release)
}

// GetDefaultKernel возвращает информацию о ядре по умолчанию (/boot/vmlinuz)
func (km *Manager) GetDefaultKernel() (*Info, error) {
	cmd := exec.Command("readlink", "/boot/vmlinuz")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to get default kernel: %s"), err.Error())
	}

	vmlinuz := strings.TrimSpace(string(output))
	release := strings.TrimPrefix(vmlinuz, "vmlinuz-")
	kernel := parseKernelRelease(release)
	if kernel != nil {
		km.enrichKernelInfoFromDB(kernel)
	}

	return kernel, nil
}

// ListKernels возвращает список доступных ядер для указанного flavour
func (km *Manager) ListKernels(ctx context.Context, flavour string) (kernels []*Info, err error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventKernelList))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventKernelList))

	filters := map[string]interface{}{
		"typePackage": int(_package.PackageTypeSystem),
	}

	if flavour != "" {
		filters["name"] = fmt.Sprintf("kernel-image-%s", flavour)
	} else {
		filters["name"] = "kernel-image-"
	}

	// Ищем в базе данных с сортировкой по версии
	packages, err := km.dbService.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to search kernel packages in database: %s"), err.Error())
	}

	currentKernel, _ := km.GetCurrentKernel(ctx)
	defaultKernel, _ := km.GetDefaultKernel()

	for _, pkg := range packages {
		// Пропускаем debuginfo пакеты
		if strings.Contains(pkg.Name, "debuginfo") {
			continue
		}

		kernel := km.ParseKernelPackageFromDB(pkg)
		if kernel == nil {
			continue
		}

		// Фильтруем по flavour если указан
		if flavour != "" && kernel.Flavour != flavour {
			continue
		}

		// Проверяем статус установки
		kernel.IsInstalled = km.checkInstallStatus(kernel, pkg.Installed)

		// Проверяем является ли текущим - сравниваем по PackageName, Version и Release
		if currentKernel != nil &&
			kernel.PackageName == currentKernel.PackageName &&
			kernel.Version == currentKernel.Version &&
			kernel.Release == currentKernel.Release {
			kernel.IsRunning = true
			// Обновляем currentKernel реальными данными из базы
			currentKernel.BuildTime = kernel.BuildTime
			currentKernel.AgeInDays = kernel.AgeInDays
			currentKernel.FullVersion = kernel.FullVersion
		}

		// Проверяем является ли по умолчанию - сравниваем по PackageName, Version и Release
		if defaultKernel != nil &&
			kernel.PackageName == defaultKernel.PackageName &&
			kernel.Version == defaultKernel.Version &&
			kernel.Release == defaultKernel.Release {
			defaultKernel.BuildTime = kernel.BuildTime
			defaultKernel.AgeInDays = kernel.AgeInDays
			defaultKernel.FullVersion = kernel.FullVersion
		}

		kernels = append(kernels, kernel)
	}

	// Сортируем по версии с учетом buildtime (новые сначала)
	sort.Slice(kernels, func(i, j int) bool {
		// Сначала сравниваем основную версию
		versionCmp := helper.CompareVersions(kernels[i].Version, kernels[j].Version)
		if versionCmp != 0 {
			return versionCmp > 0
		}
		// Если версии одинаковые, сравниваем по buildtime (новые первыми)
		return kernels[i].BuildTime.After(kernels[j].BuildTime)
	})

	return kernels, nil
}

// FindLatestKernel возвращает самое новое ядро для указанного flavour
func (km *Manager) FindLatestKernel(ctx context.Context, flavour string) (*Info, error) {
	kernels, err := km.ListKernels(ctx, flavour)
	if err != nil {
		return nil, err
	}

	if len(kernels) == 0 {
		return nil, fmt.Errorf(app.T_("no kernels found for flavour: %s"), flavour)
	}

	return kernels[0], nil
}

// FindAvailableModules возвращает список доступных модулей для ядра с информацией об установке
func (km *Manager) FindAvailableModules(kernel *Info) (modules []ModuleInfo, err error) {
	ctx := context.Background()

	// Ищем kernel-modules пакеты для указанного flavour
	likePattern := fmt.Sprintf("kernel-modules-%%-%s", kernel.Flavour)
	packages, err := km.dbService.SearchPackagesByNameLike(ctx, likePattern, false)
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to search kernel modules in database: %s"), err.Error())
	}

	for _, pkg := range packages {
		// Парсим имя модуля из пакета: kernel-modules-drm-6.12 -> drm
		parts := strings.Split(pkg.Name, "-")
		if len(parts) >= 3 {
			// kernel-modules-drm-6.12 -> drm (убираем kernel-modules и flavour)
			module := strings.Join(parts[2:len(parts)-1], "-")
			if module != "" {
				isInstalled := km.checkModuleInstallStatus(module, kernel.Flavour, pkg.Installed)

				moduleInfo := ModuleInfo{
					Name:        module,
					IsInstalled: isInstalled,
					PackageName: pkg.Name,
				}
				modules = append(modules, moduleInfo)
			}
		}
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name < modules[j].Name
	})

	return modules, nil
}

// SimulateUpgrade симулирует обновление до указанного ядра с модулями
func (km *Manager) SimulateUpgrade(kernel *Info, modules []string, includeHeaders bool) (preview *UpgradePreview, err error) {
	// Формируем список пакетов для установки
	installPackages := km.buildPackageList(kernel, modules, includeHeaders)

	// Симулируем установку через APT Actions
	changes, err := km.aptActions.SimulateInstall(installPackages)
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to simulate kernel upgrade: %s"), err.Error())
	}

	// Проверяем какие модули недоступны
	missingModules := km.findMissingModules(kernel, modules)

	preview = &UpgradePreview{
		Changes:         changes,
		SelectedModules: modules,
		MissingModules:  missingModules,
	}

	return preview, nil
}

// InstallKernel устанавливает ядро с модулями
func (km *Manager) InstallKernel(ctx context.Context, kernel *Info, modules []string, includeHeaders bool, dryRun bool) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventKernelInstall))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventKernelInstall))

	// Формируем список пакетов для установки
	installPackages := km.buildPackageList(kernel, modules, includeHeaders)

	if dryRun {
		// Используем APT Actions для симуляции
		_, err := km.aptActions.SimulateInstall(installPackages)
		return err
	}

	return km.aptActions.InstallPackages(installPackages, nil)
}

// InstallModules устанавливает или симулирует установку пакетов модулей
func (km *Manager) InstallModules(ctx context.Context, installPackages []string, dryRun bool) (*libApt.PackageChanges, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventKernelInstallMods))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventKernelInstallMods))
	if dryRun {
		return km.aptActions.SimulateInstall(installPackages)
	}

	err := km.aptActions.InstallPackages(installPackages, nil)
	return nil, err
}

// RemovePackages удаляет или симулирует удаление пакетов модулей/ядер
func (km *Manager) RemovePackages(ctx context.Context, removePackages []string, dryRun bool) (*libApt.PackageChanges, error) {
	event := reply.EventKernelRemove
	if dryRun {
		event = reply.EventKernelCheckRemove
	}

	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(event))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(event))

	if dryRun {
		return km.aptActions.SimulateRemove(removePackages, false, false)
	}

	err := km.aptActions.RemovePackages(removePackages, false, false, nil)
	return nil, err
}

// DetectCurrentFlavour определяет flavour текущего ядра
func (km *Manager) DetectCurrentFlavour(ctx context.Context) (string, error) {
	current, err := km.GetCurrentKernel(ctx)
	if err != nil {
		return "", err
	}

	if current == nil {
		return "", errors.New(app.T_("cannot detect current kernel"))
	}

	return current.Flavour, nil
}

// FindNextFlavours ищет доступные kernel-image пакеты >= указанной версии
func (km *Manager) FindNextFlavours(minVersion string) (flavours []string, err error) {
	ctx := context.Background()

	// Поиск всех kernel-image пакетов в базе данных
	filters := map[string]interface{}{
		"typePackage": int(_package.PackageTypeSystem),
		"name":        "kernel-image-",
	}
	packages, err := km.dbService.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to search kernels in database: %s"), err.Error())
	}

	// Структура для хранения информации о flavour и его максимальной версии
	type FlavourWithVersion struct {
		Flavour string
		Version string
	}

	flavourVersions := make(map[string]string)

	for _, pkg := range packages {
		if strings.Contains(pkg.Name, "debuginfo") {
			continue
		}

		kernel := km.ParseKernelPackageFromDB(pkg)
		if kernel == nil {
			continue
		}

		cmp := helper.CompareVersions(kernel.Version, minVersion)
		if cmp >= 0 {
			if currentVer, exists := flavourVersions[kernel.Flavour]; !exists || helper.CompareVersions(kernel.Version, currentVer) > 0 {
				flavourVersions[kernel.Flavour] = kernel.Version
			}
		}
	}

	var flavourList []FlavourWithVersion
	for flavour, version := range flavourVersions {
		flavourList = append(flavourList, FlavourWithVersion{
			Flavour: flavour,
			Version: version,
		})
	}

	// Сортируем по версии (старые сначала, чтобы первым был ближайший новый)
	sort.Slice(flavourList, func(i, j int) bool {
		return helper.CompareVersions(flavourList[i].Version, flavourList[j].Version) < 0
	})

	for _, item := range flavourList {
		flavours = append(flavours, item.Flavour)
	}

	return flavours, nil
}

// ValidateKernelRelease проверяет корректность строки release
func (km *Manager) ValidateKernelRelease(release string) bool {
	re := regexp.MustCompile(`^\d+\.\d+\.\d+-\w+-\w+$`)
	return re.MatchString(release)
}

// InheritModulesFromKernel наследует модули от указанного ядра
func (km *Manager) InheritModulesFromKernel(targetKernel *Info, sourceKernel *Info) ([]string, error) {
	if sourceKernel == nil {
		return nil, nil
	}

	sourceAvailableModules, err := km.FindAvailableModules(sourceKernel)
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to get available modules from source kernel: %s"), err.Error())
	}

	// Извлекаем только установленные модули из исходного ядра
	var sourceModules []string
	for _, moduleInfo := range sourceAvailableModules {
		if moduleInfo.IsInstalled {
			sourceModules = append(sourceModules, moduleInfo.Name)
		}
	}

	if len(sourceModules) == 0 {
		return nil, nil
	}

	// Проверяем какие из этих модулей доступны для целевого ядра
	availableModules, err := km.FindAvailableModules(targetKernel)
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to get available modules for target kernel: %s"), err.Error())
	}

	var inheritedModules []string
	for _, sourceModule := range sourceModules {
		for _, availableModule := range availableModules {
			if sourceModule == availableModule.Name {
				inheritedModules = append(inheritedModules, sourceModule)
				break
			}
		}
	}

	return inheritedModules, nil
}

// AutoSelectHeadersAndFirmware автоматически добавляет headers и модули от текущего ядра
func (km *Manager) AutoSelectHeadersAndFirmware(ctx context.Context, kernel *Info, includeHeaders bool) ([]string, error) {
	var additionalPackages []string

	// Добавляем headers если запрошены или уже установлены
	if includeHeaders {
		additionalPackages = append(additionalPackages,
			fmt.Sprintf("kernel-headers-%s", kernel.Flavour),
			fmt.Sprintf("kernel-headers-modules-%s", kernel.Flavour),
		)
	} else {
		// Проверяем если headers уже установлены - добавляем автоматически
		if km.areHeadersInstalled(kernel.Flavour) {
			additionalPackages = append(additionalPackages,
				fmt.Sprintf("kernel-headers-%s", kernel.Flavour),
				fmt.Sprintf("kernel-headers-modules-%s", kernel.Flavour),
			)
		}
	}

	// Автоматически добавляем модули на основе установленных модулей текущего ядра (как в bash скрипте)
	currentKernel, err := km.GetCurrentKernel(ctx)
	if err == nil && currentKernel != nil {
		inheritedModules, err := km.InheritModulesFromKernel(kernel, currentKernel)
		if err == nil && len(inheritedModules) > 0 {
			for _, moduleName := range inheritedModules {
				modulePackage := fmt.Sprintf("kernel-modules-%s-%s", moduleName, kernel.Flavour)
				additionalPackages = append(additionalPackages, modulePackage)
			}
		}
	}

	return additionalPackages, nil
}

// enrichKernelInfoFromDB обогащает информацию о ядре данными из базы
func (km *Manager) enrichKernelInfoFromDB(kernel *Info) {
	ctx := context.Background()

	filters := map[string]interface{}{
		"typePackage": int(_package.PackageTypeSystem),
		"name":        fmt.Sprintf("kernel-image-%s", kernel.Flavour),
	}
	packages, err := km.dbService.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
	if err != nil {
		return
	}

	var pkg _package.Package
	for _, p := range packages {
		release := extractReleaseFromVersion(p.VersionRaw)
		if p.Version == kernel.Version && release == kernel.Release {
			pkg = p
			break
		}
	}

	if pkg.Name == "" {
		return
	}

	// Обновляем данные из базы
	kernel.IsInstalled = km.checkInstallStatus(kernel, pkg.Installed)
	kernel.PackageName = pkg.Name

	// Используем VersionRaw для полной версии APT
	if pkg.VersionRaw != "" {
		kernel.FullVersion = fmt.Sprintf("%s#%s", pkg.Name, pkg.VersionRaw)
		kernel.AgeInDays, kernel.BuildTime = calculatePackageAgeAndTime(pkg.VersionRaw)
	}

	// Если база не смогла определить, проверяем через RPM
	if !kernel.IsInstalled && kernel.IsRunning {
		kernel.IsInstalled = km.isKernelInstalledRPM(kernel)
	}
}

// checkInstallStatus проверяет статус установки с fallback на RPM
func (km *Manager) checkInstallStatus(kernel *Info, aptInstalled bool) bool {
	rpmInstalled := km.isKernelInstalledRPM(kernel)

	// Если APT и RPM дают разные ответы, доверяем RPM
	if !aptInstalled && rpmInstalled {
		return true
	} else if aptInstalled && !rpmInstalled {
		return false
	}
	return aptInstalled
}

// checkModuleInstallStatus проверяет статус установки модуля с fallback на RPM
func (km *Manager) checkModuleInstallStatus(module, flavour string, aptInstalled bool) bool {
	rpmInstalled := km.isModuleInstalledRPM(module, flavour)

	if !aptInstalled && rpmInstalled {
		return true
	} else if aptInstalled && !rpmInstalled {
		return false
	}
	return aptInstalled
}

// buildPackageList формирует список пакетов для установки с полными версиями
func (km *Manager) buildPackageList(kernel *Info, modules []string, includeHeaders bool) []string {
	var installPackages []string

	// Добавляем само ядро - используем FullVersion если содержит #
	if strings.Contains(kernel.FullVersion, "#") {
		installPackages = append(installPackages, kernel.FullVersion)
	} else {
		installPackages = append(installPackages, kernel.PackageName)
	}

	// Добавляем модули с полными версиями
	for _, module := range modules {
		modulePackage := fmt.Sprintf("kernel-modules-%s-%s", module, kernel.Flavour)
		fullModulePackage := km.GetFullPackageNameForModule(modulePackage)
		installPackages = append(installPackages, fullModulePackage)
	}

	// Добавляем headers если нужно
	if includeHeaders {
		headerPackage := fmt.Sprintf("kernel-headers-%s", kernel.Flavour)
		moduleHeaderPackage := fmt.Sprintf("kernel-headers-modules-%s", kernel.Flavour)

		simpleHeaderPackage := km.GetSimplePackageNameForModule(headerPackage)
		simpleModuleHeaderPackage := km.GetSimplePackageNameForModule(moduleHeaderPackage)

		installPackages = append(installPackages, simpleHeaderPackage, simpleModuleHeaderPackage)
	}

	return installPackages
}

// findMissingModules находит недоступные модули
func (km *Manager) findMissingModules(kernel *Info, requestedModules []string) []string {
	availableModules, err := km.FindAvailableModules(kernel)
	if err != nil {
		return nil
	}

	var missingModules []string
	for _, module := range requestedModules {
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

	return missingModules
}

// areHeadersInstalled проверяет установлены ли headers для flavour
func (km *Manager) areHeadersInstalled(flavour string) bool {
	ctx := context.Background()
	headersPackage := fmt.Sprintf("kernel-headers-%s", flavour)

	pkg, err := km.dbService.GetPackageByName(ctx, headersPackage)
	if err != nil {
		return false
	}

	return pkg.Installed
}

// parseKernelRelease парсит строку типа "5.7.19-std-def-alt1"
func parseKernelRelease(release string) *Info {
	parts := strings.Split(release, "-")
	if len(parts) < 3 {
		return nil
	}

	version := parts[0]
	flavour := strings.Join(parts[1:len(parts)-1], "-")
	altRelease := parts[len(parts)-1]

	return &Info{
		Version:     version,
		Flavour:     flavour,
		Release:     altRelease,
		FullVersion: release,
		PackageName: fmt.Sprintf("kernel-image-%s", flavour),
	}
}

// ParseKernelPackageFromDB парсит информацию о пакете ядра из базы данных
func (km *Manager) ParseKernelPackageFromDB(pkg _package.Package) *Info {
	if !strings.HasPrefix(pkg.Name, "kernel-image-") {
		return nil
	}

	flavour := strings.TrimPrefix(pkg.Name, "kernel-image-")

	// Используем helper для правильного парсинга версии
	cleanVersion, err := helper.GetVersionFromAptCache(pkg.Version)
	if err != nil {
		// Fallback к простому парсингу если helper не смог
		versionParts := strings.Split(pkg.Version, "-")
		if len(versionParts) < 1 {
			return nil
		}
		cleanVersion = versionParts[0]
	}

	release := extractReleaseFromVersion(pkg.Version)

	// Если есть VersionRaw - используем его для полной версии APT, иначе формируем обычную
	var fullVersion string
	if pkg.VersionRaw != "" {
		fullVersion = fmt.Sprintf("%s#%s", pkg.Name, pkg.VersionRaw)
	} else {
		fullVersion = fmt.Sprintf("%s-%s-%s", cleanVersion, flavour, release)
	}

	kernel := &Info{
		PackageName:      pkg.Name,
		Flavour:          flavour,
		Version:          pkg.Version,
		VersionInstalled: pkg.VersionInstalled,
		Release:          release,
		FullVersion:      fullVersion,
	}

	// Вычисляем возраст пакета и время сборки из buildtime в полной версии
	kernel.AgeInDays, kernel.BuildTime = calculatePackageAgeAndTime(pkg.VersionRaw)

	return kernel
}

// extractReleaseFromVersion извлекает release из версии пакета
func extractReleaseFromVersion(version string) string {
	release := "alt1" // значение по умолчанию
	fullVer := version

	// Убираем epoch и buildtime
	if colonPos := strings.Index(fullVer, ":"); colonPos != -1 {
		fullVer = fullVer[:colonPos]
	}
	if atPos := strings.Index(fullVer, "@"); atPos != -1 {
		fullVer = fullVer[:atPos]
	}

	if altIdx := strings.Index(fullVer, "-alt"); altIdx != -1 {
		altPart := fullVer[altIdx+1:] // "alt1" или "alt1.something"
		if spaceIdx := strings.Index(altPart, " "); spaceIdx != -1 {
			altPart = altPart[:spaceIdx]
		}
		release = altPart
	}

	return release
}

// calculatePackageAgeAndTime вычисляет возраст пакета и время сборки из buildtime в версии APT
func calculatePackageAgeAndTime(version string) (int, time.Time) {
	if atPos := strings.LastIndex(version, "@"); atPos != -1 {
		buildTimeStr := version[atPos+1:]
		if spacePos := strings.Index(buildTimeStr, " "); spacePos != -1 {
			buildTimeStr = buildTimeStr[:spacePos]
		}

		if buildTime, err := strconv.ParseInt(buildTimeStr, 10, 64); err == nil {
			buildTimeUnix := time.Unix(buildTime, 0)
			ageInDays := int(time.Since(buildTimeUnix).Hours() / 24)
			if ageInDays < 0 {
				ageInDays = 0
			}
			return ageInDays, buildTimeUnix
		}
	}

	return 0, time.Time{}
}

// ListInstalledKernelsFromRPM возвращает все установленные ядра через прямой RPM запрос
// Использует apt биндинги для защиты от конкурентного доступа к rpmdb
func (km *Manager) ListInstalledKernelsFromRPM(ctx context.Context) ([]*Info, error) {
	rpmKernels, err := km.aptActions.RpmQueryKernelPackages(ctx)
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to query installed kernels: %s"), err.Error())
	}

	var kernels []*Info
	for _, rpmKernel := range rpmKernels {
		flavour := strings.TrimPrefix(rpmKernel.Name, "kernel-image-")
		if flavour == rpmKernel.Name {
			continue
		}

		buildTime := time.Unix(0, 0)
		if buildTimeInt, err := strconv.ParseInt(rpmKernel.BuildTime, 10, 64); err == nil {
			buildTime = time.Unix(buildTimeInt, 0)
		}

		kernel := &Info{
			PackageName:      rpmKernel.Name,
			Flavour:          flavour,
			Version:          rpmKernel.Version,
			VersionInstalled: rpmKernel.Version,
			Release:          rpmKernel.Release,
			BuildTime:        buildTime,
			AgeInDays:        int(time.Since(buildTime).Hours() / 24),
			IsInstalled:      true,
			IsRunning:        false,
			FullVersion:      fmt.Sprintf("%s=%s-%s", rpmKernel.Name, rpmKernel.Version, rpmKernel.Release),
		}

		kernels = append(kernels, kernel)
	}

	return kernels, nil
}

// isKernelInstalledRPM проверяет установлено ли ядро через RPM
// Использует apt биндинги для защиты от конкурентного доступа к rpmdb
func (km *Manager) isKernelInstalledRPM(kernel *Info) bool {
	installed, err := km.aptActions.RpmIsPackageInstalled(kernel.PackageName)
	if err != nil {
		return false
	}
	return installed
}

// isModuleInstalledRPM проверяет установлен ли модуль через RPM
// Использует apt биндинги для защиты от конкурентного доступа к rpmdb
func (km *Manager) isModuleInstalledRPM(moduleName, flavour string) bool {
	possibleNames := []string{
		fmt.Sprintf("kernel-modules-%s-%s", moduleName, flavour),
		fmt.Sprintf("kernel-module-%s-%s", moduleName, flavour),
		fmt.Sprintf("%s-kmod-%s", moduleName, flavour),
	}

	installed, err := km.aptActions.RpmIsAnyPackageInstalled(possibleNames)
	if err != nil {
		return false
	}
	return installed
}

// GetFullPackageNameForModule получает полное имя пакета модуля с версией из базы
func (km *Manager) GetFullPackageNameForModule(packageName string) string {
	ctx := context.Background()

	pkg, err := km.dbService.GetPackageByName(ctx, packageName)
	if err != nil {
		return packageName
	}

	if pkg.VersionRaw != "" {
		return fmt.Sprintf("%s#%s", packageName, pkg.VersionRaw)
	}

	return packageName
}

// GetSimplePackageNameForModule получает простое имя пакета для удаления без таймстемпов
func (km *Manager) GetSimplePackageNameForModule(packageName string) string {
	ctx := context.Background()

	pkg, err := km.dbService.GetPackageByName(ctx, packageName)
	if err != nil {
		return packageName
	}

	cleanVersion, err := helper.GetVersionFromAptCache(pkg.Version)
	if err != nil {
		versionParts := strings.Split(pkg.Version, "-")
		if len(versionParts) >= 1 {
			cleanVersion = versionParts[0]
		} else {
			cleanVersion = pkg.Version
		}
	}

	return fmt.Sprintf("%s=%s", packageName, cleanVersion)
}

// GetBackupKernel определяет backup ядро (с uptime >= 1 день) из /var/log/wtmp
func (km *Manager) GetBackupKernel(ctx context.Context) (*Info, error) {
	cmd := exec.CommandContext(ctx, "last", "-a", "reboot")
	cmd.Env = []string{"LC_ALL=C"}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf(app.T_("failed to get reboot history: %s"), err.Error())
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "system boot") && strings.Contains(line, "+") {
			fields := strings.Fields(line)
			for i, field := range fields {
				if i >= 2 && strings.Contains(field, "-") && strings.Contains(field, ".") {
					kernelRelease := field
					if strings.Contains(line, "+") {
						// Есть uptime >= 1 день
						backupKernel := parseKernelRelease(kernelRelease)
						if backupKernel != nil {
							km.enrichKernelInfoFromDB(backupKernel)
							return backupKernel, nil
						}
					}
					break
				}
			}
		}
	}

	return nil, nil
}

// GroupKernelsByFlavour группирует ядра по flavour и сортирует по версии
func (km *Manager) GroupKernelsByFlavour(kernels []*Info) map[string][]*Info {
	flavourGroups := make(map[string][]*Info)

	for _, kernel := range kernels {
		flavourGroups[kernel.Flavour] = append(flavourGroups[kernel.Flavour], kernel)
	}

	// Сортируем ядра внутри каждого flavour по версии (новые сначала)
	for flavour := range flavourGroups {
		sort.Slice(flavourGroups[flavour], func(i, j int) bool {
			versionCmp := helper.CompareVersions(flavourGroups[flavour][i].Version, flavourGroups[flavour][j].Version)
			if versionCmp != 0 {
				return versionCmp > 0
			}
			return flavourGroups[flavour][i].BuildTime.After(flavourGroups[flavour][j].BuildTime)
		})
	}

	return flavourGroups
}
