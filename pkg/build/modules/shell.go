package modules

import (
	"context"
	"fmt"

	"altlinux.space/alt-atomic/apm/pkg/build"

	"altlinux.space/alt-atomic/apm/pkg/command"
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

func (b *ShellBody) Execute(ctx context.Context, svc build.RuntimeContext) (any, error) {
	build.Log().Debug(fmt.Sprintf("Executing `%s`", b.Command))

	output := ShellOutput{}

	var opts []command.Option
	if b.Quiet {
		opts = append(opts, command.WithQuiet())
	}

	var envOutput string
	opts = append(opts, command.WithOutputCommand("env", &envOutput))

	stdout, _, err := svc.Runner().Run(ctx, []string{b.Command}, opts...)
	if err != nil {
		return nil, err
	}

	if !b.Quiet {
		svc.CollectOutput(stdout)
	}

	output.Env = GetEnvFromOutput(envOutput)

	return output, nil
}

func init() { build.Register(TypeShell, func() build.Body { return &ShellBody{} }) }
