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
	Url string `yaml:"url,omitempty" json:"url,omitempty" required:""`

	// Команды для выполнения относительно git репозитория
	Command string `yaml:"command,omitempty" json:"command,omitempty" required:""`

	// Зависимости для сборки. Они будут удалены после завершения модуля
	Deps []string `yaml:"deps,omitempty" json:"deps,omitempty"`

	// Git reference
	Ref string `yaml:"ref,omitempty" json:"ref,omitempty"`

	// Quiet command output
	Quiet bool `yaml:"quiet,omitempty" json:"quiet,omitempty"`
}

func (b *GitBody) Execute(ctx context.Context, svc Service) (any, error) {
	needInstallDeps := []string{}

	for _, dep := range b.Deps {
		pkg, err := svc.GetPackageByName(ctx, dep)
		if err != nil {
			return nil, err
		}
		if !pkg.Installed {
			needInstallDeps = append(needInstallDeps, pkg.Name)
		}
	}

	if len(needInstallDeps) != 0 {
		packagesBody := &PackagesBody{Install: needInstallDeps}

		if _, err := packagesBody.Execute(ctx, svc); err != nil {
			return nil, err
		}
	}

	tempDir, err := os.MkdirTemp(os.TempDir(), "git-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	app.Log.Debug(fmt.Sprintf("Cloning %s to %s", b.Url, tempDir))

	args := []string{"clone"}
	if b.Ref != "" {
		args = append(args, "-b", b.Ref)
	}
	args = append(args, b.Url, tempDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return nil, err
	}

	app.Log.Debug(fmt.Sprintf("Executing `%s`", b.Command))
	if _, err = osutils.ExecShWithOutput(ctx, b.Command, tempDir, b.Quiet); err != nil {
		return nil, err
	}

	if len(needInstallDeps) != 0 {
		packagesBody := &PackagesBody{Remove: needInstallDeps}

		if _, err := packagesBody.Execute(ctx, svc); err != nil {
			return nil, err
		}
	}
	return nil, nil
}
