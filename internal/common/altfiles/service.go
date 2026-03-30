package altfiles

import (
	"fmt"
	"os"
)

const (
	defaultEtcPasswd   = "/etc/passwd"
	defaultEtcGroup    = "/etc/group"
	defaultEtcNsswitch = "/etc/nsswitch.conf"
	defaultLibPasswd   = "/usr/lib/passwd"
	defaultLibGroup    = "/usr/lib/group"
)

// DefaultSyncConfigDir — директория конфигов sync-groups по умолчанию
const DefaultSyncConfigDir = "/usr/apm/grpconf.d"

// Config содержит пути к файлам passwd/group/nsswitch
type Config struct {
	EtcPasswd   string
	EtcGroup    string
	EtcNsswitch string
	LibPasswd   string
	LibGroup    string
}

// DefaultConfig возвращает конфигурацию с путями по умолчанию
func DefaultConfig() Config {
	return Config{
		EtcPasswd:   defaultEtcPasswd,
		EtcGroup:    defaultEtcGroup,
		EtcNsswitch: defaultEtcNsswitch,
		LibPasswd:   defaultLibPasswd,
		LibGroup:    defaultLibGroup,
	}
}

// Service предоставляет операции с altfiles (split/fix/sync passwd и group)
type Service struct {
	cfg Config
}

// New создаёт новый Service с заданной конфигурацией
func New(cfg Config) *Service {
	return &Service{cfg: cfg}
}

// NewDefault создаёт Service с путями по умолчанию
func NewDefault() *Service {
	return New(DefaultConfig())
}

// ApplyResult содержит статистику выполнения
type ApplyResult struct {
	EtcPasswdCount int
	LibPasswdCount int
	EtcGroupCount  int
	LibGroupCount  int
}

// ApplyBuild разделяет passwd/group и патчит nsswitch.conf (для сборки в контейнере)
func (s *Service) ApplyBuild() (*ApplyResult, error) {
	etcPasswd, libPasswd, err := s.splitPasswdFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to split passwd: %w", err)
	}

	etcGroup, libGroup, err := s.splitGroupFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to split group: %w", err)
	}

	if err = s.patchNsswitchFile(); err != nil {
		return nil, fmt.Errorf("failed to patch nsswitch.conf: %w", err)
	}

	return &ApplyResult{
		EtcPasswdCount: len(etcPasswd),
		LibPasswdCount: len(libPasswd),
		EtcGroupCount:  len(etcGroup),
		LibGroupCount:  len(libGroup),
	}, nil
}

// ApplyFix чистит /etc/passwd и /etc/group на живой системе,
// удаляя записи которые уже есть в /usr/lib (иммутабельный образ)
func (s *Service) ApplyFix() (*ApplyResult, error) {
	etcPasswdCount, libPasswdCount, err := s.cleanEtcPasswd()
	if err != nil {
		return nil, fmt.Errorf("failed to clean /etc/passwd: %w", err)
	}

	etcGroupCount, libGroupCount, err := s.cleanEtcGroup()
	if err != nil {
		return nil, fmt.Errorf("failed to clean /etc/group: %w", err)
	}

	if err = s.patchNsswitchFile(); err != nil {
		return nil, fmt.Errorf("failed to patch nsswitch.conf: %w", err)
	}

	return &ApplyResult{
		EtcPasswdCount: etcPasswdCount,
		LibPasswdCount: libPasswdCount,
		EtcGroupCount:  etcGroupCount,
		LibGroupCount:  libGroupCount,
	}, nil
}

// SyncGroups выполняет синхронизацию групп из конфигов
func (s *Service) SyncGroups(configs []SyncConfig) (*SyncResult, error) {
	libMap := loadGroupMap(s.cfg.LibGroup)

	etcData, err := os.ReadFile(s.cfg.EtcGroup)
	if err != nil {
		return nil, err
	}
	etcEntries, err := ParseGroup(etcData)
	if err != nil {
		return nil, err
	}
	etcMap := map[string]int{}
	for i, e := range etcEntries {
		etcMap[e.Name] = i
	}

	result := &SyncResult{}

	for _, cfg := range configs {
		users, err := s.resolveUsers(cfg.Sync.Users)
		if err != nil {
			return nil, err
		}
		if len(users) == 0 {
			continue
		}

		for _, grpName := range cfg.Sync.Groups {
			syncGroup(grpName, users, libMap, &etcEntries, etcMap, result)
		}
	}

	info, err := os.Stat(s.cfg.EtcGroup)
	if err != nil {
		return nil, err
	}

	return result, os.WriteFile(s.cfg.EtcGroup, FormatGroup(etcEntries), info.Mode().Perm())
}
