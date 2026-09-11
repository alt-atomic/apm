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

// planWithTransaction creates a transaction, runs setup, plans and optionally calls afterPlan
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

// SimulateDistUpgrade simulates a system upgrade
func (c *Cache) SimulateDistUpgrade() (*PackageChanges, error) {
	return c.planWithTransaction(func(tx *C.AptTransaction) C.AptResult {
		return C.apt_transaction_dist_upgrade(tx)
	}, nil)
}

// SimulateAutoRemove simulates removal of unused packages
func (c *Cache) SimulateAutoRemove() (*PackageChanges, error) {
	return c.planWithTransaction(func(tx *C.AptTransaction) C.AptResult {
		return C.apt_transaction_autoremove(tx)
	}, nil)
}
