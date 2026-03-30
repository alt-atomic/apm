package altfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

const DefaultSyncConfigDir = "/usr/apm/grpconf.d"

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

// ParseSyncConfig парсит YAML-конфиг
func ParseSyncConfig(data []byte) (*SyncConfig, error) {
	var cfg SyncConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ReadSyncConfigs читает все .yaml/.yml файлы из директории
func ReadSyncConfigs(dir string) ([]SyncConfig, error) {
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

// ResolveUsers возвращает список пользователей для sync.
// Если users не пустой - возвращает как есть.
// Иначе - все пользователи из wheel с UID 1000-60000.
func ResolveUsers(users []string) ([]string, error) {
	if len(users) > 0 {
		return users, nil
	}

	passwdData, err := os.ReadFile(EtcPasswd)
	if err != nil {
		return nil, err
	}
	passwdEntries, err := ParsePasswd(passwdData)
	if err != nil {
		return nil, err
	}

	realUsers := map[string]struct{}{}
	for _, e := range passwdEntries {
		if isRegularUser(e.UID) && e.UID != 0 {
			realUsers[e.Name] = struct{}{}
		}
	}

	wheelMembers, err := getWheelMembers()
	if err != nil {
		return nil, err
	}

	var result []string
	for _, member := range wheelMembers {
		if _, ok := realUsers[member]; ok {
			result = append(result, member)
		}
	}

	return result, nil
}

// getWheelMembers возвращает объединённый список членов группы wheel
func getWheelMembers() ([]string, error) {
	var allMembers []string

	for _, path := range []string{EtcGroup, LibGroup} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		entries, err := ParseGroup(data)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.Name == "wheel" {
				for _, m := range splitMembers(e.Members) {
					if !slices.Contains(allMembers, m) {
						allMembers = append(allMembers, m)
					}
				}
			}
		}
	}

	return allMembers, nil
}

// SyncGroups выполняет синхронизацию групп из конфигов
func SyncGroups(configs []SyncConfig) (*SyncResult, error) {
	libMap := loadGroupMap(LibGroup)

	etcData, err := os.ReadFile(EtcGroup)
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
		users, err := ResolveUsers(cfg.Sync.Users)
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

	info, err := os.Stat(EtcGroup)
	if err != nil {
		return nil, err
	}

	return result, os.WriteFile(EtcGroup, FormatGroup(etcEntries), info.Mode().Perm())
}

func loadGroupMap(path string) map[string]GroupEntry {
	m := map[string]GroupEntry{}
	data, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	entries, err := ParseGroup(data)
	if err != nil {
		return m
	}
	for _, e := range entries {
		m[e.Name] = e
	}
	return m
}

func syncGroup(grpName string, users []string, libMap map[string]GroupEntry, etcEntries *[]GroupEntry, etcMap map[string]int, result *SyncResult) {
	libEntry, inLib := libMap[grpName]
	etcIdx, inEtc := etcMap[grpName]

	if !inLib && !inEtc {
		result.Skipped++
		return
	}

	gid := resolveGID(libEntry, inLib, *etcEntries, etcIdx, inEtc)

	if inEtc {
		entry := &(*etcEntries)[etcIdx]
		if inLib && entry.GID != gid {
			entry.GID = gid
			result.Fixed++
		}
		if addMembersToEntry(entry, users) {
			result.Added++
		} else {
			result.Skipped++
		}
	} else {
		*etcEntries = append(*etcEntries, GroupEntry{
			Name:     grpName,
			Password: "x",
			GID:      gid,
			Members:  strings.Join(users, ","),
		})
		etcMap[grpName] = len(*etcEntries) - 1
		result.Added++
	}
}

func resolveGID(libEntry GroupEntry, inLib bool, etcEntries []GroupEntry, etcIdx int, inEtc bool) int {
	if inLib {
		return libEntry.GID
	}
	if inEtc {
		return etcEntries[etcIdx].GID
	}
	return 0
}

func addMembersToEntry(entry *GroupEntry, users []string) bool {
	members := splitMembers(entry.Members)
	added := false
	for _, u := range users {
		if !slices.Contains(members, u) {
			members = append(members, u)
			added = true
		}
	}
	if added {
		entry.Members = strings.Join(members, ",")
	}
	return added
}

func splitMembers(members string) []string {
	if members == "" {
		return nil
	}
	var result []string
	for _, m := range strings.Split(members, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			result = append(result, m)
		}
	}
	return result
}
