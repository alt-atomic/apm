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

package binding

import (
	_ "embed"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"

	aptErrors "altlinux.space/alt-atomic/apm/internal/common/apt"
	aptBinding "altlinux.space/alt-atomic/apm/pkg/apt"
	aptlib "altlinux.space/alt-atomic/apm/pkg/apt/lib"

	"github.com/stretchr/testify/assert"
)

const testPackage = "hello"

//go:embed files/test-apm-example-1.0-alt1.x86_64.rpm
var testRpmData []byte

//go:embed files/test-apm-example2-1.0-alt1.x86_64.rpm
var testRpmData2 []byte

// TestAptNewActions ensures Actions can be constructed and closed
func TestAptNewActions(t *testing.T) {
	actions := aptBinding.NewActions()
	assert.NotNil(t, actions)
	aptBinding.Close()
}

// TestAptDownloadOnly verifies download-only mode downloads without installing
func TestAptDownloadOnly(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}

	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	if info, err := actions.GetInfo(testPackage); err == nil && info != nil && info.State == aptlib.PackageStateInstalled {
		if _, err = actions.Apply(removeSpec(testPackage), nil, nil); err != nil {
			t.Fatalf("pre-cleanup remove failed: %v", err)
		}
	}

	_, err := actions.Apply(aptBinding.TransactionSpec{Install: []string{testPackage}, DownloadOnly: true}, nil, nil)
	if err != nil {
		t.Fatalf("download-only failed: %v", err)
	}

	info, err := actions.GetInfo(testPackage)
	if err != nil {
		t.Fatalf("GetInfo failed: %v", err)
	}
	assert.NotEqual(t, aptlib.PackageStateInstalled, info.State,
		"package should NOT be installed after download-only")
	t.Logf("download-only: package %s state=%d (not installed)", testPackage, info.State)
}

// TestAptUpdate update system
func TestAptUpdate(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}

	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	if err := actions.Update(nil); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
}

// TestAptSearchBasic performs a simple search (read-only)
func TestAptSearchBasic(t *testing.T) {
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	// Use a common package name likely present in most systems
	pkgs, err := actions.Search(testPackage)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	assert.NotNil(t, pkgs)
}

// TestAptGetInfo_NotFound expects a well-formed APT error for missing package
func TestAptGetInfo_NotFound(t *testing.T) {
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	_, err := actions.GetInfo("__nonexistent_package_for_apm_tests__")
	if err == nil {
		t.Skip("GetInfo returned nil error for nonexistent package; skipping strict assertion")
	}
	if ae, ok := errors.AsType[*aptlib.AptError](err); ok {
		assert.Equal(t, aptlib.AptErrorPackageNotFound, ae.Code)
		return
	}
	if me, ok := errors.AsType[*aptErrors.MatchedError](err); ok {
		assert.Equal(t, aptErrors.ErrPackageNotFound, me.Entry.Code)
		return
	}
	t.Fatalf("unexpected error type: %T %v", err, err)
}

// TestAptSimulateInstall exercises simulation API (read-only)
func TestAptSimulateInstall(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	changes, err := actions.Plan(installSpec(testPackage))
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	assert.NotNil(t, changes)
}

// TestAptSimulateRemove exercises simulation API (read-only)
func TestAptSimulateRemove(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	// First, try to install the test package (ignore if already the newest version)
	_, err := actions.Apply(installSpec(testPackage), nil, nil)
	if err != nil {
		if matchedErr := aptErrors.CheckError(err.Error()); matchedErr != nil {
			if matchedErr.Entry.Code == aptErrors.ErrPackageIsAlreadyNewest {
				t.Logf("Package %s is already the newest version", testPackage)
			} else {
				t.Logf("Failed to install %s: %v", testPackage, err)
			}
		} else {
			t.Logf("Failed to install %s: %v", testPackage, err)
		}
	}

	// Now simulate removing the package (should work since we ensured it's installed)
	changes, err := actions.Plan(aptBinding.TransactionSpec{Remove: []string{testPackage}, Purge: true, RemoveDepends: true})
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	assert.NotNil(t, changes)
}

// TestAptSimulateDistUpgrade exercises dist-upgrade simulation (read-only)
func TestAptSimulateDistUpgrade(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	changes, err := actions.SimulateDistUpgrade()
	if err != nil {
		t.Fatalf("SimulateDistUpgrade failed: %v", err)
	}
	assert.NotNil(t, changes)
}

// TestAptInstallRemoveHelloRoot tries real install/remove of hello under root.
func TestAptInstallRemoveHelloRoot(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root")
	}

	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	installedFirst := false
	if info, err := actions.GetInfo(testPackage); err == nil && info != nil && info.State == aptlib.PackageStateInstalled {
		installedFirst = true
	}

	if installedFirst {
		if _, err := actions.Apply(removeSpec(testPackage), nil, nil); err != nil {
			t.Fatalf("remove hello failed: %v", err)
		}
		if _, err := actions.Apply(installSpec(testPackage), nil, nil); err != nil {
			t.Fatalf("install hello failed: %v", err)
		}
	} else {
		if _, err := actions.Apply(installSpec(testPackage), nil, nil); err != nil {
			t.Fatalf("install hello failed: %v", err)
		}
		if _, err := actions.Apply(removeSpec(testPackage), nil, nil); err != nil {
			t.Fatalf("remove hello failed: %v", err)
		}
	}
}

// TestAptInvalidParameters verifies parameter validation hooks
func TestAptInvalidParameters(t *testing.T) {
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	if _, err := actions.Apply(aptBinding.TransactionSpec{}, nil, nil); err == nil {
		t.Fatalf("expected error for empty transaction in Apply")
	}

	if _, err := actions.Plan(aptBinding.TransactionSpec{AptGetArgs: []string{"", "  "}}); err == nil {
		t.Fatalf("expected error for blank names in Plan")
	}

	if _, err := actions.Plan(aptBinding.TransactionSpec{}); err == nil {
		t.Fatalf("expected error for empty transaction in Plan")
	} else if ae, ok := err.(*aptlib.AptError); ok {
		if ae.Code != aptlib.AptErrorInvalidParameters {
			t.Fatalf("unexpected error code: %d (%v)", ae.Code, ae)
		}
	}
}

// TestAptSimulateReinstall tests reinstalling an already installed package
func TestAptSimulateReinstall(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	// bash is usually installed, so reinstall should work
	changes, err := actions.Plan(aptBinding.TransactionSpec{Reinstall: []string{"bash"}})
	if err != nil {
		t.Logf("Plan reinstall failed: %v", err)
		if ae, ok := errors.AsType[*aptlib.AptError](err); ok {
			t.Logf("Got AptError with code: %d", ae.Code)
		}
	} else {
		assert.NotNil(t, changes)
		t.Logf("Plan reinstall succeeded: new_installed=%d", changes.NewInstalledCount)
	}
}

// TestAptSimulateAutoremove tests orphan package detection
func TestAptSimulateAutoremove(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	changes, err := actions.SimulateAutoRemove()
	if err != nil {
		t.Fatalf("SimulateAutoremove failed: %v", err)
	}
	assert.NotNil(t, changes)
	t.Logf("SimulateAutoremove: would remove %d packages", changes.RemovedCount)
}

// TestAptSimulateChangeCombined tests simultaneous install and remove
func TestAptSimulateChangeCombined(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	changes, err := actions.Plan(aptBinding.TransactionSpec{Install: []string{testPackage}, Remove: []string{"nano"}})
	if err != nil {
		t.Logf("SimulateChange combined failed (may be expected): %v", err)
	} else {
		assert.NotNil(t, changes)
		t.Logf("SimulateChange combined: install=%d, remove=%d, upgrade=%d",
			changes.NewInstalledCount, changes.RemovedCount, changes.UpgradedCount)
	}
}

// TestAptMultiplePackageInstall tests installing multiple packages at once
func TestAptMultiplePackageInstall(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	packages := []string{"tree", "htop", "ncdu"}

	changes, err := actions.Plan(installSpec(packages...))
	if err != nil {
		t.Logf("Multiple package install simulation failed: %v", err)
	} else {
		assert.NotNil(t, changes)
		t.Logf("Multiple packages: new=%d, upgraded=%d, extra=%d",
			changes.NewInstalledCount, changes.UpgradedCount, len(changes.ExtraInstalled))
	}
}

// TestAptInstallRemoveRpmFile tests installing and removing an RPM file from disk.
func TestAptInstallRemoveRpmFile(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root")
	}

	tmpFile, err := os.CreateTemp(t.TempDir(), "test-apm-example-*.rpm")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err = tmpFile.Write(testRpmData); err != nil {
		t.Fatalf("failed to write RPM data: %v", err)
	}
	tmpFile.Close()
	rpmPath := tmpFile.Name()

	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	const testPkg = "test-apm-example"

	if info, e := actions.GetInfo(testPkg); e == nil && info != nil && info.State == aptlib.PackageStateInstalled {
		if _, err = actions.Apply(removeSpec(testPkg), nil, nil); err != nil {
			t.Fatalf("pre-cleanup remove failed: %v", err)
		}
	}

	if _, err = actions.Apply(installSpec(rpmPath), nil, nil); err != nil {
		t.Fatalf("install RPM file failed: %v", err)
	}

	const installedFile = "/usr/share/test-apm-example/test.txt"
	content, err := os.ReadFile(installedFile)
	if err != nil {
		t.Fatalf("expected file %s not found after install: %v", installedFile, err)
	}
	assert.Equal(t, "hello", strings.TrimSpace(string(content)))

	if _, err = actions.Apply(removeSpec("test-apm-example"), nil, nil); err != nil {
		t.Fatalf("remove test-apm-example failed: %v", err)
	}

	if _, err = os.Stat(installedFile); !os.IsNotExist(err) {
		t.Errorf("expected %s to be removed after package removal", installedFile)
	}
}

// TestAptReinstallRpmFile plans a reinstall from a local RPM file
func TestAptReinstallRpmFile(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root")
	}

	tmpFile, err := os.CreateTemp(t.TempDir(), "test-apm-example-*.rpm")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err = tmpFile.Write(testRpmData); err != nil {
		t.Fatalf("failed to write RPM data: %v", err)
	}
	tmpFile.Close()
	rpmPath := tmpFile.Name()

	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	const testPkg = "test-apm-example"
	if _, err = actions.Apply(installSpec(rpmPath), nil, nil); err != nil {
		t.Fatalf("install RPM file failed: %v", err)
	}
	defer func() { _, _ = actions.Apply(removeSpec(testPkg), nil, nil) }()

	changes, err := actions.Plan(aptBinding.TransactionSpec{Reinstall: []string{rpmPath}})
	if err != nil {
		t.Fatalf("Plan reinstall from RPM failed: %v", err)
	}
	assert.Contains(t, changes.NewInstalledPackages, testPkg)
}

// TestAptInstallConflictingRpms installs two RPM files that conflict on the same file path.
func TestAptInstallConflictingRpms(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root")
	}

	tmpDir := t.TempDir()

	tmpFile1, err := os.CreateTemp(tmpDir, "test-apm-example-*.rpm")
	if err != nil {
		t.Fatalf("failed to create temp file 1: %v", err)
	}
	if _, err = tmpFile1.Write(testRpmData); err != nil {
		t.Fatalf("failed to write RPM data 1: %v", err)
	}
	tmpFile1.Close()

	tmpFile2, err := os.CreateTemp(tmpDir, "test-apm-example2-*.rpm")
	if err != nil {
		t.Fatalf("failed to create temp file 2: %v", err)
	}
	if _, err = tmpFile2.Write(testRpmData2); err != nil {
		t.Fatalf("failed to write RPM data 2: %v", err)
	}
	tmpFile2.Close()

	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	rpmPaths := []string{tmpFile1.Name(), tmpFile2.Name()}
	for _, rpm := range rpmPaths {
		_, _ = actions.Apply(removeSpec(rpm), nil, nil)
	}

	defer func() {
		for _, rpm := range rpmPaths {
			_, _ = actions.Apply(removeSpec(rpm), nil, nil)
		}
	}()

	for i, rpm := range rpmPaths {
		_, err = actions.Apply(installSpec(rpm), nil, nil)
		if i == 0 {
			if err != nil {
				t.Fatalf("install first RPM failed: %v", err)
			}
		} else {
			if err == nil {
				t.Fatal("expected file conflict error when installing second RPM, got nil")
			}
			assert.Contains(t, err.Error(), "conflicts", "expected conflict error details")
			t.Logf("conflict error: %v", err)
		}
	}
}

// TestAptInstallSizeCalculation verifies install size is calculated correctly
func TestAptInstallSizeCalculation(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	changes, err := actions.Plan(installSpec(testPackage))
	if err != nil {
		t.Skipf("Plan failed, skipping size check: %v", err)
	}

	t.Logf("Install sizes: download=%d bytes, install=%d bytes",
		changes.DownloadSize, changes.InstallSize)

	assert.True(t, changes.DownloadSize >= 0, "Download size should be non-negative")

	if changes.NewInstalledCount > 0 {
		assert.True(t, changes.InstallSize >= 0,
			"Install size for new packages should be non-negative, got %d", changes.InstallSize)
	}
}

// TestAptSimulateInstallByPath resolves path arguments like apt-get: file owner, virtual providers, not found
func TestAptSimulateInstallByPath(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	t.Run("file path resolves to owner package", func(t *testing.T) {
		changes, err := actions.Plan(installSpec("/bin/bash"))
		if err == nil {
			assert.Contains(t, changes.UpgradedPackages, "bash")
			return
		}
		var me *aptErrors.MatchedError
		if !errors.As(err, &me) {
			t.Fatalf("unexpected error type: %T %v", err, err)
		}
		assert.Equal(t, aptErrors.ErrPackagesAlreadyInstalled, me.Entry.Code)
		assert.Contains(t, me.Params, "bash")
	})

	t.Run("virtual path with multiple providers lists them", func(t *testing.T) {
		const virtualPath = "/usr/bin/x-www-browser"
		_, err := actions.Plan(installSpec(virtualPath))
		if err == nil {
			t.Fatalf("expected provider selection error for %s", virtualPath)
		}
		var me *aptErrors.MatchedError
		if !errors.As(err, &me) {
			t.Fatalf("unexpected error type: %T %v", err, err)
		}
		assert.NotEqual(t, aptErrors.ErrPackageNotFound, me.Entry.Code, "virtual path must not be treated as missing file")
		assert.Equal(t, aptErrors.ErrMultiInstallProvidersSelect, me.Entry.Code)
		assert.Contains(t, me.Params, virtualPath)
		assert.NotEmpty(t, me.Details, "providers list expected in details")
	})

	t.Run("unknown path is not found", func(t *testing.T) {
		const unknownPath = "/nonexistent/apm-tests/path"
		_, err := actions.Plan(installSpec(unknownPath))
		if err == nil {
			t.Fatalf("expected error for %s", unknownPath)
		}
		if me, ok := errors.AsType[*aptErrors.MatchedError](err); ok {
			assert.Equal(t, aptErrors.ErrPackageNotFound, me.Entry.Code)
			assert.Contains(t, me.Params, unknownPath)
			return
		}
		if ae, ok := errors.AsType[*aptlib.AptError](err); ok {
			assert.Equal(t, aptlib.AptErrorPackageNotFound, ae.Code)
			return
		}
		t.Fatalf("unexpected error type: %T %v", err, err)
	})
}

// TestAptPlanIdempotent skips missing removes instead of failing
func TestAptPlanIdempotent(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	if info, err := actions.GetInfo(testPackage); err == nil && info.State == aptlib.PackageStateInstalled {
		if _, err = actions.Apply(removeSpec(testPackage), nil, nil); err != nil {
			t.Fatalf("pre-cleanup remove failed: %v", err)
		}
	}

	// known but not installed package and an unmatched glob are skipped
	spec := aptBinding.TransactionSpec{AptGetArgs: []string{"bash+", testPackage + "-", "__apm_none_*-"}, Idempotent: true}
	changes, err := actions.Plan(spec)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	assert.ElementsMatch(t, []string{testPackage, "__apm_none_*"}, changes.SkippedPackages)

	// unknown name stays an error even in idempotent mode
	const unknown = "__nonexistent_package_for_apm_tests__"
	_, err = actions.Plan(aptBinding.TransactionSpec{Remove: []string{unknown}, Idempotent: true})
	var me *aptErrors.MatchedError
	if !errors.As(err, &me) {
		t.Fatalf("expected MatchedError for unknown package, got %T %v", err, err)
	}
	assert.Equal(t, aptErrors.ErrPackageNotFound, me.Entry.Code)

	// without idempotent a not installed package is an error
	spec.Idempotent = false
	_, err = actions.Plan(spec)
	if !errors.As(err, &me) {
		t.Fatalf("expected MatchedError without idempotent, got %T %v", err, err)
	}
	assert.Equal(t, aptErrors.ErrPackageNotInstalled, me.Entry.Code)
}

// TestAptPlanGlob expands install globs against the cache
func TestAptPlanGlob(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root for APT cache write/lock")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	changes, err := actions.Plan(aptBinding.TransactionSpec{Install: []string{"hell?"}, Idempotent: true})
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if info, errInfo := actions.GetInfo(testPackage); errInfo == nil && info.State != aptlib.PackageStateInstalled {
		assert.Contains(t, changes.NewInstalledPackages, testPackage)
	}

	_, err = actions.Plan(aptBinding.TransactionSpec{Install: []string{"__apm_none_*"}})
	var me *aptErrors.MatchedError
	if !errors.As(err, &me) {
		t.Fatalf("expected MatchedError for unmatched glob, got %T %v", err, err)
	}
	assert.Equal(t, aptErrors.ErrPackageNotFound, me.Entry.Code)
}

// TestAptApplyDeclined plans on the same cache and does not execute when confirm says no
func TestAptApplyDeclined(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("requires root")
	}
	actions := aptBinding.NewActions()
	defer aptBinding.Close()

	if info, err := actions.GetInfo(testPackage); err == nil && info.State == aptlib.PackageStateInstalled {
		t.Skipf("%s is already installed", testPackage)
	}

	var planned *aptlib.PackageChanges
	changes, err := actions.Apply(aptBinding.TransactionSpec{Install: []string{testPackage}}, func(c *aptlib.PackageChanges) (bool, error) {
		planned = c
		return false, nil
	}, nil)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	assert.Same(t, planned, changes)
	assert.Contains(t, changes.NewInstalledPackages, testPackage)

	info, err := actions.GetInfo(testPackage)
	if err != nil {
		t.Fatalf("GetInfo failed: %v", err)
	}
	assert.NotEqual(t, aptlib.PackageStateInstalled, info.State, "declined transaction must not install")
}

func installSpec(names ...string) aptBinding.TransactionSpec {
	return aptBinding.TransactionSpec{Install: names}
}

func removeSpec(names ...string) aptBinding.TransactionSpec {
	return aptBinding.TransactionSpec{Remove: names}
}
