package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

type MoveBody struct {
	// Что
	Target string `yaml:"target,omitempty" json:"target,omitempty" required:""`

	// Куда
	Destination string `yaml:"destination,omitempty" json:"destination,omitempty" required:""`

	// Заменить объект по destination
	Replace bool `yaml:"replace,omitempty" json:"replace,omitempty"`

	// Создать ссылку из родительской директории цели на назначение
	CreateLink bool `yaml:"create-link,omitempty" json:"create-link,omitempty"`
}

func (b *MoveBody) Execute(ctx context.Context, svc Service) error {
	var withText []string
	if b.CreateLink {
		withText = append(withText, "with linking")
	}
	if b.Replace {
		withText = append(withText, "with replacing")
	}
	app.Log.Info(fmt.Sprintf("Moving %s to %s%s", b.Target, b.Destination, " "+strings.Join(withText, " and ")))

	if !filepath.IsAbs(b.Target) {
		return fmt.Errorf("target in move type must be absolute path")
	}
	if !filepath.IsAbs(b.Destination) {
		return fmt.Errorf("destination in move type must be absolute path")
	}

	if err := osutils.Move(b.Target, b.Destination, b.Replace); err != nil {
		return err
	}

	if b.CreateLink {
		linkBody := &LinkBody{Target: b.Target, To: b.Destination}

		if err := linkBody.Execute(ctx, svc); err != nil {
			return err
		}
	}
	return nil
}
