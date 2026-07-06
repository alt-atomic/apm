package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestSimulateRemove_CommentedRepo(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	writeSourcesList(t, s, "# rpm http://example.com/repo x86_64 classic\n")

	willRemove, err := s.SimulateRemove(ctx, []string{"rpm http://example.com/repo x86_64 classic"}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	// Закомментированный репозиторий в симуляцию удаления не попадает
	if len(willRemove) != 0 {
		t.Errorf("expected 0 for commented repo, got %d", len(willRemove))
	}
}

func TestRemoveRepository_PreservesUnrelatedLines(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	writeSourcesList(t, s, "# comment header\n\nrpm http://example.com/repo x86_64 classic\nrpm http://other.com/repo x86_64 classic\n")

	_, err := s.RemoveRepository(ctx, []string{"rpm http://example.com/repo x86_64 classic"}, "", false)
	if err != nil {
		t.Fatal(err)
	}

	content := readSourcesList(t, s)
	if !strings.Contains(content, "# comment header") {
		t.Error("unrelated comment should survive removal")
	}
	if !strings.Contains(content, "other.com") {
		t.Error("unrelated repo should survive removal")
	}
	if strings.Contains(content, "example.com") {
		t.Error("target repo should be removed")
	}
}

func TestAddRepository_UncommentKeepsIndentedNeighbors(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	writeSourcesList(t, s, "# just a note about repos\n# rpm http://example.com/repo x86_64 classic\n")

	added, err := s.AddRepository(ctx, []string{"rpm http://example.com/repo x86_64 classic"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 {
		t.Fatalf("expected 1 uncommented repo, got %d", len(added))
	}

	content := readSourcesList(t, s)
	if !strings.Contains(content, "# just a note about repos") {
		t.Error("unrelated comment should stay commented")
	}
	if !strings.Contains(content, "rpm http://example.com/repo x86_64 classic") {
		t.Error("repo should be uncommented")
	}
}

func TestConcurrentMutations(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	writeSourcesList(t, s, "")

	repos := []string{
		"rpm http://a.example.com/repo x86_64 classic",
		"rpm http://b.example.com/repo x86_64 classic",
		"rpm http://c.example.com/repo x86_64 classic",
		"rpm http://d.example.com/repo x86_64 classic",
	}

	var wg sync.WaitGroup
	for _, line := range repos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.AddRepository(ctx, []string{line}, ""); err != nil {
				t.Errorf("AddRepository(%q): %v", line, err)
			}
		}()
	}
	wg.Wait()

	// Параллельные добавления не должны терять правки друг друга
	content := readSourcesList(t, s)
	for _, line := range repos {
		if !strings.Contains(content, line) {
			t.Errorf("lost repo line under concurrency: %s", line)
		}
	}
}

func TestRemoveRepository_Task(t *testing.T) {
	ctx := context.Background()

	t.Run("active task without network", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm http://git.altlinux.org/repo/12345/ x86_64 task\nrpm http://git.altlinux.org/repo/12345/ x86_64-i586 task\n")

		removed, err := s.RemoveRepository(ctx, []string{"12345"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(removed) != 2 {
			t.Errorf("expected 2 removed, got %d", len(removed))
		}
		if content := readSourcesList(t, s); strings.Contains(content, "12345") {
			t.Errorf("task should be removed: %s", content)
		}
	})

	t.Run("archived task by number", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm http://git.altlinux.org/tasks/archive/done/_12/12345/build/repo/ x86_64 task\n")

		removed, err := s.RemoveRepository(ctx, []string{"task", "12345"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(removed) != 1 {
			t.Errorf("expected 1 removed, got %d", len(removed))
		}
		if content := readSourcesList(t, s); strings.Contains(content, "12345") {
			t.Errorf("archived task should be removed: %s", content)
		}
	})

	t.Run("nonexistent task is no-op", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm http://example.com/repo x86_64 classic\n")

		removed, err := s.RemoveRepository(ctx, []string{"99999"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(removed) != 0 {
			t.Errorf("expected 0 removed, got %d", len(removed))
		}
	})
}

func TestRemoveRepository_SchemeMismatch(t *testing.T) {
	ctx := context.Background()

	t.Run("https entry removed by http source", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm https://example.com/repo x86_64 classic\n")

		removed, err := s.RemoveRepository(ctx, []string{"http://example.com/repo"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(removed) != 1 {
			t.Errorf("expected 1 removed, got %d", len(removed))
		}
		if content := readSourcesList(t, s); strings.Contains(content, "example.com") {
			t.Errorf("repo should be removed despite scheme mismatch: %s", content)
		}
	})

	t.Run("simulate remove sees scheme mismatch", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm https://example.com/repo x86_64 classic\n")

		willRemove, err := s.SimulateRemove(ctx, []string{"http://example.com/repo"}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(willRemove) != 1 {
			t.Errorf("expected 1, got %d", len(willRemove))
		}
	})
}
