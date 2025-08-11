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

package system

import (
	"apm/internal/system"
	"apm/lib"
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

const testPackage = "hello"

func isInstalled(pkg string) bool {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s rpm -q %s", lib.Env.CommandPrefix, pkg))
	return cmd.Run() == nil
}

func TestNonAtomicInstallRemoveHello(t *testing.T) {
	if lib.Env.IsAtomic {
		t.Skip("This test is available only for non-atomic systems")
	}
	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	ctx := context.Background()
	actions := system.NewActions()
	assert.NotNil(t, actions)

	if isInstalled(testPackage) {
		_, err := actions.Remove(ctx, []string{testPackage}, false, false)
		if err != nil {
			t.Fatalf("Remove failed: %v", err)
		}
	}
	t.Cleanup(func() {
		if isInstalled(testPackage) {
			_, err := actions.Remove(ctx, []string{testPackage}, false, false)
			if err != nil {
				t.Fatalf("Remove failed: %v", err)
			}
		}
	})

	resp, err := actions.Install(ctx, []string{testPackage}, false)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Error)

	resp, err = actions.Remove(ctx, []string{testPackage}, false, false)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Error)
}

// TestNonAtomicUpdate тестирует обновление системы в неатомарной системе
func TestNonAtomicUpdate(t *testing.T) {
	if lib.Env.IsAtomic {
		t.Skip("This test is available only for non-atomic systems")
	}

	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Update(ctx)
	if err != nil {
		t.Logf("Update error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Update successful")
	}
}

// TestNonAtomicUpgrade тестирует обновление системы в неатомарной системе
func TestNonAtomicUpgrade(t *testing.T) {
	if lib.Env.IsAtomic {
		t.Skip("This test is available only for non-atomic systems")
	}

	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Upgrade(ctx)
	if err != nil {
		t.Logf("Upgrade error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Upgrade successful")
	}
}

// TestNonAtomicUpdateKernel тестирует обновление ядра в неатомарной системе
func TestNonAtomicUpdateKernel(t *testing.T) {
	if lib.Env.IsAtomic {
		t.Skip("This test is available only for non-atomic systems")
	}

	if syscall.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Run with sudo.")
	}

	actions := system.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.UpdateKernel(ctx)
	if err != nil {
		t.Logf("UpdateKernel error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are required")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("UpdateKernel successful")
	}
}
