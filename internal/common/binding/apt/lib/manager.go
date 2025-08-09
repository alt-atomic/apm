package lib

/*
// cgo-timestamp: 1754763594
#include "apt_wrapper.h"
#include <stdlib.h>
*/
import "C"

import "runtime"

// PackageManager handles install/remove operations via C++ layer
type PackageManager struct{ Ptr *C.AptPackageManager }

func NewPackageManager(cache *Cache) (*PackageManager, error) {
	aptMutex.Lock()
	defer aptMutex.Unlock()
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
	aptMutex.Lock()
	defer aptMutex.Unlock()
	if res := C.apt_install_packages(pm.Ptr, nil, nil); res.code != C.APT_SUCCESS {
		return ErrorFromResult(res)
	}
	return nil
}
