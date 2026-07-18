package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"altlinux.space/alt-atomic/apm/pkg/build"

	"altlinux.space/alt-atomic/apm/pkg/osutils"
)

type MoveBody struct {
	// Что
	Source string `yaml:"source,omitempty" json:"source,omitempty" required:""`

	// Куда
	Destination string `yaml:"destination,omitempty" json:"destination,omitempty" required:""`

	// Заменить объект по destination
	Replace bool `yaml:"replace,omitempty" json:"replace,omitempty"`

	// Создать ссылку из родительской директории цели на назначение
	CreateLink bool `yaml:"create-link,omitempty" json:"create-link,omitempty"`
}

func (b *MoveBody) Execute(ctx context.Context, svc build.RuntimeContext) (any, error) {
	var withText []string
	if b.CreateLink {
		withText = append(withText, "with linking")
	}
	if b.Replace {
		withText = append(withText, "with replacing")
	}
	build.Log().Info(fmt.Sprintf("Moving %s to %s%s", b.Source, b.Destination, " "+strings.Join(withText, " and ")))

	if !filepath.IsAbs(b.Source) {
		return nil, fmt.Errorf("source in move type must be absolute path")
	}
	if !filepath.IsAbs(b.Destination) {
		return nil, fmt.Errorf("destination in move type must be absolute path")
	}

	if err := osutils.Move(b.Source, b.Destination, b.Replace); err != nil {
		return nil, err
	}

	if b.CreateLink {
		linkBody := &LinkBody{Target: b.Source, To: b.Destination}

		if _, err := linkBody.Execute(ctx, svc); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func init() { build.Register(TypeMove, func() build.Body { return &MoveBody{} }) }
