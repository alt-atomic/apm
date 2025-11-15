package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
)

type ShellBody struct {
	// Команды на выполнение
	Commands string `yaml:"command,omitempty" json:"command,omitempty"`
}

func (b *ShellBody) Check() error {
	return nil
}

func (b *ShellBody) Execute(ctx context.Context, svc Service) error {
	app.Log.Info(fmt.Sprintf("Executing `%s`", b.Commands))
	if err := osutils.ExecSh(ctx, b.Commands, svc.ResourcesDir(), true); err != nil {
		return err
	}
	return nil
}
