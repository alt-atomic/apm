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
	"slices"
	"strings"
)

type ReposBody struct {
	// Очистить все репозитории
	Clean bool `yaml:"clean,omitempty" json:"clean,omitempty"`

	// Кастомные записи в sources.list
	Custom []string `yaml:"custom,omitempty" json:"custom,omitempty"`

	// Ветка репозитория ALT. Закомментирует остальные репозитории, для очистки есть clean
	Branch string `yaml:"branch,omitempty" json:"branch,omitempty"`

	// Дата в формате YYYYMMDD или YYYY/MM/DD. Если пуст, берется обычный репозиторий. Может быть latest
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
		removed, err := repoSvc.RemoveRepository(ctx, "all", "", true)
		if err != nil {
			return nil, err
		}
		app.Log.Info(fmt.Sprintf("Cleaned all repos: \n%s", strings.Join(removed, ",\n")))
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
		added, err := repoSvc.AddRepository(ctx, source, "")
		if err != nil {
			return nil, err
		}
		if len(added) > 0 {
			app.Log.Info(fmt.Sprintf("Added repo: %s", strings.Join(added, ", ")))
		}
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
