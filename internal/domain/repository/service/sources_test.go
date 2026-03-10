package service

import (
	"apm/internal/common/command"
	"context"
	"errors"
	"strings"
	"testing"
)

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

func TestParseLine(t *testing.T) {
	s, _ := newTestService(t)

	tests := []struct {
		name       string
		line       string
		wantNil    bool
		wantURL    string
		wantArch   string
		wantComps  []string
		wantActive bool
		wantBranch string
	}{
		{
			name:       "full line with key",
			line:       "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic gostcrypto",
			wantURL:    "http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch",
			wantArch:   "x86_64",
			wantComps:  []string{"classic", "gostcrypto"},
			wantActive: true,
			wantBranch: "p11",
		},
		{
			name:       "line without key",
			line:       "rpm http://example.com/repo x86_64 classic",
			wantURL:    "http://example.com/repo",
			wantArch:   "x86_64",
			wantComps:  []string{"classic"},
			wantActive: true,
			wantBranch: "",
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
			wantURL:    "http://example.com/repo",
			wantArch:   "x86_64",
			wantComps:  []string{"classic"},
			wantActive: false,
			wantBranch: "",
		},
		{
			name:       "apt-repo new_format",
			line:       "rpm [p11] https://ftp.altlinux.org/pub/distributions/ALTLinux p11/branch/x86_64 classic",
			wantURL:    "https://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch",
			wantArch:   "x86_64",
			wantComps:  []string{"classic"},
			wantActive: true,
			wantBranch: "p11",
		},
		{
			name:       "apt-repo new_format noarch",
			line:       "rpm [p11] https://ftp.altlinux.org/pub/distributions/ALTLinux p11/branch/noarch classic",
			wantURL:    "https://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch",
			wantArch:   "noarch",
			wantComps:  []string{"classic"},
			wantActive: true,
			wantBranch: "p11",
		},
		{
			name:       "apt-repo new_format arepo",
			line:       "rpm [p11] https://ftp.altlinux.org/pub/distributions/ALTLinux p11/branch/x86_64-i586 classic",
			wantURL:    "https://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch",
			wantArch:   "x86_64-i586",
			wantComps:  []string{"classic"},
			wantActive: true,
			wantBranch: "p11",
		},
		{
			name:       "ftp mirror without key",
			line:       "rpm ftp://mirror.yandex.ru/altlinux/Sisyphus x86_64 classic",
			wantURL:    "ftp://mirror.yandex.ru/altlinux/Sisyphus",
			wantArch:   "x86_64",
			wantComps:  []string{"classic"},
			wantActive: true,
			wantBranch: "sisyphus",
		},
		{
			name:       "ftp mirror noarch",
			line:       "rpm ftp://mirror.yandex.ru/altlinux/Sisyphus noarch classic",
			wantURL:    "ftp://mirror.yandex.ru/altlinux/Sisyphus",
			wantArch:   "noarch",
			wantComps:  []string{"classic"},
			wantActive: true,
			wantBranch: "sisyphus",
		},
		{
			name:       "http mirror msu",
			line:       "rpm http://mirror.cs.msu.ru/alt/p11/branch x86_64 classic gostcrypto",
			wantURL:    "http://mirror.cs.msu.ru/alt/p11/branch",
			wantArch:   "x86_64",
			wantComps:  []string{"classic", "gostcrypto"},
			wantActive: true,
			wantBranch: "p11",
		},
		{
			name:       "cdrom repo",
			line:       "rpm cdrom:[ALT Linux p11] /media/ALTLinux x86_64 classic",
			wantURL:    "cdrom:[ALT",
			wantArch:   "Linux",
			wantComps:  []string{"p11]", "/media/ALTLinux", "x86_64", "classic"},
			wantActive: true,
			wantBranch: "",
		},
		{
			name:       "task repo without key",
			line:       "rpm http://git.altlinux.org/repo/410804/ x86_64 task",
			wantURL:    "http://git.altlinux.org/repo/410804",
			wantArch:   "x86_64",
			wantComps:  []string{"task"},
			wantActive: true,
			wantBranch: "task",
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
			if repo.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", repo.URL, tt.wantURL)
			}
			if repo.Arch != tt.wantArch {
				t.Errorf("Arch = %q, want %q", repo.Arch, tt.wantArch)
			}
			if repo.Active != tt.wantActive {
				t.Errorf("Active = %v, want %v", repo.Active, tt.wantActive)
			}
			if repo.Branch != tt.wantBranch {
				t.Errorf("Branch = %q, want %q", repo.Branch, tt.wantBranch)
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
		arch := detectArch(runner)
		valid := map[string]bool{"x86_64": true, "i586": true, "armh": true, "aarch64": true}
		if !valid[arch] {
			if arch == "" {
				t.Error("arch should not be empty")
			}
		}
	})
}

func TestStripScheme(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"http", "http://ftp.altlinux.org/pub", "ftp.altlinux.org/pub"},
		{"https", "https://ftp.altlinux.org/pub", "ftp.altlinux.org/pub"},
		{"ftp", "ftp://mirror.yandex.ru/altlinux", "mirror.yandex.ru/altlinux"},
		{"file", "file:///tmp/repo", "/tmp/repo"},
		{"cdrom", "cdrom:/media/disk", "/media/disk"},
		{"rsync", "rsync://mirror.example.com/alt", "mirror.example.com/alt"},
		{"no scheme", "example.com/repo", "example.com/repo"},
		{"trailing slash", "http://example.com/repo/", "example.com/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripScheme(tt.url)
			if got != tt.want {
				t.Errorf("stripScheme(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestDetectBranch(t *testing.T) {
	s, _ := newTestService(t)

	tests := []struct {
		name string
		url  string
		want string
	}{
		// Точное совпадение с известными ветками
		{"p11 http", "http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch", "p11"},
		{"p11 https", "https://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch", "p11"},
		{"sisyphus", "http://ftp.altlinux.org/pub/distributions/ALTLinux/Sisyphus", "sisyphus"},
		{"sisyphus https", "https://ftp.altlinux.org/pub/distributions/ALTLinux/Sisyphus", "sisyphus"},
		{"p10", "http://ftp.altlinux.org/pub/distributions/ALTLinux/p10/branch", "p10"},

		// Зеркала (fallback по имени в пути)
		{"yandex sisyphus ftp", "ftp://mirror.yandex.ru/altlinux/Sisyphus", "sisyphus"},
		{"yandex p11 http", "http://mirror.yandex.ru/altlinux/p11/branch", "p11"},
		{"msu p11", "http://mirror.cs.msu.ru/alt/p11/branch", "p11"},
		{"mephi sisyphus", "http://mirror.mephi.ru/ALTLinux/Sisyphus", "sisyphus"},
		{"truenetwork p10", "https://mirror.truenetwork.ru/altlinux/p10/branch", "p10"},
		{"datacenter.by p11", "http://mirror.datacenter.by/pub/ALTLinux/p11/branch", "p11"},
		{"hoster.kz sisyphus", "https://mirror.hoster.kz/altlinux/Sisyphus", "sisyphus"},

		// Архивные репозитории
		{"archive p11", "http://ftp.altlinux.org/pub/distributions/archive/p11/date/2025/01/01", "p11"},
		{"archive sisyphus", "https://ftp.altlinux.org/pub/distributions/archive/sisyphus/date/2024/03/15", "sisyphus"},

		// Task-репозитории
		{"task repo", "http://git.altlinux.org/repo/410804/", "task"},
		{"task repo https", "https://git.altlinux.org/repo/370123", "task"},

		// URL с расширением файла
		{"sisyphus.repo", "https://altlinux.space/api/packages/alt-atomic/alt/group/apm-nightly/sisyphus.repo", "sisyphus"},
		{"p11.list", "https://example.com/repos/p11.list", "p11"},

		// Неизвестный
		{"unknown", "http://random.example.com/something", ""},

		// Trailing slash
		{"trailing slash", "http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch/", "p11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.detectBranch(tt.url)
			if got != tt.want {
				t.Errorf("detectBranch(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestParseLine_Branch(t *testing.T) {
	s, _ := newTestService(t)

	tests := []struct {
		name       string
		line       string
		wantBranch string
	}{
		{"p11 branch", "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic", "p11"},
		{"sisyphus", "rpm [alt] http://ftp.altlinux.org/pub/distributions/ALTLinux/Sisyphus x86_64 classic", "sisyphus"},
		{"yandex mirror", "rpm ftp://mirror.yandex.ru/altlinux/Sisyphus x86_64 classic", "sisyphus"},
		{"task", "rpm http://git.altlinux.org/repo/410804/ x86_64 task", "task"},
		{"unknown", "rpm http://random.example.com/repo x86_64 classic", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := s.parseLine(tt.line, "/test/file", true)
			if repo == nil {
				t.Fatal("expected non-nil repo")
			}
			if repo.Branch != tt.wantBranch {
				t.Errorf("Branch = %q, want %q", repo.Branch, tt.wantBranch)
			}
		})
	}
}

func TestGetRepositories_BranchField(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()

	writeSourcesList(t, s, strings.Join([]string{
		"rpm [alt] http://ftp.altlinux.org/pub/distributions/ALTLinux/Sisyphus x86_64 classic",
		"rpm [alt] http://ftp.altlinux.org/pub/distributions/ALTLinux/Sisyphus noarch classic",
		"rpm http://git.altlinux.org/repo/410804/ x86_64 task",
	}, "\n")+"\n")

	repos, err := s.GetRepositories(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 3 {
		t.Fatalf("got %d repos, want 3", len(repos))
	}
	for _, r := range repos[:2] {
		if r.Branch != "sisyphus" {
			t.Errorf("expected branch=sisyphus, got %q for %s", r.Branch, r.URL)
		}
	}
	if repos[2].Branch != "task" {
		t.Errorf("expected branch=task, got %q", repos[2].Branch)
	}
}
