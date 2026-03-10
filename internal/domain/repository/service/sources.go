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
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GetRepositories возвращает список репозиториев
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

// parseSourceFile парсит один файл с репозиториями
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

		// Активные репозитории
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

// parseLine парсит строку репозитория
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

	// Пропускаем тип (rpm) и опциональный ключ ([key])
	idx := 1
	if strings.HasPrefix(parts[idx], "[") {
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

	// Определяем ветку по URL
	repo.Branch = s.detectBranch(repo.URL)

	return repo
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

	canonical := canonicalizeRepoLine(repoLine)

	for _, repo := range repos {
		repoCanonical := canonicalizeRepoLine(repo.Entry)
		if repoCanonical == canonical {
			return repo.Active, !repo.Active, nil
		}
	}

	return false, false, nil
}

// canonicalizeRepoLine приводит строку репозитория к каноническому виду для сравнения.
// apt-repo использует "new_format", перемещая последние компоненты URL-пути в поле arch:
//
//	Old: rpm [key] http://host/path/comp1/comp2 x86_64 classic
//	New: rpm [key] http://host/path comp1/comp2/x86_64 classic
//
// Функция разворачивает new_format обратно, перемещая путевые компоненты из arch в URL.
// TODO Возможно стоит использовать так называемый "новый" формат как в коде apt-repo, а не поддерживать ОБА
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

// stripScheme убирает схему
func stripScheme(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return strings.TrimRight(rawURL, "/")
	}
	return strings.TrimRight(u.Host+u.Path, "/")
}

// detectBranch определяет название ветки по URL репозитория
func (s *RepoService) detectBranch(repoURL string) string {
	repoClean := stripScheme(repoURL)

	// Для task-репозиториев
	if strings.Contains(repoClean, RepoTaskURL) {
		return "task"
	}

	for _, branch := range s.branches {
		if repoClean == stripScheme(branch.URL) {
			return strings.ToLower(branch.Name)
		}
	}

	repoLower := strings.ToLower(repoClean)
	for _, branch := range s.branches {
		name := strings.ToLower(branch.Name)
		if strings.Contains(repoLower, "/"+name+"/") || strings.HasSuffix(repoLower, "/"+name) {
			return name
		}
	}

	return ""
}

// uncommentRepo раскомментирует репозиторий и возвращает имя файла, в котором он был найден
func (s *RepoService) uncommentRepo(repoLine string) (string, error) {
	files, err := s.getSourceFiles()
	if err != nil {
		return "", err
	}

	canonical := canonicalizeRepoLine(repoLine)
	var foundFile string

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
				if canonicalizeRepoLine(commented) == canonical {
					lines[i] = commented
					modified = true
					foundFile = filename
				}
			}
		}

		if modified {
			errWrite := os.WriteFile(filename, []byte(strings.Join(lines, "\n")), 0644)
			if errWrite != nil {
				return "", errWrite
			}
		}
	}

	return foundFile, nil
}

// appendRepo добавляет репозиторий в sources.list
func (s *RepoService) appendRepo(repoLine string) error {
	parts := strings.Fields(repoLine)
	if len(parts) < 3 {
		return fmt.Errorf(app.T_("Invalid repository line: %s"), repoLine)
	}

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
		if err = s.commentInFile(filename, canonical); err != nil {
			continue
		}
	}

	return nil
}

// removeFromFile удаляет строку из файла
func (s *RepoService) removeFromFile(filename string, canonicalLine string) error {
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
		if canonicalizeRepoLine(line) != canonicalLine {
			newLines = append(newLines, line)
		}
	}

	return os.WriteFile(filename, []byte(strings.Join(newLines, "\n")), 0644)
}

// commentInFile комментирует строку в файле
func (s *RepoService) commentInFile(filename string, canonicalLine string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	modified := false

	for i, line := range lines {
		if canonicalizeRepoLine(line) == canonicalLine {
			lines[i] = "#" + line
			modified = true
		}
	}

	if modified {
		return os.WriteFile(filename, []byte(strings.Join(lines, "\n")), 0644)
	}

	return nil
}

// purgeAllRepos полностью удаляет все файлы репозиториев
func (s *RepoService) purgeAllRepos() error {
	entries, err := os.ReadDir(s.confDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(app.T_("Failed to read %s: %v"), s.confDir, err)
	}

	for _, entry := range entries {
		path := filepath.Join(s.confDir, entry.Name())
		if err = os.RemoveAll(path); err != nil {
			return fmt.Errorf(app.T_("Failed to remove %s: %v"), path, err)
		}
	}

	if err = os.WriteFile(s.confMain, []byte("\n"), 0644); err != nil {
		return fmt.Errorf(app.T_("Failed to clear %s: %v"), s.confMain, err)
	}

	return nil
}
