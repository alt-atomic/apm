package models

import (
	"apm/internal/common/app"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

type LinkBody struct {
	// Где создать ссылку, абсолютный путь
	Target string `yaml:"target,omitempty" json:"target,omitempty" required:""`

	// Куда она будет вести
	Destination string `yaml:"destination,omitempty" json:"destination,omitempty" required:""`

	// Заменить ли target
	Replace bool `yaml:"replace,omitempty" json:"replace,omitempty"`
}

func (b *LinkBody) Execute(ctx context.Context, svc Service) error {
	if !filepath.IsAbs(b.Target) {
		return fmt.Errorf("target in link type must be absolute path")
	}

	mkdirBody := MkdirBody{
		Targets: []string{path.Dir(b.Target)},
		Perm:    "rw-r--r--",
	}
	mkdirBody.Execute(ctx, svc)

	app.Log.Info(fmt.Sprintf("Linking %s to %s", b.Target, b.Destination))
	if b.Replace {
		if err := os.RemoveAll(b.Target); err != nil {
			return err
		}
	}

	if filepath.IsAbs(b.Destination) {
		relativePath, err := filepath.Rel(path.Dir(b.Target), b.Destination)
		if err != nil {
			relativePath = b.Destination
		}
		return os.Symlink(relativePath, b.Target)
	}
	return os.Symlink(b.Destination, b.Target)
}
