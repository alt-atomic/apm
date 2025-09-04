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
// cgo-timestamp: 1756991486
#include "apt_wrapper.h"
#include <stdlib.h>
*/
import "C"

import (
	"runtime"
	cgoRuntime "runtime/cgo"
	"unsafe"
)

// PackageManager обрабатывает операции установки/удаления через уровень C++
type PackageManager struct{ Ptr *C.AptPackageManager }

// NewPackageManager создает новый экземпляр менеджера пакетов
func NewPackageManager(cache *Cache) (*PackageManager, error) {
	var pm *PackageManager
	err := withMutex(func() error {
		var ptr *C.AptPackageManager
		if res := C.apt_package_manager_create(cache.Ptr, &ptr); res.code != C.APT_SUCCESS || ptr == nil {
			return ErrorFromResult(res)
		}
		pm = &PackageManager{Ptr: ptr}
		runtime.SetFinalizer(pm, (*PackageManager).Close)
		return nil
	})
	return pm, err
}

// Close уничтожает менеджер пакетов и освобождает ресурсы
func (pm *PackageManager) Close() {
	if pm.Ptr != nil {
		C.apt_package_manager_destroy(pm.Ptr)
		pm.Ptr = nil
		runtime.SetFinalizer(pm, nil)
	}
}

// InstallPackages выполняет установку пакета без обратного вызова прогресса
func (pm *PackageManager) InstallPackages() error {
	return withMutex(func() error {
		if res := C.apt_install_packages(pm.Ptr, nil, nil); res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		return nil
	})
}

// InstallPackagesWithProgress выполняет установку пакета с обратным вызовом прогресса
func (pm *PackageManager) InstallPackagesWithProgress(handler ProgressHandler) error {
	return withMutex(func() error {
		var userData unsafe.Pointer
		if handler != nil {
			handle := cgoRuntime.NewHandle(handler)
			defer handle.Delete()
			// Note: go vet warns about unsafe.Pointer(uintptr(handle)), but this is the correct
			userData = unsafe.Pointer(uintptr(handle))
			C.apt_use_go_progress_callback(userData)
		}
		if res := C.apt_install_packages(pm.Ptr, nil, userData); res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		return nil
	})
}

// DistUpgradeWithProgress выполняет полное обновление системы с прогрессом
func (c *Cache) DistUpgradeWithProgress(handler ProgressHandler) error {
	return withMutex(func() error {
		var userData unsafe.Pointer
		if handler != nil {
			handle := cgoRuntime.NewHandle(handler)
			defer handle.Delete()
			// Note: go vet warns about unsafe.Pointer(uintptr(handle)), but this is the correct
			userData = unsafe.Pointer(uintptr(handle))
			C.apt_use_go_progress_callback(userData)
		}
		if res := C.apt_dist_upgrade_with_progress(c.Ptr, nil, userData); res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		return nil
	})
}
