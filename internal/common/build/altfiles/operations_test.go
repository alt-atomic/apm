package altfiles

import (
	"os"
	"slices"
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/build/etcfiles"
)

func TestSyncGroupsAddNew(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nwheel:x:10:dm\ndm:x:1000:\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("docker:x:948:\naudio:x:81:\n"), 0644)

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"docker", "audio"},
			Users:  []UserSelector{{Name: "dm"}},
		},
	}}

	result, err := svc.SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups: %v", err)
	}

	if result.Added != 2 {
		t.Errorf("expected 2 added, got %d", result.Added)
	}

	data, _ := os.ReadFile(svc.cfg.EtcGroup)
	entries, _ := etcfiles.ParseGroup(data)

	grpMap := map[string]etcfiles.GroupEntry{}
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
	if !slices.Equal(docker.Members, []string{"dm"}) {
		t.Errorf("docker members: got %v, want [dm]", docker.Members)
	}
}

func TestSyncGroupsFixGID(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\ndocker:x:999:dm\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("docker:x:948:\n"), 0644)

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"docker"},
			Users:  []UserSelector{{Name: "dm"}},
		},
	}}

	result, err := svc.SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups: %v", err)
	}

	if result.Fixed != 1 {
		t.Errorf("expected 1 fixed, got %d", result.Fixed)
	}

	data, _ := os.ReadFile(svc.cfg.EtcGroup)
	entries, _ := etcfiles.ParseGroup(data)

	for _, e := range entries {
		if e.Name == "docker" {
			if e.GID != 948 {
				t.Errorf("docker GID after fix: got %d, want 948", e.GID)
			}
			if !slices.Equal(e.Members, []string{"dm"}) {
				t.Errorf("docker members after fix: got %v, want [dm]", e.Members)
			}
		}
	}
}

func TestSyncGroupsAlreadyMember(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\ndocker:x:948:dm\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("docker:x:948:\n"), 0644)

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"docker"},
			Users:  []UserSelector{{Name: "dm"}},
		},
	}}

	result, err := svc.SyncGroups(configs)
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
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("docker:x:948:\n"), 0644)

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"nonexistent"},
			Users:  []UserSelector{{Name: "dm"}},
		},
	}}

	result, err := svc.SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}
}

func TestSyncGroupsNonexistentUser(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nwheel:x:10:dm\ndm:x:1000:\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("docker:x:948:\n"), 0644)
	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"docker"},
			Users:  []UserSelector{{Name: "dm"}, {Name: "fakeuser"}},
		},
	}}

	result, err := svc.SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups: %v", err)
	}

	if result.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Added)
	}

	data, _ := os.ReadFile(svc.cfg.EtcGroup)
	entries, _ := etcfiles.ParseGroup(data)

	for _, e := range entries {
		if e.Name == "docker" {
			if !slices.Equal(e.Members, []string{"dm"}) {
				t.Errorf("docker members: got %v, want [dm] (fakeuser should be filtered)", e.Members)
			}
		}
	}
}

// Селекторы uid и uid_range резолвятся по объединённому passwd,
// дубликаты между селекторами схлопываются.
func TestSyncGroupsUIDSelectors(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte(
		"root:x:0:0:root:/root:/bin/bash\n"+
			"sysd:x:500:500:System:/var/empty:/sbin/nologin\n"+
			"dm:x:1000:1000::/home/dm:/bin/bash\n"+
			"alice:x:1001:1001::/home/alice:/bin/bash\n"+
			"bob:x:70000:70000::/home/bob:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.LibPasswd, []byte("svc:x:2000:2000:Svc:/var/lib/svc:/sbin/nologin\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nwheel:x:10:dm\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("docker:x:948:\n"), 0644)

	uid := 1001
	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"docker"},
			Users: []UserSelector{
				{UID: &uid},
				{UIDRange: &UIDRange{}}, // дефолт 1000-60000: dm, alice, svc; без sysd и bob
			},
		},
	}}

	result, err := svc.SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups: %v", err)
	}

	if result.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Added)
	}

	data, _ := os.ReadFile(svc.cfg.EtcGroup)
	entries, _ := etcfiles.ParseGroup(data)

	for _, e := range entries {
		if e.Name == "docker" {
			if !slices.Equal(e.Members, []string{"alice", "dm", "svc"}) {
				t.Errorf("docker members: got %v, want [alice dm svc]", e.Members)
			}
		}
	}
}

// Невалидный uid_range должен возвращать ошибку из SyncGroups.
func TestSyncGroupsInvalidUIDRange(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte("dm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("wheel:x:10:dm\n"), 0644)

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"wheel"},
			Users:  []UserSelector{{UIDRange: &UIDRange{Min: 5000, Max: 1000}}},
		},
	}}

	if _, err := svc.SyncGroups(configs); err == nil {
		t.Fatal("expected error for min > max, got nil")
	}
}

// Системный юзер из /usr/lib/passwd должен синкаться в системную группу из
// /usr/lib/group: в /etc/group появляется overlay-строка.
func TestSyncGroupsSystemUserFromLibPasswd(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.LibPasswd, []byte("appsvc:x:497:497:App Service:/var/lib/appsvc:/sbin/nologin\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nwheel:x:10:dm\ndm:x:1000:\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("appgrp:x:190:\n"), 0644)

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"appgrp"},
			Users:  []UserSelector{{Name: "appsvc"}},
		},
	}}

	result, err := svc.SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups: %v", err)
	}

	if result.Added != 1 {
		t.Fatalf("expected 1 added, got %d", result.Added)
	}

	data, _ := os.ReadFile(svc.cfg.EtcGroup)
	entries, _ := etcfiles.ParseGroup(data)

	grpMap := map[string]etcfiles.GroupEntry{}
	for _, e := range entries {
		grpMap[e.Name] = e
	}

	appgrp, ok := grpMap["appgrp"]
	if !ok {
		t.Fatal("appgrp overlay not written to /etc/group")
	}
	if appgrp.GID != 190 {
		t.Errorf("appgrp GID: got %d, want 190", appgrp.GID)
	}
	if !slices.Equal(appgrp.Members, []string{"appsvc"}) {
		t.Errorf("appgrp members: got %v, want [appsvc]", appgrp.Members)
	}
}

func TestSyncGroupsIdempotent(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nwheel:x:10:dm\ndm:x:1000:\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("docker:x:948:\naudio:x:81:\n"), 0644)

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"docker", "audio"},
			Users:  []UserSelector{{Name: "dm"}},
		},
	}}

	result1, err := svc.SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups first: %v", err)
	}
	if result1.Added != 2 {
		t.Errorf("first run: expected 2 added, got %d", result1.Added)
	}

	result2, err := svc.SyncGroups(configs)
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

// Осиротевшая локальная группа, чей GID переиспользован образом,
// удаляется: приоритет у групп из /usr/lib/group.
func TestSyncGroupsRemovesGIDConflict(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nwheel:x:10:dm\ndm:x:1000:\nusershares:x:946:dm\ntcpdump:x:945:\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("hashman:x:946:\ndocker:x:948:\n"), 0644)

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"docker"},
			Users:  []UserSelector{{Name: "dm"}},
		},
	}}

	result, err := svc.SyncGroups(configs)
	if err != nil {
		t.Fatalf("SyncGroups: %v", err)
	}

	if result.Removed != 1 {
		t.Errorf("expected 1 removed, got %d", result.Removed)
	}

	grpMap := groupByName(parseGroupFile(t, svc.cfg.EtcGroup))

	if _, ok := grpMap["usershares"]; ok {
		t.Error("usershares must be removed: GID 946 belongs to hashman in /usr/lib/group")
	}
	// Локальная группа без конфликта GID остаётся
	if _, ok := grpMap["tcpdump"]; !ok {
		t.Error("tcpdump must be preserved: GID 945 is free in /usr/lib/group")
	}
	if _, ok := grpMap["wheel"]; !ok {
		t.Error("wheel must be preserved")
	}
	if docker, ok := grpMap["docker"]; !ok || docker.GID != 948 {
		t.Errorf("docker must be synced with GID 948, got %+v", grpMap["docker"])
	}
}

func TestSyncGroupsKeepsProtectedOnGIDConflict(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nwheel:x:10:dm\ndm:x:1000:\n"), 0644)
	// Кривой образ занял GID wheel и обычного пользователя
	os.WriteFile(svc.cfg.LibGroup, []byte("badgrp:x:10:\nbadusr:x:1000:\n"), 0644)

	result, err := svc.SyncGroups(nil)
	if err != nil {
		t.Fatalf("SyncGroups: %v", err)
	}

	if result.Removed != 0 {
		t.Errorf("expected 0 removed, got %d", result.Removed)
	}

	grpMap := groupByName(parseGroupFile(t, svc.cfg.EtcGroup))
	for _, name := range []string{"root", "wheel", "dm"} {
		if _, ok := grpMap[name]; !ok {
			t.Errorf("%s must never be removed", name)
		}
	}
}

func TestFixNssRemovesGIDConflict(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nwheel:x:10:dm\ndm:x:1000:\nusershares:x:946:dm\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("hashman:x:946:\n"), 0644)
	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.LibPasswd, []byte("bin:x:1:1:bin:/:/dev/null\n"), 0644)
	os.WriteFile(svc.cfg.EtcNsswitch, []byte("passwd: files\ngroup: files\n"), 0644)

	if _, err := svc.ApplyFix(); err != nil {
		t.Fatalf("ApplyFix: %v", err)
	}

	grpMap := groupByName(parseGroupFile(t, svc.cfg.EtcGroup))
	if _, ok := grpMap["usershares"]; ok {
		t.Error("usershares must be removed: GID 946 belongs to hashman in /usr/lib/group")
	}
	if _, ok := grpMap["wheel"]; !ok {
		t.Error("wheel must be preserved")
	}
}

func TestFixNssRemovesUIDConflict(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte(
		"root:x:0:0:root:/root:/bin/bash\n"+
			"dm:x:1000:1000::/home/dm:/bin/bash\n"+
			"stapler-builder:x:975:952::/var/cache/stplr:/sbin/nologin\n"+
			"tcpdump:x:974:945::/dev/null:/dev/null\n"), 0644)
	os.WriteFile(svc.cfg.LibPasswd, []byte(
		"bin:x:1:1:bin:/:/dev/null\n"+
			"sshd:x:975:947::/var/empty:/dev/null\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nwheel:x:10:dm\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("sshd:x:947:\n"), 0644)
	os.WriteFile(svc.cfg.EtcNsswitch, []byte("passwd: files\ngroup: files\n"), 0644)

	result, err := svc.ApplyFix()
	if err != nil {
		t.Fatalf("ApplyFix: %v", err)
	}
	if result.RemovedUIDConflicts != 1 {
		t.Errorf("RemovedUIDConflicts: got %d, want 1", result.RemovedUIDConflicts)
	}

	data, _ := os.ReadFile(svc.cfg.EtcPasswd)
	entries, _ := etcfiles.ParsePasswd(data)
	byName := map[string]etcfiles.PasswdEntry{}
	for _, e := range entries {
		byName[e.Name] = e
	}

	if _, ok := byName["stapler-builder"]; ok {
		t.Error("stapler-builder must be removed: UID 975 belongs to sshd in /usr/lib/passwd")
	}
	if _, ok := byName["tcpdump"]; !ok {
		t.Error("tcpdump must be preserved: UID 974 is free in /usr/lib/passwd")
	}
	if _, ok := byName["root"]; !ok {
		t.Error("root must be preserved")
	}
	if _, ok := byName["dm"]; !ok {
		t.Error("dm must be preserved")
	}
}

func TestFixNssKeepsProtectedOnUIDConflict(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	// root и обычный пользователь не удаляются даже при совпадении UID с lib
	os.WriteFile(svc.cfg.EtcPasswd, []byte(
		"root:x:0:0:root:/root:/bin/bash\n"+
			"dm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.LibPasswd, []byte(
		"weird0:x:0:0::/:/dev/null\n"+
			"weird1000:x:1000:1000::/:/dev/null\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("bin:x:1:\n"), 0644)
	os.WriteFile(svc.cfg.EtcNsswitch, []byte("passwd: files\ngroup: files\n"), 0644)

	result, err := svc.ApplyFix()
	if err != nil {
		t.Fatalf("ApplyFix: %v", err)
	}
	if result.RemovedUIDConflicts != 0 {
		t.Errorf("RemovedUIDConflicts: got %d, want 0", result.RemovedUIDConflicts)
	}

	data, _ := os.ReadFile(svc.cfg.EtcPasswd)
	entries, _ := etcfiles.ParsePasswd(data)
	if len(entries) != 2 {
		t.Errorf("expected root and dm preserved, got %d entries", len(entries))
	}
}

func TestFixNssReportsRemovedGIDConflicts(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nusershares:x:946:dm\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("hashman:x:946:\n"), 0644)
	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.LibPasswd, []byte("bin:x:1:1:bin:/:/dev/null\n"), 0644)
	os.WriteFile(svc.cfg.EtcNsswitch, []byte("passwd: files\ngroup: files\n"), 0644)

	result, err := svc.ApplyFix()
	if err != nil {
		t.Fatalf("ApplyFix: %v", err)
	}
	if result.RemovedGIDConflicts != 1 {
		t.Errorf("RemovedGIDConflicts: got %d, want 1", result.RemovedGIDConflicts)
	}
}

func TestFixNssPreservesOverlayAndFixesGID(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nwheel:x:10:dm\ndm:x:1000:\naudio:x:82:dm\nvideo:x:990:\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("audio:x:81:\nvideo:x:990:\n"), 0644)
	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.LibPasswd, []byte("bin:x:1:1:bin:/:/dev/null\n"), 0644)
	os.WriteFile(svc.cfg.EtcNsswitch, []byte("passwd: files\ngroup: files\n"), 0644)

	_, err := svc.ApplyFix()
	if err != nil {
		t.Fatalf("ApplyFix: %v", err)
	}

	data, _ := os.ReadFile(svc.cfg.EtcGroup)
	entries, _ := etcfiles.ParseGroup(data)

	grpMap := map[string]etcfiles.GroupEntry{}
	for _, e := range entries {
		grpMap[e.Name] = e
	}

	audio, ok := grpMap["audio"]
	if !ok {
		t.Fatal("audio should be preserved in /etc/group (has member overlay)")
	}
	if audio.GID != 81 {
		t.Errorf("audio GID: got %d, want 81 (should be fixed from /usr/lib)", audio.GID)
	}
	if !slices.Equal(audio.Members, []string{"dm"}) {
		t.Errorf("audio members: got %v, want [dm]", audio.Members)
	}

	if _, ok := grpMap["video"]; ok {
		t.Error("video should be removed from /etc/group (no unique members)")
	}
}

// parsePasswdFile читает и парсит passwd-файл в тесте.
func parsePasswdFile(t *testing.T, path string) []etcfiles.PasswdEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	entries, err := etcfiles.ParsePasswd(data)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return entries
}

// parseGroupFile читает и парсит group-файл в тесте.
func parseGroupFile(t *testing.T, path string) []etcfiles.GroupEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	entries, err := etcfiles.ParseGroup(data)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return entries
}

func passwdNames(entries []etcfiles.PasswdEntry) map[string]bool {
	m := map[string]bool{}
	for _, e := range entries {
		m[e.Name] = true
	}
	return m
}

func groupByName(entries []etcfiles.GroupEntry) map[string]etcfiles.GroupEntry {
	m := map[string]etcfiles.GroupEntry{}
	for _, e := range entries {
		m[e.Name] = e
	}
	return m
}

// ApplyJoin → всё в /etc; ApplyBuild → исходное split-состояние.
func TestApplyJoinRoundTrip(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.LibPasswd, []byte("appsvc:x:497:497:App:/var/lib/appsvc:/sbin/nologin\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nwheel:x:10:dm\ndm:x:1000:\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("appgrp:x:190:\n"), 0644)
	os.WriteFile(svc.cfg.EtcNsswitch, []byte("passwd: files\ngroup: files\n"), 0644)

	if _, err := svc.ApplyJoin(); err != nil {
		t.Fatalf("ApplyJoin: %v", err)
	}

	// После join всё в /etc.
	etcUsers := passwdNames(parsePasswdFile(t, svc.cfg.EtcPasswd))
	if !etcUsers["root"] || !etcUsers["dm"] || !etcUsers["appsvc"] {
		t.Fatalf("after join /etc/passwd must hold all users, got %v", etcUsers)
	}

	// Re-split восстанавливает исходное состояние.
	if _, err := svc.ApplyBuild(); err != nil {
		t.Fatalf("ApplyBuild: %v", err)
	}

	etc := passwdNames(parsePasswdFile(t, svc.cfg.EtcPasswd))
	if !etc["root"] || !etc["dm"] || etc["appsvc"] {
		t.Errorf("after re-split /etc/passwd want {root,dm}, got %v", etc)
	}
	lib := passwdNames(parsePasswdFile(t, svc.cfg.LibPasswd))
	if !lib["appsvc"] || lib["dm"] || lib["root"] {
		t.Errorf("after re-split /usr/lib/passwd want {appsvc}, got %v", lib)
	}

	etcG := groupByName(parseGroupFile(t, svc.cfg.EtcGroup))
	if _, ok := etcG["appgrp"]; ok {
		t.Errorf("after re-split appgrp must leave /etc/group")
	}
	libG := groupByName(parseGroupFile(t, svc.cfg.LibGroup))
	if g, ok := libG["appgrp"]; !ok || g.GID != 190 {
		t.Errorf("after re-split appgrp must be in /usr/lib/group with GID 190, got %+v", libG)
	}
}

// После ApplyJoin /usr/lib/{passwd,group} пусты.
func TestApplyJoinEmptiesLib(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.LibPasswd, []byte("appsvc:x:497:497:App:/var/lib/appsvc:/sbin/nologin\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("appgrp:x:190:\n"), 0644)

	if _, err := svc.ApplyJoin(); err != nil {
		t.Fatalf("ApplyJoin: %v", err)
	}

	if got := parsePasswdFile(t, svc.cfg.LibPasswd); len(got) != 0 {
		t.Errorf("/usr/lib/passwd must be empty after join, got %v", got)
	}
	if got := parseGroupFile(t, svc.cfg.LibGroup); len(got) != 0 {
		t.Errorf("/usr/lib/group must be empty after join, got %v", got)
	}
}

// Членство, добавленное после join, переживает re-split и уезжает в lib.
func TestApplyJoinPreservesMemberOnReSplit(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.LibPasswd, []byte("appsvc:x:497:497:App:/var/lib/appsvc:/sbin/nologin\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nwheel:x:10:\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("appgrp:x:190:\n"), 0644)
	os.WriteFile(svc.cfg.EtcNsswitch, []byte("passwd: files\ngroup: files\n"), 0644)

	if _, err := svc.ApplyJoin(); err != nil {
		t.Fatalf("ApplyJoin: %v", err)
	}

	// Эмуляция `usermod -aG appgrp appsvc`.
	entries := parseGroupFile(t, svc.cfg.EtcGroup)
	for i := range entries {
		if entries[i].Name == "appgrp" {
			entries[i].Members = append(entries[i].Members, "appsvc")
		}
	}
	os.WriteFile(svc.cfg.EtcGroup, etcfiles.FormatGroup(entries), 0644)

	if _, err := svc.ApplyBuild(); err != nil {
		t.Fatalf("ApplyBuild: %v", err)
	}

	libG := groupByName(parseGroupFile(t, svc.cfg.LibGroup))
	appgrp, ok := libG["appgrp"]
	if !ok {
		t.Fatal("appgrp must be back in /usr/lib/group after re-split")
	}
	if !slices.Contains(appgrp.Members, "appsvc") {
		t.Fatalf("appsvc membership lost after re-split: %v", appgrp.Members)
	}
}
