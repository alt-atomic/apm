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
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// GetRepositories returns sources from all config files
func (s *RepoService) GetRepositories(_ context.Context, all bool) ([]Repository, error) {
	s.ensureInitialized()
	var repos []Repository

	files, err := s.getSourceFiles()
	if err != nil {
		return nil, err
	}

	for _, filename := range files {
		parsed, errParse := s.parseSourceFile(filename, all)
		if errParse != nil {
			continue
		}
		repos = append(repos, parsed...)
	}

	return repos, nil
}

// parseSourceFile parses a single sources file
func (s *RepoService) parseSourceFile(filename string, all bool) ([]Repository, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var repos []Repository
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}

		// Active sources
		if !strings.HasPrefix(trimmed, "#") {
			if repo := s.parseLine(trimmed, filename, true); repo != nil {
				repos = append(repos, *repo)
			}
		} else if all {
			commented := strings.TrimPrefix(trimmed, "#")
			commented = strings.TrimSpace(commented)
			if strings.HasPrefix(commented, "rpm") {
				if repo := s.parseLine(commented, filename, false); repo != nil {
					repos = append(repos, *repo)
				}
			}
		}
	}

	return repos, nil
}

// parseLine parses a single source line
func (s *RepoService) parseLine(line string, filename string, active bool) *Repository {
	if !strings.HasPrefix(line, "rpm") {
		return nil
	}
	canonical := canonicalizeRepoLine(line)

	parts := strings.Fields(canonical)
	if len(parts) < 4 {
		return nil
	}

	repo := &Repository{
		Active: active,
		File:   filename,
		Entry:  line,
	}

	// Skip type (rpm) and optional key ([key])
	idx := 1
	if strings.HasPrefix(parts[idx], "[") {
		idx++
	}

	// URL
	if idx < len(parts) {
		repo.URL = parts[idx]
		idx++
	}

	// Architecture
	if idx < len(parts) {
		repo.Arch = parts[idx]
		idx++
	}

	// Components
	if idx < len(parts) {
		repo.Components = parts[idx:]
	}

	// Detect branch by URL
	repo.Branch = s.detectBranch(repo.URL)

	return repo
}

// getSourceFiles returns the list of sources files
func (s *RepoService) getSourceFiles() ([]string, error) {
	var files []string

	if _, err := os.Stat(s.confMain); err == nil {
		files = append(files, s.confMain)
	}

	pattern := filepath.Join(s.confDir, "*.list")
	matches, err := filepath.Glob(pattern)
	if err == nil {
		slices.Sort(matches)
		files = append(files, matches...)
	}

	return files, nil
}

// checkRepoExists reports whether the source is present and whether it is active
func (s *RepoService) checkRepoExists(ctx context.Context, repoLine string) (found bool, active bool, err error) {
	repos, err := s.GetRepositories(ctx, true)
	if err != nil {
		return false, false, err
	}

	canonical := canonicalizeRepoLine(repoLine)

	for _, repo := range repos {
		if canonicalizeRepoLine(repo.Entry) == canonical {
			return true, repo.Active, nil
		}
	}

	return false, false, nil
}

// canonicalizeRepoLine normalizes a source line for comparison.
// apt-repo uses "new_format", moving trailing URL path components into the arch field:
//
//	Old: rpm [key] http://host/path/comp1/comp2 x86_64 classic
//	New: rpm [key] http://host/path comp1/comp2/x86_64 classic
//
// The function unfolds new_format back, moving path components from arch into the URL.
// TODO Consider writing the "new" format like apt-repo does instead of supporting BOTH
func canonicalizeRepoLine(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return strings.Join(fields, " ")
	}

	idx := 1
	if strings.HasPrefix(fields[idx], "[") {
		idx++
	}

	if idx+1 >= len(fields) {
		return strings.Join(fields, " ")
	}

	fields[idx] = strings.TrimRight(fields[idx], "/")

	archField := fields[idx+1]

	if !strings.Contains(archField, "/") {
		return strings.Join(fields, " ")
	}

	archParts := strings.Split(archField, "/")
	realArch := archParts[len(archParts)-1]
	pathPrefix := strings.Join(archParts[:len(archParts)-1], "/")

	fields[idx] = fields[idx] + "/" + pathPrefix
	fields[idx+1] = realArch

	return strings.Join(fields, " ")
}

// stripScheme drops the URL scheme
func stripScheme(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return strings.TrimRight(rawURL, "/")
	}
	return strings.TrimRight(u.Host+u.Path, "/")
}

// detectBranch resolves the branch name from a source URL
func (s *RepoService) detectBranch(repoURL string) string {
	repoClean := stripScheme(repoURL)

	// Task repositories
	if strings.Contains(repoClean, repoTaskURL) {
		return "task"
	}

	for _, branch := range s.branches {
		if repoClean == stripScheme(branch.URL) {
			return strings.ToLower(branch.Name)
		}
	}

	repoLower := strings.ToLower(repoClean)
	if idx := strings.LastIndex(repoLower, "."); idx > strings.LastIndex(repoLower, "/") {
		repoLower = repoLower[:idx]
	}
	for _, branch := range s.branches {
		name := strings.ToLower(branch.Name)
		if strings.Contains(repoLower, "/"+name+"/") || strings.HasSuffix(repoLower, "/"+name) {
			return name
		}
	}

	return ""
}

// rewriteLines applies transform to file lines and rewrites the file if anything changed.
func rewriteLines(filename string, transform func(line string) (string, bool)) (bool, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(content), "\n")
	newLines := make([]string, 0, len(lines))
	modified := false

	for _, line := range lines {
		newLine, keep := transform(line)
		if !keep {
			modified = true
			continue
		}
		if newLine != line {
			modified = true
		}
		newLines = append(newLines, newLine)
	}

	if !modified {
		return false, nil
	}

	return true, os.WriteFile(filename, []byte(strings.Join(newLines, "\n")), 0644)
}

// uncommentRepo uncomments the source and returns the file it was found in
func (s *RepoService) uncommentRepo(repoLine string) (string, error) {
	files, err := s.getSourceFiles()
	if err != nil {
		return "", err
	}

	canonical := canonicalizeRepoLine(repoLine)
	var foundFile string

	for _, filename := range files {
		modified, errRewrite := rewriteLines(filename, func(line string) (string, bool) {
			trimmed := strings.TrimSpace(line)
			if commented, ok := strings.CutPrefix(trimmed, "#"); ok {
				commented = strings.TrimSpace(commented)
				if canonicalizeRepoLine(commented) == canonical {
					return commented, true
				}
			}
			return line, true
		})
		if errRewrite != nil {
			if os.IsNotExist(errRewrite) {
				continue
			}
			return "", errRewrite
		}
		if modified {
			foundFile = filename
		}
	}

	return foundFile, nil
}

// appendRepo appends the source to the main sources.list
func (s *RepoService) appendRepo(repoLine string) error {
	parts := strings.Fields(repoLine)
	if len(parts) < 3 {
		return fmt.Errorf(T_("Invalid repository line: %s"), repoLine)
	}

	file, err := os.OpenFile(s.confMain, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf(T_("Failed to open %s: %v"), s.confMain, err)
	}
	defer func() { _ = file.Close() }()

	_, err = file.WriteString(repoLine + "\n")
	if err != nil {
		return fmt.Errorf(T_("Failed to write to %s: %v"), s.confMain, err)
	}

	return nil
}

// removeOrCommentRepo removes the source from the main file and comments it in the others
func (s *RepoService) removeOrCommentRepo(repoLine string) error {
	canonical := canonicalizeRepoLine(repoLine)

	if err := s.removeFromFile(s.confMain, canonical); err != nil {
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
		_ = s.commentInFile(filename, canonical)
	}

	return nil
}

// removeFromFile deletes matching lines from the file
func (s *RepoService) removeFromFile(filename string, canonicalLine string) error {
	_, err := rewriteLines(filename, func(line string) (string, bool) {
		return line, canonicalizeRepoLine(line) != canonicalLine
	})
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// commentInFile comments out matching lines in the file
func (s *RepoService) commentInFile(filename string, canonicalLine string) error {
	_, err := rewriteLines(filename, func(line string) (string, bool) {
		if canonicalizeRepoLine(line) == canonicalLine {
			return "#" + line, true
		}
		return line, true
	})
	return err
}

// purgeAllRepos deletes all sources files entirely
func (s *RepoService) purgeAllRepos() error {
	entries, err := os.ReadDir(s.confDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(T_("Failed to read %s: %v"), s.confDir, err)
	}

	for _, entry := range entries {
		path := filepath.Join(s.confDir, entry.Name())
		if err = os.RemoveAll(path); err != nil {
			return fmt.Errorf(T_("Failed to remove %s: %v"), path, err)
		}
	}

	if err = os.WriteFile(s.confMain, []byte("\n"), 0644); err != nil {
		return fmt.Errorf(T_("Failed to clear %s: %v"), s.confMain, err)
	}

	return nil
}
