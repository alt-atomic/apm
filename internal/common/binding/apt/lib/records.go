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

import "unsafe"

func (c *Cache) GetPackageInfo(packageName string) (*PackageInfo, error) {
	var info *PackageInfo
	err := withMutex(func() error {
		cname := C.CString(packageName)
		defer C.free(unsafe.Pointer(cname))

		var ci C.AptPackageInfo
		if res := C.apt_package_get(c.Ptr, cname, &ci); res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		defer C.apt_package_free(&ci)

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
		if res := C.apt_packages_search(c.Ptr, cPattern, &list); res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		defer C.apt_packages_free(&list)

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
