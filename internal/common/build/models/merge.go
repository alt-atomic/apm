package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"path/filepath"
)

type MergeBody struct {
	// Путь до файла, содеримое которого нужно взять
	Target string `yaml:"target,omitempty" json:"target,omitempty" required:""`

	// Путь до файла, куда нужно добавить содержимое
	Destination string `yaml:"destination,omitempty" json:"destination,omitempty" required:""`
}

func (b *MergeBody) Execute(_ context.Context, _ Service) (any, error) {
	app.Log.Info(fmt.Sprintf("Merging %s with %s", b.Target, b.Destination))

	if !filepath.IsAbs(b.Target) {
		return nil, fmt.Errorf("target in merge type must be absolute path")
	}

	return nil, osutils.AppendFile(b.Target, b.Destination, 0)
}
