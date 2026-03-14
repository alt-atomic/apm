// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package service

import (
	"apm/internal/common/app"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// AddRepository добавляет репозиторий
func (s *RepoService) AddRepository(ctx context.Context, args []string, date string) ([]Repository, error) {
	s.ensureInitialized()
	urls, err := s.parseSourceArgs(ctx, args, date)
	if err != nil {
		return nil, err
	}

	if len(urls) == 0 {
		return nil, errors.New(app.T_("Failed to parse repository source"))
	}

	var added []Repository

	for _, u := range urls {
		exists, commented, err := s.checkRepoExists(ctx, u)
		if err != nil {
			return added, err
		}

		if exists {
			continue
		}

		var file string
		if commented {
			file, err = s.uncommentRepo(u)
			if err != nil {
				return added, err
			}
		} else {
			err = s.appendRepo(u)
			if err != nil {
				return added, err
			}
			file = s.confMain
		}

		if repo := s.parseLine(u, file, true); repo != nil {
			added = append(added, *repo)
		}
	}

	if len(args) > 0 {
		s.setPriorityMacro(args[0], date)
	}

	return added, nil
}

// RemoveRepository удаляет репозиторий
func (s *RepoService) RemoveRepository(ctx context.Context, args []string, date string, purge bool) ([]Repository, error) {
	s.ensureInitialized()
	var removed []Repository

	if len(args) == 0 {
		return nil, errors.New(app.T_("Repository source must be specified"))
	}

	source := args[0]

	if source == "all" {
		repos, err := s.GetRepositories(ctx, false)
		if err != nil {
			return nil, err
		}

		removed = append(removed, repos...)

		if purge {
			if err = s.purgeAllRepos(); err != nil {
				return removed, err
			}
		} else {
			for _, repo := range repos {
				if err = s.removeOrCommentRepo(repo.Entry); err != nil {
					return removed, err
				}
			}
		}

		s.removePriorityMacro()

		return removed, nil
	}

	urls, err := s.parseSourceArgs(ctx, args, date)
	if err != nil {
		return nil, err
	}

	// Получаем текущие репозитории для поиска файлов
	allRepos, _ := s.GetRepositories(ctx, true)
	repoByEntry := make(map[string]Repository, len(allRepos))
	for _, r := range allRepos {
		repoByEntry[canonicalizeRepoLine(r.Entry)] = r
	}

	for _, u := range urls {
		active, commented, checkErr := s.checkRepoExists(ctx, u)
		if checkErr != nil {
			continue
		}
		if !active && !commented {
			continue
		}

		err = s.removeOrCommentRepo(u)
		if err != nil {
			continue
		}

		canonical := canonicalizeRepoLine(u)
		if repo, ok := repoByEntry[canonical]; ok {
			repo.Active = false
			removed = append(removed, repo)
		} else if parsed := s.parseLine(u, "", false); parsed != nil {
			removed = append(removed, *parsed)
		}
	}

	if _, ok := s.branches[source]; ok {
		s.removePriorityMacro()
	}

	return removed, nil
}

// SetBranch устанавливает ветку (удаляет все и добавляет)
func (s *RepoService) SetBranch(ctx context.Context, branch, date string) (added []Repository, removed []Repository, err error) {
	s.ensureInitialized()
	if _, ok := s.branches[branch]; !ok {
		return nil, nil, fmt.Errorf(app.T_("Unknown branch: %s"), branch)
	}

	removed, err = s.RemoveRepository(ctx, []string{"all"}, "", false)
	if err != nil {
		return nil, removed, err
	}

	added, err = s.AddRepository(ctx, []string{branch}, date)
	if err != nil {
		return added, removed, err
	}

	return added, removed, nil
}

// CleanTemporary удаляет cdrom и task репозитории
func (s *RepoService) CleanTemporary(ctx context.Context) ([]Repository, error) {
	s.ensureInitialized()
	var removed []Repository

	repos, err := s.GetRepositories(ctx, false)
	if err != nil {
		return nil, err
	}

	for _, repo := range repos {
		isCdrom := strings.Contains(repo.URL, "cdrom:")
		isTask := false
		for _, comp := range repo.Components {
			if comp == "task" {
				isTask = true
				break
			}
		}

		if isCdrom || isTask {
			err = s.removeOrCommentRepo(repo.Entry)
			if err != nil {
				continue
			}
			repo.Active = false
			removed = append(removed, repo)
		}
	}

	return removed, nil
}

// SimulateAdd симулирует добавление репозитория
func (s *RepoService) SimulateAdd(ctx context.Context, args []string, date string, force bool) ([]Repository, error) {
	s.ensureInitialized()
	urls, err := s.parseSourceArgs(ctx, args, date)
	if err != nil {
		return nil, err
	}

	var willAdd []Repository
	for _, u := range urls {
		exists, _, _ := s.checkRepoExists(ctx, u)
		if !exists || force {
			if repo := s.parseLine(u, s.confMain, true); repo != nil {
				willAdd = append(willAdd, *repo)
			}
		}
	}

	return willAdd, nil
}

// SimulateRemove симулирует удаление репозитория
func (s *RepoService) SimulateRemove(ctx context.Context, args []string, date string, purge bool) ([]Repository, error) {
	s.ensureInitialized()
	var willRemove []Repository

	if len(args) == 0 {
		return nil, errors.New(app.T_("Repository source must be specified"))
	}

	source := args[0]

	if source == "all" {
		repos, err := s.GetRepositories(ctx, false)
		if err != nil {
			return nil, err
		}
		return repos, nil
	}

	urls, err := s.parseSourceArgs(ctx, args, date)
	if err != nil {
		return nil, err
	}

	allRepos, _ := s.GetRepositories(ctx, true)
	repoByEntry := make(map[string]Repository, len(allRepos))
	for _, r := range allRepos {
		repoByEntry[canonicalizeRepoLine(r.Entry)] = r
	}

	for _, u := range urls {
		exists, _, _ := s.checkRepoExists(ctx, u)
		if exists {
			canonical := canonicalizeRepoLine(u)
			if repo, ok := repoByEntry[canonical]; ok {
				willRemove = append(willRemove, repo)
			} else if parsed := s.parseLine(u, "", true); parsed != nil {
				willRemove = append(willRemove, *parsed)
			}
		}
	}

	return willRemove, nil
}

// setPriorityMacro устанавливает макрос %_priority_distbranch
func (s *RepoService) setPriorityMacro(source, date string) {
	if date != "" {
		return
	}

	priorityBranches := []string{"p10", "p11", "sisyphus"}

	for _, pb := range priorityBranches {
		if source == pb {
			if err := os.MkdirAll(RPMMacrosDir, 0755); err != nil {
				app.Log.Debugf("failed to create macros dir: %v", err)
				return
			}

			content := fmt.Sprintf("%%_priority_distbranch %s\n", source)
			if err := os.WriteFile(PriorityDistbranchMacro, []byte(content), 0644); err != nil {
				app.Log.Debugf("failed to write priority macro: %v", err)
			}
			return
		}
	}
}

// removePriorityMacro удаляет макрос приоритета
func (s *RepoService) removePriorityMacro() {
	if err := os.Remove(PriorityDistbranchMacro); err != nil && !os.IsNotExist(err) {
		app.Log.Debugf("failed to remove priority macro: %v", err)
	}
	if err := os.Remove(LegacyP10Macro); err != nil && !os.IsNotExist(err) {
		app.Log.Debugf("failed to remove legacy macro: %v", err)
	}
}
