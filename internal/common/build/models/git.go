package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
)

type GitBody struct {
	// URL git-репозитория
	Url string `yaml:"url,omitempty" json:"url,omitempty" required:""`

	// Команды для выполнения относительно git репозитория
	Command string `yaml:"command,omitempty" json:"command,omitempty" required:""`

	// Зависимости для сборки. Они будут удалены после завершения модуля
	BuildDeps []string `yaml:"build-deps,omitempty" json:"build-deps,omitempty"`

	// Зависимости для самой программы. Они не будут удалены после завершения модуля, даже если указаны в build-deps
	Deps []string `yaml:"deps,omitempty" json:"deps,omitempty"`

	// Git revision
	Rev string `yaml:"rev,omitempty" json:"rev,omitempty"`

	// Quiet command output
	Quiet bool `yaml:"quiet,omitempty" json:"quiet,omitempty"`
}

func (b *GitBody) Execute(ctx context.Context, svc Service) (any, error) {
	needInstallDeps := []string{}
	needRemoveDeps := []string{}

	for _, dep := range append(b.Deps, b.BuildDeps...) {
		pkg, err := svc.GetPackageByName(ctx, dep)
		if err != nil {
			return nil, err
		}
		if !pkg.Installed {
			needInstallDeps = append(needInstallDeps, pkg.Name)

			if slices.Contains(b.BuildDeps, pkg.Name) && !slices.Contains(b.Deps, pkg.Name) {
				needRemoveDeps = append(needRemoveDeps, pkg.Name)
			}
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

	args := []string{"clone"}
	if b.Rev != "" {
		args = append(args, "--revision="+b.Rev)
	}
	args = append(args, b.Url, tempDir)

	app.Log.Debug(fmt.Sprintf("Cloning %s to %s", b.Url, tempDir))
	if err = osutils.ExecSh(ctx, "git"+strings.Join(args, " "), "", false); err != nil {
		return nil, err
	}

	app.Log.Debug(fmt.Sprintf("Executing `%s`", b.Command))
	if err = osutils.ExecSh(ctx, b.Command, tempDir, b.Quiet); err != nil {
		return nil, err
	}

	if len(needRemoveDeps) != 0 {
		packagesBody := &PackagesBody{Remove: needRemoveDeps}

		if _, err := packagesBody.Execute(ctx, svc); err != nil {
			return nil, err
		}
	}
	return nil, nil
}
