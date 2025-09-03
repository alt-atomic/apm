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

package lib

/*
// cgo-timestamp: 1756924224
#include "apt_wrapper.h"
#include <stdlib.h>
*/
import "C"

import (
	"runtime"
	"unsafe"
)

// Cache represents package cache
type Cache struct {
	Ptr    *C.AptCache
	system *System
}

// OpenCache opens the package cache
func OpenCache(system *System, readOnly bool) (*Cache, error) {
	var cache *Cache
	err := withMutex(func() error {
		var err error
		cache, err = openCacheUnsafe(system, readOnly)
		return err
	})
	return cache, err
}

func (c *Cache) Close() {
	if c.Ptr != nil {
		C.apt_cache_close(c.Ptr)
		c.Ptr = nil
		runtime.SetFinalizer(c, nil)
	}
}

func (c *Cache) Update() error {
	return withMutex(func() error {
		if res := C.apt_cache_update(c.Ptr); res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		return nil
	})
}

func (c *Cache) Refresh() error {
	return withMutex(func() error {
		if res := C.apt_cache_refresh(c.Ptr); res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		return nil
	})
}

func (c *Cache) BrokenCount() int {
	var count int
	_ = withMutex(func() error {
		count = int(C.apt_get_broken_count(c.Ptr))
		return nil
	})
	return count
}

func (c *Cache) HasBrokenPackages() bool {
	var result bool
	_ = withMutex(func() error {
		result = bool(C.apt_has_broken_packages(c.Ptr))
		return nil
	})
	return result
}

func (c *Cache) MarkInstall(packageName string) error {
	return withMutex(func() error {
		cname := C.CString(packageName)
		defer C.free(unsafe.Pointer(cname))
		if res := C.apt_mark_install(c.Ptr, cname); res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		return nil
	})
}

func (c *Cache) MarkRemove(packageName string, purge bool) error {
	return withMutex(func() error {
		cname := C.CString(packageName)
		defer C.free(unsafe.Pointer(cname))
		if res := C.apt_mark_remove(c.Ptr, cname, C.bool(purge)); res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		return nil
	})
}

func (c *Cache) GetPackageInfo(packageName string) (*PackageInfo, error) {
	var info *PackageInfo
	err := withMutex(func() error {
		cname := C.CString(packageName)
		defer C.free(unsafe.Pointer(cname))

		var ci C.AptPackageInfo
		if res := C.apt_get_package_info(c.Ptr, cname, &ci); res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		defer C.apt_free_package_info(&ci)

		info = &PackageInfo{}
		info.fromCStruct(&ci)
		return nil
	})
	return info, err
}

func (c *Cache) SearchPackages(pattern string) ([]PackageInfo, error) {
	var pkgs []PackageInfo
	err := withMutex(func() error {
		cPattern := C.CString(pattern)
		defer C.free(unsafe.Pointer(cPattern))

		var list C.AptPackageList
		if res := C.apt_search_packages(c.Ptr, cPattern, &list); res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		defer C.apt_free_package_list(&list)

		if list.count > 0 {
			pkgs = make([]PackageInfo, int(list.count))
			cp := unsafe.Slice(list.packages, int(list.count))
			for i, cpi := range cp {
				pkgs[i].fromCStruct(&cpi)
			}
		}
		return nil
	})
	return pkgs, err
}

// SimulateDistUpgrade симулирует обновление системы
func (c *Cache) SimulateDistUpgrade() (*PackageChanges, error) {
	var changes *PackageChanges
	err := withMutex(func() error {
		var cc C.AptPackageChanges
		res := C.apt_simulate_dist_upgrade(c.Ptr, &cc)
		defer C.apt_free_package_changes(&cc)

		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}

		changes = convertPackageChanges(&cc)
		return nil
	})
	return changes, err
}

// SimulateAutoRemove симулирует автоматическое удаление неиспользуемых пакетов
func (c *Cache) SimulateAutoRemove() (*PackageChanges, error) {
	var changes *PackageChanges
	err := withMutex(func() error {
		var cc C.AptPackageChanges
		res := C.apt_simulate_autoremove(c.Ptr, &cc)
		defer C.apt_free_package_changes(&cc)

		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}

		changes = convertPackageChanges(&cc)
		return nil
	})
	return changes, err
}

// SimulateInstall симулирует установку пакетов
func (c *Cache) SimulateInstall(packageNames []string) (*PackageChanges, error) {
	if len(packageNames) == 0 {
		return nil, CustomError(AptErrorInvalidParameters, "Invalid parameters")
	}

	var changes *PackageChanges
	err := withMutex(func() error {
		cNames := makeCStringArray(packageNames)
		defer freeCStringArray(cNames)

		var cc C.AptPackageChanges
		res := C.apt_simulate_install(c.Ptr, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(packageNames)), &cc)
		defer C.apt_free_package_changes(&cc)

		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}

		changes = convertPackageChanges(&cc)
		return nil
	})
	return changes, err
}

// SimulateRemove симулирует удаление пакетов
func (c *Cache) SimulateRemove(packageNames []string, purge bool) (*PackageChanges, error) {
	if len(packageNames) == 0 {
		return nil, CustomError(AptErrorInvalidParameters, "Invalid parameters")
	}

	var changes *PackageChanges
	err := withMutex(func() error {
		cNames := makeCStringArray(packageNames)
		defer freeCStringArray(cNames)

		var cc C.AptPackageChanges
		res := C.apt_simulate_remove(c.Ptr, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(packageNames)), C.bool(purge), &cc)
		defer C.apt_free_package_changes(&cc)

		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}

		changes = convertPackageChanges(&cc)
		return nil
	})
	return changes, err
}

// SimulateChange симулирует установку и удаление пакетов в одной транзакции
func (c *Cache) SimulateChange(installNames []string, removeNames []string, purge bool) (*PackageChanges, error) {
	if len(installNames) == 0 && len(removeNames) == 0 {
		return nil, CustomError(AptErrorInvalidParameters, "Invalid parameters")
	}

	var changes *PackageChanges
	err := withMutex(func() error {
		var cInst **C.char
		var instCount C.size_t
		var installArr []*C.char

		if len(installNames) > 0 {
			installArr = makeCStringArray(installNames)
			defer freeCStringArray(installArr)
			cInst = (**C.char)(unsafe.Pointer(&installArr[0]))
			instCount = C.size_t(len(installNames))
		}

		var cRem **C.char
		var remCount C.size_t
		var removeArr []*C.char

		if len(removeNames) > 0 {
			removeArr = makeCStringArray(removeNames)
			defer freeCStringArray(removeArr)
			cRem = (**C.char)(unsafe.Pointer(&removeArr[0]))
			remCount = C.size_t(len(removeNames))
		}

		var cc C.AptPackageChanges
		res := C.apt_simulate_change(c.Ptr, cInst, instCount, cRem, remCount, C.bool(purge), &cc)
		defer C.apt_free_package_changes(&cc)

		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}

		changes = convertPackageChanges(&cc)
		return nil
	})
	return changes, err
}

// Helper: safely convert C string to Go string
func cStringToGo(cstr *C.char) string {
	if cstr != nil {
		return C.GoString(cstr)
	}
	return ""
}

// Convert C AptPackageInfo to Go PackageInfo
func (p *PackageInfo) fromCStruct(c *C.AptPackageInfo) {
	p.Name = cStringToGo(c.name)
	p.Version = cStringToGo(c.version)
	p.Description = cStringToGo(c.description)
	p.ShortDescription = cStringToGo(c.short_description)
	p.Section = cStringToGo(c.section)
	p.Architecture = cStringToGo(c.architecture)
	p.Maintainer = cStringToGo(c.maintainer)
	p.Homepage = cStringToGo(c.homepage)
	p.Priority = cStringToGo(c.priority)
	p.MD5Hash = cStringToGo(c.md5_hash)
	p.Blake2bHash = cStringToGo(c.blake2b_hash)
	p.SourcePackage = cStringToGo(c.source_package)
	p.Changelog = cStringToGo(c.changelog)
	p.Filename = cStringToGo(c.filename)
	p.Depends = cStringToGo(c.depends)
	p.Provides = cStringToGo(c.provides)
	p.Conflicts = cStringToGo(c.conflicts)
	p.Obsoletes = cStringToGo(c.obsoletes)
	p.Recommends = cStringToGo(c.recommends)
	p.Suggests = cStringToGo(c.suggests)
	p.State = PackageState(c.state)
	p.AutoInstalled = bool(c.auto_installed)
	p.Essential = bool(c.essential)
	p.InstalledSize = uint64(c.installed_size)
	p.DownloadSize = uint64(c.download_size)
	p.PackageID = uint32(c.package_id)

	if c.alias_count > 0 && c.aliases != nil {
		p.Aliases = convertCStringArray(c.aliases, c.alias_count)
	}
}
