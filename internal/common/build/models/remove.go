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

type RemoveBody struct {
	// Путь до объектов, которые нужно удалить
	Targets []string `yaml:"targets,omitempty" json:"targets,omitempty" required:""`

	// Очистить объекты вместо удаления
	Inside bool `yaml:"inside,omitempty" json:"inside,omitempty"`
}

func (b *RemoveBody) Execute(_ context.Context, _ Service) error {
	for _, pathTarget := range b.Targets {
		if !filepath.IsAbs(pathTarget) {
			return fmt.Errorf("target in remove type must be absolute path")
		}

		if b.Inside {
			app.Log.Info(fmt.Sprintf("Cleaning %s", strings.Join(b.Targets, ", ")))
			if err := osutils.Clean(pathTarget); err != nil {
				return err
			}
		} else {
			app.Log.Info(fmt.Sprintf("Removing %s", strings.Join(b.Targets, ", ")))
			if err := os.RemoveAll(pathTarget); err != nil {
				return err
			}
		}
	}
	return nil
}
