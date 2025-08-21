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

package apt

import (
	"apm/internal/common/apt"
	"apm/internal/common/binding/apt/lib"
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
	PackageName string    `json:"packageName"`
	Flavour     string    `json:"flavour"`
	Version     string    `json:"version"`
	Release     string    `json:"release"`
	BuildTime   time.Time `json:"buildTime"`
	IsInstalled bool      `json:"isInstalled"`
	IsRunning   bool      `json:"isRunning"`
	IsDefault   bool      `json:"isDefault"`
	FullVersion string    `json:"fullVersion"`
	AgeInDays   int       `json:"ageInDays"`
	Modules     []string  `json:"modules"`
}

// UpgradePreview показывает что будет происходить при обновлении ядра
type UpgradePreview struct {
	KernelInfo      *Info               `json:"kernelInfo"`
	Changes         *lib.PackageChanges `json:"changes"`
	SelectedModules []string            `json:"selectedModules"`
	MissingModules  []string            `json:"missingModules"`
	DownloadSize    uint64              `json:"downloadSize"`
	InstallSize     uint64              `json:"installSize"`
}

// Manager KernelManager управляет операциями с ядрами
type Manager struct {
	cache *lib.Cache
}

// NewKernelManager создает новый KernelManager
func NewKernelManager(cache *lib.Cache) *Manager {
	return &Manager{cache: cache}
}

// operationWrapper обёртка для всех операций с APT (как в actions.go)
func (km *Manager) operationWrapper(fn func() error) error {
	lib.StartOperation()
	defer lib.EndOperation()

	logs := make([]string, 0, 256)
	lib.SetLogHandler(func(msg string) { logs = append(logs, msg) })
	lib.CaptureStdIO(true)

	// Выполняем основную функцию
	err := fn()

	// Очищаем и анализируем ошибки
	lib.CaptureStdIO(false)
	lib.SetLogHandler(nil)

	return km.checkAnyError(logs, err)
}

// checkAnyError анализ всех ошибок, включает в себя stdout из apt-lib
func (km *Manager) checkAnyError(logs []string, err error) error {
	aptErrors := apt.ErrorLinesAnalyseAll(logs)
	for _, errApr := range aptErrors {
		return errApr
	}

	if err == nil {
		return nil
	}

	if msg := strings.TrimSpace(err.Error()); msg != "" {
		lines := strings.Split(msg, "\n")
		if m := apt.ErrorLinesAnalise(lines); m != nil {
			// Если это ошибка с провайдерами, захватываем весь список
			if m.Entry.Code == apt.ErrMultiInstallProvidersSelect && len(lines) > 1 {
				var providers []string
				for i := 1; i < len(lines); i++ {
					line := strings.TrimSpace(lines[i])
					if line != "" && !strings.HasPrefix(line, "You should") {
						providers = append(providers, line)
					}
				}
				if len(providers) > 0 {
					m.Params = append(m.Params, strings.Join(providers, "\n"))
				}
			}
			return m
		}
		if m := apt.CheckError(msg); m != nil {
			return m
		}
	}

	return err
}

// GetCurrentKernel возвращает информацию о текущем запущенном ядре
func (km *Manager) GetCurrentKernel() (*Info, error) {
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get current kernel: %w", err)
	}

	release := strings.TrimSpace(string(output))
	kernel := parseKernelRelease(release)
	if kernel != nil {
		kernel.IsRunning = true
	}

	return kernel, nil
}

// GetDefaultKernel возвращает информацию о ядре по умолчанию (/boot/vmlinuz)
func (km *Manager) GetDefaultKernel() (*Info, error) {
	cmd := exec.Command("readlink", "/boot/vmlinuz")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get default kernel: %w", err)
	}

	vmlinuz := strings.TrimSpace(string(output))
	release := strings.TrimPrefix(vmlinuz, "vmlinuz-")
	kernel := parseKernelRelease(release)
	if kernel != nil {
		kernel.IsDefault = true
	}

	return kernel, nil
}

// ListKernels возвращает список доступных ядер для указанного flavour  
func (km *Manager) ListKernels(flavour string) (kernels []*Info, err error) {
	err = km.operationWrapper(func() error {
		pattern := fmt.Sprintf("kernel-image-%s", flavour)
		if flavour == "" {
			pattern = "kernel-image-"
		}

		packages, e := km.cache.SearchPackages(pattern)
		if e != nil {
			return fmt.Errorf("failed to search kernel packages: %w", e)
		}

		currentKernel, _ := km.GetCurrentKernel()
		defaultKernel, _ := km.GetDefaultKernel()

		for _, pkg := range packages {
			// Пропускаем debuginfo пакеты
			if strings.Contains(pkg.Name, "debuginfo") {
				continue
			}

			kernel := parseKernelPackage(pkg)
			if kernel == nil {
				continue
			}

			// Проверяем статус установки
			kernel.IsInstalled = pkg.State != 0

			// Проверяем является ли текущим
			if currentKernel != nil && kernel.FullVersion == currentKernel.FullVersion {
				kernel.IsRunning = true
			}

			// Проверяем является ли по умолчанию
			if defaultKernel != nil && kernel.FullVersion == defaultKernel.FullVersion {
				kernel.IsDefault = true
			}

			kernels = append(kernels, kernel)
		}

		// Сортируем по версии (новые сначала)
		sort.Slice(kernels, func(i, j int) bool {
			return compareVersions(kernels[i].Version, kernels[j].Version) > 0
		})

		return nil
	})
	return
}

// FindLatestKernel возвращает самое новое ядро для указанного flavour
func (km *Manager) FindLatestKernel(flavour string) (*Info, error) {
	kernels, err := km.ListKernels(flavour)
	if err != nil {
		return nil, err
	}

	if len(kernels) == 0 {
		return nil, fmt.Errorf("no kernels found for flavour: %s", flavour)
	}

	return kernels[0], nil
}

// FindAvailableModules возвращает список доступных модулей для ядра
func (km *Manager) FindAvailableModules(kernel *Info) (modules []string, err error) {
	err = km.operationWrapper(func() error {
		pattern := fmt.Sprintf("kernel-modules-*-%s", kernel.Flavour)
		packages, e := km.cache.SearchPackages(pattern)
		if e != nil {
			return fmt.Errorf("failed to search kernel modules: %w", e)
		}

		for _, pkg := range packages {
			if strings.Contains(pkg.Name, kernel.Release) {
				parts := strings.Split(pkg.Name, "-")
				if len(parts) >= 3 {
					module := strings.Join(parts[2:len(parts)-2], "-") // убираем kernel-modules и flavour
					if module != "" {
						modules = append(modules, module)
					}
				}
			}
		}

		sort.Strings(modules)
		return nil
	})
	return
}

// SimulateUpgrade симулирует обновление до указанного ядра с модулями
func (km *Manager) SimulateUpgrade(kernel *Info, modules []string, includeHeaders bool) (preview *UpgradePreview, err error) {
	err = km.operationWrapper(func() error {
		var installPackages []string

		installPackages = append(installPackages, kernel.PackageName)

		// Добавляем модули
		for _, module := range modules {
			modulePackage := fmt.Sprintf("kernel-modules-%s-%s", module, kernel.Flavour)
			installPackages = append(installPackages, modulePackage)
		}

		if includeHeaders {
			installPackages = append(installPackages, fmt.Sprintf("kernel-headers-%s", kernel.Flavour))
			installPackages = append(installPackages, fmt.Sprintf("kernel-headers-modules-%s", kernel.Flavour))
		}

		// Симулируем установку
		changes, e := km.cache.SimulateInstall(installPackages)
		if e != nil {
			return fmt.Errorf("failed to simulate kernel upgrade: %w", e)
		}

		// Проверяем какие модули недоступны
		availableModules, _ := km.FindAvailableModules(kernel)
		var missingModules []string
		for _, module := range modules {
			found := false
			for _, available := range availableModules {
				if module == available {
					found = true
					break
				}
			}
			if !found {
				missingModules = append(missingModules, module)
			}
		}

		preview = &UpgradePreview{
			KernelInfo:      kernel,
			Changes:         changes,
			SelectedModules: modules,
			MissingModules:  missingModules,
			DownloadSize:    changes.DownloadSize,
			InstallSize:     changes.InstallSize,
		}

		return nil
	})
	return
}

// InstallKernel устанавливает ядро с модулями
func (km *Manager) InstallKernel(kernel *Info, modules []string, includeHeaders bool, dryRun bool) error {
	return km.operationWrapper(func() error {
		var installPackages []string

		// Добавляем само ядро
		installPackages = append(installPackages, kernel.PackageName)

		for _, module := range modules {
			modulePackage := fmt.Sprintf("kernel-modules-%s-%s", module, kernel.Flavour)
			installPackages = append(installPackages, modulePackage)
		}

		if includeHeaders {
			installPackages = append(installPackages, fmt.Sprintf("kernel-headers-%s", kernel.Flavour))
			installPackages = append(installPackages, fmt.Sprintf("kernel-headers-modules-%s", kernel.Flavour))
		}

		if dryRun {
			_, err := km.cache.SimulateInstall(installPackages)
			return err
		}

		// TODO: Реальная установка через PackageManager
		// Пока что возвращаем ошибку что функция не реализована
		return fmt.Errorf("actual installation not implemented yet - use dryRun mode")
	})
}

// GetInstalledModules возвращает установленные модули для указанного ядра
func (km *Manager) GetInstalledModules(kernel *Info) (modules []string, err error) {
	err = km.operationWrapper(func() error {
		pattern := fmt.Sprintf("kernel-modules-*-%s", kernel.Flavour)
		packages, e := km.cache.SearchPackages(pattern)
		if e != nil {
			return fmt.Errorf("failed to search installed modules: %w", e)
		}

		for _, pkg := range packages {
			if pkg.State != 0 && strings.Contains(pkg.Name, kernel.Release) {
				parts := strings.Split(pkg.Name, "-")
				if len(parts) >= 3 {
					module := strings.Join(parts[2:len(parts)-2], "-")
					if module != "" {
						modules = append(modules, module)
					}
				}
			}
		}

		sort.Strings(modules)
		return nil
	})
	return
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
		PackageName: fmt.Sprintf("kernel-image-%s", release),
	}
}

// parseKernelPackage парсит информацию о пакете ядра
func parseKernelPackage(pkg lib.PackageInfo) *Info {
	if !strings.HasPrefix(pkg.Name, "kernel-image-") {
		return nil
	}

	flavour := strings.TrimPrefix(pkg.Name, "kernel-image-")

	// Версия пакета содержит version-release
	versionParts := strings.Split(pkg.Version, "-")
	if len(versionParts) < 2 {
		return nil
	}

	version := versionParts[0]
	release := strings.Join(versionParts[1:], "-")
	fullVersion := fmt.Sprintf("%s-%s-%s", version, flavour, release)

	kernel := &Info{
		PackageName: pkg.Name,
		Flavour:     flavour,
		Version:     version,
		Release:     release,
		FullVersion: fullVersion,
	}

	// Вычисляем возраст пакета (упрощенно)
	kernel.AgeInDays = 0

	return kernel
}

// compareVersions сравнивает две версии (returns: 1 if a > b, -1 if a < b, 0 if equal)
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		aVal := 0
		bVal := 0

		if i < len(aParts) {
			if val, err := strconv.Atoi(aParts[i]); err == nil {
				aVal = val
			}
		}

		if i < len(bParts) {
			if val, err := strconv.Atoi(bParts[i]); err == nil {
				bVal = val
			}
		}

		if aVal > bVal {
			return 1
		}
		if aVal < bVal {
			return -1
		}
	}

	return 0
}

// DetectCurrentFlavour определяет flavour текущего ядра
func (km *Manager) DetectCurrentFlavour() (string, error) {
	current, err := km.GetCurrentKernel()
	if err != nil {
		return "", err
	}

	if current == nil {
		return "", fmt.Errorf("cannot detect current kernel")
	}

	return current.Flavour, nil
}

// FindNextFlavours ищет доступные flavour'ы новее указанной версии
func (km *Manager) FindNextFlavours(minVersion string) (flavours []string, err error) {
	err = km.operationWrapper(func() error {
		// Поиск всех kernel-image пакетов
		packages, e := km.cache.SearchPackages("kernel-image-")
		if e != nil {
			return fmt.Errorf("failed to search kernels: %w", e)
		}

		flavourVersions := make(map[string]string)

		for _, pkg := range packages {
			kernel := parseKernelPackage(pkg)
			if kernel == nil {
				continue
			}

			// Проверяем что версия больше минимальной
			if compareVersions(kernel.Version, minVersion) > 0 {
				if currentVer, exists := flavourVersions[kernel.Flavour]; !exists || compareVersions(kernel.Version, currentVer) > 0 {
					flavourVersions[kernel.Flavour] = kernel.Version
				}
			}
		}

		for flavour := range flavourVersions {
			flavours = append(flavours, flavour)
		}

		sort.Strings(flavours)
		return nil
	})
	return
}

// ValidateKernelRelease проверяет корректность строки release
func (km *Manager) ValidateKernelRelease(release string) bool {
	re := regexp.MustCompile(`^\d+\.\d+\.\d+-\w+-\w+$`)
	return re.MatchString(release)
}
