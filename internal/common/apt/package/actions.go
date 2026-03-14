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

package _package

import (
	"apm/internal/common/app"
	aptParser "apm/internal/common/apt"
	aptBinding "apm/internal/common/binding/apt"
	aptLib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
)

type Actions struct {
	appConfig          *app.Config
	serviceAptDatabase *PackageDBService
	serviceAptBinding  *aptBinding.Actions
}

func NewActions(serviceAptDatabase *PackageDBService, appConfig *app.Config) *Actions {
	return &Actions{
		appConfig:          appConfig,
		serviceAptDatabase: serviceAptDatabase,
		serviceAptBinding:  aptBinding.NewActions(),
	}
}

// SetAptConfigOverrides устанавливает переопределения конфигурации APT
func (a *Actions) SetAptConfigOverrides(overrides map[string]string) {
	a.serviceAptBinding.SetConfigOverrides(overrides)
}

// GetAptConfigOverrides возвращает текущие переопределения конфигурации APT
func (a *Actions) GetAptConfigOverrides() map[string]string {
	return a.serviceAptBinding.GetConfigOverrides()
}

// PrepareInstallPackages разбирает список пакетов с суффиксами +/- и возвращает два списка
func (a *Actions) PrepareInstallPackages(ctx context.Context, packages []string) (install []string, remove []string, err error) {
	for _, pkg := range packages {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" {
			continue
		}

		// Сначала проверяем, существует ли пакет с таким именем как есть
		existsAsIs := a.checkPackageExists(ctx, pkg)

		// Пакет существует с таким именем - добавляем на установку
		if existsAsIs {
			install = append(install, pkg)
			continue
		}

		if strings.HasSuffix(pkg, "+") {
			baseName := strings.TrimSuffix(pkg, "+")
			if baseName != "" {
				install = append(install, baseName)
			}
		} else if strings.HasSuffix(pkg, "-") {
			baseName := strings.TrimSuffix(pkg, "-")
			if baseName != "" {
				remove = append(remove, baseName)
			}
		} else {
			install = append(install, pkg)
		}
	}

	return install, remove, nil
}

// checkPackageExists проверяет существует ли пакет в базе данных
func (a *Actions) checkPackageExists(ctx context.Context, packageName string) bool {
	_, err := a.serviceAptDatabase.GetPackageByName(ctx, packageName)
	return err == nil
}

func (a *Actions) FindPackage(ctx context.Context, installed []string, removed []string, purge bool, depends bool, reinstall bool) ([]string, []string, []Package, *aptLib.PackageChanges, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemCheck))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheck))

	expandedInstall, expandedRemove, rpmFiles, packagesInfo, seenInfo, err := a.expandPackageLists(ctx, installed, removed)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if len(expandedInstall) == 0 && len(expandedRemove) == 0 {
		if len(installed) > 0 || len(removed) > 0 {
			return nil, nil, nil, nil, errors.New(app.T_("No packages found matching the specified patterns"))
		}
	}

	packageChanges, err := a.simulateChanges(ctx, expandedInstall, expandedRemove, rpmFiles, purge, depends, reinstall)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	packagesInfo, err = a.enrichPackagesInfo(ctx, packagesInfo, seenInfo, packageChanges)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return expandedInstall, expandedRemove, packagesInfo, packageChanges, nil
}

// expandPackageLists обрабатывает wildcard-пакеты и RPM-файлы, возвращая расширенные списки.
func (a *Actions) expandPackageLists(ctx context.Context, installed, removed []string) (
	expandedInstall, expandedRemove, rpmFiles []string, packagesInfo []Package, seenInfo map[string]bool, err error,
) {
	seenInfo = make(map[string]bool)
	seenNames := make(map[string]bool)

	processPackageList := func(packages []string, targetList *[]string, onlyInstalled bool) error {
		for _, original := range packages {
			if strings.Contains(original, "*") {
				like := strings.ReplaceAll(original, "*", "%")
				if strings.TrimSpace(like) != "" {
					matched, errSearch := a.serviceAptDatabase.SearchPackagesByNameLike(ctx, like, onlyInstalled)
					if errSearch != nil {
						return errSearch
					}
					for _, mp := range matched {
						if !seenInfo[mp.Name] {
							seenInfo[mp.Name] = true
							packagesInfo = append(packagesInfo, mp)
						}
						if !seenNames[mp.Name] {
							seenNames[mp.Name] = true
							*targetList = append(*targetList, mp.Name)
						}
					}
				}
			} else {
				if aptParser.IsRegularFileAndIsPackage(original) {
					rpmFiles = append(rpmFiles, original)
				}
				seenNames[original] = true
				*targetList = append(*targetList, original)
			}
		}
		return nil
	}

	// Обрабатываем пакеты на установку (ищем среди всех доступных)
	if err = processPackageList(installed, &expandedInstall, false); err != nil {
		return
	}

	// Обрабатываем пакеты на удаление (ищем только среди установленных)
	if err = processPackageList(removed, &expandedRemove, true); err != nil {
		return
	}

	return
}

// simulateChanges выполняет симуляцию изменений через APT binding.
func (a *Actions) simulateChanges(ctx context.Context, expandedInstall, expandedRemove, rpmFiles []string,
	purge, depends, reinstall bool,
) (*aptLib.PackageChanges, error) {
	if reinstall {
		return a.CheckReinstall(ctx, expandedInstall)
	}

	if len(rpmFiles) > 0 {
		packageChanges, rpmInfos, aptError := a.serviceAptBinding.SimulateChangeWithRpmInfo(expandedInstall, expandedRemove, purge, depends, rpmFiles)
		if aptError != nil {
			return nil, aptError
		}
		for _, rpmInfo := range rpmInfos {
			if err := a.saveRpmInfoToDatabase(ctx, rpmInfo); err != nil {
				return nil, err
			}
		}
		return packageChanges, nil
	}

	return a.serviceAptBinding.SimulateChange(expandedInstall, expandedRemove, purge, depends)
}

// enrichPackagesInfo добавляет информацию о пакетах из packageChanges.
func (a *Actions) enrichPackagesInfo(ctx context.Context, packagesInfo []Package, seenInfo map[string]bool,
	packageChanges *aptLib.PackageChanges,
) ([]Package, error) {
	if packageChanges == nil {
		return packagesInfo, nil
	}

	var namesToFetch []string
	for _, list := range [][]string{
		packageChanges.ExtraInstalled,
		packageChanges.UpgradedPackages,
		packageChanges.NewInstalledPackages,
		packageChanges.RemovedPackages,
	} {
		for _, pkgName := range list {
			cleanName := helper.CleanPackageName(strings.TrimSpace(pkgName))
			if cleanName == "" {
				continue
			}
			if !seenInfo[cleanName] {
				seenInfo[cleanName] = true
				namesToFetch = append(namesToFetch, cleanName)
			}
		}
	}

	if len(namesToFetch) > 0 {
		batchInfo, err := a.serviceAptDatabase.GetPackagesByNames(ctx, namesToFetch)
		if err != nil {
			return nil, err
		}
		packagesInfo = append(packagesInfo, batchInfo...)
	}

	return packagesInfo, nil
}

func (a *Actions) Install(ctx context.Context, packages []string, downloadOnly bool) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemWorking))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemWorking))

	err := a.serviceAptBinding.InstallPackages(packages, a.getHandler(ctx, len(packages)), downloadOnly)
	if err != nil {
		return err
	}

	return nil
}

func (a *Actions) CombineInstallRemovePackages(ctx context.Context, packagesInstall []string,
	packagesRemove []string, purge bool, depends bool, downloadOnly bool) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemWorking))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemWorking))

	err := a.serviceAptBinding.CombineInstallRemovePackages(
		packagesInstall,
		packagesRemove,
		a.getHandler(ctx, len(packagesInstall)+len(packagesRemove)),
		purge,
		depends,
		downloadOnly,
	)
	if err != nil {
		return err
	}

	return nil
}

func (a *Actions) Remove(ctx context.Context, packages []string, purge bool, depends bool) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemWorking))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemWorking))

	err := a.serviceAptBinding.RemovePackages(packages, purge, depends, a.getHandler(ctx, len(packages)))
	if err != nil {
		return err
	}

	return nil
}

func (a *Actions) Upgrade(ctx context.Context, downloadOnly bool) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemUpgrade))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemUpgrade))

	err := a.serviceAptBinding.DistUpgrade(a.getHandler(ctx), downloadOnly)
	if err != nil {
		return err
	}

	return nil
}

func (a *Actions) CheckInstall(ctx context.Context, packageName []string) (packageChanges *aptLib.PackageChanges, err error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemCheck))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheck))

	packageChanges, err = a.serviceAptBinding.SimulateInstall(packageName)
	return
}

func (a *Actions) CheckReinstall(ctx context.Context, packageName []string) (packageChanges *aptLib.PackageChanges, err error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemCheck))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheck))

	packageChanges, err = a.serviceAptBinding.SimulateReinstall(packageName)
	return
}

func (a *Actions) ReinstallPackages(ctx context.Context, packages []string) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemWorking))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemWorking))

	err := a.serviceAptBinding.ReinstallPackages(packages, a.getHandler(ctx, len(packages)))
	if err != nil {
		return err
	}

	return nil
}

func (a *Actions) CheckRemove(ctx context.Context, packageName []string, purge bool, depends bool) (packageChanges *aptLib.PackageChanges, err error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemCheck))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheck))

	packageChanges, err = a.serviceAptBinding.SimulateRemove(packageName, purge, depends)
	return
}

func (a *Actions) CheckAutoRemove(ctx context.Context) (packageChanges *aptLib.PackageChanges, err error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemCheck))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheck))

	packageChanges, err = a.serviceAptBinding.SimulateAutoRemove()
	return
}

func (a *Actions) GetInfo(ctx context.Context, packageName string) (packageChanges *aptLib.PackageInfo, err error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemCheck))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheck))

	packageChanges, err = a.serviceAptBinding.GetInfo(packageName)
	return
}

func (a *Actions) CheckUpgrade(ctx context.Context) (packageChanges *aptLib.PackageChanges, err error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemCheck))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheck))

	packageChanges, err = a.serviceAptBinding.SimulateDistUpgrade()
	return
}

func (a *Actions) Update(ctx context.Context, noLock ...bool) ([]Package, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemUpdate))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemUpdate))

	err := a.AptUpdate(ctx, noLock...)
	if err != nil {
		return nil, err
	}

	aptPackages, err := a.serviceAptBinding.Search("", noLock...)
	if err != nil {
		return nil, err
	}

	packages := make([]Package, len(aptPackages))
	var wg sync.WaitGroup
	chunkSize := max((len(aptPackages)+runtime.NumCPU()-1)/runtime.NumCPU(), 1)
	for start := 0; start < len(aptPackages); start += chunkSize {
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for i := start; i < end; i++ {
				packages[i] = convertAptPackage(&aptPackages[i])
				packages[i].Changelog = extractLastMessage(packages[i].Changelog)
			}
		}(start, min(start+chunkSize, len(aptPackages)))
	}
	wg.Wait()

	// @TODO Обновляем информацию о том, установлены ли пакеты локально, на самом деле об этом можно узнать из биндингов
	packages, err = a.updateInstalledInfo(ctx, packages, noLock...)
	if err != nil {
		return nil, fmt.Errorf(app.T_("Error updating information about installed packages: %w"), err)
	}

	err = a.serviceAptDatabase.SavePackagesToDB(ctx, packages)
	if err != nil {
		return nil, err
	}

	return packages, nil
}

// UpdateDBOnly обновляет статус установленных пакетов в БД без обновления репозиториев.
func (a *Actions) UpdateDBOnly(ctx context.Context, noLock ...bool) ([]Package, error) {
	packages, err := a.serviceAptDatabase.QueryHostImagePackages(ctx, nil, "", "", 0, 0)
	if err != nil {
		return nil, err
	}

	packages, err = a.updateInstalledInfo(ctx, packages, noLock...)
	if err != nil {
		return nil, fmt.Errorf(app.T_("Error updating information about installed packages: %w"), err)
	}

	err = a.serviceAptDatabase.SavePackagesToDB(ctx, packages)
	if err != nil {
		return nil, err
	}

	return packages, nil
}

// updateInstalledInfo обновляет срез пакетов, устанавливая поля Installed и InstalledVersion, если пакет найден в системе.
func (a *Actions) updateInstalledInfo(ctx context.Context, packages []Package, noLock ...bool) ([]Package, error) {
	installed, err := a.GetInstalledPackages(ctx, noLock...)
	if err != nil {
		return nil, err
	}

	for i, pkg := range packages {
		if version, found := installed[pkg.Name]; found {
			packages[i].Installed = true
			packages[i].VersionInstalled = version
		}
	}

	return packages, nil
}

// GetInstalledPackages возвращает карту, где ключ – имя пакета, а значение – его установленная версия.
func (a *Actions) GetInstalledPackages(ctx context.Context, noLock ...bool) (map[string]string, error) {
	commandPrefix := a.appConfig.ConfigManager.GetConfig().CommandPrefix
	return a.serviceAptBinding.RpmGetInstalledPackages(ctx, commandPrefix, noLock...)
}

func (a *Actions) AptUpdate(ctx context.Context, noLock ...bool) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemAptUpdate))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemAptUpdate))

	return a.serviceAptBinding.Update(a.getUpdateHandler(ctx), noLock...)
}

// saveRpmInfoToDatabase сохраняет PackageInfo в базу данных
func (a *Actions) saveRpmInfoToDatabase(ctx context.Context, ap *aptLib.PackageInfo) error {
	_, errFind := a.serviceAptDatabase.GetPackageByName(ctx, ap.Name)
	if errFind == nil {
		return nil
	}

	p := convertAptPackage(ap)
	p.Changelog = extractLastMessage(p.Changelog)

	// Создаем слайс с одним пакетом
	packages := []Package{p}

	// Обновляем информацию об установке
	var err error
	packages, err = a.updateInstalledInfo(ctx, packages)
	if err != nil {
		return fmt.Errorf("error updating installed info: %w", err)
	}

	// Сохраняем один пакет в базу данных (не очищая остальные)
	err = a.serviceAptDatabase.SaveSinglePackage(ctx, packages[0])
	if err != nil {
		return fmt.Errorf("error saving package to database: %w", err)
	}

	return nil
}
