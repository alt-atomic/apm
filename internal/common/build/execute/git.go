package execute

import (
	"apm/internal/common/app"
	"apm/internal/common/build/core"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func Git(ctx context.Context, svc Service, b *core.Body) error {
	if len(b.Deps) != 0 {
		var ops []string
		for _, p := range b.Deps {
			ops = append(ops, p+"+")
		}
		app.Log.Info(fmt.Sprintf("Installing %s", strings.Join(b.Deps, ", ")))
		if err := svc.CombineInstallRemovePackages(ctx, ops, false, false); err != nil {
			return err
		}
	}

	tempDir, err := os.MkdirTemp(os.TempDir(), "git-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	app.Log.Info(fmt.Sprintf("Cloning %s to %s", b.Target, tempDir))

	args := []string{"clone"}
	if b.Ref != "" {
		args = append(args, "-b", b.Ref)
	}
	args = append(args, b.Target, tempDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return err
	}

	for _, cmdSh := range b.GetCommands() {
		app.Log.Info(fmt.Sprintf("Executing `%s`", cmdSh))
		if err := osutils.ExecSh(ctx, cmdSh, tempDir, true); err != nil {
			return err
		}
	}

	if len(b.Deps) != 0 {
		var ops []string
		for _, p := range b.Deps {
			ops = append(ops, p+"-")
		}
		app.Log.Info(fmt.Sprintf("Removing %s", strings.Join(b.Deps, ", ")))
		if err = svc.CombineInstallRemovePackages(ctx, ops, true, true); err != nil {
			return err
		}
	}
	return nil
}
