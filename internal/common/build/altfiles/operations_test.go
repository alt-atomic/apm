package altfiles

import (
	"apm/internal/common/build/etcfiles"
	"os"
	"slices"
	"testing"
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
			Users:  []string{"dm"},
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
			Users:  []string{"dm"},
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
			Users:  []string{"dm"},
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
			Users:  []string{"dm"},
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
			Users:  []string{"dm", "fakeuser"},
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

func TestSyncGroupsIdempotent(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(dir)

	os.WriteFile(svc.cfg.EtcPasswd, []byte("root:x:0:0:root:/root:/bin/bash\ndm:x:1000:1000::/home/dm:/bin/bash\n"), 0644)
	os.WriteFile(svc.cfg.EtcGroup, []byte("root:x:0:\nwheel:x:10:dm\ndm:x:1000:\n"), 0644)
	os.WriteFile(svc.cfg.LibGroup, []byte("docker:x:948:\naudio:x:81:\n"), 0644)

	configs := []SyncConfig{{
		Sync: SyncBody{
			Groups: []string{"docker", "audio"},
			Users:  []string{"dm"},
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
