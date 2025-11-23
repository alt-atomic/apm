package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

var (
	goodBranches = []string{
		"sisyphus",
		// "p11",
	}
	aptSourcesList  = "/etc/apt/sources.list"
	aptSourcesListD = "/etc/apt/sources.list.d"
)

type ReposBody struct {
	// Очистить репозитории
	Clean bool `yaml:"clean,omitempty" json:"clean,omitempty"`

	// Кастомные записи в sources.list
	Custom []string `yaml:"custom,omitempty" json:"custom,omitempty" needs:"Name"`

	// Ветка репозитория ALT. Сейчас доступен только sisyphus
	Branch string `yaml:"branch,omitempty" json:"branch,omitempty" needs:"Name"`

	// Дата в формате YYYY.MM.DD. Если пуст, берется обычныц репозиторий. Может быть latest
	Date string `yaml:"date,omitempty" json:"date,omitempty" needs:"Branch"`

	// Задачи для подключения в качестве репозиториев
	Tasks []string `yaml:"tasks,omitempty" json:"tasks,omitempty" needs:"Name"`

	// Имя файла репозиториев
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Не обновлять базу данных после сохранения репозиториев
	NoUpdate bool `yaml:"no-update,omitempty" json:"no-update,omitempty"`
}

func (b *ReposBody) AllRepos() []string {
	var repos []string
	repos = append(repos, b.Custom...)
	repos = append(repos, b.TasksRepos()...)
	repos = append(repos, b.BranchRepos()...)
	return repos
}

func (b *ReposBody) TasksRepos() []string {
	var repos []string
	var templates []string

	switch runtime.GOARCH {
	case "amd64":
		templates = append(templates, "rpm http://git.altlinux.org repo/%s/x86_64 task")
	case "arm64", "aarch64":
		templates = append(templates, "rpm http://git.altlinux.org repo/%s/aarch64 task")
	default:
		return []string{}
	}

	for _, task := range b.Tasks {
		for _, template := range templates {
			repos = append(repos, fmt.Sprintf(template, task))
		}
	}

	return repos
}

func (b *ReposBody) BranchRepos() []string {
	if b.Branch == "" {
		return []string{}
	}

	var repos []string
	repoArchs := []string{}

	switch runtime.GOARCH {
	case "amd64":
		repoArchs = append(repoArchs, "x86_64", "noarch", "x86_64-i586")
	case "arm64", "aarch64":
		repoArchs = append(repoArchs, "aarch64", "noarch")
	default:
		return []string{}
	}

	if b.Date == "" {
		branchName := ""
		if b.Branch == "sisyphus" {
			branchName = osutils.Capitalize(b.Branch)
		} else {
			branchName = b.Branch
		}
		for _, arch := range repoArchs {
			repos = append(repos, fmt.Sprintf("rpm [alt] https://ftp.altlinux.org/pub/distributions ALTLinux/%s/%s classic", branchName, arch))
		}
	} else {
		for _, arch := range repoArchs {
			repos = append(repos, fmt.Sprintf("rpm [alt] https://ftp.altlinux.org/pub/distributions/archive %s/%s/%s classic", b.Branch, fmt.Sprintf("date/%s", strings.ReplaceAll(b.Date, ".", "/")), arch))
		}
	}

	return repos
}

func (b *ReposBody) Execute(ctx context.Context, svc Service) error {
	if b.Branch == "" || !slices.Contains(goodBranches, b.Branch) {
		return fmt.Errorf(app.T_("unknown branch %s"), b.Branch)
	}

	if b.Clean {
		app.Log.Info(fmt.Sprintf("Cleaning repos in %s", aptSourcesListD))
		if err := filepath.Walk(aptSourcesListD, func(p string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if p != aptSourcesListD {
				if err = os.RemoveAll(p); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}

		app.Log.Info(fmt.Sprintf("Cleaning repos in %s", aptSourcesList))
		if err := os.WriteFile(aptSourcesList, []byte("\n"), 0644); err != nil {
			return err
		}
	}

	allRepos := b.AllRepos()
	if len(allRepos) == 0 {
		return nil
	}

	sourcesPath := path.Join(
		aptSourcesListD,
		fmt.Sprintf("%s.list", strings.ReplaceAll(b.Name, " ", "-")),
	)
	app.Log.Info(fmt.Sprintf("Setting repos to %s", sourcesPath))

	if err := svc.InstallPackages(ctx, []string{"ca-certificates"}); err != nil {
		return err
	}

	if err := os.WriteFile(sourcesPath, []byte(strings.Join(allRepos, "\n")+"\n"), 0644); err != nil {
		return err
	}

	if !b.NoUpdate {
		return svc.UpdatePackages(ctx)
	}

	return nil
}
