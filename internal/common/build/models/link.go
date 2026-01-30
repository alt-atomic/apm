package models

import (
	"apm/internal/common/app"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

type LinkBody struct {
	// Где создать ссылку, абсолютный путь
	Target string `yaml:"target,omitempty" json:"target,omitempty" required:""`

	// Куда она будет вести
	To string `yaml:"to,omitempty" json:"to,omitempty" required:""`

	// Заменить ли target
	Replace bool `yaml:"replace,omitempty" json:"replace,omitempty"`
}

func (b *LinkBody) Execute(ctx context.Context, svc Service) (any, error) {
	if !filepath.IsAbs(b.Target) {
		return nil, errors.New(app.T_("target in link type must be absolute path"))
	}

	if _, err := os.Stat(path.Dir(b.Target)); os.IsNotExist(err) {
		return nil, fmt.Errorf(app.T_("path %s for link doesn't exists"), path.Dir(b.Target))
	}

	app.Log.Info(fmt.Sprintf("Linking %s to %s", b.Target, b.To))
	if b.Replace {
		if err := os.RemoveAll(b.Target); err != nil {
			return nil, err
		}
	}

	if filepath.IsAbs(b.To) {
		relativePath, err := filepath.Rel(path.Dir(b.Target), b.To)
		if err != nil {
			relativePath = b.To
		}
		return nil, os.Symlink(relativePath, b.Target)
	}
	return nil, os.Symlink(b.To, b.Target)
}

func (b *LinkBody) Hash(_ string, env map[string]string) string {
	return hashWithEnv(b, env)
}
