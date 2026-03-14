package service

import (
	"context"
	"strings"
	"testing"
)

func TestParseArchiveDate(t *testing.T) {
	s, _ := newTestService(t)

	tests := []struct {
		name    string
		branch  string
		date    string
		want    string
		wantErr bool
	}{
		{"YYYYMMDD format", "p11", "20250101", "2025/01/01", false},
		{"YYYY/MM/DD format", "p10", "2024/06/15", "2024/06/15", false},
		{"non-archiving branch", "c8", "20250101", "", true},
		{"bad format", "p11", "2025-01-01", "", true},
		{"sisyphus archive", "sisyphus", "20240315", "2024/03/15", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.parseArchiveDate(tt.branch, tt.date)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetBranches(t *testing.T) {
	s, _ := newTestService(t)

	branches := s.GetBranches()

	if len(branches) == 0 {
		t.Fatal("expected branches")
	}

	for _, b := range branches {
		if b == "Sisyphus" {
			t.Error("Sisyphus (capital) should be filtered out")
		}
		if strings.HasPrefix(b, "autoimports.") {
			t.Error("autoimports should be filtered out")
		}
	}

	found := false
	for _, b := range branches {
		if b == "sisyphus" {
			found = true
			break
		}
	}
	if !found {
		t.Error("sisyphus should be in list")
	}

	// Проверка сортировки
	for i := 1; i < len(branches); i++ {
		if branches[i] < branches[i-1] {
			t.Errorf("branches not sorted: %v", branches)
			break
		}
	}
}

func TestHasGostcryptoInSources(t *testing.T) {
	ctx := context.Background()

	t.Run("gostcrypto present", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm [p11] http://ftp.altlinux.org x86_64 classic gostcrypto\n")
		if !s.hasGostcryptoInSources(ctx) {
			t.Error("expected true")
		}
	})

	t.Run("gostcrypto absent", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm [p11] http://ftp.altlinux.org x86_64 classic\n")
		if s.hasGostcryptoInSources(ctx) {
			t.Error("expected false")
		}
	})

	t.Run("gostcrypto in commented line", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "# rpm [p11] http://ftp.altlinux.org x86_64 classic gostcrypto\n")
		if !s.hasGostcryptoInSources(ctx) {
			t.Error("expected true (commented repos included with all=true)")
		}
	})
}

func TestBuildBranchURLs(t *testing.T) {
	ctx := context.Background()

	t.Run("p11 without gostcrypto", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm http://example.com x86_64 classic\n")

		branch := s.branches["p11"]
		urls := s.buildBranchURLs(ctx, branch)

		// x86_64 + noarch + arepo = 3 URLs
		if len(urls) != 3 {
			t.Fatalf("got %d URLs, want 3: %v", len(urls), urls)
		}

		// Главная строка — только classic
		if strings.Contains(urls[0], "gostcrypto") {
			t.Errorf("expected no gostcrypto in main URL: %s", urls[0])
		}
		if !strings.Contains(urls[0], "x86_64 classic") {
			t.Errorf("expected 'x86_64 classic' in main URL: %s", urls[0])
		}

		// noarch
		if !strings.Contains(urls[1], "noarch classic") {
			t.Errorf("expected 'noarch classic' in second URL: %s", urls[1])
		}

		// arepo
		if !strings.Contains(urls[2], "x86_64-i586") {
			t.Errorf("expected arepo URL: %s", urls[2])
		}
	})

	t.Run("p11 with gostcrypto", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm [p11] http://ftp.altlinux.org x86_64 classic gostcrypto\n")

		branch := s.branches["p11"]
		urls := s.buildBranchURLs(ctx, branch)

		if !strings.Contains(urls[0], "classic gostcrypto") {
			t.Errorf("expected gostcrypto in main URL: %s", urls[0])
		}
	})

	t.Run("p8 no gostcrypto branch", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "")

		branch := s.branches["p8"]
		urls := s.buildBranchURLs(ctx, branch)

		for _, u := range urls {
			if strings.Contains(u, "gostcrypto") {
				t.Errorf("p8 should not have gostcrypto: %s", u)
			}
		}
	})

	t.Run("no arepo on non-x86_64", func(t *testing.T) {
		s, _ := newTestService(t)
		s.arch = "aarch64"
		writeSourcesList(t, s, "")

		branch := s.branches["p11"]
		urls := s.buildBranchURLs(ctx, branch)

		for _, u := range urls {
			if strings.Contains(u, "x86_64-i586") {
				t.Errorf("should not have arepo on aarch64: %s", u)
			}
		}
		if len(urls) != 2 {
			t.Errorf("got %d URLs, want 2", len(urls))
		}
	})

	t.Run("arepo disabled", func(t *testing.T) {
		s, _ := newTestService(t)
		s.useArepo = false
		writeSourcesList(t, s, "")

		branch := s.branches["p11"]
		urls := s.buildBranchURLs(ctx, branch)

		for _, u := range urls {
			if strings.Contains(u, "x86_64-i586") {
				t.Errorf("should not have arepo when disabled: %s", u)
			}
		}
	})

	t.Run("autoimports no arepo", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "")

		branch := s.branches["autoimports.p11"]
		urls := s.buildBranchURLs(ctx, branch)

		for _, u := range urls {
			if strings.Contains(u, "x86_64-i586") {
				t.Errorf("autoimports should not have arepo: %s", u)
			}
		}
	})
}

func TestBuildBranchURLsWithArchive(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	writeSourcesList(t, s, "")

	branch := s.branches["p11"]
	urls := s.buildBranchURLsWithArchive(ctx, branch, "2025/01/01")

	if len(urls) == 0 {
		t.Fatal("expected at least one URL")
	}

	if !strings.Contains(urls[0], "archive") || !strings.Contains(urls[0], "2025/01/01") {
		t.Errorf("expected archive URL with date: %s", urls[0])
	}
}
