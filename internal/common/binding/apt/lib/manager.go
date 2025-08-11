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
// cgo-timestamp: 1754917234
#include "apt_wrapper.h"
#include <stdlib.h>
*/
import "C"

import "runtime"

// PackageManager handles install/remove operations via C++ layer
type PackageManager struct{ Ptr *C.AptPackageManager }

func NewPackageManager(cache *Cache) (*PackageManager, error) {
	AptMutex.Lock()
	defer AptMutex.Unlock()
	var ptr *C.AptPackageManager
	if res := C.apt_package_manager_create(cache.Ptr, &ptr); res.code != C.APT_SUCCESS || ptr == nil {
		return nil, ErrorFromResult(res)
	}
	pm := &PackageManager{Ptr: ptr}
	runtime.SetFinalizer(pm, (*PackageManager).Close)
	return pm, nil
}

func (pm *PackageManager) Close() {
	if pm.Ptr != nil {
		C.apt_package_manager_destroy(pm.Ptr)
		pm.Ptr = nil
		runtime.SetFinalizer(pm, nil)
	}
}

func (pm *PackageManager) InstallPackages() error {
	AptMutex.Lock()
	defer AptMutex.Unlock()
	if res := C.apt_install_packages(pm.Ptr, nil, nil); res.code != C.APT_SUCCESS {
		return ErrorFromResult(res)
	}
	return nil
}
