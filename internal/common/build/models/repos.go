/*
 * Copyright (C) 2026 Vladimir Romanov <rirusha@altlinux.org>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see
 * <https://www.gnu.org/licenses/gpl-3.0-standalone.html>.
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */
package models

import (
	"apm/internal/common/app"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var (
	aptSourcesList  = "/etc/apt/sources.list"
	aptSourcesListD = "/etc/apt/sources.list.d"
)

type ReposBody struct {
	// Очистить все репозитории
	Clean bool `yaml:"clean,omitempty" json:"clean,omitempty"`

	// Кастомные записи в sources.list
	Custom []string `yaml:"custom,omitempty" json:"custom,omitempty"`

	// Ветка репозитория ALT. Закомментирует остальные репозитории, для очистки есть clean
	Branch string `yaml:"branch,omitempty" json:"branch,omitempty"`

	// Дата в формате YYYY.MM.DD. Если пуст, берется обычный репозиторий. Может быть latest
	Date string `yaml:"date,omitempty" json:"date,omitempty" needs:"Branch"`

	// Задачи для подключения в качестве репозиториев
	Tasks []string `yaml:"tasks,omitempty" json:"tasks,omitempty"`

	// Имя файла репозиториев
	Name string `yaml:"name,omitempty" json:"name,omitempty" depricated:"0.4.0"`

	// Не обновлять базу данных после сохранения репозиториев
	NoUpdate bool `yaml:"no-update,omitempty" json:"no-update,omitempty"`

	// Очистить временные репозитории
	CleanTemporary bool `yaml:"clean-temporary,omitempty" json:"clean-temporary,omitempty" conflicts:"Clean"`
}

func (b *ReposBody) Execute(ctx context.Context, svc Service) (any, error) {
	var repoSvc = svc.RepoService()

	if b.Branch != "" && !slices.Contains(repoSvc.GetBranches(), b.Branch) {
		return nil, fmt.Errorf(app.T_("unknown branch %s"), b.Branch)
	}

	if b.Clean {
		removedRepos, err := repoSvc.GetRepositories(ctx, true)
		if err != nil {
			return nil, err
		}

		removedReposEntry := []string{}
		for _, repo := range removedRepos {
			removedReposEntry = append(removedReposEntry, repo.Entry)
		}

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
			return nil, err
		}

		app.Log.Info(fmt.Sprintf("Cleaning repos in %s", aptSourcesList))
		if err := os.WriteFile(aptSourcesList, []byte("\n"), 0644); err != nil {
			return nil, err
		}
		app.Log.Info(fmt.Sprintf("Cleaned all repos: \n%s", strings.Join(removedReposEntry, ",\n")))
	} else if b.CleanTemporary {
		removed, err := repoSvc.CleanTemporary(ctx)
		if err != nil {
			return nil, err
		}
		app.Log.Info(fmt.Sprintf("Cleaned temporary repos: \n%s", strings.Join(removed, ",\n")))
	}

	if b.Branch != "" {
		added, _, err := repoSvc.SetBranch(ctx, b.Branch, b.Date)
		if err != nil {
			return nil, err
		}
		app.Log.Info(fmt.Sprintf("Added repos for branch %s: \n%s", b.Branch, strings.Join(added, ",\n")))
	}

	for _, source := range append(b.Custom, b.Tasks...) {
		repoSvc.AddRepository(ctx, source, "")
	}

	if !b.NoUpdate {
		if err := svc.UpdatePackages(ctx); err != nil {
			return nil, err
		}

		if err := svc.InstallPackages(ctx, []string{"ca-certificates"}); err != nil {
			return nil, err
		}
	}

	return nil, nil
}
