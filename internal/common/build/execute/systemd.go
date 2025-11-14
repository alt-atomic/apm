package execute

import (
	"apm/internal/common/app"
	"apm/internal/common/build/core"
	"context"
	"fmt"
	"os"
	"os/exec"
)

func Systemd(ctx context.Context, _ Service, b *core.Body) error {
	for _, target := range b.GetTargets() {
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
		app.Log.Info(text)

		var args []string
		if b.Global {
			args = append(args, "--global")
		}
		args = append(args, action, target)

		cmd := exec.CommandContext(ctx, "systemctl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}
