package system

import (
	"context"
	"errors"
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/testutil"
	aptLib "altlinux.space/alt-atomic/apm/pkg/apt/lib"
)

func TestCheckInstall(t *testing.T) {
	t.Run("success returns package changes", func(t *testing.T) {
		changes := &aptLib.PackageChanges{
			NewInstalledCount:    1,
			NewInstalledPackages: []string{"vim"},
		}
		apt := &mockAptActions{
			prepareInstall: []string{"vim"},
			findChanges:    changes,
		}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		resp, err := actions.CheckInstall(context.Background(), []string{"vim"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Info.NewInstalledCount != 1 {
			t.Errorf("expected 1 new install, got %d", resp.Info.NewInstalledCount)
		}
	})

	t.Run("empty packages returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, &mockAptDB{}, nil)
		_, err := actions.CheckInstall(context.Background(), []string{})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("prepare error returns apt error", func(t *testing.T) {
		apt := &mockAptActions{prepareErr: errors.New("bad package spec")}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		_, err := actions.CheckInstall(context.Background(), []string{"bad+"})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})

	t.Run("find error returns apt error", func(t *testing.T) {
		apt := &mockAptActions{
			prepareInstall: []string{"vim"},
			findErr:        errors.New("dependency conflict"),
		}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		_, err := actions.CheckInstall(context.Background(), []string{"vim"})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})
}

func TestCheckRemove(t *testing.T) {
	t.Run("success shows removal candidates with dependencies", func(t *testing.T) {
		changes := &aptLib.PackageChanges{
			RemovedCount:    2,
			RemovedPackages: []string{"vim", "vim-common"},
		}
		apt := &mockAptActions{checkRemoveRes: changes}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		resp, err := actions.CheckRemove(context.Background(), []string{"vim"}, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Info.RemovedCount != 2 {
			t.Errorf("expected 2 removed, got %d", resp.Info.RemovedCount)
		}
	})

	t.Run("apt error propagates", func(t *testing.T) {
		apt := &mockAptActions{checkRemoveErr: errors.New("cannot remove essential")}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		_, err := actions.CheckRemove(context.Background(), []string{"glibc"}, false, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})
}

func TestCheckUpgrade(t *testing.T) {
	t.Run("success shows available upgrades", func(t *testing.T) {
		changes := &aptLib.PackageChanges{UpgradedCount: 15}
		apt := &mockAptActions{checkUpgradeRes: changes}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		resp, err := actions.CheckUpgrade(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Info.UpgradedCount != 15 {
			t.Errorf("expected 15 upgrades, got %d", resp.Info.UpgradedCount)
		}
	})

	t.Run("apt error propagates", func(t *testing.T) {
		apt := &mockAptActions{checkUpgradeErr: errors.New("repo unreachable")}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		_, err := actions.CheckUpgrade(context.Background())
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})
}

func TestCheckReinstall(t *testing.T) {
	t.Run("empty packages returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, &mockAptDB{}, nil)
		_, err := actions.CheckReinstall(context.Background(), []string{})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("success returns reinstall changes", func(t *testing.T) {
		changes := &aptLib.PackageChanges{NewInstalledCount: 1}
		apt := &mockAptActions{
			prepareInstall: []string{"bash"},
			findChanges:    changes,
		}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		resp, err := actions.CheckReinstall(context.Background(), []string{"bash"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Info.NewInstalledCount != 1 {
			t.Errorf("expected 1 reinstall, got %d", resp.Info.NewInstalledCount)
		}
	})
}
