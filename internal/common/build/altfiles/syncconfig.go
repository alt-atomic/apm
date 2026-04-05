package altfiles

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

// SyncConfig представляет YAML-конфиг для sync-groups
type SyncConfig struct {
	Sync SyncBody `yaml:"sync"`
}

// SyncBody содержит списки групп и пользователей
type SyncBody struct {
	Groups []string `yaml:"groups"`
	Users  []string `yaml:"users,omitempty"`
}

// SyncResult статистика выполнения sync-groups
type SyncResult struct {
	Added   int
	Fixed   int
	Skipped int
}

// ReadSyncConfigsDirs читает конфиги из нескольких директорий
func (s *Service) ReadSyncConfigsDirs(dirs []string) ([]SyncConfig, error) {
	var all []SyncConfig
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		configs, err := s.ReadSyncConfigs(dir)
		if err != nil {
			return nil, err
		}
		all = append(all, configs...)
	}
	return all, nil
}

// ReadSyncConfigs читает все .yaml/.yml файлы из директории
func (s *Service) ReadSyncConfigs(dir string) ([]SyncConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read config dir %s: %w", dir, err)
	}

	var configs []SyncConfig
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", entry.Name(), err)
		}

		cfg, err := ParseSyncConfig(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", entry.Name(), err)
		}
		configs = append(configs, *cfg)
	}

	return configs, nil
}

// ParseSyncConfig парсит YAML-конфиг
func ParseSyncConfig(data []byte) (*SyncConfig, error) {
	var cfg SyncConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
