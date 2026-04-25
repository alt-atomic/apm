package etcfiles

import (
	"bufio"
	"fmt"
	"os"
)

// ParseColonFile парсит файл с colon-separated записями (passwd, group)
func ParseColonFile[T any](path string, parseLine func(string) (T, error)) ([]T, error) {
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

// TokenizeQuoted разбивает строку на токены, поддерживая строки в кавычках
func TokenizeQuoted(line string) []string {
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
