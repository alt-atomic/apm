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
	Target string `yaml:"target,omitempty" json:"target,omitempty"`

	// Regex шаблон
	Pattern string `yaml:"pattern,omitempty" json:"pattern,omitempty"`

	// Замена
	Repl string `yaml:"repl,omitempty" json:"repl,omitempty"`
}

func (b *ReplaceBody) Check() error {
	return nil
}

func (b *ReplaceBody) Execute(_ context.Context, _ Service) error {
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
