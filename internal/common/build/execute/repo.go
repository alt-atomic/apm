package execute

import (
	"apm/internal/common/app"
	"apm/internal/common/build/core"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func Repos(ctx context.Context, svc Service) error {
	repos := svc.Config().Repos
	if repos.Clean {
		app.Log.Info(fmt.Sprintf("Cleaning repos in %s", core.AptSourcesListD))
		if err := filepath.Walk(core.AptSourcesListD, func(p string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if p != core.AptSourcesListD {
				if err := os.RemoveAll(p); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}

		if err := os.WriteFile(core.AptSourcesList, []byte(""), 0644); err != nil {
			return err
		}
	}

	allRepos := repos.AllRepos()
	if len(allRepos) == 0 {
		return nil
	}

	sourcesPath := path.Join(
		core.AptSourcesListD,
		fmt.Sprintf("%s.list", strings.ReplaceAll(svc.Config().Name, " ", "-")),
	)
	app.Log.Info(fmt.Sprintf("Setting repos to %s", sourcesPath))

	if err := svc.InstallPackages(ctx, []string{"ca-certificates"}); err != nil {
		return err
	}

	return os.WriteFile(sourcesPath, []byte(strings.Join(allRepos, "\n")+"\n"), 0644)
}
