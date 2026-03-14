package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("add URL repo", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "")

		added, err := s.AddRepository(ctx, []string{"http://example.com/repo"}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(added) == 0 {
			t.Fatal("expected repos to be added")
		}

		content := readSourcesList(t, s)
		if !strings.Contains(content, "http://example.com/repo") {
			t.Errorf("repo not found in sources.list: %s", content)
		}
	})

	t.Run("add already active repo is no-op", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm http://example.com/repo x86_64 classic\nrpm http://example.com/repo noarch classic\n")

		added, err := s.AddRepository(ctx, []string{"http://example.com/repo"}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(added) != 0 {
			t.Errorf("expected no additions for existing repo, got %d", len(added))
		}
	})

	t.Run("add uncomments commented repo", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "# rpm http://example.com/repo x86_64 classic\n# rpm http://example.com/repo noarch classic\n")

		added, err := s.AddRepository(ctx, []string{"http://example.com/repo"}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(added) != 2 {
			t.Errorf("expected 2 uncommented repos, got %d", len(added))
		}

		content := readSourcesList(t, s)
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "#") {
				t.Errorf("expected uncommented, got: %s", line)
			}
		}
	})
}

func TestRemoveRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("remove specific repo", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm http://example.com/repo x86_64 classic\nrpm http://other.com/repo x86_64 classic\n")

		removed, err := s.RemoveRepository(ctx, []string{"http://example.com/repo"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(removed) == 0 {
			t.Fatal("expected repos to be removed")
		}

		content := readSourcesList(t, s)
		if strings.Contains(content, "example.com") {
			t.Errorf("repo should be removed from sources.list: %s", content)
		}
		if !strings.Contains(content, "other.com") {
			t.Error("other repo should remain")
		}
	})

	t.Run("remove all", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm http://a.com x86_64 classic\nrpm http://b.com x86_64 classic\n")

		removed, err := s.RemoveRepository(ctx, []string{"all"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(removed) != 2 {
			t.Errorf("expected 2 removed, got %d", len(removed))
		}
	})

	t.Run("remove nonexistent is no-op", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm http://example.com x86_64 classic\n")

		removed, err := s.RemoveRepository(ctx, []string{"http://nonexistent.com/repo"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(removed) != 0 {
			t.Errorf("expected 0 removed, got %d", len(removed))
		}
	})

	t.Run("comments in extra list file", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "")
		writeExtraList(t, s, "test.list", "rpm http://example.com/repo x86_64 classic\n")

		removed, err := s.RemoveRepository(ctx, []string{"http://example.com/repo"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(removed) == 0 {
			t.Fatal("expected repos to be removed")
		}

		// В extra-файлах repo должен быть закомментирован, а не удалён
		data, _ := os.ReadFile(filepath.Join(s.confDir, "test.list"))
		content := string(data)
		if !strings.Contains(content, "#") {
			t.Errorf("expected commented line in extra file: %s", content)
		}
	})
}

func TestSetBranch(t *testing.T) {
	ctx := context.Background()

	t.Run("set branch replaces all", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm http://old.example.com x86_64 classic\n")

		added, removed, err := s.SetBranch(ctx, "p11", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(removed) == 0 {
			t.Error("expected old repos to be removed")
		}
		if len(added) == 0 {
			t.Error("expected new repos to be added")
		}

		content := readSourcesList(t, s)
		if strings.Contains(content, "old.example.com") {
			t.Error("old repo should be gone")
		}
		if !strings.Contains(content, "p11") {
			t.Errorf("p11 should be in sources: %s", content)
		}
	})

	t.Run("unknown branch", func(t *testing.T) {
		s, _ := newTestService(t)
		_, _, err := s.SetBranch(ctx, "unknown_branch", "")
		if err == nil {
			t.Error("expected error for unknown branch")
		}
	})
}

func TestCleanTemporary(t *testing.T) {
	ctx := context.Background()

	t.Run("removes cdrom and task repos", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, `rpm cdrom:/media/disk x86_64 classic
rpm http://git.altlinux.org/repo/12345/ x86_64 task
rpm http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic
`)

		removed, err := s.CleanTemporary(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(removed) != 2 {
			t.Errorf("expected 2 removed, got %d", len(removed))
		}

		content := readSourcesList(t, s)
		if strings.Contains(content, "cdrom") {
			t.Error("cdrom repo should be removed")
		}
		if strings.Contains(content, "task") {
			t.Error("task repo should be removed")
		}
		if !strings.Contains(content, "p11") {
			t.Error("p11 repo should remain")
		}
	})

	t.Run("nothing to clean", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm http://ftp.altlinux.org x86_64 classic\n")

		removed, err := s.CleanTemporary(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(removed) != 0 {
			t.Errorf("expected 0, got %d", len(removed))
		}
	})
}

func TestSimulateAdd(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	writeSourcesList(t, s, "rpm http://example.com/repo x86_64 classic\nrpm http://example.com/repo noarch classic\n")

	t.Run("already exists", func(t *testing.T) {
		willAdd, err := s.SimulateAdd(ctx, []string{"http://example.com/repo"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(willAdd) != 0 {
			t.Errorf("expected 0, got %d", len(willAdd))
		}
	})

	t.Run("new repo", func(t *testing.T) {
		willAdd, err := s.SimulateAdd(ctx, []string{"http://new.example.com/repo"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(willAdd) == 0 {
			t.Error("expected repos in simulation")
		}
	})

	t.Run("force adds existing", func(t *testing.T) {
		willAdd, err := s.SimulateAdd(ctx, []string{"http://example.com/repo"}, "", true)
		if err != nil {
			t.Fatal(err)
		}
		if len(willAdd) == 0 {
			t.Error("expected repos with force=true")
		}
	})
}

func TestSimulateRemove(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	writeSourcesList(t, s, "rpm http://example.com/repo x86_64 classic\nrpm http://example.com/repo noarch classic\n")

	t.Run("existing repo", func(t *testing.T) {
		willRemove, err := s.SimulateRemove(ctx, []string{"http://example.com/repo"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(willRemove) != 2 {
			t.Errorf("expected 2, got %d", len(willRemove))
		}
	})

	t.Run("all", func(t *testing.T) {
		willRemove, err := s.SimulateRemove(ctx, []string{"all"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(willRemove) != 2 {
			t.Errorf("expected 2, got %d", len(willRemove))
		}
	})

	t.Run("nonexistent", func(t *testing.T) {
		willRemove, err := s.SimulateRemove(ctx, []string{"http://nonexistent.com"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(willRemove) != 0 {
			t.Errorf("expected 0, got %d", len(willRemove))
		}
	})

	t.Run("empty args", func(t *testing.T) {
		_, err := s.SimulateRemove(ctx, []string{}, "", false)
		if err == nil {
			t.Error("expected error")
		}
	})
}
