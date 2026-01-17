package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"path/filepath"
)

// CopyBody копирование файлов и директорий
type CopyBody struct {
	// Путь до файла, кого копировать
	Source string `yaml:"source,omitempty" json:"source,omitempty" required:""`

	// Путь до файла, куда копировать
	Destination string `yaml:"destination,omitempty" json:"destination,omitempty" required:""`

	// Заменять ли destination
	Replace bool `yaml:"replace,omitempty" json:"replace,omitempty"`
}

func (b *CopyBody) Check() error {

	return nil
}

func (b *CopyBody) Execute(_ context.Context, _ Service) (any, error) {
	replaceText := ""
	if b.Replace {
		replaceText = " with replacing"
	}
	app.Log.Info(fmt.Sprintf("Copying %s to %s%s", b.Source, b.Destination, replaceText))

	if !filepath.IsAbs(b.Destination) {
		return nil, fmt.Errorf("destination in move type must be absolute path")
	}

	return nil, osutils.Copy(b.Source, b.Destination, b.Replace)
}
