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
// cgo-timestamp: 1755028593
#include "apt_wrapper.h"
#include <stdlib.h>
*/
import "C"

import (
	"runtime"
	"strconv"
	"unsafe"
)

// System represents APT system configuration
type System struct{ Ptr *C.AptSystem }

// NewSystem initializes APT system
func NewSystem() (*System, error) {
	AptMutex.Lock()
	defer AptMutex.Unlock()
	if res := C.apt_init_config(); res.code != C.APT_SUCCESS {
		return nil, ErrorFromResult(res)
	}
	setCfg := func(key, val string) {
		cKey := C.CString(key)
		cVal := C.CString(val)
		_ = C.apt_set_config(cKey, cVal)
		C.free(unsafe.Pointer(cKey))
		C.free(unsafe.Pointer(cVal))
	}
	setCfg("Acquire::Retries", strconv.Itoa(1))
	setCfg("Acquire::http::Timeout", strconv.Itoa(20))
	setCfg("Acquire::https::Timeout", strconv.Itoa(20))
	setCfg("Acquire::ftp::Timeout", strconv.Itoa(20))
	setCfg("Acquire::http::ConnectTimeout", strconv.Itoa(20))
	setCfg("Acquire::ftp::ConnectTimeout", strconv.Itoa(20))
	var ptr *C.AptSystem
	if res := C.apt_init_system(&ptr); res.code != C.APT_SUCCESS || ptr == nil {
		return nil, ErrorFromResult(res)
	}
	s := &System{Ptr: ptr}
	runtime.SetFinalizer(s, (*System).Close)
	return s, nil
}

// Close frees the system resources
func (s *System) Close() {
	if s.Ptr != nil {
		C.apt_cleanup_system(s.Ptr)
		s.Ptr = nil
		runtime.SetFinalizer(s, nil)
	}
}
