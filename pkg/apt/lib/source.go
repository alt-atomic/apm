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
	cgoRuntime "runtime/cgo"
	"unsafe"
)

// SourcePackage describes a downloaded source package (.src.rpm)
type SourcePackage struct {
	Name    string
	Version string
	File    string
	Size    uint64
}

// DownloadSource downloads .src.rpm files for the given names into destDir
func (c *Cache) DownloadSource(names []string, destDir string, handler ProgressHandler) ([]SourcePackage, error) {
	var packages []SourcePackage
	err := withMutex(func() error {
		cNames := makeCStringArray(names)
		defer freeCStringArray(cNames)

		var cDest *C.char
		if destDir != "" {
			cDest = C.CString(destDir)
			defer C.free(unsafe.Pointer(cDest))
		}

		if handler != nil {
			handle := cgoRuntime.NewHandle(handler)
			defer handle.Delete()
			C.apt_use_go_progress_callback(C.uintptr_t(handle))
		}

		var result C.AptSourceResult
		res := C.apt_source_download(c.Ptr, (**C.char)(unsafe.Pointer(&cNames[0])),
			C.size_t(len(names)), cDest, &result)
		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		defer C.apt_free_source_result(&result)

		count := int(result.count)
		packages = make([]SourcePackage, 0, count)
		for i := 0; i < count; i++ {
			var cName, cVersion, cFile *C.char
			var cSize C.uint64_t
			C.apt_get_source_package(&result, C.size_t(i), &cName, &cVersion, &cFile, &cSize)
			packages = append(packages, SourcePackage{
				Name:    cStringToGo(cName),
				Version: cStringToGo(cVersion),
				File:    cStringToGo(cFile),
				Size:    uint64(cSize),
			})
		}
		return nil
	})
	return packages, err
}
