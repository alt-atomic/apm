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
	"regexp"
	"sort"
	"strings"
)

// archivingBranches список веток, для которых есть архивы
var archivingBranches = []string{"p7", "p8", "p9", "p10", "p11", "t7", "sisyphus"}

// initBranches инициализирует известные ветки ALT Linux
func (s *RepoService) initBranches() {
	s.branches = map[string]Branch{
		"sisyphus": {
			Name:       "sisyphus",
			URL:        RepoBaseURL + "/Sisyphus",
			Key:        "alt",
			Components: []string{"classic", "gostcrypto"},
		},
		"Sisyphus": {
			Name:       "Sisyphus",
			URL:        RepoBaseURL + "/Sisyphus",
			Key:        "alt",
			Components: []string{"classic", "gostcrypto"},
		},
		"p11": {
			Name:       "p11",
			URL:        RepoBaseURL + "/p11/branch",
			Key:        "p11",
			Components: []string{"classic", "gostcrypto"},
		},
		"p10": {
			Name:       "p10",
			URL:        RepoBaseURL + "/p10/branch",
			Key:        "p10",
			Components: []string{"classic", "gostcrypto"},
		},
		"p9": {
			Name:       "p9",
			URL:        RepoBaseURL + "/p9/branch",
			Key:        "p9",
			Components: []string{"classic", "gostcrypto"},
		},
		"p8": {
			Name:       "p8",
			URL:        RepoBaseURL + "/p8/branch",
			Key:        "updates",
			Components: []string{"classic"},
		},
		"c8": {
			Name:       "c8",
			URL:        RepoCert8URL,
			Key:        "cert8",
			Components: []string{"classic"},
		},
		"c8.1": {
			Name:       "c8.1",
			URL:        RepoBaseURL + "/c8.1/branch",
			Key:        "updates",
			Components: []string{"classic"},
		},
		"c9f2": {
			Name:       "c9f2",
			URL:        RepoBaseURL + "/c9f2/branch",
			Key:        "c9f2",
			Components: []string{"classic"},
		},
		"c10f1": {
			Name:       "c10f1",
			URL:        RepoBaseURL + "/c10f1/branch",
			Key:        "c10f1",
			Components: []string{"classic"},
		},
		"c10f2": {
			Name:       "c10f2",
			URL:        RepoBaseURL + "/c10f2/branch",
			Key:        "c10f2",
			Components: []string{"classic"},
		},
		"autoimports.sisyphus": {
			Name:       "autoimports.sisyphus",
			URL:        RepoBaseURLRu + "/autoimports/Sisyphus",
			Key:        "",
			Components: []string{"autoimports"},
		},
		"autoimports.p10": {
			Name:       "autoimports.p10",
			URL:        RepoBaseURLRu + "/autoimports/p10",
			Key:        "",
			Components: []string{"autoimports"},
		},
		"autoimports.p11": {
			Name:       "autoimports.p11",
			URL:        RepoBaseURLRu + "/autoimports/p11",
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
		if name == "Sisyphus" || strings.HasPrefix(name, "autoimports.") {
			continue
		}
		branches = append(branches, name)
	}
	branches = append(branches, "task")
	sort.Strings(branches)
	return branches
}

// parseArchiveDate парсит и валидирует дату архива
func (s *RepoService) parseArchiveDate(branchName, date string) (string, error) {
	// Проверяем, поддерживает ли ветка архивы
	hasArchive := false
	for _, b := range archivingBranches {
		if b == branchName {
			hasArchive = true
			break
		}
	}
	if !hasArchive {
		return "", fmt.Errorf(app.T_("Branch %s has no archive"), branchName)
	}

	// Формат YYYYMMDD -> YYYY/MM/DD
	if len(date) == 8 && isTaskNumber(date) {
		return fmt.Sprintf("%s/%s/%s", date[0:4], date[4:6], date[6:8]), nil
	}

	// Формат YYYY/MM/DD
	if regexp.MustCompile(`^\d{4}/\d{2}/\d{2}$`).MatchString(date) {
		return date, nil
	}

	return "", errors.New(app.T_("Archive date should be YYYYMMDD or YYYY/MM/DD format"))
}

// buildBranchURLs формирует URL для ветки
func (s *RepoService) buildBranchURLs(ctx context.Context, branch Branch) []string {
	return s.buildBranchURLsWithArchive(ctx, branch, "")
}

// buildBranchURLsWithArchive формирует URL для ветки с опциональной датой архива
func (s *RepoService) buildBranchURLsWithArchive(ctx context.Context, branch Branch, archiveDate string) []string {
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
		baseURL = fmt.Sprintf("%s%s/%s/date/%s", s.httpScheme(ctx), RepoArchiveURL, branch.Name, archiveDate)
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
		for _, comp := range repo.Components {
			if comp == "gostcrypto" {
				return true
			}
		}
	}

	return false
}
