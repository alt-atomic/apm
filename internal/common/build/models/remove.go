package models

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/pkg/osutils"
)

type RemoveBody struct {
	// Путь до объектов, которые нужно удалить
	Targets []string `yaml:"targets,omitempty" json:"targets,omitempty" required:""`

	// Очистить объекты вместо удаления
	Inside bool `yaml:"inside,omitempty" json:"inside,omitempty"`
}

func (b *RemoveBody) Execute(_ context.Context, _ Service) (any, error) {
	for _, pathTarget := range b.Targets {
		if !filepath.IsAbs(pathTarget) {
			return nil, fmt.Errorf("target in remove type must be absolute path")
		}
	}
	if b.Inside {
		app.Log.Info(fmt.Sprintf("Cleaning %s", strings.Join(b.Targets, ", ")))
	} else {
		app.Log.Info(fmt.Sprintf("Removing %s", strings.Join(b.Targets, ", ")))
	}

	for _, pathTarget := range b.Targets {
		if b.Inside {
			if err := osutils.Clean(pathTarget); err != nil {
				return nil, err
			}
		} else {
			if err := os.RemoveAll(pathTarget); err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
}
