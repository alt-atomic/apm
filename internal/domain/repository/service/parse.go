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
	"path/filepath"
	"strings"
)

// parseSourceArgs парсит аргументы в URL
func (s *RepoService) parseSourceArgs(ctx context.Context, args []string, date string) ([]string, error) {
	if len(args) == 0 {
		return nil, errors.New(app.T_("Repository source must be specified"))
	}

	date = strings.TrimSpace(date)
	source := strings.TrimSpace(args[0])

	if len(args) == 1 {
		return s.parseSource(ctx, source, date)
	}

	if strings.HasPrefix(source, "rpm") {
		return s.buildRepoFromArgs(args), nil
	}

	if isURL(source) {
		return s.buildURLReposFromArgs(args), nil
	}

	combined := strings.Join(args, " ")
	return s.parseSource(ctx, combined, date)
}

// parseSource парсит источник в URL
func (s *RepoService) parseSource(ctx context.Context, source, date string) ([]string, error) {
	source = strings.TrimSpace(source)
	date = strings.TrimSpace(date)

	if branch, ok := s.branches[source]; ok {
		if date != "" {
			formattedDate, err := s.parseArchiveDate(source, date)
			if err != nil {
				return nil, err
			}
			return s.buildBranchURLsWithArchive(ctx, branch, formattedDate), nil
		}
		return s.buildBranchURLs(ctx, branch), nil
	}

	if isTaskNumber(source) {
		return s.buildTaskURLs(ctx, source)
	}

	if strings.HasPrefix(source, "task ") {
		taskNum := strings.TrimPrefix(source, "task ")
		taskNum = strings.TrimSpace(taskNum)
		if isTaskNumber(taskNum) {
			return s.buildTaskURLs(ctx, taskNum)
		}
	}

	if isURL(source) {
		return s.buildURLRepos(source), nil
	}

	if strings.HasPrefix(source, "/") {
		return []string{fmt.Sprintf("rpm file://%s %s hasher", source, s.arch)}, nil
	}

	if strings.HasPrefix(source, "rpm") {
		// Заменяем _arch_ на текущую архитектуру
		source = strings.ReplaceAll(source, "_arch_", s.arch)
		return []string{source}, nil
	}

	return nil, fmt.Errorf(app.T_("Unknown repository format: %s"), source)
}

// buildRepoFromArgs собирает строку репозитория из аргументов [type, url, arch, components...]
func (s *RepoService) buildRepoFromArgs(args []string) []string {
	var processed []string
	for _, arg := range args {
		if arg == "_arch_" {
			processed = append(processed, s.arch)
		} else {
			processed = append(processed, arg)
		}
	}
	return []string{strings.Join(processed, " ")}
}

// buildURLReposFromArgs формирует репозитории из аргументов [url, arch, components...]
func (s *RepoService) buildURLReposFromArgs(args []string) []string {
	if len(args) < 2 {
		return s.buildURLRepos(args[0])
	}

	url := args[0]
	archArg := args[1]
	components := args[2:]

	if archArg == "_arch_" {
		archArg = s.arch
	}

	if len(components) == 0 {
		components = []string{"classic"}
	}

	return []string{fmt.Sprintf("rpm %s %s %s", url, archArg, strings.Join(components, " "))}
}

// buildURLRepos формирует репозитории из URL
func (s *RepoService) buildURLRepos(url string) []string {
	var urls []string

	urls = append(urls, fmt.Sprintf("rpm %s %s classic", url, s.arch))
	urls = append(urls, fmt.Sprintf("rpm %s noarch classic", url))

	if s.useArepo && s.arch == "x86_64" && strings.HasPrefix(url, "file://") {
		path := strings.TrimPrefix(url, "file://")
		arepoPath := filepath.Join(path, "x86_64-i586")
		if _, err := os.Stat(arepoPath); err == nil {
			urls = append(urls, fmt.Sprintf("rpm %s x86_64-i586 classic", url))
		}
	}

	return urls
}

// isURL проверяет, является ли строка URL
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "ftp://") || strings.HasPrefix(s, "rsync://") ||
		strings.HasPrefix(s, "file://") || strings.HasPrefix(s, "cdrom:")
}
