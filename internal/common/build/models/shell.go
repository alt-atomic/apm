package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
)

type ShellBody struct {
	// Команды на выполнение
	Command string `yaml:"command,omitempty" json:"command,omitempty" required:""`

	// Quite command output
	Quite bool `yaml:"quite,omitempty" json:"quite,omitempty"`
}

func (b *ShellBody) Execute(ctx context.Context, svc Service) error {
	app.Log.Debug(fmt.Sprintf("Executing `%s`", b.Command))

	if err := osutils.ExecSh(ctx, b.Command, svc.ResourcesDir(), true, b.Quite); err != nil {
		return err
	}

	return nil
}
