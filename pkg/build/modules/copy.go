package modules

import (
	"context"
	"fmt"
	"path/filepath"

	"altlinux.space/alt-atomic/apm/pkg/build"

	"altlinux.space/alt-atomic/apm/pkg/osutils"
)

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

func (b *CopyBody) Execute(_ context.Context, _ build.RuntimeContext) (any, error) {
	replaceText := ""
	if b.Replace {
		replaceText = " with replacing"
	}
	build.Log().Info(fmt.Sprintf("Copying %s to %s%s", b.Source, b.Destination, replaceText))

	if !filepath.IsAbs(b.Destination) {
		return nil, fmt.Errorf("destination in move type must be absolute path")
	}

	return nil, osutils.Copy(b.Source, b.Destination, b.Replace)
}

func init() { build.Register(TypeCopy, func() build.Body { return &CopyBody{} }) }
