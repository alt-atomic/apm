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
	"apm/internal/common/appstream"
	aptParser "apm/internal/common/apt"
	aptBinding "apm/internal/common/binding/apt"
	aptLib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"apm/lib"
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// syncAptMutex защищает операции apt-get от дублированного вызова
var syncAptMutex sync.Mutex

type Actions struct {
	appStream          *appstream.SwCatService
	serviceAptDatabase *PackageDBService
	serviceStplr       *StplrService
}

func NewActions(serviceAptDatabase *PackageDBService, serviceStplr *StplrService) *Actions {
	return &Actions{
		appStream:          appstream.NewSwCatService("/usr/share/swcatalog/xml"),
		serviceAptDatabase: serviceAptDatabase,
		serviceStplr:       serviceStplr,
	}
}

// Package описывает структуру для хранения информации о пакете.
type Package struct {
	Name             string               `json:"name"`
	Section          string               `json:"section"`
	InstalledSize    int                  `json:"installedSize"`
	Maintainer       string               `json:"maintainer"`
	Version          string               `json:"version"`
	VersionInstalled string               `json:"versionInstalled"`
	Depends          []string             `json:"depends"`
	Provides         []string             `json:"provides"`
	Size             int                  `json:"size"`
	Filename         string               `json:"filename"`
	Description      string               `json:"description"`
	AppStream        *appstream.Component `json:"appStream"`
	Changelog        string               `json:"lastChangelog"`
	Installed        bool                 `json:"installed"`
	TypePackage      int                  `json:"typePackage"`
}

func (a *Actions) FindPackage(ctx context.Context, req []string) ([]string, []Package, error) {
	var packageNames []string
	var packagesInfo []Package

	seenNames := make(map[string]bool)
	seenInfo := make(map[string]bool)

	for _, original := range req {
		if info, err := a.serviceAptDatabase.GetPackageByName(ctx, original); err == nil {
			if !seenInfo[info.Name] {
				seenInfo[info.Name] = true
				packagesInfo = append(packagesInfo, info)
			}
			if info.TypePackage == int(PackageTypeStplr) {
				rpmPath, errStplr := a.serviceStplr.PreInstall(ctx, original)
				if errStplr != nil {
					return nil, nil, errStplr
				}
				if !seenNames[rpmPath] {
					seenNames[rpmPath] = true
					packageNames = append(packageNames, rpmPath)
				}
			} else {
				if !seenNames[original] {
					seenNames[original] = true
					packageNames = append(packageNames, original)
				}
			}
			continue
		}

		if strings.Contains(original, "*") {
			like := strings.ReplaceAll(original, "*", "%")
			if strings.TrimSpace(like) != "" {
				matched, errSearch := a.serviceAptDatabase.SearchPackagesByNameLike(ctx, like, false)
				if errSearch != nil {
					return nil, nil, errSearch
				}
				for _, mp := range matched {
					if !seenInfo[mp.Name] {
						seenInfo[mp.Name] = true
						packagesInfo = append(packagesInfo, mp)
					}
					if !seenNames[mp.Name] {
						seenNames[mp.Name] = true
						packageNames = append(packageNames, mp.Name)
					}
				}
			}
		}

		filters := map[string]interface{}{"provides": original}
		alts, errFind := a.serviceAptDatabase.QueryHostImagePackages(ctx, filters, "", "", 100, 0)
		if errFind != nil {
			return nil, nil, errFind
		}
		if len(alts) == 0 {
			return nil, nil, fmt.Errorf(lib.T_("Failed to retrieve information about the package %s"), original)
		}
		for _, alt := range alts {
			if infoAlt, errAlt := a.serviceAptDatabase.GetPackageByName(ctx, alt.Name); errAlt == nil {
				if !seenInfo[infoAlt.Name] {
					seenInfo[infoAlt.Name] = true
					packagesInfo = append(packagesInfo, infoAlt)
				}
			}
			if !seenNames[alt.Name] {
				seenNames[alt.Name] = true
				packageNames = append(packageNames, alt.Name)
			}
		}
	}

	return packageNames, packagesInfo, nil
}

func (a *Actions) getHandler(ctx context.Context) func(pkg string, event aptLib.ProgressType, cur, total uint64) {
	return func(pkg string, event aptLib.ProgressType, cur, total uint64) {
		switch event {
		case aptLib.CallbackDownloadStart:
			ev := fmt.Sprintf("system.downloadProgress-%s", pkg)
			reply.CreateEventNotification(ctx, reply.StateBefore,
				reply.WithEventName(ev),
				reply.WithEventView(fmt.Sprintf(lib.T_("Downloading: %s"), pkg)),
			)
		case aptLib.CallbackDownloadProgress:
			if total > 0 {
				percent := int((cur * 100) / total)
				ev := fmt.Sprintf("system.downloadProgress-%s", pkg)
				reply.CreateEventNotification(ctx, reply.StateBefore,
					reply.WithEventName(ev),
					reply.WithProgress(true),
					reply.WithProgressPercent(float64(percent)),
					reply.WithEventView(fmt.Sprintf(lib.T_("Downloading: %s"), pkg)),
				)
			}
		case aptLib.CallbackDownloadStop:
			ev := fmt.Sprintf("system.downloadProgress-%s", pkg)
			reply.CreateEventNotification(ctx, reply.StateAfter,
				reply.WithEventName(ev),
				reply.WithProgress(true),
				reply.WithProgressDoneText(pkg),
				reply.WithProgressPercent(100),
			)
		case aptLib.CallbackInstallStart:
			ev := fmt.Sprintf("system.installProgress-%s", pkg)
			reply.CreateEventNotification(ctx, reply.StateBefore,
				reply.WithEventName(ev),
				reply.WithEventView(fmt.Sprintf(lib.T_("Install: %s"), pkg)),
			)
		case aptLib.CallbackInstallProgress:
			ev := fmt.Sprintf("system.installProgress-%s", pkg)
			if total > 0 {
				percent := int((cur * 100) / total)
				reply.CreateEventNotification(ctx, reply.StateBefore,
					reply.WithEventName(ev),
					reply.WithProgress(true),
					reply.WithProgressPercent(float64(percent)),
					reply.WithEventView(fmt.Sprintf(lib.T_("Install progress: %s"), pkg)),
				)
			}
		case aptLib.CallbackInstallStop:
			ev := fmt.Sprintf("system.installProgress-%s", pkg)
			reply.CreateEventNotification(ctx, reply.StateAfter,
				reply.WithEventName(ev),
				reply.WithProgress(true),
				reply.WithProgressDoneText(pkg),
				reply.WithProgressPercent(100),
			)
		}
	}
}

func (a *Actions) Install(ctx context.Context, packages []string) error {
	syncAptMutex.Lock()
	defer syncAptMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Working"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Working"))

	aptService := aptBinding.NewActions()
	err := aptService.InstallPackages(packages, a.getHandler(ctx))
	if err != nil {
		return err
	}

	return nil
}

func (a *Actions) Remove(ctx context.Context, packages []string, purge bool) error {
	syncAptMutex.Lock()
	defer syncAptMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Working"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Working"))

	aptService := aptBinding.NewActions()
	err := aptService.RemovePackages(packages, purge, a.getHandler(ctx))
	if err != nil {
		return err
	}

	return nil
}

func (a *Actions) Upgrade(ctx context.Context) error {
	syncAptMutex.Lock()
	defer syncAptMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Upgrade"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Upgrade"))

	aptService := aptBinding.NewActions()
	err := aptService.DistUpgrade(a.getHandler(ctx))
	if err != nil {
		return err
	}

	return nil
}

func (a *Actions) CheckInstall(ctx context.Context, packageName []string) (packageChanges *aptLib.PackageChanges, err error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Check"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Check"))

	aptService := aptBinding.NewActions()
	packageChanges, err = aptService.SimulateInstall(packageName)
	return
}

func (a *Actions) CheckRemove(ctx context.Context, packageName []string) (packageChanges *aptLib.PackageChanges, err error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Check"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Check"))

	aptService := aptBinding.NewActions()
	packageChanges, err = aptService.SimulateRemove(packageName)
	return
}

func (a *Actions) CheckUpgrade(ctx context.Context) (packageChanges *aptLib.PackageChanges, err error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Check"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Check"))

	aptService := aptBinding.NewActions()
	packageChanges, err = aptService.SimulateDistUpgrade()
	return
}

func (a *Actions) Update(ctx context.Context) ([]Package, error) {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.Update"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.Update"))

	err := a.AptUpdate(ctx)
	if err != nil {
		return nil, err
	}

	var packages []Package

	asComponents, errAS := a.appStream.Load(ctx)
	if errAS != nil {
		lib.Log.Warnf(lib.T_("AppStream load failed: %v"), errAS)
	}

	asMap := make(map[string]*appstream.Component, len(asComponents))
	for i := range asComponents {
		c := &asComponents[i]
		asMap[c.PkgName] = c
	}

	aptService := aptBinding.NewActions()
	aptPackages, err := aptService.Search("")
	if err != nil {
		return nil, err
	}

	packages = make([]Package, 0, len(aptPackages))
	for _, ap := range aptPackages {
		var depends []string
		seen := make(map[string]bool)

		if ap.Depends != "" {
			depList := strings.Split(ap.Depends, ",")
			for _, dep := range depList {
				clean := strings.TrimSpace(dep)
				if clean == "" {
					continue
				}
				if strings.Contains(clean, ".so") {
					clean = aptParser.CleanDependency(clean)
				}
				if !seen[clean] {
					seen[clean] = true
					depends = append(depends, clean)
				}
			}
		}
		var provides []string
		seen = make(map[string]bool)

		if ap.Provides != "" {
			provList := strings.Split(ap.Provides, ",")
			for _, prov := range provList {
				clean := strings.TrimSpace(prov)
				if clean == "" {
					continue
				}
				if strings.Contains(clean, ".so") {
					clean = aptParser.CleanDependency(clean)
				}
				if !seen[clean] {
					seen[clean] = true
					provides = append(provides, clean)
				}
			}
		}

		formattedVersion := ap.Version
		if v, errParse := helper.GetVersionFromAptCache(ap.Version); errParse == nil && v != "" {
			formattedVersion = v
		}

		p := Package{
			Name:             ap.Name,
			Section:          ap.Section,
			InstalledSize:    int(ap.InstalledSize),
			Maintainer:       ap.Maintainer,
			Version:          formattedVersion,
			VersionInstalled: "",
			Depends:          depends,
			Provides:         provides,
			Size:             int(ap.DownloadSize),
			Filename:         ap.Filename,
			Description:      ap.Description,
			AppStream:        nil,
			Changelog:        ap.Changelog,
			Installed:        false,
			TypePackage:      int(PackageTypeSystem),
		}

		if p.Description == "" {
			p.Description = ap.ShortDescription
		}
		packages = append(packages, p)
	}

	for i := range packages {
		if comp, ok := asMap[packages[i].Name]; ok {
			packages[i].AppStream = comp
		}
		packages[i].Changelog = extractLastMessage(packages[i].Changelog)
	}

	if lib.Env.ExistStplr {
		packages, err = a.serviceStplr.UpdateWithStplrPackages(ctx, packages)
		if err != nil {
			lib.Log.Errorf(err.Error())
		}
	}

	// @TODO Обновляем информацию о том, установлены ли пакеты локально, на самом деле об этом можно узнать из биндингов
	packages, err = a.updateInstalledInfo(ctx, packages)
	if err != nil {
		return nil, fmt.Errorf(lib.T_("Error updating information about installed packages: %w"), err)
	}

	err = a.serviceAptDatabase.SavePackagesToDB(ctx, packages)
	if err != nil {
		return nil, err
	}

	return packages, nil
}

// CleanPackageName очищаем странный суффикс в ответе apt
func (a *Actions) CleanPackageName(pkg string, packageNames []string) string {
	if strings.HasSuffix(pkg, ".32bit") {
		basePkg := strings.TrimSuffix(pkg, ".32bit")
		for _, validPkg := range packageNames {
			if validPkg == basePkg {
				return basePkg
			}
		}
	}

	return pkg
}

// updateInstalledInfo обновляет срез пакетов, устанавливая поля Installed и InstalledVersion, если пакет найден в системе.
func (a *Actions) updateInstalledInfo(ctx context.Context, packages []Package) ([]Package, error) {
	installed, err := a.GetInstalledPackages(ctx)
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
func (a *Actions) GetInstalledPackages(ctx context.Context) (map[string]string, error) {
	command := fmt.Sprintf("%s rpm -qia", lib.Env.CommandPrefix)
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Env = []string{"LC_ALL=C"}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf(lib.T_("Error executing the rpm -qia command: %w"), err)
	}

	installed := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	var currentName, currentVersion string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "Name") {
			if currentName != "" {
				installed[currentName] = currentVersion
				currentName, currentVersion = "", ""
			}
		}
		if line == "" {
			if currentName != "" {
				installed[currentName] = currentVersion
				currentName, currentVersion = "", ""
			}
			continue
		}

		if strings.HasPrefix(line, "Name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentName = strings.TrimSpace(parts[1])
			}
			continue
		}

		if strings.HasPrefix(line, "Version") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentVersion = strings.TrimSpace(parts[1])
			}
			continue
		}
	}

	if currentName != "" {
		installed[currentName] = currentVersion
	}

	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf(lib.T_("Error scanning rpm output: %w"), err)
	}

	return installed, nil
}

func (a *Actions) AptUpdate(ctx context.Context) error {
	syncAptMutex.Lock()
	defer syncAptMutex.Unlock()
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName("system.AptUpdate"))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName("system.AptUpdate"))

	aptService := aptBinding.NewActions()
	err := aptService.Update()
	if err != nil {
		return err
	}

	return nil
}

func extractLastMessage(changelog string) string {
	lines := strings.Split(changelog, "\n")
	var result []string
	found := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "*") {
			if !found {
				result = append(result, trimmed)
				found = true
			} else {
				break
			}
		} else if found {
			result = append(result, trimmed)
		}
	}

	return strings.Join(result, "\n")
}
