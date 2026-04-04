package altfiles

import (
	"apm/internal/common/build/etcfiles"
	"fmt"
	"os"
	"slices"
)

// cleanEtcPasswd удаляет из /etc/passwd записи, которые уже есть в /usr/lib/passwd
func (s *Service) cleanEtcPasswd() (etcCount int, libCount int, err error) {
	libData, err := os.ReadFile(s.cfg.LibPasswd)
	if err != nil {
		return 0, 0, fmt.Errorf("%s not found, build the image first", s.cfg.LibPasswd)
	}
	libEntries, err := etcfiles.ParsePasswd(libData)
	if err != nil {
		return 0, 0, err
	}

	libNames := make(map[string]struct{}, len(libEntries))
	for _, e := range libEntries {
		libNames[e.Name] = struct{}{}
	}

	etcData, err := os.ReadFile(s.cfg.EtcPasswd)
	if err != nil {
		return 0, 0, err
	}
	etcEntries, err := etcfiles.ParsePasswd(etcData)
	if err != nil {
		return 0, 0, err
	}

	var cleaned []etcfiles.PasswdEntry
	for _, e := range etcEntries {
		if _, inLib := libNames[e.Name]; !inLib {
			cleaned = append(cleaned, e)
		}
	}

	info, err := os.Stat(s.cfg.EtcPasswd)
	if err != nil {
		return 0, 0, err
	}

	return len(cleaned), len(libEntries), os.WriteFile(s.cfg.EtcPasswd, etcfiles.FormatPasswd(cleaned), info.Mode().Perm())
}

// cleanEtcGroup удаляет из /etc/group записи, которые уже есть в /usr/lib/group.
// Сохраняет записи у которых есть member-оверлейды (пользователи, отсутствующие в /usr/lib)
func (s *Service) cleanEtcGroup() (etcCount int, libCount int, err error) {
	libData, err := os.ReadFile(s.cfg.LibGroup)
	if err != nil {
		return 0, 0, fmt.Errorf("%s not found, build the image first", s.cfg.LibGroup)
	}
	libEntries, err := etcfiles.ParseGroup(libData)
	if err != nil {
		return 0, 0, err
	}

	libMap := make(map[string]etcfiles.GroupEntry, len(libEntries))
	for _, e := range libEntries {
		libMap[e.Name] = e
	}

	etcData, err := os.ReadFile(s.cfg.EtcGroup)
	if err != nil {
		return 0, 0, err
	}
	etcEntries, err := etcfiles.ParseGroup(etcData)
	if err != nil {
		return 0, 0, err
	}

	var cleaned []etcfiles.GroupEntry
	for _, e := range etcEntries {
		libEntry, inLib := libMap[e.Name]
		if !inLib {
			cleaned = append(cleaned, e)
			continue
		}
		if hasUniqueMembers(e.Members, libEntry.Members) {
			if e.GID != libEntry.GID {
				e.GID = libEntry.GID
			}
			cleaned = append(cleaned, e)
		}
	}

	info, err := os.Stat(s.cfg.EtcGroup)
	if err != nil {
		return 0, 0, err
	}

	return len(cleaned), len(libEntries), os.WriteFile(s.cfg.EtcGroup, etcfiles.FormatGroup(cleaned), info.Mode().Perm())
}

// splitPasswdFiles для сборки: мержит /etc и /usr/lib, сплитит, пишет оба файла
func (s *Service) splitPasswdFiles() (etc []etcfiles.PasswdEntry, lib []etcfiles.PasswdEntry, err error) {
	etcData, err := os.ReadFile(s.cfg.EtcPasswd)
	if err != nil {
		return nil, nil, err
	}
	etcEntries, err := etcfiles.ParsePasswd(etcData)
	if err != nil {
		return nil, nil, err
	}

	var libEntries []etcfiles.PasswdEntry
	if libData, readErr := os.ReadFile(s.cfg.LibPasswd); readErr == nil {
		libEntries, _ = etcfiles.ParsePasswd(libData)
	}

	merged := etcfiles.MergePasswd(etcEntries, libEntries)
	newEtc, newLib := etcfiles.SplitPasswd(merged)

	info, err := os.Stat(s.cfg.EtcPasswd)
	if err != nil {
		return nil, nil, err
	}
	perm := info.Mode().Perm()

	if err = os.WriteFile(s.cfg.LibPasswd, etcfiles.FormatPasswd(newLib), perm); err != nil {
		return nil, nil, err
	}
	if err = os.WriteFile(s.cfg.EtcPasswd, etcfiles.FormatPasswd(newEtc), perm); err != nil {
		return nil, nil, err
	}

	return newEtc, newLib, nil
}

// splitGroupFiles для сборки: мержит /etc и /usr/lib, сплитит, пишет оба файла
func (s *Service) splitGroupFiles() (etc []etcfiles.GroupEntry, lib []etcfiles.GroupEntry, err error) {
	etcData, err := os.ReadFile(s.cfg.EtcGroup)
	if err != nil {
		return nil, nil, err
	}
	etcEntries, err := etcfiles.ParseGroup(etcData)
	if err != nil {
		return nil, nil, err
	}

	var libEntries []etcfiles.GroupEntry
	if libData, readErr := os.ReadFile(s.cfg.LibGroup); readErr == nil {
		libEntries, _ = etcfiles.ParseGroup(libData)
	}

	merged := etcfiles.MergeGroup(etcEntries, libEntries)
	newEtc, newLib := etcfiles.SplitGroup(merged)

	info, err := os.Stat(s.cfg.EtcGroup)
	if err != nil {
		return nil, nil, err
	}
	perm := info.Mode().Perm()

	if err = os.WriteFile(s.cfg.LibGroup, etcfiles.FormatGroup(newLib), perm); err != nil {
		return nil, nil, err
	}
	if err = os.WriteFile(s.cfg.EtcGroup, etcfiles.FormatGroup(newEtc), perm); err != nil {
		return nil, nil, err
	}

	return newEtc, newLib, nil
}

func (s *Service) patchNsswitchFile() error {
	data, err := os.ReadFile(s.cfg.EtcNsswitch)
	if err != nil {
		return err
	}

	info, err := os.Stat(s.cfg.EtcNsswitch)
	if err != nil {
		return err
	}

	patched := patchNsswitch(data)
	return os.WriteFile(s.cfg.EtcNsswitch, patched, info.Mode().Perm())
}

// resolveUsers возвращает валидированный список пользователей для sync.
// Если users указаны - проверяет что они существуют в /etc/passwd.
// Иначе - все пользователи из wheel с UID 1000-60000.
func (s *Service) resolveUsers(users []string) ([]string, error) {
	passwdData, err := os.ReadFile(s.cfg.EtcPasswd)
	if err != nil {
		return nil, err
	}
	passwdEntries, err := etcfiles.ParsePasswd(passwdData)
	if err != nil {
		return nil, err
	}

	existingUsers := map[string]struct{}{}
	realUsers := map[string]struct{}{}
	for _, e := range passwdEntries {
		existingUsers[e.Name] = struct{}{}
		if etcfiles.IsRegularUser(e.UID) && e.UID != 0 {
			realUsers[e.Name] = struct{}{}
		}
	}

	if len(users) > 0 {
		var validated []string
		for _, u := range users {
			if _, ok := existingUsers[u]; ok {
				validated = append(validated, u)
			}
		}
		return validated, nil
	}

	wheelMembers, err := s.getWheelMembers()
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
func (s *Service) getWheelMembers() ([]string, error) {
	var allMembers []string

	for _, path := range []string{s.cfg.EtcGroup, s.cfg.LibGroup} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		entries, err := etcfiles.ParseGroup(data)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.Name == "wheel" {
				for _, m := range e.Members {
					if !slices.Contains(allMembers, m) {
						allMembers = append(allMembers, m)
					}
				}
			}
		}
	}

	return allMembers, nil
}

func hasUniqueMembers(etcMembers, libMembers []string) bool {
	if len(etcMembers) == 0 {
		return false
	}
	libSet := make(map[string]struct{}, len(libMembers))
	for _, m := range libMembers {
		libSet[m] = struct{}{}
	}
	for _, m := range etcMembers {
		if _, ok := libSet[m]; !ok {
			return true
		}
	}
	return false
}

func loadGroupMap(path string) map[string]etcfiles.GroupEntry {
	m := map[string]etcfiles.GroupEntry{}
	data, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	entries, err := etcfiles.ParseGroup(data)
	if err != nil {
		return m
	}
	for _, e := range entries {
		m[e.Name] = e
	}
	return m
}

func syncGroup(grpName string, users []string, libMap map[string]etcfiles.GroupEntry, etcEntries *[]etcfiles.GroupEntry, etcMap map[string]int, result *SyncResult) {
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
		*etcEntries = append(*etcEntries, etcfiles.GroupEntry{
			Name:     grpName,
			Password: "x",
			GID:      gid,
			Members:  users,
		})
		etcMap[grpName] = len(*etcEntries) - 1
		result.Added++
	}
}

func resolveGID(libEntry etcfiles.GroupEntry, inLib bool, etcEntries []etcfiles.GroupEntry, etcIdx int, inEtc bool) int {
	if inLib {
		return libEntry.GID
	}
	if inEtc {
		return etcEntries[etcIdx].GID
	}
	return 0
}

func addMembersToEntry(entry *etcfiles.GroupEntry, users []string) bool {
	added := false
	for _, u := range users {
		if !slices.Contains(entry.Members, u) {
			entry.Members = append(entry.Members, u)
			added = true
		}
	}
	return added
}
