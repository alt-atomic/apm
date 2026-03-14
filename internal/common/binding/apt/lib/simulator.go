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

import "unsafe"

// planWithTransaction создаёт транзакцию, выполняет setup, планирует и опционально вызывает afterPlan
func (c *Cache) planWithTransaction(setup func(tx *C.AptTransaction) C.AptResult, afterPlan func() error) (*PackageChanges, error) {
	var changes *PackageChanges
	err := withMutex(func() error {
		var tx *C.AptTransaction
		res := C.apt_transaction_new(c.Ptr, &tx)
		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}
		defer C.apt_transaction_free(tx)

		if res = setup(tx); res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}

		var cc C.AptPackageChanges
		res = C.apt_transaction_plan(tx, &cc)
		defer C.apt_free_package_changes(&cc)

		if res.code != C.APT_SUCCESS {
			return ErrorFromResult(res)
		}

		changes = convertPackageChanges(&cc)

		if afterPlan != nil {
			return afterPlan()
		}
		return nil
	})
	return changes, err
}

// SimulateDistUpgrade симулирует обновление системы
func (c *Cache) SimulateDistUpgrade() (*PackageChanges, error) {
	return c.planWithTransaction(func(tx *C.AptTransaction) C.AptResult {
		return C.apt_transaction_dist_upgrade(tx)
	}, nil)
}

// SimulateAutoRemove симулирует автоматическое удаление неиспользуемых пакетов
func (c *Cache) SimulateAutoRemove() (*PackageChanges, error) {
	return c.planWithTransaction(func(tx *C.AptTransaction) C.AptResult {
		return C.apt_transaction_autoremove(tx)
	}, nil)
}

// SimulateInstall симулирует установку пакетов
func (c *Cache) SimulateInstall(packageNames []string) (*PackageChanges, error) {
	if len(packageNames) == 0 {
		return nil, CustomError(AptErrorInvalidParameters, "Invalid parameters")
	}

	return c.planWithTransaction(func(tx *C.AptTransaction) C.AptResult {
		cNames := makeCStringArray(packageNames)
		defer freeCStringArray(cNames)
		return C.apt_transaction_install(tx, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(packageNames)))
	}, nil)
}

// SimulateRemove симулирует удаление пакетов
func (c *Cache) SimulateRemove(packageNames []string, purge bool, depends bool) (*PackageChanges, error) {
	if len(packageNames) == 0 {
		return nil, CustomError(AptErrorInvalidParameters, "Invalid parameters")
	}

	return c.planWithTransaction(func(tx *C.AptTransaction) C.AptResult {
		cNames := makeCStringArray(packageNames)
		defer freeCStringArray(cNames)
		return C.apt_transaction_remove(tx, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(packageNames)),
			C.bool(purge), C.bool(depends))
	}, nil)
}

// SimulateReinstall симулирует переустановку пакетов
func (c *Cache) SimulateReinstall(packageNames []string) (*PackageChanges, error) {
	if len(packageNames) == 0 {
		return nil, CustomError(AptErrorInvalidParameters, "Invalid parameters")
	}

	return c.planWithTransaction(func(tx *C.AptTransaction) C.AptResult {
		cNames := makeCStringArray(packageNames)
		defer freeCStringArray(cNames)
		return C.apt_transaction_reinstall(tx, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(packageNames)))
	}, nil)
}

// SimulateChange симулирует установку и удаление пакетов в одной транзакции
func (c *Cache) SimulateChange(installNames []string, removeNames []string, purge bool, depends bool) (*PackageChanges, error) {
	if len(installNames) == 0 && len(removeNames) == 0 {
		return nil, CustomError(AptErrorInvalidParameters, "Invalid parameters")
	}

	return c.planWithTransaction(func(tx *C.AptTransaction) C.AptResult {
		if len(installNames) > 0 {
			cNames := makeCStringArray(installNames)
			defer freeCStringArray(cNames)
			res := C.apt_transaction_install(tx, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(installNames)))
			if res.code != C.APT_SUCCESS {
				return res
			}
		}

		if len(removeNames) > 0 {
			cNames := makeCStringArray(removeNames)
			defer freeCStringArray(cNames)
			res := C.apt_transaction_remove(tx, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(removeNames)),
				C.bool(purge), C.bool(depends))
			if res.code != C.APT_SUCCESS {
				return res
			}
		}

		return C.AptResult{code: C.APT_SUCCESS}
	}, nil)
}

// SimulateChangeWithRpmInfo симуляция изменений с получением информации о RPM файлах за одну сессию кеша
func (c *Cache) SimulateChangeWithRpmInfo(installNames []string, removeNames []string, purge bool, depends bool, rpmFiles []string) (*PackageChanges, []*PackageInfo, error) {
	if len(installNames) == 0 && len(removeNames) == 0 {
		return nil, nil, CustomError(AptErrorInvalidParameters, "Invalid parameters")
	}

	var rpmInfos []*PackageInfo

	changes, err := c.planWithTransaction(func(tx *C.AptTransaction) C.AptResult {
		if len(installNames) > 0 {
			cNames := makeCStringArray(installNames)
			defer freeCStringArray(cNames)
			res := C.apt_transaction_install(tx, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(installNames)))
			if res.code != C.APT_SUCCESS {
				return res
			}
		}

		if len(removeNames) > 0 {
			cNames := makeCStringArray(removeNames)
			defer freeCStringArray(cNames)
			res := C.apt_transaction_remove(tx, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(removeNames)),
				C.bool(purge), C.bool(depends))
			if res.code != C.APT_SUCCESS {
				return res
			}
		}

		return C.AptResult{code: C.APT_SUCCESS}
	}, func() error {
		for _, rpmFile := range rpmFiles {
			cname := C.CString(rpmFile)
			var ci C.AptPackageInfo
			res := C.apt_package_get(c.Ptr, cname, &ci)
			C.free(unsafe.Pointer(cname))
			if res.code != C.APT_SUCCESS {
				C.apt_package_free(&ci)
				return ErrorFromResult(res)
			}
			info := &PackageInfo{}
			info.fromCStruct(&ci)
			C.apt_package_free(&ci)
			rpmInfos = append(rpmInfos, info)
		}
		return nil
	})
	return changes, rpmInfos, err
}
