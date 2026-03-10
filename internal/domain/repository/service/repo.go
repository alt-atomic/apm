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
	"apm/internal/common/command"
	"bufio"
	"context"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
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
	RepoBaseURL = "ftp.altlinux.org/pub/distributions/ALTLinux"
	// RepoBaseURLRu is the base URL for ALT Linux repositories (ftp.altlinux.ru).
	RepoBaseURLRu = "ftp.altlinux.ru/pub/distributions/ALTLinux"
	// RepoArchiveURL is the base URL for archived ALT Linux repositories.
	RepoArchiveURL = "ftp.altlinux.org/pub/distributions/archive"
	// RepoTaskURL is the URL for active task repositories.
	RepoTaskURL = "git.altlinux.org/repo"
	// RepoTasksURL is the URL for task information.
	RepoTasksURL = "git.altlinux.org/tasks"
	// RepoCert8URL RepoTasksArchive = "git.altlinux.org/tasks/archive/done"
	RepoCert8URL = "update.altsp.su/pub/distributions/ALTLinux/c8/branch"

	// HTTPTimeout is the default timeout for HTTP requests.
	HTTPTimeout = 10 * time.Second
)

// Repository представляет информацию о репозитории
type Repository struct {
	URL        string   `json:"url"`
	Arch       string   `json:"arch"`
	Components []string `json:"components"`
	Active     bool     `json:"active"`
	File       string   `json:"file"`
	Entry      string   `json:"entry"`
	Branch     string   `json:"branch,omitempty"`
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
	confMain           string
	confDir            string
	arch               string
	branches           map[string]Branch
	useArepo           bool
	httpClient         *http.Client
	serviceAptDatabase packageDBService
	runner             commandRunner
	initOnce           sync.Once
}

// NewRepoService создает новый сервис для работы с репозиториями
func NewRepoService(dbService packageDBService, runner commandRunner) *RepoService {
	return &RepoService{
		confMain: DefaultSourcesList,
		confDir:  DefaultSourcesListDir,
		arch:     detectArch(runner),
		useArepo: checkArepoEnabled(),
		httpClient: &http.Client{
			Timeout: HTTPTimeout,
		},
		serviceAptDatabase: dbService,
		runner:             runner,
	}
}

// ensureInitialized выполняет отложенную инициализацию конфигурации APT и веток
func (s *RepoService) ensureInitialized() {
	s.initOnce.Do(func() {
		s.detectAPTConfig()
		s.initBranches()
	})
}

// detectArch определяет архитектуру системы
func detectArch(runner commandRunner) string {
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
	stdout, _, err := runner.Run(context.Background(), []string{"uname", "-m"}, command.WithQuiet())
	if err == nil {
		archStr := strings.TrimSpace(stdout)
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

// httpScheme возвращает схему URL (http или https) в зависимости от наличия apt-https
func (s *RepoService) httpScheme(ctx context.Context) string {
	if s.checkHTTPSEnabled(ctx) {
		return "https://"
	}
	return "http://"
}

// checkHTTPSEnabled проверяет установлен ли пакет apt-https
func (s *RepoService) checkHTTPSEnabled(ctx context.Context) bool {
	_, err := s.serviceAptDatabase.GetPackageByName(ctx, "apt-https")
	return err == nil
}

// detectAPTConfig получает пути конфигурации из apt-config
func (s *RepoService) detectAPTConfig() {
	stdout, _, err := s.runner.Run(context.Background(), []string{"apt-config", "shell", "FILE", "Dir::Etc::sourcelist/f"}, command.WithQuiet())
	if err == nil {
		if matches := regexp.MustCompile(`^FILE=(.*)$`).FindStringSubmatch(strings.TrimSpace(stdout)); len(matches) > 1 {
			path := strings.Trim(matches[1], `"'`)
			if path != "" {
				s.confMain = path
			}
		}
	}

	stdout, _, err = s.runner.Run(context.Background(), []string{"apt-config", "shell", "DIR", "Dir::Etc::sourceparts/d"}, command.WithQuiet())
	if err == nil {
		if matches := regexp.MustCompile(`^DIR=(.*)$`).FindStringSubmatch(strings.TrimSpace(stdout)); len(matches) > 1 {
			path := strings.Trim(matches[1], `"'`)
			if path != "" {
				s.confDir = path
			}
		}
	}
}

// GetArch возвращает архитектуру системы
func (s *RepoService) GetArch() string {
	return s.arch
}

// GetConfMain возвращает путь к основному файлу конфигурации
func (s *RepoService) GetConfMain() string {
	s.ensureInitialized()
	return s.confMain
}
