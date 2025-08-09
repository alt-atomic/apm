package service

/*
#include "apt_wrapper.h"
#include <stdlib.h>
*/
import "C"

import (
	cgo_runtime "runtime/cgo"
	"unsafe"
)

type ProgressType int

const (
	CallbackUnknown          ProgressType = 0
	CallbackInstallProgress  ProgressType = 1
	CallbackInstallStart     ProgressType = 2
	CallbackInstallStop      ProgressType = 3
	CallbackRemoveProgress   ProgressType = 4
	CallbackRemoveStart      ProgressType = 5
	CallbackRemoveStop       ProgressType = 6
	CallbackError            ProgressType = 7
	CallbackTransProgress    ProgressType = 8
	CallbackTransStart       ProgressType = 9
	CallbackTransStop        ProgressType = 10
	CallbackElemProgress     ProgressType = 11
	CallbackDownloadStart    ProgressType = 20
	CallbackDownloadProgress ProgressType = 21
	CallbackDownloadStop     ProgressType = 22
)

type ProgressHandler func(packageName string, eventType ProgressType, current, total uint64)

//export goAptProgressCallback
func goAptProgressCallback(cname *C.char, ctype C.int, ccurrent C.ulonglong, ctotal C.ulonglong, user unsafe.Pointer) {
	defer func() { _ = recover() }()
	h := cgo_runtime.Handle(uintptr(user))
	if v := h.Value(); v != nil {
		if handler, ok := v.(ProgressHandler); ok && handler != nil {
			handler(C.GoString(cname), ProgressType(int(ctype)), uint64(ccurrent), uint64(ctotal))
		}
	}
}

func (pm *PackageManager) InstallPackagesWithProgress(handler ProgressHandler) error {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	var userData unsafe.Pointer
	if handler != nil {
		handle := cgo_runtime.NewHandle(handler)
		defer handle.Delete()
		userData = unsafe.Pointer(uintptr(handle))
		C.apt_use_go_progress_callback(userData)
	}
	if res := C.apt_install_packages(pm.Ptr, nil, userData); res != 0 {
		return &AptError{Code: int(res), Message: "Failed to install packages"}
	}
	return nil
}

func (c *Cache) DistUpgradeWithProgress(handler ProgressHandler) error {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	var userData unsafe.Pointer
	if handler != nil {
		handle := cgo_runtime.NewHandle(handler)
		defer handle.Delete()
		userData = unsafe.Pointer(uintptr(handle))
		C.apt_use_go_progress_callback(userData)
	}
	if res := C.apt_dist_upgrade_with_progress(c.Ptr, nil, userData); res != 0 {
		return &AptError{Code: int(res), Message: "Failed to perform dist-upgrade with progress"}
	}
	return nil
}
