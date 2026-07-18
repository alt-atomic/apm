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

package aptrepo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// buildTaskURLs builds source lines for a task
func (s *RepoService) buildTaskURLs(ctx context.Context, taskNum string) ([]string, error) {
	exists, baseURL, err := s.checkTaskExists(ctx, taskNum)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf(T_("Task %s not found or still building"), taskNum)
	}

	var repoURL string
	if strings.Contains(baseURL, "archive/done") {
		repoURL = baseURL + "/build/repo/"
	} else {
		repoURL = fmt.Sprintf("%s%s/%s/", s.httpScheme(ctx), repoTaskURL, taskNum)
	}

	urls := []string{fmt.Sprintf("rpm %s %s task", repoURL, s.arch)}

	if s.useArepo && s.arch == "x86_64" {
		hasArepo, err := s.checkTaskHasArepo(ctx, taskNum)
		if err != nil {
			logger.Debugf("failed to check arepo for task %s: %v", taskNum, err)
		}
		if hasArepo {
			urls = append(urls, fmt.Sprintf("rpm %s x86_64-i586 task", repoURL))
		}
	}

	return urls, nil
}

// buildTaskURLsForRemove builds both active and archived task source lines without network checks.
func (s *RepoService) buildTaskURLsForRemove(ctx context.Context, taskNum string) []string {
	scheme := s.httpScheme(ctx)
	num, _ := strconv.Atoi(taskNum)
	archiveURL := fmt.Sprintf("%s%s/archive/done/_%d/%s/build/repo/", scheme, repoTasksURL, num/1024, taskNum)
	activeURL := fmt.Sprintf("%s%s/%s/", scheme, repoTaskURL, taskNum)

	urls := []string{
		fmt.Sprintf("rpm %s %s task", archiveURL, s.arch),
		fmt.Sprintf("rpm %s %s task", activeURL, s.arch),
	}
	if s.useArepo && s.arch == "x86_64" {
		urls = append(urls,
			fmt.Sprintf("rpm %s x86_64-i586 task", archiveURL),
			fmt.Sprintf("rpm %s x86_64-i586 task", activeURL))
	}
	return urls
}

// taskNumberArg extracts the task number from ["12345"] or ["task", "12345"] arguments
func taskNumberArg(args []string) (string, bool) {
	source := strings.TrimSpace(strings.Join(args, " "))
	if num, ok := strings.CutPrefix(source, "task "); ok {
		source = strings.TrimSpace(num)
	}
	if source != "" && isDigits(source) {
		return source, true
	}
	return "", false
}

// checkTaskExists reports whether the task exists and returns its base URL (following the archive redirect)
func (s *RepoService) checkTaskExists(ctx context.Context, taskNum string) (exists bool, baseURL string, err error) {
	url := fmt.Sprintf("%s%s/%s/plan/add-bin", s.httpScheme(ctx), repoTasksURL, taskNum)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return false, "", nil
	}

	if resp.StatusCode == http.StatusOK {
		finalURL := resp.Request.URL.String()
		baseURL = strings.TrimSuffix(finalURL, "/plan/add-bin")
		return true, baseURL, nil
	}

	return false, "", nil
}

// checkTaskHasArepo reports whether the task has an arepo plan
func (s *RepoService) checkTaskHasArepo(ctx context.Context, taskNum string) (bool, error) {
	url := fmt.Sprintf("%s%s/%s/plan/arepo-add-x86_64-i586", s.httpScheme(ctx), repoTasksURL, taskNum)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	return len(strings.TrimSpace(string(body))) > 0, nil
}

// isDigits reports whether the string consists of digits only
func isDigits(s string) bool {
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

// GetTaskPackages returns the package list of a task
func (s *RepoService) GetTaskPackages(ctx context.Context, taskNum string) ([]string, error) {
	url := fmt.Sprintf("%s%s/%s/plan/add-bin", s.httpScheme(ctx), repoTasksURL, taskNum)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(T_("Task %s not found or still building"), taskNum)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var packages []string
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
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

	return packages, nil
}
