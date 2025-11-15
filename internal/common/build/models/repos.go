package models

import (
	"apm/internal/common/app"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

var aptSourcesList = "/etc/apt/sources.list"
var aptSourcesListD = "/etc/apt/sources.list.d"

type ReposBody struct {
	// Очистить репозитории
	Clean bool `yaml:"clean,omitempty" json:"clean,omitempty"`

	// Кастомные записи в sources.list
	Custom []string `yaml:"custom,omitempty" json:"custom,omitempty"`

	// Ветка репозитория ALT. Сейчас доступен только sisyphus
	Branch string `yaml:"branch,omitempty" json:"branch,omitempty"`

	// Дата в формате YYYY/MM/DD. Если пуст, берется latest
	Date string `yaml:"date,omitempty" json:"date,omitempty"`

	// Задачи для подключения в качестве репозиториев
	Tasks []string `yaml:"tasks,omitempty" json:"tasks,omitempty"`

	// Имя файла репозиториев
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
}

func (r *ReposBody) AllRepos() []string {
	var repos []string
	repos = append(repos, r.Custom...)
	repos = append(repos, r.TasksRepos()...)
	repos = append(repos, r.BranchRepos()...)
	return repos
}

func (r *ReposBody) TasksRepos() []string {
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

	for _, task := range r.Tasks {
		for _, template := range templates {
			repos = append(repos, fmt.Sprintf(template, task))
		}
	}

	return repos
}

func (r *ReposBody) BranchRepos() []string {
	if r.Branch == "" {
		return []string{}
	}

	date := "latest"
	if r.Date != "" {
		date = fmt.Sprintf("date/%s", r.Date)
	}

	var repos []string
	var templates []string

	switch runtime.GOARCH {
	case "amd64":
		templates = append(
			templates,
			"rpm [alt] https://ftp.altlinux.org/pub/distributions/archive %s/%s/x86_64 classic",
			"rpm [alt] https://ftp.altlinux.org/pub/distributions/archive %s/%s/x86_64-i586 classic",
			"rpm [alt] https://ftp.altlinux.org/pub/distributions/archive %s/%s/noarch classic",
		)
	case "arm64", "aarch64":
		templates = append(
			templates,
			"rpm [alt] https://ftp.altlinux.org/pub/distributions/archive %s/%s/aarch64 classic",
			"rpm [alt] https://ftp.altlinux.org/pub/distributions/archive %s/%s/noarch classic",
		)
	default:
		return []string{}
	}

	for _, template := range templates {
		repos = append(repos, fmt.Sprintf(template, r.Branch, date))
	}

	return repos
}

func (b *ReposBody) Check() error {
	return nil
}

func (b *ReposBody) Execute(ctx context.Context, svc Service) error {
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

		if err := os.WriteFile(aptSourcesList, []byte(""), 0644); err != nil {
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

	return os.WriteFile(sourcesPath, []byte(strings.Join(allRepos, "\n")+"\n"), 0644)
}
