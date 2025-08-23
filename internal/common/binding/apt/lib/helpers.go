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
// cgo-timestamp: 1755965157
#include "apt_wrapper.h"
#include <stdlib.h>
*/
import "C"

import (
	"runtime"
	"sync"
	"unsafe"
)

// AptMutex Глобальный mutex на все операции apt-lib
var AptMutex sync.Mutex

// convertCStringArray конвертирует массив C строк в Go slice
func convertCStringArray(ptr **C.char, count C.size_t) []string {
	if ptr == nil || count == 0 {
		return nil
	}

	result := make([]string, int(count))
	// Используем unsafe.Slice для безопасной работы с C массивом
	cStrings := unsafe.Slice(ptr, int(count))
	for i, cStr := range cStrings {
		if cStr != nil {
			result[i] = C.GoString(cStr)
		}
	}
	return result
}

// freeCStringArray освобождает память C массива строк
func freeCStringArray(arr []*C.char) {
	for _, str := range arr {
		if str != nil {
			C.free(unsafe.Pointer(str))
		}
	}
}

// makeCStringArray создаёт массив C строк из Go slice
func makeCStringArray(strs []string) []*C.char {
	if len(strs) == 0 {
		return nil
	}
	result := make([]*C.char, len(strs))
	for i, str := range strs {
		result[i] = C.CString(str)
	}
	return result
}

// convertPackageChanges конвертирует C структуру AptPackageChanges в Go
func convertPackageChanges(cc *C.AptPackageChanges) *PackageChanges {
	if cc == nil {
		return nil
	}

	changes := &PackageChanges{
		UpgradedCount:     int(cc.upgraded_count),
		NewInstalledCount: int(cc.new_installed_count),
		RemovedCount:      int(cc.removed_count),
		NotUpgradedCount:  int(cc.not_upgraded_count),
		DownloadSize:      uint64(cc.download_size),
		InstallSize:       uint64(cc.install_size),
	}

	// Конвертируем массивы с использованием helper функции
	changes.ExtraInstalled = convertCStringArray(cc.extra_installed, cc.extra_installed_count)
	changes.UpgradedPackages = convertCStringArray(cc.upgraded_packages, cc.upgraded_count)
	changes.NewInstalledPackages = convertCStringArray(cc.new_installed_packages, cc.new_installed_count)
	changes.RemovedPackages = convertCStringArray(cc.removed_packages, cc.removed_count)

	return changes
}

// withMutex выполняет функцию под защитой глобального мьютекса APT
func withMutex(fn func() error) error {
	AptMutex.Lock()
	defer AptMutex.Unlock()
	return fn()
}

// openCacheUnsafe открывает кеш без блокировки мьютекса (должен вызываться под мьютексом)
func openCacheUnsafe(system *System, readOnly bool) (*Cache, error) {
	var ptr *C.AptCache
	withLock := C.bool(!readOnly)
	if res := C.apt_cache_open(system.Ptr, &ptr, withLock); res.code != C.APT_SUCCESS || ptr == nil {
		return nil, ErrorFromResult(res)
	}
	c := &Cache{Ptr: ptr, system: system}
	runtime.SetFinalizer(c, (*Cache).Close)
	return c, nil
}

// simulateOperation выполняет симуляцию операции и конвертирует результат
func (c *Cache) simulateOperation(
	simFunc func() (C.AptResult, *C.AptPackageChanges),
) (*PackageChanges, error) {
	var result *PackageChanges
	err := withMutex(func() error {
		var cc C.AptPackageChanges
		res, changes := simFunc()
		if changes != nil {
			cc = *changes
		}
		defer C.apt_free_package_changes(&cc)

		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}

		result = convertPackageChanges(&cc)
		return nil
	})
	return result, err
}

// markPackages помечает пакеты для установки или удаления
func (c *Cache) markPackages(installNames, removeNames []string, purge bool) error {
	return withMutex(func() error {
		for _, name := range removeNames {
			cname := C.CString(name)
			res := C.apt_mark_remove(c.Ptr, cname, C.bool(purge))
			C.free(unsafe.Pointer(cname))
			if res.code != C.APT_SUCCESS {
				return ErrorFromResult(res)
			}
		}

		// Затем помечаем для установки
		for _, name := range installNames {
			cname := C.CString(name)
			res := C.apt_mark_install(c.Ptr, cname)
			C.free(unsafe.Pointer(cname))
			if res.code != C.APT_SUCCESS {
				return ErrorFromResult(res)
			}
		}

		return nil
	})
}
