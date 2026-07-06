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
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// archiveDateRe формат даты архива YYYY/MM/DD
var archiveDateRe = regexp.MustCompile(`^\d{4}/\d{2}/\d{2}$`)

// archivingBranches список веток, для которых есть архивы
var archivingBranches = []string{"p7", "p8", "p9", "p10", "p11", "t7", "sisyphus"}

// initBranches инициализирует известные ветки ALT Linux
func (s *RepoService) initBranches() {
	s.branches = map[string]branchInfo{
		"sisyphus": {
			Name:       "sisyphus",
			URL:        repoBaseURL + "/Sisyphus",
			Key:        "alt",
			Components: []string{"classic", "gostcrypto"},
		},
		"p11": {
			Name:       "p11",
			URL:        repoBaseURL + "/p11/branch",
			Key:        "p11",
			Components: []string{"classic", "gostcrypto"},
		},
		"p10": {
			Name:       "p10",
			URL:        repoBaseURL + "/p10/branch",
			Key:        "p10",
			Components: []string{"classic", "gostcrypto"},
		},
		"p9": {
			Name:       "p9",
			URL:        repoBaseURL + "/p9/branch",
			Key:        "p9",
			Components: []string{"classic", "gostcrypto"},
		},
		"p8": {
			Name:       "p8",
			URL:        repoBaseURL + "/p8/branch",
			Key:        "updates",
			Components: []string{"classic"},
		},
		"c8": {
			Name:       "c8",
			URL:        repoCert8URL,
			Key:        "cert8",
			Components: []string{"classic"},
		},
		"c8.1": {
			Name:       "c8.1",
			URL:        repoBaseURL + "/c8.1/branch",
			Key:        "updates",
			Components: []string{"classic"},
		},
		"c9f2": {
			Name:       "c9f2",
			URL:        repoBaseURL + "/c9f2/branch",
			Key:        "c9f2",
			Components: []string{"classic"},
		},
		"c10f1": {
			Name:       "c10f1",
			URL:        repoBaseURL + "/c10f1/branch",
			Key:        "c10f1",
			Components: []string{"classic"},
		},
		"c10f2": {
			Name:       "c10f2",
			URL:        repoBaseURL + "/c10f2/branch",
			Key:        "c10f2",
			Components: []string{"classic"},
		},
		"autoimports.sisyphus": {
			Name:       "autoimports.sisyphus",
			URL:        repoBaseURLRu + "/autoimports/Sisyphus",
			Key:        "",
			Components: []string{"autoimports"},
		},
		"autoimports.p10": {
			Name:       "autoimports.p10",
			URL:        repoBaseURLRu + "/autoimports/p10",
			Key:        "",
			Components: []string{"autoimports"},
		},
		"autoimports.p11": {
			Name:       "autoimports.p11",
			URL:        repoBaseURLRu + "/autoimports/p11",
			Key:        "",
			Components: []string{"autoimports"},
		},
	}
}

// GetBranches возвращает список доступных веток
func (s *RepoService) GetBranches() []string {
	s.ensureInitialized()
	branches := make([]string, 0, len(s.branches))
	for name := range s.branches {
		if strings.HasPrefix(name, "autoimports.") {
			continue
		}
		branches = append(branches, name)
	}
	branches = append(branches, "task")
	slices.Sort(branches)
	return branches
}

// lookupBranch ищет ветку по имени без учёта регистра
func (s *RepoService) lookupBranch(name string) (branchInfo, bool) {
	if branch, ok := s.branches[name]; ok {
		return branch, true
	}
	branch, ok := s.branches[strings.ToLower(name)]
	return branch, ok
}

// parseArchiveDate парсит и валидирует дату архива
func (s *RepoService) parseArchiveDate(branchName, date string) (string, error) {
	if !slices.Contains(archivingBranches, branchName) {
		return "", fmt.Errorf(T_("Branch %s has no archive"), branchName)
	}

	// Формат YYYYMMDD -> YYYY/MM/DD
	if len(date) == 8 && isDigits(date) {
		return fmt.Sprintf("%s/%s/%s", date[0:4], date[4:6], date[6:8]), nil
	}

	// Формат YYYY/MM/DD
	if archiveDateRe.MatchString(date) {
		return date, nil
	}

	return "", errors.New(T_("Archive date should be YYYYMMDD or YYYY/MM/DD format"))
}

// buildBranchURLs формирует URL для ветки
func (s *RepoService) buildBranchURLs(ctx context.Context, branch branchInfo) []string {
	return s.buildBranchURLsWithArchive(ctx, branch, "")
}

// buildBranchURLsWithArchive формирует URL для ветки с опциональной датой архива
func (s *RepoService) buildBranchURLsWithArchive(ctx context.Context, branch branchInfo, archiveDate string) []string {
	var urls []string

	keyPart := ""
	if branch.Key != "" {
		keyPart = fmt.Sprintf("[%s] ", branch.Key)
	}

	mainComponents := branch.Components[0]
	allComponents := mainComponents
	if len(branch.Components) > 1 && s.hasGostcryptoInSources(ctx) {
		allComponents = strings.Join(branch.Components, " ")
	}

	// Формируем базовый URL с учётом схемы и архива
	var baseURL string
	if archiveDate != "" {
		baseURL = fmt.Sprintf("%s%s/%s/date/%s", s.httpScheme(ctx), repoArchiveURL, branch.Name, archiveDate)
	} else {
		baseURL = s.httpScheme(ctx) + branch.URL
	}

	urls = append(urls, fmt.Sprintf("rpm %s%s %s %s", keyPart, baseURL, s.arch, allComponents))

	if !strings.Contains(branch.URL, "altlinuxclub") {
		urls = append(urls, fmt.Sprintf("rpm %s%s noarch %s", keyPart, baseURL, mainComponents))
	}

	if s.useArepo && s.arch == "x86_64" && !strings.Contains(branch.URL, "altlinuxclub") && !strings.Contains(branch.URL, "autoimports") {
		urls = append(urls, fmt.Sprintf("rpm %s%s x86_64-i586 %s", keyPart, baseURL, mainComponents))
	}

	return urls
}

// hasGostcryptoInSources проверяет есть ли gostcrypto в существующих репозиториях
func (s *RepoService) hasGostcryptoInSources(ctx context.Context) bool {
	repos, err := s.GetRepositories(ctx, true)
	if err != nil {
		return false
	}

	for _, repo := range repos {
		if slices.Contains(repo.Components, "gostcrypto") {
			return true
		}
	}

	return false
}
