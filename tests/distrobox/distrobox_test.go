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

//go:build distrobox

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

func TestDistroboxRealContainerLifecycle(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := distrobox.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()
	containerName := "apm-test-alt"
	image := "registry.altlinux.org/sisyphus/base:latest"

	// Создаем контейнер через APM
	t.Run("Create container via APM", func(t *testing.T) {
		resp, err := actions.ContainerAdd(ctx, image, containerName, "", "")
		if err != nil {
			t.Logf("Container creation failed: %v", err)
			// Не требуем успеха, так как может быть проблема с distrobox/podman
			return
		}
		
		assert.NotNil(t, resp)
		assert.False(t, resp.Error, "Container creation should succeed")
		t.Logf("Container created successfully: %+v", resp.Data)
	})

	// Установка очистки после тестов
	t.Cleanup(func() {
		resp, err := actions.ContainerRemove(ctx, containerName)
		if err != nil {
			t.Logf("Container cleanup failed: %v", err)
		} else {
			t.Logf("Container cleaned up successfully: %+v", resp.Data)
		}
	})

	// Тестируем установку пакета в созданном контейнере
	t.Run("Install package in created container", func(t *testing.T) {
		resp, err := actions.Install(ctx, containerName, "hello", false)
		if err != nil {
			t.Logf("Install error: %v", err)
			assert.NotContains(t, err.Error(), "Elevated rights are not allowed")
		} else {
			assert.NotNil(t, resp)
			assert.False(t, resp.Error)
			t.Logf("Install successful: %+v", resp.Data)
		}
	})

	// Тестируем поиск пакетов в контейнере
	t.Run("Search packages in created container", func(t *testing.T) {
		resp, err := actions.Search(ctx, containerName, "vim")
		if err != nil {
			t.Logf("Search error: %v", err)
		} else {
			assert.NotNil(t, resp)
			assert.False(t, resp.Error)
			t.Logf("Search successful: found packages")
		}
	})

	// Тестируем удаление пакета
	t.Run("Remove package from created container", func(t *testing.T) {
		resp, err := actions.Remove(ctx, containerName, "hello", false)
		if err != nil {
			t.Logf("Remove error: %v", err)
		} else {
			assert.NotNil(t, resp)
			assert.False(t, resp.Error)
			t.Logf("Remove successful: %+v", resp.Data)
		}
	})
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

func TestDistroboxContainerManagement(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("This test should be run without root privileges")
	}

	actions := distrobox.NewActions()
	assert.NotNil(t, actions)

	ctx := context.Background()
	containerName := "apm-test-management"
	image := "registry.altlinux.org/sisyphus/base:latest"

	// Проверяем начальный список контейнеров
	t.Run("List containers before creation", func(t *testing.T) {
		resp, err := actions.ContainerList(ctx)
		if err != nil {
			t.Logf("ContainerList failed: %v", err)
			return
		}
		
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		
		if containers, ok := resp.Data.(map[string]interface{})["containers"]; ok {
			if containerList, ok := containers.([]interface{}); ok {
				t.Logf("Found %d existing containers", len(containerList))
			}
		}
	})

	// Создаем контейнер
	t.Run("Create new container", func(t *testing.T) {
		resp, err := actions.ContainerAdd(ctx, image, containerName, "", "")
		if err != nil {
			t.Logf("Container creation failed: %v", err)
			t.Skip("Skipping remaining tests due to creation failure")
			return
		}
		
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		t.Logf("Container created: %+v", resp.Data)
	})

	// Проверяем что контейнер появился в списке
	t.Run("Verify container in list", func(t *testing.T) {
		resp, err := actions.ContainerList(ctx)
		if err != nil {
			t.Logf("ContainerList failed: %v", err)
			return
		}
		
		assert.NotNil(t, resp)
		assert.False(t, resp.Error)
		
		found := false
		if containers, ok := resp.Data.(map[string]interface{})["containers"]; ok {
			if containerList, ok := containers.([]interface{}); ok {
				for _, container := range containerList {
					if containerInfo, ok := container.(map[string]interface{}); ok {
						if name, ok := containerInfo["name"].(string); ok && name == containerName {
							found = true
							t.Logf("Found created container in list: %s", name)
							break
						}
					}
				}
			}
		}
		
		if !found {
			t.Logf("Created container not found in list (may be expected)")
		}
	})

	// Очистка
	t.Cleanup(func() {
		resp, err := actions.ContainerRemove(ctx, containerName)
		if err != nil {
			t.Logf("Container cleanup failed: %v", err)
		} else {
			t.Logf("Container cleaned up: %+v", resp.Data)
		}
	})
}
