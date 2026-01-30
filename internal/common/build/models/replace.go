package models

import (
	"apm/internal/common/app"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ReplaceBody struct {
	// Путь до файла
	Target string `yaml:"target,omitempty" json:"target,omitempty" required:""`

	// Regex шаблон
	Pattern string `yaml:"pattern,omitempty" json:"pattern,omitempty" required:""`

	// Замена
	Repl string `yaml:"repl,omitempty" json:"repl,omitempty" required:""`
}

func (b *ReplaceBody) Execute(_ context.Context, _ Service) (any, error) {
	app.Log.Info(fmt.Sprintf("Replacing %s to %s in %s", b.Pattern, b.Repl, b.Target))

	if !filepath.IsAbs(b.Target) {
		return nil, fmt.Errorf("target in replace type must be absolute path")
	}

	info, err := os.Stat(b.Target)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(b.Target)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(b.Pattern)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		lines[i] = re.ReplaceAllString(line, b.Repl)
	}

	return nil, os.WriteFile(b.Target, []byte(strings.Join(lines, "\n")), info.Mode().Perm())
}

func (b *ReplaceBody) Hash(_ string, env map[string]string) string {
	return hashWithEnv(b, env)
}
