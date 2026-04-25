package altfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSyncConfig(t *testing.T) {
	input := []byte(`sync:
  groups:
    - docker
    - audio
    - video
  users:
    - dm
    - testuser
`)
	cfg, err := ParseSyncConfig(input)
	if err != nil {
		t.Fatalf("ParseSyncConfig: %v", err)
	}

	if len(cfg.Sync.Groups) != 3 {
		t.Errorf("expected 3 groups, got %d", len(cfg.Sync.Groups))
	}
	if len(cfg.Sync.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(cfg.Sync.Users))
	}
	if cfg.Sync.Groups[0] != "docker" {
		t.Errorf("expected docker, got %s", cfg.Sync.Groups[0])
	}
}

func TestParseSyncConfigNoUsers(t *testing.T) {
	input := []byte(`sync:
  groups:
    - docker
    - audio
`)
	cfg, err := ParseSyncConfig(input)
	if err != nil {
		t.Fatalf("ParseSyncConfig: %v", err)
	}

	if len(cfg.Sync.Groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(cfg.Sync.Groups))
	}
	if len(cfg.Sync.Users) != 0 {
		t.Errorf("expected 0 users, got %d", len(cfg.Sync.Users))
	}
}

func TestReadSyncConfigs(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "desktop.yaml"), []byte(`sync:
  groups:
    - docker
    - audio
  users:
    - dm
`), 0644)

	os.WriteFile(filepath.Join(dir, "extra.yml"), []byte(`sync:
  groups:
    - libvirt
`), 0644)

	// Не yaml — должен быть проигнорирован
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a config"), 0644)

	svc := newTestService(dir)
	configs, err := svc.ReadSyncConfigs(dir)
	if err != nil {
		t.Fatalf("ReadSyncConfigs: %v", err)
	}

	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
}

func TestReadSyncConfigsDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	dirMissing := filepath.Join(t.TempDir(), "nonexistent")

	os.WriteFile(filepath.Join(dir1, "desktop.yaml"), []byte(`sync:
  groups:
    - docker
    - audio
  users:
    - dm
`), 0644)

	os.WriteFile(filepath.Join(dir2, "extra.yaml"), []byte(`sync:
  groups:
    - libvirt
`), 0644)

	svc := newTestService(t.TempDir())

	configs, err := svc.ReadSyncConfigsDirs([]string{dir1, dirMissing, dir2})
	if err != nil {
		t.Fatalf("ReadSyncConfigsDirs: %v", err)
	}

	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}

	allGroups := make(map[string]bool)
	for _, c := range configs {
		for _, g := range c.Sync.Groups {
			allGroups[g] = true
		}
	}

	for _, expected := range []string{"docker", "audio", "libvirt"} {
		if !allGroups[expected] {
			t.Errorf("expected group %s not found", expected)
		}
	}
}

func TestReadSyncConfigsDirsAllMissing(t *testing.T) {
	svc := newTestService(t.TempDir())

	configs, err := svc.ReadSyncConfigsDirs([]string{
		filepath.Join(t.TempDir(), "a"),
		filepath.Join(t.TempDir(), "b"),
	})
	if err != nil {
		t.Fatalf("ReadSyncConfigsDirs: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("expected 0 configs, got %d", len(configs))
	}
}
