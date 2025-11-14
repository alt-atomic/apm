package execute

import (
	"apm/internal/common/app"
	"apm/internal/common/build/core"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func Replace(_ context.Context, _ Service, b *core.Body) error {
	app.Log.Info(fmt.Sprintf("Replacing %s to %s in %s", b.Pattern, b.Repl, b.Target))

	if !filepath.IsAbs(b.Target) {
		return fmt.Errorf("target in replace type must be absolute path")
	}

	info, err := os.Stat(b.Target)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(b.Target)
	if err != nil {
		return err
	}
	re, err := regexp.Compile(b.Pattern)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		lines[i] = re.ReplaceAllString(line, b.Repl)
	}

	return os.WriteFile(b.Target, []byte(strings.Join(lines, "\n")), info.Mode().Perm())
}
