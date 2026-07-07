package modules

import (
	"context"
	"fmt"
	"strings"

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

func (b *SystemdBody) Execute(ctx context.Context, svc build.RuntimeContext) (any, error) {
	for _, target := range b.Targets {
		action, verb := systemdAction(b.Masked, b.Enabled)
		build.Log().Info(fmt.Sprintf("%s %s", verb, target))

		args := systemctlArgs(action, b.Global, target)
		stdout, stderr, err := svc.Runner().Run(ctx, args)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w: %s", action, target, err, strings.TrimSpace(stderr))
		}
		svc.CollectOutput(stdout)
	}
	return nil, nil
}

// systemdAction picks the systemctl action and its log verb; enable wins over mask.
func systemdAction(masked, enabled bool) (action, verb string) {
	switch {
	case enabled:
		return "enable", "Enabling"
	case masked:
		return "mask", "Masking"
	default:
		return "disable", "Disabling"
	}
}

// systemctlArgs builds the systemctl command line.
func systemctlArgs(action string, global bool, target string) []string {
	args := []string{"systemctl"}
	if global {
		args = append(args, "--global")
	}
	return append(args, action, target)
}

func init() { build.Register(TypeSystemd, func() build.Body { return &SystemdBody{} }) }
