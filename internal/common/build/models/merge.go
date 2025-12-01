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
	Source string `yaml:"source,omitempty" json:"source,omitempty" required:""`

	// Путь до файла, куда нужно добавить содержимое
	Destination string `yaml:"destination,omitempty" json:"destination,omitempty" required:""`
}

func (b *MergeBody) Execute(_ context.Context, _ Service) (any, error) {
	app.Log.Info(fmt.Sprintf("Merging %s with %s", b.Source, b.Destination))

	if !filepath.IsAbs(b.Destination) {
		return nil, fmt.Errorf("destination in merge type must be absolute path")
	}

	return nil, osutils.AppendFile(b.Source, b.Destination, 0)
}
