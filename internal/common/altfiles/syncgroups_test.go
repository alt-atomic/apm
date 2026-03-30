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

	configs, err := ReadSyncConfigs(dir)
	if err != nil {
		t.Fatalf("ReadSyncConfigs: %v", err)
	}

	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
}

func TestSyncGroupsAddNew(t *testing.T) {
	dir := t.TempDir()
	etcGroup := filepath.Join(dir, "etc_group")
	libGroup := filepath.Join(dir, "lib_group")

	os.WriteFile(etcGroup, []byte("root:x:0:\nwheel:x:10:dm\ndm:x:1000:\n"), 0644)
	os.WriteFile(libGroup, []byte("docker:x:948:\naudio:x:81:\n"), 0644)

	// Подменяем пути
	origEtc, origLib := EtcGroup, LibGroup
	EtcGroup, LibGroup = etcGroup, libGroup
	defer func() { EtcGroup, LibGroup = origEtc, origLib }()

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"docker", "audio"},
			Users:  []string{"dm"},
		},
	}}

	result, err := SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups: %v", err)
	}

	if result.Added != 2 {
		t.Errorf("expected 2 added, got %d", result.Added)
	}

	// Проверяем что группы создались в /etc/group
	data, _ := os.ReadFile(etcGroup)
	entries, _ := ParseGroup(data)

	grpMap := map[string]GroupEntry{}
	for _, e := range entries {
		grpMap[e.Name] = e
	}

	docker, ok := grpMap["docker"]
	if !ok {
		t.Fatal("docker not found in /etc/group")
	}
	if docker.GID != 948 {
		t.Errorf("docker GID: got %d, want 948", docker.GID)
	}
	if docker.Members != "dm" {
		t.Errorf("docker members: got %q, want %q", docker.Members, "dm")
	}
}

func TestSyncGroupsFixGID(t *testing.T) {
	dir := t.TempDir()
	etcGroup := filepath.Join(dir, "etc_group")
	libGroup := filepath.Join(dir, "lib_group")

	// /etc/group имеет неверный GID для docker (999 вместо 948)
	os.WriteFile(etcGroup, []byte("root:x:0:\ndocker:x:999:dm\n"), 0644)
	os.WriteFile(libGroup, []byte("docker:x:948:\n"), 0644)

	origEtc, origLib := EtcGroup, LibGroup
	EtcGroup, LibGroup = etcGroup, libGroup
	defer func() { EtcGroup, LibGroup = origEtc, origLib }()

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"docker"},
			Users:  []string{"dm"},
		},
	}}

	result, err := SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups: %v", err)
	}

	if result.Fixed != 1 {
		t.Errorf("expected 1 fixed, got %d", result.Fixed)
	}

	data, _ := os.ReadFile(etcGroup)
	entries, _ := ParseGroup(data)

	for _, e := range entries {
		if e.Name == "docker" {
			if e.GID != 948 {
				t.Errorf("docker GID after fix: got %d, want 948", e.GID)
			}
			if e.Members != "dm" {
				t.Errorf("docker members after fix: got %q, want %q", e.Members, "dm")
			}
		}
	}
}

func TestSyncGroupsAlreadyMember(t *testing.T) {
	dir := t.TempDir()
	etcGroup := filepath.Join(dir, "etc_group")
	libGroup := filepath.Join(dir, "lib_group")

	// dm уже в docker
	os.WriteFile(etcGroup, []byte("root:x:0:\ndocker:x:948:dm\n"), 0644)
	os.WriteFile(libGroup, []byte("docker:x:948:\n"), 0644)

	origEtc, origLib := EtcGroup, LibGroup
	EtcGroup, LibGroup = etcGroup, libGroup
	defer func() { EtcGroup, LibGroup = origEtc, origLib }()

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"docker"},
			Users:  []string{"dm"},
		},
	}}

	result, err := SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}
	if result.Added != 0 {
		t.Errorf("expected 0 added, got %d", result.Added)
	}
}

func TestSyncGroupsNonexistent(t *testing.T) {
	dir := t.TempDir()
	etcGroup := filepath.Join(dir, "etc_group")
	libGroup := filepath.Join(dir, "lib_group")

	os.WriteFile(etcGroup, []byte("root:x:0:\n"), 0644)
	os.WriteFile(libGroup, []byte("docker:x:948:\n"), 0644)

	origEtc, origLib := EtcGroup, LibGroup
	EtcGroup, LibGroup = etcGroup, libGroup
	defer func() { EtcGroup, LibGroup = origEtc, origLib }()

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"nonexistent"},
			Users:  []string{"dm"},
		},
	}}

	result, err := SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}
}

func TestFixNssPreservesOverlayAndFixesGID(t *testing.T) {
	dir := t.TempDir()
	etcGroup := filepath.Join(dir, "etc_group")
	libGroup := filepath.Join(dir, "lib_group")
	etcPasswd := filepath.Join(dir, "etc_passwd")
	libPasswd := filepath.Join(dir, "lib_passwd")

	// audio в /etc с неверным GID (82 вместо 81) и member-оверлейдом (dm)
	// video в /etc без уникальных members — должна удалиться
	os.WriteFile(etcGroup, []byte("root:x:0:\nwheel:x:10:dm\ndm:x:1000:\naudio:x:82:dm\nvideo:x:990:\n"), 0644)
	os.WriteFile(libGroup, []byte("audio:x:81:\nvideo:x:990:\n"), 0644)
	os.WriteFile(etcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(libPasswd, []byte("bin:x:1:1:bin:/:/dev/null\n"), 0644)

	etcNsswitch := filepath.Join(dir, "nsswitch.conf")
	os.WriteFile(etcNsswitch, []byte("passwd: files\ngroup: files\n"), 0644)

	origEtcG, origLibG := EtcGroup, LibGroup
	origEtcP, origLibP := EtcPasswd, LibPasswd
	origNss := EtcNsswitch
	EtcGroup, LibGroup = etcGroup, libGroup
	EtcPasswd, LibPasswd = etcPasswd, libPasswd
	EtcNsswitch = etcNsswitch
	defer func() {
		EtcGroup, LibGroup = origEtcG, origLibG
		EtcPasswd, LibPasswd = origEtcP, origLibP
		EtcNsswitch = origNss
	}()

	_, err := ApplyFix()
	if err != nil {
		t.Fatalf("ApplyFix: %v", err)
	}

	data, _ := os.ReadFile(etcGroup)
	entries, _ := ParseGroup(data)

	grpMap := map[string]GroupEntry{}
	for _, e := range entries {
		grpMap[e.Name] = e
	}

	// audio сохранилась (member-оверлей dm) и GID исправлен на 81
	audio, ok := grpMap["audio"]
	if !ok {
		t.Fatal("audio should be preserved in /etc/group (has member overlay)")
	}
	if audio.GID != 81 {
		t.Errorf("audio GID: got %d, want 81 (should be fixed from /usr/lib)", audio.GID)
	}
	if audio.Members != "dm" {
		t.Errorf("audio members: got %q, want %q", audio.Members, "dm")
	}

	// video удалилась (нет уникальных members)
	if _, ok := grpMap["video"]; ok {
		t.Error("video should be removed from /etc/group (no unique members)")
	}
}

func TestSyncGroupsIdempotent(t *testing.T) {
	dir := t.TempDir()
	etcGroup := filepath.Join(dir, "etc_group")
	libGroup := filepath.Join(dir, "lib_group")

	os.WriteFile(etcGroup, []byte("root:x:0:\nwheel:x:10:dm\ndm:x:1000:\n"), 0644)
	os.WriteFile(libGroup, []byte("docker:x:948:\naudio:x:81:\n"), 0644)

	origEtc, origLib := EtcGroup, LibGroup
	EtcGroup, LibGroup = etcGroup, libGroup
	defer func() { EtcGroup, LibGroup = origEtc, origLib }()

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"docker", "audio"},
			Users:  []string{"dm"},
		},
	}}

	// Первый запуск
	result1, err := SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups first: %v", err)
	}
	if result1.Added != 2 {
		t.Errorf("first run: expected 2 added, got %d", result1.Added)
	}

	// Второй запуск — всё должно быть skipped
	result2, err := SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups second: %v", err)
	}
	if result2.Added != 0 {
		t.Errorf("second run: expected 0 added, got %d", result2.Added)
	}
	if result2.Skipped != 2 {
		t.Errorf("second run: expected 2 skipped, got %d", result2.Skipped)
	}
}
