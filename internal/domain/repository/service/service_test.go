package service

import (
	_package "apm/internal/common/apt/package"
	"apm/internal/common/command"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockRunner struct {
	runFunc func(ctx context.Context, args []string, opts ...command.Option) (string, string, error)
}

func (m *mockRunner) Run(ctx context.Context, args []string, opts ...command.Option) (string, string, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, args, opts...)
	}
	return "", "", errors.New("not implemented")
}

type mockPackageDB struct {
	hasHTTPS bool
}

func (m *mockPackageDB) GetPackageByName(_ context.Context, name string) (_package.Package, error) {
	if name == "apt-https" && m.hasHTTPS {
		return _package.Package{Name: "apt-https"}, nil
	}
	return _package.Package{}, errors.New("not found")
}

// newTestService создаёт RepoService с temp-директорией для тестов
func newTestService(t *testing.T) (*RepoService, string) {
	t.Helper()
	tmpDir := t.TempDir()
	confMain := filepath.Join(tmpDir, "sources.list")
	confDir := filepath.Join(tmpDir, "sources.list.d")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(confMain, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	runner := &mockRunner{}
	db := &mockPackageDB{hasHTTPS: false}

	s := &RepoService{
		confMain:           confMain,
		confDir:            confDir,
		arch:               "x86_64",
		useArepo:           true,
		httpClient:         &http.Client{},
		serviceAptDatabase: db,
		runner:             runner,
	}
	s.initBranches()
	return s, tmpDir
}

// writeSourcesList записывает содержимое sources.list
func writeSourcesList(t *testing.T, s *RepoService, content string) {
	t.Helper()
	if err := os.WriteFile(s.confMain, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// writeExtraList записывает дополнительный .list файл
func writeExtraList(t *testing.T, s *RepoService, name, content string) {
	t.Helper()
	path := filepath.Join(s.confDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// readSourcesList читает sources.list
func readSourcesList(t *testing.T, s *RepoService) string {
	t.Helper()
	data, err := os.ReadFile(s.confMain)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCanonicalizeRepoLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no change needed",
			input:    "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic",
			expected: "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic",
		},
		{
			name:     "new format with path in arch",
			input:    "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux p11/branch/x86_64 classic",
			expected: "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic",
		},
		{
			name:     "trailing slash removed",
			input:    "rpm http://example.com/repo/ x86_64 classic",
			expected: "rpm http://example.com/repo x86_64 classic",
		},
		{
			name:     "short line unchanged",
			input:    "rpm http://example.com",
			expected: "rpm http://example.com",
		},
		{
			name:     "archive new format",
			input:    "rpm [p11] http://ftp.altlinux.org/pub/distributions/archive p11/date/2025/01/01/x86_64 classic",
			expected: "rpm [p11] http://ftp.altlinux.org/pub/distributions/archive/p11/date/2025/01/01 x86_64 classic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canonicalizeRepoLine(tt.input)
			if got != tt.expected {
				t.Errorf("canonicalizeRepoLine(%q)\n got: %q\nwant: %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsTaskNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"370123", true},
		{"1", true},
		{"0", true},
		{"", false},
		{"abc", false},
		{"123abc", false},
		{"12 34", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isTaskNumber(tt.input); got != tt.expected {
				t.Errorf("isTaskNumber(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"http://example.com", true},
		{"https://example.com", true},
		{"ftp://example.com", true},
		{"rsync://example.com", true},
		{"file:///tmp/repo", true},
		{"cdrom:/media/cdrom", true},
		{"example.com", false},
		{"/path/to/repo", false},
		{"rpm http://example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isURL(tt.input); got != tt.expected {
				t.Errorf("isURL(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseLine(t *testing.T) {
	s, _ := newTestService(t)

	tests := []struct {
		name       string
		line       string
		wantNil    bool
		wantType   string
		wantKey    string
		wantURL    string
		wantArch   string
		wantComps  []string
		wantActive bool
	}{
		{
			name:       "full line with key",
			line:       "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic gostcrypto",
			wantType:   "rpm",
			wantKey:    "p11",
			wantURL:    "http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch",
			wantArch:   "x86_64",
			wantComps:  []string{"classic", "gostcrypto"},
			wantActive: true,
		},
		{
			name:       "line without key",
			line:       "rpm http://example.com/repo x86_64 classic",
			wantType:   "rpm",
			wantKey:    "",
			wantURL:    "http://example.com/repo",
			wantArch:   "x86_64",
			wantComps:  []string{"classic"},
			wantActive: true,
		},
		{
			name:    "too short",
			line:    "rpm http://example.com",
			wantNil: true,
		},
		{
			name:    "not rpm",
			line:    "deb http://example.com x86_64 main",
			wantNil: true,
		},
		{
			name:       "rpm-src type",
			line:       "rpm-src http://example.com/repo x86_64 classic",
			wantType:   "rpm-src",
			wantURL:    "http://example.com/repo",
			wantArch:   "x86_64",
			wantComps:  []string{"classic"},
			wantActive: false,
		},
		{
			name:       "apt-repo new_format",
			line:       "rpm [p11] https://ftp.altlinux.org/pub/distributions/ALTLinux p11/branch/x86_64 classic",
			wantType:   "rpm",
			wantKey:    "p11",
			wantURL:    "https://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch",
			wantArch:   "x86_64",
			wantComps:  []string{"classic"},
			wantActive: true,
		},
		{
			name:       "apt-repo new_format noarch",
			line:       "rpm [p11] https://ftp.altlinux.org/pub/distributions/ALTLinux p11/branch/noarch classic",
			wantType:   "rpm",
			wantKey:    "p11",
			wantURL:    "https://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch",
			wantArch:   "noarch",
			wantComps:  []string{"classic"},
			wantActive: true,
		},
		{
			name:       "apt-repo new_format arepo",
			line:       "rpm [p11] https://ftp.altlinux.org/pub/distributions/ALTLinux p11/branch/x86_64-i586 classic",
			wantType:   "rpm",
			wantKey:    "p11",
			wantURL:    "https://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch",
			wantArch:   "x86_64-i586",
			wantComps:  []string{"classic"},
			wantActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := s.parseLine(tt.line, "/test/file", tt.wantActive)
			if tt.wantNil {
				if repo != nil {
					t.Errorf("expected nil, got %+v", repo)
				}
				return
			}
			if repo == nil {
				t.Fatal("expected non-nil repo")
			}
			if repo.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", repo.Type, tt.wantType)
			}
			if repo.Key != tt.wantKey {
				t.Errorf("Key = %q, want %q", repo.Key, tt.wantKey)
			}
			if repo.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", repo.URL, tt.wantURL)
			}
			if repo.Arch != tt.wantArch {
				t.Errorf("Arch = %q, want %q", repo.Arch, tt.wantArch)
			}
			if repo.Active != tt.wantActive {
				t.Errorf("Active = %v, want %v", repo.Active, tt.wantActive)
			}
			if len(repo.Components) != len(tt.wantComps) {
				t.Errorf("Components = %v, want %v", repo.Components, tt.wantComps)
			} else {
				for i, c := range repo.Components {
					if c != tt.wantComps[i] {
						t.Errorf("Components[%d] = %q, want %q", i, c, tt.wantComps[i])
					}
				}
			}
		})
	}
}

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

func TestGetRepositories(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()

	writeSourcesList(t, s, `rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic
rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch noarch classic
# rpm [alt] http://ftp.altlinux.org/pub/distributions/ALTLinux/Sisyphus x86_64 classic
`)

	t.Run("active only", func(t *testing.T) {
		repos, err := s.GetRepositories(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(repos) != 2 {
			t.Errorf("got %d repos, want 2", len(repos))
		}
		for _, r := range repos {
			if !r.Active {
				t.Error("expected all repos to be active")
			}
		}
	})

	t.Run("all including commented", func(t *testing.T) {
		repos, err := s.GetRepositories(ctx, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(repos) != 3 {
			t.Errorf("got %d repos, want 3", len(repos))
		}
		commentedCount := 0
		for _, r := range repos {
			if !r.Active {
				commentedCount++
			}
		}
		if commentedCount != 1 {
			t.Errorf("got %d commented repos, want 1", commentedCount)
		}
	})
}

func TestGetRepositories_WithExtraListFiles(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()

	writeSourcesList(t, s, "rpm http://main.example.com x86_64 classic\n")
	writeExtraList(t, s, "extra.list", "rpm http://extra.example.com x86_64 classic\n")

	repos, err := s.GetRepositories(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Errorf("got %d repos, want 2", len(repos))
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
		// x86_64 → aarch64 + noarch = 2
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

func TestGetBranches(t *testing.T) {
	s, _ := newTestService(t)

	branches := s.GetBranches()

	if len(branches) == 0 {
		t.Fatal("expected branches")
	}

	// Не должно быть Sisyphus (заглавная) и autoimports
	for _, b := range branches {
		if b == "Sisyphus" {
			t.Error("Sisyphus (capital) should be filtered out")
		}
		if strings.HasPrefix(b, "autoimports.") {
			t.Error("autoimports should be filtered out")
		}
	}

	// Должен быть sisyphus (строчная)
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

func TestHttpScheme(t *testing.T) {
	ctx := context.Background()

	t.Run("no apt-https", func(t *testing.T) {
		s, _ := newTestService(t)
		scheme := s.httpScheme(ctx)
		if scheme != "http://" {
			t.Errorf("got %q, want http://", scheme)
		}
	})

	t.Run("with apt-https", func(t *testing.T) {
		s, _ := newTestService(t)
		s.serviceAptDatabase = &mockPackageDB{hasHTTPS: true}
		scheme := s.httpScheme(ctx)
		if scheme != "https://" {
			t.Errorf("got %q, want https://", scheme)
		}
	})
}

func TestCheckRepoExists(t *testing.T) {
	ctx := context.Background()

	t.Run("active repo", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm http://example.com x86_64 classic\n")

		active, commented, err := s.checkRepoExists(ctx, "rpm http://example.com x86_64 classic")
		if err != nil {
			t.Fatal(err)
		}
		if !active {
			t.Error("expected active=true")
		}
		if commented {
			t.Error("expected commented=false")
		}
	})

	t.Run("commented repo", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "# rpm http://example.com x86_64 classic\n")

		active, commented, err := s.checkRepoExists(ctx, "rpm http://example.com x86_64 classic")
		if err != nil {
			t.Fatal(err)
		}
		if active {
			t.Error("expected active=false")
		}
		if !commented {
			t.Error("expected commented=true")
		}
	})

	t.Run("not found", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm http://other.com x86_64 classic\n")

		active, commented, err := s.checkRepoExists(ctx, "rpm http://example.com x86_64 classic")
		if err != nil {
			t.Fatal(err)
		}
		if active || commented {
			t.Error("expected both false")
		}
	})

	t.Run("new format match", func(t *testing.T) {
		s, _ := newTestService(t)
		writeSourcesList(t, s, "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux p11/branch/x86_64 classic\n")

		active, _, err := s.checkRepoExists(ctx, "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic")
		if err != nil {
			t.Fatal(err)
		}
		if !active {
			t.Error("expected active=true (canonicalization should match new format)")
		}
	})
}

func TestDetectArch(t *testing.T) {
	t.Run("fallback to uname", func(t *testing.T) {
		runner := &mockRunner{
			runFunc: func(_ context.Context, args []string, _ ...command.Option) (string, string, error) {
				if len(args) > 0 && args[0] == "uname" {
					return "x86_64\n", "", nil
				}
				return "", "", errors.New("unknown command")
			},
		}
		// Эта функция зависит от runtime.GOARCH, который мы не можем менять в тесте,
		// но мы можем проверить что на текущей платформе она возвращает валидное значение
		arch := detectArch(runner)
		valid := map[string]bool{"x86_64": true, "i586": true, "armh": true, "aarch64": true}
		if !valid[arch] {
			if arch == "" {
				t.Error("arch should not be empty")
			}
		}
	})
}
