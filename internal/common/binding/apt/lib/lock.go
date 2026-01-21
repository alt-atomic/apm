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
#include "apt_wrapper.h"
#include <stdlib.h>
*/
import "C"

import (
	"apm/internal/common/app"
	"fmt"
)

// LockStatus represents the status of APT/RPM locks
type LockStatus struct {
	IsLocked     bool
	CanAcquire   bool
	LockPID      int
	LockHolder   string
	LockFilePath string
	ErrorMessage string
}

// CheckLockStatus checks if APT locks can be acquired without actually acquiring them.
func CheckLockStatus() LockStatus {
	cStatus := C.apt_check_lock_status()
	defer C.apt_free_lock_status(&cStatus)

	status := LockStatus{
		IsLocked:   bool(cStatus.is_locked),
		CanAcquire: bool(cStatus.can_acquire),
		LockPID:    int(cStatus.lock_pid),
	}

	if cStatus.lock_holder != nil {
		status.LockHolder = C.GoString(cStatus.lock_holder)
	}
	if cStatus.lock_file_path != nil {
		status.LockFilePath = C.GoString(cStatus.lock_file_path)
	}
	if cStatus.error_message != nil {
		status.ErrorMessage = C.GoString(cStatus.error_message)
	}

	return status
}

// ErrLocked is returned when APT/RPM is locked by another process
type ErrLocked struct {
	Status LockStatus
}

func (e *ErrLocked) Error() string {
	lockFile := e.Status.LockFilePath

	lockSuffix := ""
	if lockFile != "" {
		lockSuffix = fmt.Sprintf(", %s: %s", app.T_("lock file"), lockFile)
	}

	if e.Status.LockHolder != "" && e.Status.LockPID > 0 {
		return fmt.Sprintf(app.T_("Package operations are locked by %s (PID %d)%s"),
			e.Status.LockHolder, e.Status.LockPID, lockSuffix)
	}
	if e.Status.LockPID > 0 {
		return fmt.Sprintf(app.T_("Package operations are locked by process with PID %d%s"),
			e.Status.LockPID, lockSuffix)
	}
	if e.Status.ErrorMessage != "" {
		if lockSuffix != "" {
			return fmt.Sprintf("%s%s", e.Status.ErrorMessage, lockSuffix)
		}
		return e.Status.ErrorMessage
	}
	return fmt.Sprintf(app.T_("Package operations are locked by another process%s"), lockSuffix)
}

// CheckLockOrError checks if APT/RPM is locked and returns an error if it is.
func CheckLockOrError() error {
	status := CheckLockStatus()
	if status.IsLocked || !status.CanAcquire {
		return &ErrLocked{Status: status}
	}
	return nil
}
