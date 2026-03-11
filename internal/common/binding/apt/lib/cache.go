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
#include "apt.h"
#include <stdlib.h>
*/
import "C"

import (
	"runtime"
	cgoRuntime "runtime/cgo"
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

func (c *Cache) Update(handler ProgressHandler) error {
	return withMutex(func() error {
		var userData C.uintptr_t
		if handler != nil {
			handle := cgoRuntime.NewHandle(handler)
			defer handle.Delete()
			userData = C.uintptr_t(handle)
			C.apt_use_go_progress_callback(userData)
		}
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
	if c.file_count > 0 && c.files != nil {
		p.Files = convertCStringArray(c.files, c.file_count)
	}
}
