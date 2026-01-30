package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var placeholderRegexp = regexp.MustCompile(`\$\{\{\s*([A-Za-z0-9_.\-]+)\s*}}`)

// resolveEnvPlaceholders заменяет ${{ Env.XXX }} на значения из env map
func resolveEnvPlaceholders(s string, env map[string]string) string {
	return placeholderRegexp.ReplaceAllStringFunc(s, func(match string) string {
		// Извлекаем имя переменной из ${{ Env.XXX }}
		submatch := placeholderRegexp.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		expr := submatch[1]

		// Поддерживаем формат Env.XXX
		if strings.HasPrefix(expr, "Env.") {
			envKey := strings.TrimPrefix(expr, "Env.")
			if val, ok := env[envKey]; ok {
				return val
			}
		}
		return match
	})
}

// hashJSON хеширует структуру через JSON
func hashJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "error"
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])[:8]
}

// hashFile хеширует содержимое файла
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil))[:8], nil
}

// hashDir хеширует директорию рекурсивно
func hashDir(dir string) (string, error) {
	var hashes []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(dir, path)
		fileHash, err := hashFile(path)
		if err != nil {
			return err
		}

		hashes = append(hashes, relPath+":"+fileHash)
		return nil
	})

	if err != nil {
		return "", err
	}

	sort.Strings(hashes)
	combined := strings.Join(hashes, "\n")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])[:8], nil
}

// hashPath хеширует файл или директорию
func hashPath(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		return hashDir(path)
	}
	return hashFile(path)
}

// hashWithEnv создаёт хэш от структуры с учётом env переменных
func hashWithEnv(v any, env map[string]string) string {
	// Хэшируем структуру + env для уникальности
	data := struct {
		Body any               `json:"body"`
		Env  map[string]string `json:"env"`
	}{
		Body: v,
		Env:  env,
	}
	return hashJSON(data)
}