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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	// DefaultSourcesList is the main APT sources configuration file.
	DefaultSourcesList = "/etc/apt/sources.list"
	// DefaultSourcesListDir is the directory for additional APT source files.
	DefaultSourcesListDir = "/etc/apt/sources.list.d/"
	// ArepoConfigFile is the arepo configuration file path.
	ArepoConfigFile = "/etc/sysconfig/apt-repo"

	// RPMMacrosDir is the directory for RPM macros.
	RPMMacrosDir = "/etc/rpm/macros.d"
	// PriorityDistbranchMacro is the priority distbranch macro file.
	PriorityDistbranchMacro = "/etc/rpm/macros.d/priority_distbranch"
	// LegacyP10Macro is the legacy p10 macro file.
	LegacyP10Macro = "/etc/rpm/macros.d/p10"

	// RepoBaseURL is the base URL for ALT Linux repositories (ftp.altlinux.org).
	RepoBaseURL = "http://ftp.altlinux.org/pub/distributions/ALTLinux"
	// RepoBaseURLRu is the base URL for ALT Linux repositories (ftp.altlinux.ru).
	RepoBaseURLRu = "http://ftp.altlinux.ru/pub/distributions/ALTLinux"
	// RepoTaskURL is the URL for active task repositories.
	RepoTaskURL = "http://git.altlinux.org/repo"
	// RepoTasksURL is the URL for task information.
	RepoTasksURL = "http://git.altlinux.org/tasks"
	// RepoTasksArchive is the URL for archived tasks.
	RepoTasksArchive = "http://git.altlinux.org/tasks/archive/done"
	// RepoCert8URL is the URL for c8 certification branch.
	RepoCert8URL = "http://update.altsp.su/pub/distributions/ALTLinux/c8/branch"

	// HTTPTimeout is the default timeout for HTTP requests.
	HTTPTimeout = 10 * time.Second
)

// Repository представляет информацию о репозитории
type Repository struct {
	Type       string   `json:"type"`
	URL        string   `json:"url"`
	Arch       string   `json:"arch"`
	Key        string   `json:"key"`
	Components []string `json:"components"`
	Active     bool     `json:"active"`
	File       string   `json:"file"`
	Raw        string   `json:"raw"`
}

// Branch представляет информацию о ветке ALT Linux
type Branch struct {
	Name       string
	URL        string
	Key        string
	Components []string
}

// RepoService сервис для работы с репозиториями APT
type RepoService struct {
	confMain   string
	confDir    string
	arch       string
	branches   map[string]Branch
	useArepo   bool
	httpClient *http.Client
}

// NewRepoService создает новый сервис для работы с репозиториями
func NewRepoService() *RepoService {
	svc := &RepoService{
		confMain: DefaultSourcesList,
		confDir:  DefaultSourcesListDir,
		arch:     detectArch(),
		useArepo: checkArepoEnabled(),
		httpClient: &http.Client{
			Timeout: HTTPTimeout,
		},
	}

	// Получаем пути из apt-config если возможно
	svc.detectAPTConfig()

	// Инициализируем известные ветки
	svc.initBranches()

	return svc
}

// detectArch определяет архитектуру системы
func detectArch() string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		return "x86_64"
	}
	if arch == "386" {
		return "i586"
	}
	if arch == "arm" {
		return "armh"
	}
	if arch == "arm64" {
		return "aarch64"
	}

	// Fallback через uname
	cmd := exec.Command("uname", "-m")
	output, err := cmd.Output()
	if err == nil {
		archStr := strings.TrimSpace(string(output))
		if archStr == "i686" {
			return "i586"
		}
		if archStr == "armv7l" {
			return "armh"
		}
		return archStr
	}

	return "x86_64"
}

// checkArepoEnabled проверяет включен ли arepo
func checkArepoEnabled() bool {
	file, err := os.Open(ArepoConfigFile)
	if err != nil {
		return true
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "AREPO") && strings.Contains(line, "NO") {
			return false
		}
	}

	return true
}

// detectAPTConfig получает пути конфигурации из apt-config
func (s *RepoService) detectAPTConfig() {
	// Получаем sources.list
	cmd := exec.Command("apt-config", "shell", "FILE", "Dir::Etc::sourcelist/f")
	output, err := cmd.Output()
	if err == nil {
		if matches := regexp.MustCompile(`^FILE=(.*)$`).FindStringSubmatch(strings.TrimSpace(string(output))); len(matches) > 1 {
			path := strings.Trim(matches[1], `"'`)
			if path != "" {
				s.confMain = path
			}
		}
	}

	// Получаем sources.list.d
	cmd = exec.Command("apt-config", "shell", "DIR", "Dir::Etc::sourceparts/d")
	output, err = cmd.Output()
	if err == nil {
		if matches := regexp.MustCompile(`^DIR=(.*)$`).FindStringSubmatch(strings.TrimSpace(string(output))); len(matches) > 1 {
			path := strings.Trim(matches[1], `"'`)
			if path != "" {
				s.confDir = path
			}
		}
	}
}

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
	branches := make([]string, 0, len(s.branches))
	for name := range s.branches {
		if name == "Sisyphus" || strings.HasPrefix(name, "autoimports.") {
			continue
		}
		branches = append(branches, name)
	}
	sort.Strings(branches)
	return branches
}

// GetRepositories возвращает список репозиториев
func (s *RepoService) GetRepositories(_ context.Context, all bool) ([]Repository, error) {
	var repos []Repository

	files, err := s.getSourceFiles()
	if err != nil {
		return nil, err
	}

	for _, filename := range files {
		file, err := os.Open(filename)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)

			if trimmed == "" {
				continue
			}

			// Активные репозитории
			if !strings.HasPrefix(trimmed, "#") {
				if repo := s.parseLine(trimmed, filename, true); repo != nil {
					repos = append(repos, *repo)
				}
			} else if all {
				// Закомментированные репозитории (только с -a)
				commented := strings.TrimPrefix(trimmed, "#")
				commented = strings.TrimSpace(commented)
				if strings.HasPrefix(commented, "rpm") {
					if repo := s.parseLine(commented, filename, false); repo != nil {
						repos = append(repos, *repo)
					}
				}
			}
		}
		_ = file.Close()
	}

	return repos, nil
}

// parseLine парсит строку репозитория
func (s *RepoService) parseLine(line string, filename string, active bool) *Repository {
	if !strings.HasPrefix(line, "rpm") {
		return nil
	}

	parts := strings.Fields(line)
	if len(parts) < 4 {
		return nil
	}

	repo := &Repository{
		Active: active,
		File:   filename,
		Raw:    line,
	}

	idx := 0

	repo.Type = parts[idx]
	idx++

	// Ключ (опционально, в квадратных скобках)
	if strings.HasPrefix(parts[idx], "[") {
		repo.Key = strings.Trim(parts[idx], "[]")
		idx++
	}

	// URL
	if idx < len(parts) {
		repo.URL = parts[idx]
		idx++
	}

	// Архитектура
	if idx < len(parts) {
		repo.Arch = parts[idx]
		idx++
	}

	// Компоненты
	if idx < len(parts) {
		repo.Components = parts[idx:]
	}

	return repo
}

// AddRepository добавляет репозиторий
func (s *RepoService) AddRepository(ctx context.Context, source string) ([]string, error) {
	urls, err := s.parseSource(ctx, source)
	if err != nil {
		return nil, err
	}

	if len(urls) == 0 {
		return nil, errors.New(app.T_("Failed to parse repository source"))
	}

	var added []string

	for _, u := range urls {
		exists, commented, err := s.checkRepoExists(ctx, u)
		if err != nil {
			return added, err
		}

		if exists {
			continue
		}

		if commented {
			err = s.uncommentRepo(u)
			if err != nil {
				return added, err
			}
			added = append(added, u)
		} else {
			err = s.appendRepo(u)
			if err != nil {
				return added, err
			}
			added = append(added, u)
		}
	}

	s.setPriorityMacro(source)

	return added, nil
}

// RemoveRepository удаляет репозиторий
func (s *RepoService) RemoveRepository(ctx context.Context, source string) ([]string, error) {
	var removed []string

	if source == "all" {
		repos, err := s.GetRepositories(ctx, false)
		if err != nil {
			return nil, err
		}

		for _, repo := range repos {
			err = s.removeOrCommentRepo(repo.Raw)
			if err != nil {
				return removed, err
			}
			removed = append(removed, repo.Raw)
		}

		s.removePriorityMacro()

		return removed, nil
	}

	urls, err := s.parseSource(ctx, source)
	if err != nil {
		return nil, err
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
		removed = append(removed, u)
	}

	if _, ok := s.branches[source]; ok {
		s.removePriorityMacro()
	}

	return removed, nil
}

// SetBranch устанавливает ветку (удаляет все и добавляет)
func (s *RepoService) SetBranch(ctx context.Context, branch string) (added []string, removed []string, err error) {
	if _, ok := s.branches[branch]; !ok {
		return nil, nil, fmt.Errorf(app.T_("Unknown branch: %s"), branch)
	}

	removed, err = s.RemoveRepository(ctx, "all")
	if err != nil {
		return nil, removed, err
	}

	// Добавляем ветку
	added, err = s.AddRepository(ctx, branch)
	if err != nil {
		return added, removed, err
	}

	return added, removed, nil
}

// CleanTemporary удаляет cdrom и task репозитории
func (s *RepoService) CleanTemporary(ctx context.Context) ([]string, error) {
	var removed []string

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
			err = s.removeOrCommentRepo(repo.Raw)
			if err != nil {
				continue
			}
			removed = append(removed, repo.Raw)
		}
	}

	return removed, nil
}

// parseSource парсит источник в URL(ы)
func (s *RepoService) parseSource(ctx context.Context, source string) ([]string, error) {
	source = strings.TrimSpace(source)

	// 1. Проверяем известную ветку
	if branch, ok := s.branches[source]; ok {
		return s.buildBranchURLs(branch), nil
	}

	// 2. Проверяем номер задачи
	if isTaskNumber(source) {
		return s.buildTaskURLs(ctx, source)
	}

	// 3. Проверяем "task <number>"
	if strings.HasPrefix(source, "task ") {
		taskNum := strings.TrimPrefix(source, "task ")
		taskNum = strings.TrimSpace(taskNum)
		if isTaskNumber(taskNum) {
			return s.buildTaskURLs(ctx, taskNum)
		}
	}

	// 4. Проверяем URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") ||
		strings.HasPrefix(source, "ftp://") || strings.HasPrefix(source, "rsync://") ||
		strings.HasPrefix(source, "file://") || strings.HasPrefix(source, "cdrom:") {
		return s.buildURLRepos(source), nil
	}

	// 5. Локальный путь
	if strings.HasPrefix(source, "/") {
		return []string{fmt.Sprintf("rpm file://%s %s hasher", source, s.arch)}, nil
	}

	// 6. Формат sources.list
	if strings.HasPrefix(source, "rpm") {
		return []string{source}, nil
	}

	return nil, fmt.Errorf(app.T_("Unknown repository format: %s"), source)
}

// buildBranchURLs формирует URL для ветки
func (s *RepoService) buildBranchURLs(branch Branch) []string {
	var urls []string

	keyPart := ""
	if branch.Key != "" {
		keyPart = fmt.Sprintf("[%s] ", branch.Key)
	}

	mainComponents := branch.Components[0]
	allComponents := strings.Join(branch.Components, " ")

	urls = append(urls, fmt.Sprintf("rpm %s%s %s %s", keyPart, branch.URL, s.arch, allComponents))

	if !strings.Contains(branch.URL, "altlinuxclub") {
		urls = append(urls, fmt.Sprintf("rpm %s%s noarch %s", keyPart, branch.URL, mainComponents))
	}

	if s.useArepo && s.arch == "x86_64" && !strings.Contains(branch.URL, "altlinuxclub") && !strings.Contains(branch.URL, "autoimports") {
		urls = append(urls, fmt.Sprintf("rpm %s%s x86_64-i586 %s", keyPart, branch.URL, mainComponents))
	}

	return urls
}

// buildTaskURLs формирует URL для задачи
func (s *RepoService) buildTaskURLs(ctx context.Context, taskNum string) ([]string, error) {
	// Проверяем существование задачи
	exists, baseURL, err := s.checkTaskExists(ctx, taskNum)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf(app.T_("Task %s not found or still building"), taskNum)
	}

	// Формируем URL репозитория
	var repoURL string
	if strings.Contains(baseURL, "archive/done") {
		// Архивная задача
		repoURL = baseURL + "/build/repo/"
	} else {
		// Активная задача - используем git.altlinux.org/repo
		repoURL = fmt.Sprintf("%s/%s/", RepoTaskURL, taskNum)
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

// buildURLRepos формирует репозитории из URL
func (s *RepoService) buildURLRepos(url string) []string {
	var urls []string

	urls = append(urls, fmt.Sprintf("rpm %s %s classic", url, s.arch))
	urls = append(urls, fmt.Sprintf("rpm %s noarch classic", url))

	// Arepo
	if s.useArepo && s.arch == "x86_64" && strings.HasPrefix(url, "file://") {
		path := strings.TrimPrefix(url, "file://")
		arepoPath := filepath.Join(path, "x86_64-i586")
		if _, err := os.Stat(arepoPath); err == nil {
			urls = append(urls, fmt.Sprintf("rpm %s x86_64-i586 classic", url))
		}
	}

	return urls
}

// checkTaskExists проверяет существование задачи и возвращает базовый URL (с учётом редиректа для архивных задач)
func (s *RepoService) checkTaskExists(ctx context.Context, taskNum string) (exists bool, baseURL string, err error) {
	url := fmt.Sprintf("%s/%s/plan/add-bin", RepoTasksURL, taskNum)

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
	url := fmt.Sprintf("%s/%s/plan/arepo-add-x86_64-i586", RepoTasksURL, taskNum)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == 200, nil
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

// getSourceFiles возвращает список файлов с источниками
func (s *RepoService) getSourceFiles() ([]string, error) {
	var files []string

	if _, err := os.Stat(s.confMain); err == nil {
		files = append(files, s.confMain)
	}

	pattern := filepath.Join(s.confDir, "*.list")
	matches, err := filepath.Glob(pattern)
	if err == nil {
		sort.Strings(matches)
		files = append(files, matches...)
	}

	return files, nil
}

// checkRepoExists проверяет, существует ли репозиторий
func (s *RepoService) checkRepoExists(ctx context.Context, repoLine string) (active bool, commented bool, err error) {
	repos, err := s.GetRepositories(ctx, true)
	if err != nil {
		return false, false, err
	}

	normalized := normalizeRepoLine(repoLine)

	for _, repo := range repos {
		repoNormalized := normalizeRepoLine(repo.Raw)
		if repoNormalized == normalized {
			return repo.Active, !repo.Active, nil
		}
	}

	return false, false, nil
}

// normalizeRepoLine нормализует строку репозитория для сравнения
func normalizeRepoLine(line string) string {
	fields := strings.Fields(line)
	return strings.Join(fields, " ")
}

// uncommentRepo раскомментирует репозиторий
func (s *RepoService) uncommentRepo(repoLine string) error {
	files, err := s.getSourceFiles()
	if err != nil {
		return err
	}

	normalized := normalizeRepoLine(repoLine)

	for _, filename := range files {
		content, err := os.ReadFile(filename)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		modified := false

		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				commented := strings.TrimPrefix(trimmed, "#")
				commented = strings.TrimSpace(commented)
				if normalizeRepoLine(commented) == normalized {
					lines[i] = commented
					modified = true
				}
			}
		}

		if modified {
			errWrite := os.WriteFile(filename, []byte(strings.Join(lines, "\n")), 0644)
			if errWrite != nil {
				return errWrite
			}
		}
	}

	return nil
}

// appendRepo добавляет репозиторий в sources.list
func (s *RepoService) appendRepo(repoLine string) error {
	file, err := os.OpenFile(s.confMain, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf(app.T_("Failed to open %s: %v"), s.confMain, err)
	}
	defer func() { _ = file.Close() }()

	_, err = file.WriteString(repoLine + "\n")
	if err != nil {
		return fmt.Errorf(app.T_("Failed to write to %s: %v"), s.confMain, err)
	}

	return nil
}

// removeOrCommentRepo удаляет или комментирует репозиторий
func (s *RepoService) removeOrCommentRepo(repoLine string) error {
	normalized := normalizeRepoLine(repoLine)

	if err := s.removeFromFile(s.confMain, normalized); err != nil {
		return err
	}

	files, err := s.getSourceFiles()
	if err != nil {
		return err
	}

	for _, filename := range files {
		if filename == s.confMain {
			continue
		}
		if err = s.commentInFile(filename, normalized); err != nil {
			continue
		}
	}

	return nil
}

// removeFromFile удаляет строку из файла
func (s *RepoService) removeFromFile(filename string, normalizedLine string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	for _, line := range lines {
		if normalizeRepoLine(line) != normalizedLine {
			newLines = append(newLines, line)
		}
	}

	return os.WriteFile(filename, []byte(strings.Join(newLines, "\n")), 0644)
}

// commentInFile комментирует строку в файле
func (s *RepoService) commentInFile(filename string, normalizedLine string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	modified := false

	for i, line := range lines {
		if normalizeRepoLine(line) == normalizedLine {
			lines[i] = "#" + line
			modified = true
		}
	}

	if modified {
		return os.WriteFile(filename, []byte(strings.Join(lines, "\n")), 0644)
	}

	return nil
}

// setPriorityMacro устанавливает макрос %_priority_distbranch
func (s *RepoService) setPriorityMacro(branch string) {
	priorityBranches := []string{"p10", "p11", "sisyphus"}

	for _, pb := range priorityBranches {
		if branch == pb {
			if err := os.MkdirAll(RPMMacrosDir, 0755); err != nil {
				app.Log.Debugf("failed to create macros dir: %v", err)
				return
			}

			content := fmt.Sprintf("%%_priority_distbranch %s\n", branch)
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

// SimulateAdd симулирует добавление репозитория
func (s *RepoService) SimulateAdd(ctx context.Context, source string) ([]string, error) {
	urls, err := s.parseSource(ctx, source)
	if err != nil {
		return nil, err
	}

	var willAdd []string
	for _, u := range urls {
		exists, commented, _ := s.checkRepoExists(ctx, u)
		if !exists {
			if commented {
				willAdd = append(willAdd, fmt.Sprintf(app.T_("Will uncomment: %s"), u))
			} else {
				willAdd = append(willAdd, fmt.Sprintf(app.T_("Will add: %s"), u))
			}
		}
	}

	return willAdd, nil
}

// SimulateRemove симулирует удаление репозитория
func (s *RepoService) SimulateRemove(ctx context.Context, source string) ([]string, error) {
	var willRemove []string

	if source == "all" {
		repos, err := s.GetRepositories(ctx, false)
		if err != nil {
			return nil, err
		}
		for _, repo := range repos {
			willRemove = append(willRemove, fmt.Sprintf(app.T_("Will remove: %s"), repo.Raw))
		}
		return willRemove, nil
	}

	urls, err := s.parseSource(ctx, source)
	if err != nil {
		return nil, err
	}

	for _, u := range urls {
		exists, _, _ := s.checkRepoExists(ctx, u)
		if exists {
			willRemove = append(willRemove, fmt.Sprintf(app.T_("Will remove: %s"), u))
		}
	}

	return willRemove, nil
}

// GetArch возвращает архитектуру системы
func (s *RepoService) GetArch() string {
	return s.arch
}

// GetConfMain возвращает путь к основному файлу конфигурации
func (s *RepoService) GetConfMain() string {
	return s.confMain
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
			if !seen[pkg] {
				seen[pkg] = true
				packages = append(packages, pkg)
			}
		}
	}

	return packages, nil
}
