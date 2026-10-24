package altfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestService(dir string) *Service {
	return New(Config{
		EtcPasswd:   filepath.Join(dir, "etc_passwd"),
		EtcGroup:    filepath.Join(dir, "etc_group"),
		EtcNsswitch: filepath.Join(dir, "nsswitch.conf"),
		LibPasswd:   filepath.Join(dir, "lib_passwd"),
		LibGroup:    filepath.Join(dir, "lib_group"),
	})
}

// writeFile пишет тестовый файл, падая при ошибке.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
