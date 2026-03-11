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
	cgoRuntime "runtime/cgo"
	"unsafe"
)

// Transaction инкапсулирует жизненный цикл операции с пакетами
type Transaction struct {
	ptr *C.AptTransaction
}

// NewTransaction создаёт новую транзакцию для данного кеша
func (c *Cache) NewTransaction() (*Transaction, error) {
	var tx *Transaction
	err := withMutex(func() error {
		var ptr *C.AptTransaction
		if res := C.apt_transaction_new(c.Ptr, &ptr); res.code != C.APT_SUCCESS || ptr == nil {
			return ErrorFromResult(res)
		}
		tx = &Transaction{ptr: ptr}
		runtime.SetFinalizer(tx, (*Transaction).Close)
		return nil
	})
	return tx, err
}

// Close освобождает ресурсы транзакции
func (tx *Transaction) Close() {
	if tx.ptr != nil {
		C.apt_transaction_free(tx.ptr)
		tx.ptr = nil
		runtime.SetFinalizer(tx, nil)
	}
}

// Install добавляет пакеты для установки
func (tx *Transaction) Install(names []string) error {
	if len(names) == 0 {
		return CustomError(AptErrorInvalidParameters, "No package names")
	}
	return withMutex(func() error {
		cNames := makeCStringArray(names)
		defer freeCStringArray(cNames)
		res := C.apt_transaction_install(tx.ptr, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(names)))
		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		return nil
	})
}

// Remove добавляет пакеты для удаления
func (tx *Transaction) Remove(names []string, purge, depends bool) error {
	if len(names) == 0 {
		return CustomError(AptErrorInvalidParameters, "No package names")
	}
	return withMutex(func() error {
		cNames := makeCStringArray(names)
		defer freeCStringArray(cNames)
		res := C.apt_transaction_remove(tx.ptr, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(names)),
			C.bool(purge), C.bool(depends))
		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		return nil
	})
}

// Reinstall добавляет пакеты для переустановки
func (tx *Transaction) Reinstall(names []string) error {
	if len(names) == 0 {
		return CustomError(AptErrorInvalidParameters, "No package names")
	}
	return withMutex(func() error {
		cNames := makeCStringArray(names)
		defer freeCStringArray(cNames)
		res := C.apt_transaction_reinstall(tx.ptr, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(names)))
		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		return nil
	})
}

// DistUpgrade помечает транзакцию как обновление системы
func (tx *Transaction) DistUpgrade() error {
	return withMutex(func() error {
		res := C.apt_transaction_dist_upgrade(tx.ptr)
		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		return nil
	})
}

// AutoRemove помечает транзакцию как автоматическое удаление неиспользуемых пакетов
func (tx *Transaction) AutoRemove() error {
	return withMutex(func() error {
		res := C.apt_transaction_autoremove(tx.ptr)
		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		return nil
	})
}

// Plan выполняет симуляцию транзакции
func (tx *Transaction) Plan() (*PackageChanges, error) {
	var changes *PackageChanges
	err := withMutex(func() error {
		var cc C.AptPackageChanges
		res := C.apt_transaction_plan(tx.ptr, &cc)
		defer C.apt_free_package_changes(&cc)

		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}

		changes = convertPackageChanges(&cc)
		return nil
	})
	return changes, err
}

// Execute выполняет транзакцию
func (tx *Transaction) Execute(handler ProgressHandler, downloadOnly bool) error {
	return withMutex(func() error {
		var userData C.uintptr_t
		if handler != nil {
			handle := cgoRuntime.NewHandle(handler)
			defer handle.Delete()
			userData = C.uintptr_t(handle)
			C.apt_use_go_progress_callback(userData)
		}
		res := C.apt_transaction_execute(tx.ptr, nil, userData, C.bool(downloadOnly))
		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		return nil
	})
}
