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

	// Quiet command output
	Quiet bool `yaml:"quiet,omitempty" json:"quiet,omitempty"`
}

type ShellOutput struct {
	Env map[string]string
}

func (b *ShellBody) Execute(ctx context.Context, svc Service) (any, error) {
	app.Log.Debug(fmt.Sprintf("Executing `%s`", b.Command))

	output := ShellOutput{}

	if _, cmdOutputShell, err := osutils.ExecShWithDivider(
		ctx,
		b.Command,
		"env",
		Divider,
		b.Quiet,
	); err != nil {
		return nil, err
	} else {
		output.Env = GetEnvFromOutput(cmdOutputShell)
	}

	return output, nil
}
