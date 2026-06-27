package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

// HashBody возвращает хэш модели из её JSON и дополнительных данных.
func HashBody(b Body, extra ...[]byte) (string, error) {
	data, err := json.Marshal(b)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	h.Write(data)
	for _, e := range extra {
		h.Write(e)
	}

	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// resourceDigest считает дайджест содержимого файла или дерева.
func resourceDigest(baseDir, src string) ([]byte, error) {
	if src == "" {
		return nil, nil
	}

	root := src
	if !filepath.IsAbs(root) {
		root = filepath.Join(baseDir, src)
	}

	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		info, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s|%o|", rel, info.Mode())
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write(data)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

// fetchURL загружает тело по URL.
func fetchURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
