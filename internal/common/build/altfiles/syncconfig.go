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

// SyncBody содержит списки групп и селекторы пользователей
type SyncBody struct {
	Groups []string       `yaml:"groups"`
	Users  []UserSelector `yaml:"users,omitempty"`
}

// UserSelector выбирает пользователей: по имени, точному UID или диапазону UID.
type UserSelector struct {
	Name     string
	UID      *int
	UIDRange *UIDRange
}

const (
	defaultUIDRangeMin = 1000
	defaultUIDRangeMax = 60000
)

type UIDRange struct {
	Min int `yaml:"min"`
	Max int `yaml:"max"`
}

// Bounds возвращает границы диапазона с подстановкой дефолтов
func (r UIDRange) Bounds() (int, int, error) {
	lo, hi := r.Min, r.Max
	if lo == 0 {
		lo = defaultUIDRangeMin
	}
	if hi == 0 {
		hi = defaultUIDRangeMax
	}
	if lo > hi {
		return 0, 0, fmt.Errorf("invalid uid_range: min %d greater than max %d", lo, hi)
	}
	return lo, hi, nil
}

// UnmarshalYAML принимает строку (сокращение для name) или объект с name/uid/uid_range
func (u *UserSelector) UnmarshalYAML(unmarshal func(any) error) error {
	var name string
	if err := unmarshal(&name); err == nil {
		if name == "" {
			return fmt.Errorf("user selector: empty name")
		}
		*u = UserSelector{Name: name}
		return nil
	}

	var body struct {
		Name     string    `yaml:"name"`
		UID      *int      `yaml:"uid"`
		UIDRange *UIDRange `yaml:"uid_range"`
	}
	if err := unmarshal(&body); err != nil {
		return fmt.Errorf("user selector must be a name or an object with one of name, uid, uid_range: %w", err)
	}

	set := 0
	if body.Name != "" {
		set++
	}
	if body.UID != nil {
		set++
	}
	if body.UIDRange != nil {
		set++
	}
	if set != 1 {
		return fmt.Errorf("user selector must specify exactly one of: name, uid, uid_range")
	}

	*u = UserSelector{Name: body.Name, UID: body.UID, UIDRange: body.UIDRange}
	return nil
}

// SyncResult статистика выполнения sync-groups
type SyncResult struct {
	Added   int
	Fixed   int
	Removed int
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
