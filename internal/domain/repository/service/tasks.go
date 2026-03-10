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
	"fmt"
	"io"
	"net/http"
	"strings"
)

// buildTaskURLs формирует URL для задачи
func (s *RepoService) buildTaskURLs(ctx context.Context, taskNum string) ([]string, error) {
	exists, baseURL, err := s.checkTaskExists(ctx, taskNum)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf(app.T_("Task %s not found or still building"), taskNum)
	}

	var repoURL string
	if strings.Contains(baseURL, "archive/done") {
		repoURL = baseURL + "/build/repo/"
	} else {
		repoURL = fmt.Sprintf("%s%s/%s/", s.httpScheme(ctx), RepoTaskURL, taskNum)
	}

	urls := []string{fmt.Sprintf("rpm %s %s task", repoURL, s.arch)}

	if s.useArepo && s.arch == "x86_64" {
		hasArepo, err := s.checkTaskHasArepo(ctx, taskNum)
		if err != nil {
			app.Log.Debugf("failed to check arepo for task %s: %v", taskNum, err)
		}
		if hasArepo {
			urls = append(urls, fmt.Sprintf("rpm %s x86_64-i586 task", repoURL))
		}
	}

	return urls, nil
}

// checkTaskExists проверяет существование задачи и возвращает базовый URL (с учётом редиректа для архивных задач)
func (s *RepoService) checkTaskExists(ctx context.Context, taskNum string) (exists bool, baseURL string, err error) {
	url := fmt.Sprintf("%s%s/%s/plan/add-bin", s.httpScheme(ctx), RepoTasksURL, taskNum)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 {
		return false, "", nil
	}

	if resp.StatusCode == 200 {
		finalURL := resp.Request.URL.String()
		baseURL = strings.TrimSuffix(finalURL, "/plan/add-bin")
		return true, baseURL, nil
	}

	return false, "", nil
}

// checkTaskHasArepo проверяет есть ли arepo у задачи
func (s *RepoService) checkTaskHasArepo(ctx context.Context, taskNum string) (bool, error) {
	url := fmt.Sprintf("%s%s/%s/plan/arepo-add-x86_64-i586", s.httpScheme(ctx), RepoTasksURL, taskNum)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return false, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	return len(strings.TrimSpace(string(body))) > 0, nil
}

// isTaskNumber проверяет, является ли строка номером задачи
func isTaskNumber(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// GetTaskPackages возвращает список пакетов из задачи
func (s *RepoService) GetTaskPackages(ctx context.Context, taskNum string) ([]string, error) {
	exists, baseURL, err := s.checkTaskExists(ctx, taskNum)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf(app.T_("Task %s not found or still building"), taskNum)
	}

	url := baseURL + "/plan/add-bin"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf(app.T_("Failed to get task packages: HTTP %d"), resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var packages []string
	seen := make(map[string]bool)
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			pkg := fields[0]

			if strings.HasSuffix(pkg, "-debuginfo") ||
				strings.HasSuffix(pkg, "-checkinstall") ||
				strings.HasSuffix(pkg, "-devel") ||
				strings.HasPrefix(pkg, "kernel-headers-") {
				continue
			}

			if !seen[pkg] {
				seen[pkg] = true
				packages = append(packages, pkg)
			}
		}
	}

	return packages, nil
}
