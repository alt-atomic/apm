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
	"runtime"
	"strconv"
	"unsafe"
)

// System представляет конфигурацию APT системы
type System struct{ Ptr *C.AptSystem }

// NewSystem инициализирует APT систему
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
	setCfg("Acquire::Retries", strconv.Itoa(2))
	setCfg("Acquire::http::Timeout", strconv.Itoa(25))
	setCfg("Acquire::https::Timeout", strconv.Itoa(25))
	setCfg("Acquire::ftp::Timeout", strconv.Itoa(25))
	setCfg("Acquire::http::ConnectTimeout", strconv.Itoa(25))
	setCfg("Acquire::ftp::ConnectTimeout", strconv.Itoa(25))
	if isAtomicSystem() {
		setCfg("RPM::Options::", "--ignoresize")
	}
	var ptr *C.AptSystem
	if res := C.apt_init_system(&ptr); res.code != C.APT_SUCCESS || ptr == nil {
		return nil, ErrorFromResult(res)
	}
	s := &System{Ptr: ptr}
	runtime.SetFinalizer(s, (*System).Close)
	return s, nil
}

// Close освобождает ресурсы системы
func (s *System) Close() {
	if s.Ptr != nil {
		BlockSignals()
		defer RestoreSignals()
		C.apt_cleanup_system(s.Ptr)
		s.Ptr = nil
		runtime.SetFinalizer(s, nil)
	}
}

// SetConfig устанавливает значение конфигурации APT (например "Dir::Cache::Archives", "/tmp/")
func SetConfig(key, value string) {
	cKey := C.CString(key)
	cVal := C.CString(value)
	defer C.free(unsafe.Pointer(cKey))
	defer C.free(unsafe.Pointer(cVal))
	C.apt_set_config(cKey, cVal)
}

// DumpConfig возвращает всю конфигурацию APT в виде строки.
func DumpConfig() string {
	cVal := C.apt_config_dump()
	if cVal == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cVal))
	return C.GoString(cVal)
}

// ConfigSnapshot создаёт копию всего дерева конфигурации APT.
func ConfigSnapshot() unsafe.Pointer {
	return C.apt_config_snapshot()
}

// ConfigRestore восстанавливает конфигурацию APT из снимка (освобождает снимок).
func ConfigRestore(snapshot unsafe.Pointer) {
	if snapshot != nil {
		C.apt_config_restore(snapshot)
	}
}

// WithConfigOverrides сохраняет снимок конфигурации, применяет overrides, выполняет fn, затем восстанавливает из снимка.
func WithConfigOverrides(overrides map[string]string, fn func() error) error {
	if len(overrides) == 0 {
		return fn()
	}

	snapshot := ConfigSnapshot()
	if snapshot == nil {
		return fn()
	}

	defer func() {
		//fmt.Fprintln(os.Stderr, "[APT config] before restore:\n"+DumpConfig())
		ConfigRestore(snapshot)
		//fmt.Fprintln(os.Stderr, "[APT config] after restore:\n"+DumpConfig())
	}()

	for key, value := range overrides {
		//fmt.Fprintf(os.Stderr, "[APT config] override %s = %q\n", key, value)
		SetConfig(key, value)
	}

	return fn()
}

// SetNoLocking включает или отключает блокировку файлов APT.
func SetNoLocking(noLock bool) {
	val := "false"
	if noLock {
		val = "true"
	}
	cKey := C.CString("Debug::NoLocking")
	cVal := C.CString(val)
	defer C.free(unsafe.Pointer(cKey))
	defer C.free(unsafe.Pointer(cVal))
	C.apt_set_config(cKey, cVal)
}
