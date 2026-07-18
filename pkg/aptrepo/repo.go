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
	"bufio"
	"context"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"altlinux.space/alt-atomic/apm/pkg/command"
)

const (
	// defaultSourcesList is the main APT sources configuration file.
	defaultSourcesList = "/etc/apt/sources.list"
	// defaultSourcesListDir is the directory for additional APT source files.
	defaultSourcesListDir = "/etc/apt/sources.list.d/"
	// arepoConfigFile is the arepo configuration file path.
	arepoConfigFile = "/etc/sysconfig/apt-repo"

	// rpmMacrosDir is the directory for RPM macros.
	rpmMacrosDir = "/etc/rpm/macros.d"
	// priorityDistbranchMacro is the priority distbranch macro file.
	priorityDistbranchMacro = "/etc/rpm/macros.d/priority_distbranch"
	// legacyP10Macro is the legacy p10 macro file.
	legacyP10Macro = "/etc/rpm/macros.d/p10"

	// repoBaseURL is the base URL for ALT Linux repositories (ftp.altlinux.org).
	repoBaseURL = "ftp.altlinux.org/pub/distributions/ALTLinux"
	// repoBaseURLRu is the base URL for ALT Linux repositories (ftp.altlinux.ru).
	repoBaseURLRu = "ftp.altlinux.ru/pub/distributions/ALTLinux"
	// repoArchiveURL is the base URL for archived ALT Linux repositories.
	repoArchiveURL = "ftp.altlinux.org/pub/distributions/archive"
	// repoTaskURL is the URL for active task repositories.
	repoTaskURL = "git.altlinux.org/repo"
	// repoTasksURL is the URL for task information.
	repoTasksURL = "git.altlinux.org/tasks"
	// repoCert8URL is the base URL for the c8 certified branch.
	repoCert8URL = "update.altsp.su/pub/distributions/ALTLinux/c8/branch"

	// httpTimeout is the default timeout for HTTP requests.
	httpTimeout = 10 * time.Second
)

// Repository describes a single APT source entry
type Repository struct {
	URL        string   `json:"url"`
	Arch       string   `json:"arch"`
	Components []string `json:"components"`
	Active     bool     `json:"active"`
	File       string   `json:"file"`
	Entry      string   `json:"entry"`
	Branch     string   `json:"branch"`
}

// branchInfo describes a known ALT Linux branch
type branchInfo struct {
	Name       string
	URL        string
	Key        string
	Components []string
}

// RepoService manages APT repository sources
type RepoService struct {
	confMain   string
	confDir    string
	arch       string
	branches   map[string]branchInfo
	useArepo   bool
	httpClient *http.Client
	hasPackage HasPackageFunc
	runner     commandRunner
	initOnce   sync.Once
	mu         sync.Mutex
}

// NewRepoService creates a repository service.
// hasPackage is used to detect apt-https (URL scheme)
func NewRepoService(hasPackage HasPackageFunc, runner commandRunner) *RepoService {
	return &RepoService{
		confMain: defaultSourcesList,
		confDir:  defaultSourcesListDir,
		arch:     detectArch(runner),
		useArepo: checkArepoEnabled(),
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
		hasPackage: hasPackage,
		runner:     runner,
	}
}

// ensureInitialized lazily loads APT config paths and known branches
func (s *RepoService) ensureInitialized() {
	s.initOnce.Do(func() {
		s.detectAPTConfig()
		s.initBranches()
	})
}

// detectArch returns the system architecture in APT notation
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

	// Fallback via uname
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

// checkArepoEnabled reports whether arepo sources are enabled
func checkArepoEnabled() bool {
	file, err := os.Open(arepoConfigFile)
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

// httpScheme returns the URL scheme depending on apt-https presence
func (s *RepoService) httpScheme(ctx context.Context) string {
	if s.checkHTTPSEnabled(ctx) {
		return "https://"
	}
	return "http://"
}

// checkHTTPSEnabled reports whether the apt-https package is installed
func (s *RepoService) checkHTTPSEnabled(ctx context.Context) bool {
	if s.hasPackage == nil {
		return false
	}
	return s.hasPackage(ctx, "apt-https")
}

// aptConfigRe matches apt-config shell output like VAR='value'
var aptConfigRe = regexp.MustCompile(`^([A-Z]+)=(.*)$`)

// detectAPTConfig reads configuration paths from apt-config
func (s *RepoService) detectAPTConfig() {
	if path := s.aptConfigPath("FILE", "Dir::Etc::sourcelist/f"); path != "" {
		s.confMain = path
	}
	if path := s.aptConfigPath("DIR", "Dir::Etc::sourceparts/d"); path != "" {
		s.confDir = path
	}
}

// aptConfigPath queries a path from apt-config, empty string if unavailable
func (s *RepoService) aptConfigPath(varName, key string) string {
	stdout, _, err := s.runner.Run(context.Background(), []string{"apt-config", "shell", varName, key}, command.WithQuiet())
	if err != nil {
		return ""
	}
	matches := aptConfigRe.FindStringSubmatch(strings.TrimSpace(stdout))
	if len(matches) < 3 || matches[1] != varName {
		return ""
	}
	return strings.Trim(matches[2], `"'`)
}

// GetArch returns the system architecture
func (s *RepoService) GetArch() string {
	return s.arch
}

// GetConfMain returns the main sources.list path
func (s *RepoService) GetConfMain() string {
	s.ensureInitialized()
	return s.confMain
}
