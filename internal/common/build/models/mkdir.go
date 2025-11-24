package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MkdirBody struct {
	// Пути, по которым нужно создать директории
	Targets []string `yaml:"targets,omitempty" json:"targets,omitempty" required:""`

	// Права у директорий в формате rwxrwxrwx
	Perm string `yaml:"perm,omitempty" json:"perm,omitempty" required:""`
}

func (b *MkdirBody) Execute(_ context.Context, _ Service) (any, error) {
	app.Log.Info(fmt.Sprintf("Creating dirs at %s", strings.Join(b.Targets, ", ")))
	for _, pathTarget := range b.Targets {
		if !filepath.IsAbs(pathTarget) {
			return nil, fmt.Errorf("target in mkdir type must be absolute path")
		}

		mode, err := osutils.StringToFileMode(b.Perm)
		if err != nil {
			return nil, err
		}
		if err = os.MkdirAll(pathTarget, mode); err != nil {
			return nil, err
		}
	}
	return nil, nil
}
