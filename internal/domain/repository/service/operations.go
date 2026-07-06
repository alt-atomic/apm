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
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"altlinux.space/alt-atomic/apm/internal/common/app"
)

// AddRepository добавляет репозиторий
func (s *RepoService) AddRepository(ctx context.Context, args []string, date string) ([]Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addRepository(ctx, args, date)
}

func (s *RepoService) addRepository(ctx context.Context, args []string, date string) ([]Repository, error) {
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
		found, active, err := s.checkRepoExists(ctx, u)
		if err != nil {
			return added, err
		}

		if active {
			continue
		}

		var file string
		if found {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeRepository(ctx, args, date, purge)
}

func (s *RepoService) removeRepository(ctx context.Context, args []string, date string, purge bool) ([]Repository, error) {
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

	urls, err := s.removalURLs(ctx, args, date)
	if err != nil {
		return nil, err
	}

	// Получаем текущие репозитории для поиска файлов
	index := s.repoIndex(ctx)

	for _, u := range urls {
		found, _, checkErr := s.checkRepoExists(ctx, u)
		if checkErr != nil {
			continue
		}
		if !found {
			continue
		}

		err = s.removeOrCommentRepo(u)
		if err != nil {
			continue
		}

		if repo := s.findRepo(index, u, false); repo != nil {
			repo.Active = false
			removed = append(removed, *repo)
		}
	}

	if _, ok := s.lookupBranch(source); ok {
		s.removePriorityMacro()
	}

	return removed, nil
}

// SetBranch устанавливает ветку (удаляет все и добавляет)
func (s *RepoService) SetBranch(ctx context.Context, branch, date string) (added []Repository, removed []Repository, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureInitialized()
	if _, ok := s.lookupBranch(branch); !ok {
		return nil, nil, fmt.Errorf(app.T_("Unknown branch: %s"), branch)
	}

	removed, err = s.removeRepository(ctx, []string{"all"}, "", false)
	if err != nil {
		return nil, removed, err
	}

	added, err = s.addRepository(ctx, []string{branch}, date)
	if err != nil {
		return added, removed, err
	}

	return added, removed, nil
}

// CleanTemporary удаляет cdrom и task репозитории
func (s *RepoService) CleanTemporary(ctx context.Context) ([]Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureInitialized()
	var removed []Repository

	repos, err := s.GetRepositories(ctx, false)
	if err != nil {
		return nil, err
	}

	for _, repo := range repos {
		isCdrom := strings.Contains(repo.URL, "cdrom:")
		isTask := slices.Contains(repo.Components, "task")

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
		_, active, _ := s.checkRepoExists(ctx, u)
		if !active || force {
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

	urls, err := s.removalURLs(ctx, args, date)
	if err != nil {
		return nil, err
	}

	index := s.repoIndex(ctx)

	for _, u := range urls {
		_, active, _ := s.checkRepoExists(ctx, u)
		if active {
			if repo := s.findRepo(index, u, true); repo != nil {
				willRemove = append(willRemove, *repo)
			}
		}
	}

	return willRemove, nil
}

// removalURLs формирует кандидатов на удаление.
func (s *RepoService) removalURLs(ctx context.Context, args []string, date string) ([]string, error) {
	var urls []string
	if taskNum, ok := taskNumberArg(args); ok {
		urls = s.buildTaskURLsForRemove(ctx, taskNum)
	} else {
		var err error
		urls, err = s.parseSourceArgs(ctx, args, date)
		if err != nil {
			return nil, err
		}
	}
	return withAlternateSchemes(urls), nil
}

// withAlternateSchemes добавляет к каждому URL вариант с противоположной схемой http/https
func withAlternateSchemes(urls []string) []string {
	all := make([]string, 0, len(urls)*2)
	for _, u := range urls {
		all = append(all, u)
		if flipped := strings.Replace(u, " http://", " https://", 1); flipped != u {
			all = append(all, flipped)
		} else if flipped := strings.Replace(u, " https://", " http://", 1); flipped != u {
			all = append(all, flipped)
		}
	}
	return all
}

// repoIndex строит индекс текущих репозиториев по канонической строке
func (s *RepoService) repoIndex(ctx context.Context) map[string]Repository {
	allRepos, _ := s.GetRepositories(ctx, true)
	index := make(map[string]Repository, len(allRepos))
	for _, r := range allRepos {
		index[canonicalizeRepoLine(r.Entry)] = r
	}
	return index
}

// findRepo ищет репозиторий в индексе по канонической строке, иначе парсит её
func (s *RepoService) findRepo(index map[string]Repository, line string, active bool) *Repository {
	if repo, ok := index[canonicalizeRepoLine(line)]; ok {
		return &repo
	}
	return s.parseLine(line, "", active)
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
