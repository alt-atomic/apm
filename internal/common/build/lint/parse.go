package lint

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

const confName = "apm-lint.conf"

func removeConf(rootfs, subdir string) (string, error) {
	path := filepath.Join(rootfs, "usr", "lib", subdir, confName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return path, err
	}
	return path, nil
}

// writeConf записывает содержимое conf-файла в rootfs/usr/lib/<subdir>/apm-lint.conf
func writeConf(rootfs, subdir, content string) (string, error) {
	dir := filepath.Join(rootfs, "usr", "lib", subdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	path := filepath.Join(dir, confName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	return path, nil
}

// tokenizeQuoted разбивает строку на токены, поддерживая строки в кавычках
func tokenizeQuoted(line string) []string {
	var tokens []string
	i := 0
	for i < len(line) {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= len(line) {
			break
		}

		if line[i] == '"' {
			i++
			start := i
			for i < len(line) && line[i] != '"' {
				if line[i] == '\\' && i+1 < len(line) {
					i++
				}
				i++
			}
			tokens = append(tokens, line[start:i])
			if i < len(line) {
				i++
			}
		} else {
			start := i
			for i < len(line) && line[i] != ' ' && line[i] != '\t' {
				i++
			}
			tokens = append(tokens, line[start:i])
		}
	}
	return tokens
}

// parseColonFile парсит файл с colon-separated записями (passwd, group)
func parseColonFile[T any](path string, parseLine func(string) (T, error)) ([]T, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var entries []T
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' || line[0] == '+' || line[0] == '-' {
			continue
		}
		entry, err := parseLine(line)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}
