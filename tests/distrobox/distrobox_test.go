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

package distrobox_test

import (
	"apm/internal/distrobox"
	"context"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDistroboxActionsCreation(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := distrobox.NewActions()
	assert.NotNil(t, actions)
}

func TestDistroboxInstallRequiresNonRoot(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := distrobox.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Install(ctx, "test-container", "hello", false)
	if err != nil {
		t.Logf("Install error (may be expected if container not exists): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are not allowed")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Install successful: %+v", resp.Data)
	}
}

func TestDistroboxRemoveRequiresNonRoot(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := distrobox.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Remove(ctx, "test-container", "nonexistent-package", false)
	if err != nil {
		t.Logf("Remove error (may be expected if container not exists): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are not allowed")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Remove successful")
	}
}

func TestDistroboxUpdateRequiresNonRoot(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := distrobox.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Update(ctx, "test-container")
	if err != nil {
		t.Logf("Update error (may be expected if container not exists): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are not allowed")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Update successful")
	}
}

func TestDistroboxInfoRequiresNonRoot(t *testing.T) {
	// Пропускаем если запущено от root
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := distrobox.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Info(ctx, "test-container", "hello")
	if err != nil {
		t.Logf("Info error (may be expected if container not exists): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are not allowed")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Info successful")
	}
}

func TestDistroboxSearchRequiresNonRoot(t *testing.T) {
	// Пропускаем если запущено от root
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := distrobox.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.Search(ctx, "test-container", "hello")
	if err != nil {
		t.Logf("Search error (may be expected if container not exists): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are not allowed")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Search successful")
	}
}

func TestDistroboxListRequiresNonRoot(t *testing.T) {
	// Пропускаем если запущено от root
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := distrobox.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	params := distrobox.ListParams{
		Container: "test-container",
		Limit:     10,
		Offset:    0,
	}
	resp, err := actions.List(ctx, params)
	if err != nil {
		t.Logf("List error (may be expected if container not exists): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are not allowed")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("List successful")
	}
}

func TestDistroboxContainerListRequiresNonRoot(t *testing.T) {
	// Пропускаем если запущено от root
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := distrobox.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()

	resp, err := actions.ContainerList(ctx)
	if err != nil {
		t.Logf("ContainerList error (may be expected): %v", err)
		assert.NotContains(t, err.Error(), "Elevated rights are not allowed")
	} else {
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("ContainerList successful")
	}
}
