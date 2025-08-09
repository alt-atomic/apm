package lib

/*
// cgo-timestamp: 1754763594
#include "apt_wrapper.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	cgo_runtime "runtime/cgo"
	"unsafe"
)

type LogHandler func(message string)

var (
	logHandle    cgo_runtime.Handle
	logHandleSet bool
)

//export goAptLogCallback
func goAptLogCallback(cmsg *C.char, user unsafe.Pointer) {
	defer func() { _ = recover() }()
	if user != nil {
		h := cgo_runtime.Handle(uintptr(user))
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
	aptMutex.Lock()
	defer aptMutex.Unlock()

	if logHandleSet {
		logHandle.Delete()
		logHandleSet = false
	}
	if handler == nil {
		C.apt_capture_stdio(C.int(0))
		C.apt_set_log_callback(nil, nil)
		return
	}
	logHandle = cgo_runtime.NewHandle(handler)
	logHandleSet = true
	C.apt_enable_go_log_callback(unsafe.Pointer(uintptr(logHandle)))
}

// CaptureStdIO ручное включение/отключение stdout/stderr
func CaptureStdIO(enable bool) {
	aptMutex.Lock()
	defer aptMutex.Unlock()
	if enable {
		C.apt_capture_stdio(C.int(1))
	} else {
		C.apt_capture_stdio(C.int(0))
	}
}
