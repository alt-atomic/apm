package aptrepo

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"altlinux.space/alt-atomic/apm/pkg/command"
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

// noPackages сообщает, что ни один пакет не установлен (схема http)
func noPackages(context.Context, string) bool { return false }

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

	s := &RepoService{
		confMain:   confMain,
		confDir:    confDir,
		arch:       "x86_64",
		useArepo:   true,
		httpClient: &http.Client{},
		hasPackage: noPackages,
		runner:     runner,
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
