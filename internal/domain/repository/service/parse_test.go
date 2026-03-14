package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSource(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	writeSourcesList(t, s, "")

	t.Run("branch name", func(t *testing.T) {
		urls, err := s.parseSource(ctx, "p11", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(urls) == 0 {
			t.Fatal("expected URLs for branch")
		}
		if !strings.Contains(urls[0], "p11") {
			t.Errorf("expected p11 in URL: %s", urls[0])
		}
	})

	t.Run("branch with archive date", func(t *testing.T) {
		urls, err := s.parseSource(ctx, "p11", "20250101")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(urls[0], "archive") {
			t.Errorf("expected archive URL: %s", urls[0])
		}
	})

	t.Run("URL source", func(t *testing.T) {
		urls, err := s.parseSource(ctx, "http://example.com/repo", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(urls) < 2 {
			t.Fatalf("expected at least 2 URLs (arch+noarch), got %d", len(urls))
		}
		if !strings.Contains(urls[0], "x86_64 classic") {
			t.Errorf("expected arch URL: %s", urls[0])
		}
		if !strings.Contains(urls[1], "noarch classic") {
			t.Errorf("expected noarch URL: %s", urls[1])
		}
	})

	t.Run("absolute path", func(t *testing.T) {
		urls, err := s.parseSource(ctx, "/tmp/my-repo", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(urls) != 1 {
			t.Fatalf("expected 1 URL, got %d", len(urls))
		}
		if !strings.Contains(urls[0], "file:///tmp/my-repo") {
			t.Errorf("expected file:// URL: %s", urls[0])
		}
		if !strings.Contains(urls[0], "hasher") {
			t.Errorf("expected hasher component: %s", urls[0])
		}
	})

	t.Run("raw rpm entry", func(t *testing.T) {
		urls, err := s.parseSource(ctx, "rpm http://example.com/repo _arch_ classic", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(urls) != 1 {
			t.Fatalf("expected 1 URL, got %d", len(urls))
		}
		if !strings.Contains(urls[0], "x86_64") {
			t.Errorf("expected _arch_ replaced with x86_64: %s", urls[0])
		}
	})

	t.Run("unknown source", func(t *testing.T) {
		_, err := s.parseSource(ctx, "unknown_thing", "")
		if err == nil {
			t.Error("expected error for unknown source")
		}
	})
}

func TestParseSourceArgs(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	writeSourcesList(t, s, "")

	t.Run("single branch arg", func(t *testing.T) {
		urls, err := s.parseSourceArgs(ctx, []string{"p11"}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(urls) == 0 {
			t.Fatal("expected URLs")
		}
	})

	t.Run("rpm with extra args", func(t *testing.T) {
		urls, err := s.parseSourceArgs(ctx, []string{"rpm", "http://example.com", "_arch_", "classic"}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(urls) != 1 {
			t.Fatalf("expected 1 URL, got %d", len(urls))
		}
		if !strings.Contains(urls[0], "x86_64") {
			t.Errorf("expected _arch_ replaced: %s", urls[0])
		}
	})

	t.Run("URL with arch and components", func(t *testing.T) {
		urls, err := s.parseSourceArgs(ctx, []string{"http://example.com/repo", "i586", "main"}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(urls) != 1 {
			t.Fatalf("expected 1 URL, got %d", len(urls))
		}
		if !strings.Contains(urls[0], "i586") || !strings.Contains(urls[0], "main") {
			t.Errorf("unexpected URL: %s", urls[0])
		}
	})

	t.Run("empty args", func(t *testing.T) {
		_, err := s.parseSourceArgs(ctx, []string{}, "")
		if err == nil {
			t.Error("expected error for empty args")
		}
	})
}

func TestBuildURLRepos(t *testing.T) {
	t.Run("http URL", func(t *testing.T) {
		s, _ := newTestService(t)
		urls := s.buildURLRepos("http://example.com/repo")

		if len(urls) != 2 {
			t.Fatalf("expected 2 URLs (arch+noarch), got %d: %v", len(urls), urls)
		}
		if !strings.Contains(urls[0], "x86_64 classic") {
			t.Errorf("first URL should have arch: %s", urls[0])
		}
		if !strings.Contains(urls[1], "noarch classic") {
			t.Errorf("second URL should be noarch: %s", urls[1])
		}
	})

	t.Run("file URL with arepo dir", func(t *testing.T) {
		s, _ := newTestService(t)
		tmpDir := t.TempDir()
		arepoDir := filepath.Join(tmpDir, "x86_64-i586")
		if err := os.MkdirAll(arepoDir, 0755); err != nil {
			t.Fatal(err)
		}

		urls := s.buildURLRepos("file://" + tmpDir)
		if len(urls) != 3 {
			t.Fatalf("expected 3 URLs (arch+noarch+arepo), got %d: %v", len(urls), urls)
		}
		if !strings.Contains(urls[2], "x86_64-i586") {
			t.Errorf("third URL should be arepo: %s", urls[2])
		}
	})

	t.Run("file URL without arepo dir", func(t *testing.T) {
		s, _ := newTestService(t)
		tmpDir := t.TempDir()

		urls := s.buildURLRepos("file://" + tmpDir)
		if len(urls) != 2 {
			t.Fatalf("expected 2 URLs without arepo dir, got %d", len(urls))
		}
	})
}
