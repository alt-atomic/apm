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
// cgo-timestamp: 1755474493
#include "apt_wrapper.h"
#include <stdlib.h>
*/
import "C"

import (
	cgoRuntime "runtime/cgo"
	"unsafe"
)

type ProgressType int

const (
	CallbackInstallProgress  ProgressType = 1
	CallbackDownloadProgress ProgressType = 21
	CallbackDownloadComplete ProgressType = 23
)

type ProgressHandler func(packageName string, eventType ProgressType, current, total uint64)

//export goAptProgressCallback
func goAptProgressCallback(cname *C.char, ctype C.int, ccurrent C.ulonglong, ctotal C.ulonglong, user unsafe.Pointer) {
	defer func() { _ = recover() }()
	h := cgoRuntime.Handle(uintptr(user))
	if v := h.Value(); v != nil {
		if handler, ok := v.(ProgressHandler); ok && handler != nil {
			handler(C.GoString(cname), ProgressType(int(ctype)), uint64(ccurrent), uint64(ctotal))
		}
	}
}
