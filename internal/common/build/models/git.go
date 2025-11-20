package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"os"
	"os/exec"
)

type GitBody struct {
	// URL git-репозитория
	Url string `yaml:"target,omitempty" json:"target,omitempty" required:""`

	// Команды для выполнения относительно git репозитория
	Command string `yaml:"command,omitempty" json:"command,omitempty" required:""`

	// Зависимости для сборки. Они будут удалены после завершения модуля
	Deps []string `yaml:"deps,omitempty" json:"deps,omitempty"`

	// Git reference
	Ref string `yaml:"ref,omitempty" json:"ref,omitempty"`
}

func (b *GitBody) Execute(ctx context.Context, svc Service) error {
	if len(b.Deps) != 0 {
		packagesBody := &PackagesBody{Install: b.Deps}

		if err := packagesBody.Execute(ctx, svc); err != nil {
			return err
		}
	}

	tempDir, err := os.MkdirTemp(os.TempDir(), "git-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	app.Log.Info(fmt.Sprintf("Cloning %s to %s", b.Url, tempDir))

	args := []string{"clone"}
	if b.Ref != "" {
		args = append(args, "-b", b.Ref)
	}
	args = append(args, b.Url, tempDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return err
	}

	app.Log.Info(fmt.Sprintf("Executing `%s`", b.Command))
	if err = osutils.ExecSh(ctx, b.Command, tempDir, true, false); err != nil {
		return err
	}

	if len(b.Deps) != 0 {
		packagesBody := &PackagesBody{Remove: b.Deps}

		if err := packagesBody.Execute(ctx, svc); err != nil {
			return err
		}
	}
	return nil
}
