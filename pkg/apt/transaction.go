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

package apt

import (
	"slices"
	"strings"

	"altlinux.space/alt-atomic/apm/pkg/apt/lib"
)

// TransactionSpec describes one package transaction
type TransactionSpec struct {
	AptGetArgs []string
	Install    []string
	Remove     []string
	Reinstall  []string

	Purge         bool
	RemoveDepends bool
	DownloadOnly  bool
	Idempotent    bool
	Inspect       []string
}

// Confirm decides whether planned changes are applied.
// It runs inside the operation with the APT mutex held, so it must not call Actions methods
type Confirm func(changes *lib.PackageChanges) (bool, error)

// Plan simulates the transaction without changing the system
func (a *Actions) Plan(spec TransactionSpec) (*lib.PackageChanges, error) {
	return a.runTransaction(spec, nil, nil)
}

// Apply plans, asks confirm and executes on the same cache; nil confirm applies unconditionally
func (a *Actions) Apply(spec TransactionSpec, confirm Confirm, handler lib.ProgressHandler) (*lib.PackageChanges, error) {
	if confirm == nil {
		confirm = func(*lib.PackageChanges) (bool, error) { return true, nil }
	}
	return a.runTransaction(spec, confirm, handler)
}

// runTransaction opens the cache once for plan and, when confirm allows, execute.
func (a *Actions) runTransaction(spec TransactionSpec, confirm Confirm, handler lib.ProgressHandler) (changes *lib.PackageChanges, err error) {
	spec = normalizeSpec(spec)
	if len(spec.AptGetArgs)+len(spec.Install)+len(spec.Remove)+len(spec.Reinstall) == 0 {
		return nil, lib.CustomError(lib.AptErrorInvalidParameters, "no packages specified")
	}

	var confirmErr error
	rpmArgs := slices.Concat(spec.AptGetArgs, spec.Install, spec.Reinstall)
	err = a.runOperation(OperationOptions{RpmArguments: rpmArgs}, func(system *lib.System) error {
		return withCache(system, false, func(cache *lib.Cache) error {
			tx, errTx := cache.NewTransaction()
			if errTx != nil {
				return errTx
			}
			defer tx.Close()

			if errFill := fillTransaction(tx, spec); errFill != nil {
				return errFill
			}

			planned, errPlan := tx.Plan()
			if errPlan != nil {
				return errPlan
			}
			for _, name := range spec.Inspect {
				info, errInfo := cache.GetPackageInfo(name)
				if errInfo != nil {
					return errInfo
				}
				planned.Inspected = append(planned.Inspected, info)
			}
			changes = planned
			if confirm == nil {
				return nil
			}

			ok, errConfirm := confirm(changes)
			if errConfirm != nil {
				confirmErr = errConfirm
				return nil
			}
			if !ok {
				return nil
			}
			return tx.Execute(handler, spec.DownloadOnly)
		})
	})
	if err != nil {
		return changes, err
	}
	return changes, confirmErr
}

// normalizeSpec trims names and drops empty ones
func normalizeSpec(spec TransactionSpec) TransactionSpec {
	spec.AptGetArgs = cleanNames(spec.AptGetArgs)
	spec.Install = cleanNames(spec.Install)
	spec.Remove = cleanNames(spec.Remove)
	spec.Reinstall = cleanNames(spec.Reinstall)
	spec.Inspect = cleanNames(spec.Inspect)
	return spec
}

func cleanNames(names []string) []string {
	var out []string
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func fillTransaction(tx *lib.Transaction, spec TransactionSpec) error {
	tx.SetOptions(spec.Purge, spec.RemoveDepends, spec.Idempotent)
	if len(spec.AptGetArgs) > 0 {
		if err := tx.AddAptGetArgs(spec.AptGetArgs); err != nil {
			return err
		}
	}
	if len(spec.Install) > 0 {
		if err := tx.Install(spec.Install); err != nil {
			return err
		}
	}
	if len(spec.Remove) > 0 {
		if err := tx.Remove(spec.Remove); err != nil {
			return err
		}
	}
	if len(spec.Reinstall) > 0 {
		if err := tx.Reinstall(spec.Reinstall); err != nil {
			return err
		}
	}
	return nil
}
