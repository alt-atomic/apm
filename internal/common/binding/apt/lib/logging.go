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
	"fmt"
	cgoruntime "runtime/cgo"
)

type LogHandler func(message string)

var (
	logHandle    cgoruntime.Handle
	logHandleSet bool
)

//export goAptLogCallback
func goAptLogCallback(cmsg *C.char, user C.uintptr_t) {
	defer func() { _ = recover() }()
	if user != 0 {
		h := cgoruntime.Handle(user)
		if cb, ok := h.Value().(LogHandler); ok && cb != nil {
			cb(C.GoString(cmsg))
			return
		}
	}
	if logHandleSet {
		if cb, ok := logHandle.Value().(LogHandler); ok && cb != nil {
			cb(C.GoString(cmsg))
			return
		}
	}
	fmt.Println(C.GoString(cmsg))
}

// SetLogHandler перехват stdout/stderr через LogHandler
func SetLogHandler(handler LogHandler) {
	AptMutex.Lock()
	defer AptMutex.Unlock()

	if logHandleSet {
		logHandle.Delete()
		logHandleSet = false
	}
	if handler == nil {
		C.apt_capture_stdio(C.int(0))
		C.apt_set_log_callback(nil, 0)
		return
	}
	logHandle = cgoruntime.NewHandle(handler)
	logHandleSet = true
	C.apt_enable_go_log_callback(C.uintptr_t(logHandle))
}

// CaptureStdIO ручное включение/отключение stdout/stderr
func CaptureStdIO(enable bool) {
	AptMutex.Lock()
	defer AptMutex.Unlock()
	if enable {
		C.apt_capture_stdio(C.int(1))
	} else {
		C.apt_capture_stdio(C.int(0))
	}
}
