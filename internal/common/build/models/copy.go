package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"path/filepath"
)

type CopyBody struct {
	// Путь до файла, кого копировать
	Target string `yaml:"target,omitempty" json:"target,omitempty"`

	// Путь до файла, куда копировать
	Destination string `yaml:"destination,omitempty" json:"destination,omitempty"`

	// Заменять ли destination
	Replace bool `yaml:"replace,omitempty" json:"replace,omitempty"`
}

func (b *CopyBody) Check() error {
	return nil
}

func (b *CopyBody) Execute(_ context.Context, _ Service) error {
	replaceText := ""
	if b.Replace {
		replaceText = " with replacing"
	}
	app.Log.Info(fmt.Sprintf("Copying %s to %s%s", b.Target, b.Destination, replaceText))

	if !filepath.IsAbs(b.Destination) {
		return fmt.Errorf("destination in move type must be absolute path")
	}

	return osutils.Copy(b.Target, b.Destination, b.Replace)
}
