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

func TestParseSyncConfigSelectors(t *testing.T) {
	input := []byte(`sync:
  groups:
    - docker
  users:
    - dm
    - name: bob
    - uid: 1001
    - uid_range:
        min: 1001
        max: 10005
    - uid_range:
        min: 2000
    - uid_range: {}
`)
	cfg, err := ParseSyncConfig(input)
	if err != nil {
		t.Fatalf("ParseSyncConfig: %v", err)
	}

	users := cfg.Sync.Users
	if len(users) != 6 {
		t.Fatalf("expected 6 selectors, got %d", len(users))
	}

	empty := users[5]
	if empty.UIDRange == nil {
		t.Fatalf("selector 5: expected uid_range, got %+v", empty)
	}
	if lo, hi, err := empty.UIDRange.Bounds(); err != nil || lo != 1000 || hi != 60000 {
		t.Errorf("selector 5: expected default bounds 1000-60000, got %d-%d (%v)", lo, hi, err)
	}

	if users[0].Name != "dm" {
		t.Errorf("selector 0: expected name dm, got %+v", users[0])
	}
	if users[1].Name != "bob" {
		t.Errorf("selector 1: expected name bob, got %+v", users[1])
	}
	users = users[1:]
	if users[1].UID == nil || *users[1].UID != 1001 {
		t.Errorf("selector 1: expected uid 1001, got %+v", users[1])
	}
	if users[2].UIDRange == nil || users[2].UIDRange.Min != 1001 || users[2].UIDRange.Max != 10005 {
		t.Errorf("selector 2: expected uid_range 1001-10005, got %+v", users[2])
	}

	if users[3].UIDRange == nil {
		t.Fatalf("selector 3: expected uid_range, got %+v", users[3])
	}
	lo, hi, err := users[3].UIDRange.Bounds()
	if err != nil {
		t.Fatalf("Bounds: %v", err)
	}
	if lo != 2000 || hi != 60000 {
		t.Errorf("selector 3: expected bounds 2000-60000, got %d-%d", lo, hi)
	}
}

func TestParseSyncConfigSelectorErrors(t *testing.T) {
	cases := map[string]string{
		"uid and uid_range together": `sync:
  groups: [docker]
  users:
    - uid: 1001
      uid_range:
        min: 1000
`,
		"empty object": `sync:
  groups: [docker]
  users:
    - {}
`,
		"name and uid together": `sync:
  groups: [docker]
  users:
    - name: dm
      uid: 1000
`,
		"empty name": `sync:
  groups: [docker]
  users:
    - ""
`,
		"null uid_range": `sync:
  groups: [docker]
  users:
    - uid_range:
`,
	}

	for name, input := range cases {
		if _, err := ParseSyncConfig([]byte(input)); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestUIDRangeBounds(t *testing.T) {
	lo, hi, err := UIDRange{}.Bounds()
	if err != nil {
		t.Fatalf("Bounds: %v", err)
	}
	if lo != 1000 || hi != 60000 {
		t.Errorf("expected default bounds 1000-60000, got %d-%d", lo, hi)
	}

	if _, _, err = (UIDRange{Min: 5000, Max: 1000}).Bounds(); err == nil {
		t.Error("expected error for min > max, got nil")
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
