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
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	aptParser "altlinux.space/alt-atomic/apm/internal/common/apt"
	"altlinux.space/alt-atomic/apm/internal/common/helper"
	"altlinux.space/alt-atomic/apm/internal/common/reply"
	aptBinding "altlinux.space/alt-atomic/apm/pkg/apt"
	aptLib "altlinux.space/alt-atomic/apm/pkg/apt/lib"
)

type Packages struct {
	appConfig          *app.Config
	reporter           *reply.Reporter
	serviceAptDatabase *PackageDBService
	serviceAptBinding  *aptBinding.Actions
}

type aptConfigOverridesKey struct{}

type aptConfigOverridesValue struct {
	overrides map[string]string
}

// WithAptConfigOverrides binds APT configuration to one operation tree.
func WithAptConfigOverrides(ctx context.Context, overrides map[string]string) context.Context {
	return context.WithValue(ctx, aptConfigOverridesKey{}, aptConfigOverridesValue{
		overrides: maps.Clone(overrides),
	})
}

// AptConfigOverridesFromContext returns a copy of request-scoped APT configuration.
func AptConfigOverridesFromContext(ctx context.Context) (map[string]string, bool) {
	value, ok := ctx.Value(aptConfigOverridesKey{}).(aptConfigOverridesValue)
	if !ok {
		return nil, false
	}
	return maps.Clone(value.overrides), true
}

func New(serviceAptDatabase *PackageDBService, appConfig *app.Config, reporter *reply.Reporter) *Packages {
	return &Packages{
		appConfig:          appConfig,
		reporter:           reporter,
		serviceAptDatabase: serviceAptDatabase,
		serviceAptBinding:  aptBinding.NewActions(),
	}
}

func (a *Packages) aptBinding(ctx context.Context) *aptBinding.Actions {
	overrides, ok := AptConfigOverridesFromContext(ctx)
	if !ok {
		return a.serviceAptBinding
	}
	return aptBinding.NewActionsWithConfigOverrides(overrides)
}

func (a *Packages) Upgrade(ctx context.Context, downloadOnly bool) error {
	a.reporter.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemUpgrade))
	defer a.reporter.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemUpgrade))

	err := a.aptBinding(ctx).DistUpgrade(a.getHandler(ctx), downloadOnly)
	if err != nil {
		return err
	}

	return nil
}

// DownloadSource скачивает .src.rpm пакеты в директорию destDir
func (a *Packages) DownloadSource(ctx context.Context, packages []string, destDir string) ([]aptLib.SourcePackage, error) {
	a.reporter.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemWorking))
	defer a.reporter.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemWorking))

	return a.aptBinding(ctx).DownloadSourcePackages(packages, destDir, a.getHandler(ctx, len(packages)))
}

// InstallSourcePackages устанавливает .src.rpm файлы в сборочное дерево rpm
func (a *Packages) InstallSourcePackages(ctx context.Context, files []string) error {
	prefix := a.appConfig.ConfigManager.GetConfig().CommandPrefix
	return a.aptBinding(ctx).RpmInstallSourcePackages(ctx, prefix, files)
}

func (a *Packages) CheckAutoRemove(ctx context.Context) (packageChanges *aptLib.PackageChanges, err error) {
	a.reporter.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemCheck))
	defer a.reporter.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheck))

	packageChanges, err = a.aptBinding(ctx).SimulateAutoRemove()
	return
}

func (a *Packages) GetInfo(ctx context.Context, packageName string) (packageChanges *aptLib.PackageInfo, err error) {
	a.reporter.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemCheck))
	defer a.reporter.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheck))

	packageChanges, err = a.aptBinding(ctx).GetInfo(packageName)
	return
}

func (a *Packages) CheckUpgrade(ctx context.Context) (packageChanges *aptLib.PackageChanges, err error) {
	a.reporter.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemCheck))
	defer a.reporter.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheck))

	packageChanges, err = a.aptBinding(ctx).SimulateDistUpgrade()
	return
}

func (a *Packages) Update(ctx context.Context, noLock ...bool) ([]Package, error) {
	a.reporter.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemUpdate))
	defer a.reporter.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemUpdate))

	err := a.AptUpdate(ctx, noLock...)
	if err != nil {
		return nil, err
	}

	aptPackages, err := a.aptBinding(ctx).Search("", noLock...)
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
func (a *Packages) UpdateDBOnly(ctx context.Context, noLock ...bool) ([]Package, error) {
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
func (a *Packages) updateInstalledInfo(ctx context.Context, packages []Package, noLock ...bool) ([]Package, error) {
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
func (a *Packages) GetInstalledPackages(ctx context.Context, noLock ...bool) (map[string]string, error) {
	commandPrefix := a.appConfig.ConfigManager.GetConfig().CommandPrefix
	return a.aptBinding(ctx).RpmGetInstalledPackages(ctx, commandPrefix, noLock...)
}

// Plan симулирует транзакцию, система не меняется
func (a *Packages) Plan(ctx context.Context, spec aptBinding.TransactionSpec) (*aptLib.PackageChanges, error) {
	a.reporter.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemCheck))
	defer a.reporter.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheck))

	spec.Inspect = rpmFiles(spec)
	changes, err := a.aptBinding(ctx).Plan(spec)
	if err != nil {
		return nil, err
	}
	return changes, a.saveInspected(ctx, changes)
}

// Apply выполняет транзакцию: план, подтверждение через confirm, применение на одном открытии кеша.
func (a *Packages) Apply(ctx context.Context, spec aptBinding.TransactionSpec, confirm aptBinding.Confirm) (*aptLib.PackageChanges, error) {
	spec.Inspect = rpmFiles(spec)

	a.reporter.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemCheck))
	checking, working := true, false
	defer func() {
		if checking {
			a.reporter.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheck))
		}
		if working {
			a.reporter.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemWorking))
		}
	}()

	planned := func(changes *aptLib.PackageChanges) (bool, error) {
		checking = false
		a.reporter.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemCheck))

		if err := a.saveInspected(ctx, changes); err != nil {
			return false, err
		}
		if confirm != nil {
			if ok, err := confirm(changes); err != nil || !ok {
				return ok, err
			}
		}

		working = true
		a.reporter.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemWorking))
		return true, nil
	}

	total := len(spec.AptGetArgs) + len(spec.Install) + len(spec.Remove) + len(spec.Reinstall)
	return a.aptBinding(ctx).Apply(spec, planned, a.getHandler(ctx, total))
}

// DescribeChanges собирает карточки пакетов из базы по именам из плана
func (a *Packages) DescribeChanges(ctx context.Context, changes *aptLib.PackageChanges) ([]Package, error) {
	if changes == nil {
		return nil, nil
	}

	seen := make(map[string]bool)
	var names []string
	for _, list := range [][]string{
		changes.ExtraInstalled,
		changes.UpgradedPackages,
		changes.NewInstalledPackages,
		changes.RemovedPackages,
	} {
		for _, pkgName := range list {
			cleanName := helper.CleanPackageName(strings.TrimSpace(pkgName))
			if cleanName == "" || seen[cleanName] {
				continue
			}
			seen[cleanName] = true
			names = append(names, cleanName)
		}
	}
	if len(names) == 0 {
		return nil, nil
	}
	return a.serviceAptDatabase.GetPackagesByNames(ctx, names)
}

// rpmFiles возвращает пути локальных .rpm из запроса, их карточки читаются из кеша после плана
func rpmFiles(spec aptBinding.TransactionSpec) []string {
	var files []string
	for _, arg := range slices.Concat(spec.AptGetArgs, spec.Install, spec.Reinstall) {
		if path := strings.TrimSuffix(arg, "+"); aptParser.IsRegularFileAndIsPackage(path) {
			files = append(files, path)
		}
	}
	return files
}

// saveInspected кладёт в базу карточки локальных .rpm, иначе их нечем показать
func (a *Packages) saveInspected(ctx context.Context, changes *aptLib.PackageChanges) error {
	for _, info := range changes.Inspected {
		if err := a.saveRpmInfoToDatabase(ctx, info); err != nil {
			return err
		}
	}
	return nil
}

// RpmIsPackageInstalled проверяет установку пакета напрямую через rpm.
func (a *Packages) RpmIsPackageInstalled(packageName string) (bool, error) {
	return a.serviceAptBinding.RpmIsPackageInstalled(packageName)
}

func (a *Packages) AptUpdate(ctx context.Context, noLock ...bool) error {
	a.reporter.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemAptUpdate))
	defer a.reporter.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemAptUpdate))

	err := a.aptBinding(ctx).Update(a.getUpdateHandler(ctx), noLock...)
	if err == nil {
		a.touchListsStamp()
	}
	return err
}

// AptUpdateIfStale обновляет списки пакетов, только если последний успешный update был раньше, чем ttl назад
func (a *Packages) AptUpdateIfStale(ctx context.Context, ttl time.Duration, noLock ...bool) error {
	if ttl > 0 {
		if info, err := os.Stat(a.listsStampPath()); err == nil && time.Since(info.ModTime()) < ttl {
			app.Log.Debugf("Skipping package list update, last update was %s ago", time.Since(info.ModTime()).Round(time.Second))
			return nil
		}
	}

	return a.AptUpdate(ctx, noLock...)
}

// listsStampPath путь к файлу-отметке последнего успешного apt update
func (a *Packages) listsStampPath() string {
	return filepath.Join(filepath.Dir(a.appConfig.ConfigManager.GetConfig().PathDBSQLSystem), "lists-update.stamp")
}

// touchListsStamp обновляет отметку времени последнего успешного apt update
func (a *Packages) touchListsStamp() {
	stamp := a.listsStampPath()
	if err := app.EnsureDir(filepath.Dir(stamp)); err != nil {
		app.Log.Debugf("Failed to create lists stamp dir: %v", err)
		return
	}
	if err := os.WriteFile(stamp, nil, 0644); err != nil {
		app.Log.Debugf("Failed to write lists update stamp: %v", err)
	}
}

// saveRpmInfoToDatabase сохраняет PackageInfo в базу данных
func (a *Packages) saveRpmInfoToDatabase(ctx context.Context, ap *aptLib.PackageInfo) error {
	_, errFind := a.serviceAptDatabase.GetPackageByName(ctx, ap.Name)
	if errFind == nil {
		return nil
	}

	p := convertAptPackage(ap)
	p.Changelog = extractLastMessage(p.Changelog)

	if err := a.serviceAptDatabase.SaveSinglePackage(ctx, p); err != nil {
		return fmt.Errorf("error saving package to database: %w", err)
	}
	return nil
}
