package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"altlinux.space/alt-atomic/apm/pkg/build"
)

type SystemdBody struct {
	// Имена сервисов
	Targets []string `yaml:"targets,omitempty" json:"targets,omitempty" required:""`

	// Включать или нет
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty" conflicts:"Masked"`

	// Включать ли сервис глобально, для всех пользователей
	Global bool `yaml:"global,omitempty" json:"global,omitempty"`

	// Маскировать ли сервис
	Masked bool `yaml:"masked,omitempty" json:"masked,omitempty" conflicts:"Enabled"`
}

func (b *SystemdBody) Execute(ctx context.Context, _ build.RuntimeContext) (any, error) {
	for _, target := range b.Targets {
		var text = fmt.Sprintf("Disabling %s", target)
		var action = "disable"
		if b.Masked {
			text = fmt.Sprintf("Masking %s", target)
			action = "mask"
		}
		if b.Enabled {
			text = fmt.Sprintf("Enabling %s", target)
			action = "enable"
		}
		build.Log().Info(text)

		var args []string
		if b.Global {
			args = append(args, "--global")
		}
		args = append(args, action, target)

		cmd := exec.CommandContext(ctx, "systemctl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func init() { build.Register(TypeSystemd, func() build.Body { return &SystemdBody{} }) }
