package models

import (
	"apm/internal/common/app"
	"apm/internal/common/command"
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

	var opts []command.Option
	if b.Quiet {
		opts = append(opts, command.WithQuiet())
	}

	var envOutput string
	opts = append(opts, command.WithOutputCommand("env", &envOutput))

	if _, _, err := svc.Runner().Run(ctx, []string{b.Command}, opts...); err != nil {
		return nil, err
	}

	output.Env = GetEnvFromOutput(envOutput)

	return output, nil
}
