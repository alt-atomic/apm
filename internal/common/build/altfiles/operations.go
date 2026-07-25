package altfiles

import (
	"fmt"
	"os"
	"slices"

	"altlinux.space/alt-atomic/apm/internal/common/build/etcfiles"
)

// cleanEtcPasswd удаляет из /etc/passwd записи, которые уже есть в /usr/lib/passwd
func (s *Service) cleanEtcPasswd() (etcCount int, libCount int, removed int, err error) {
	libData, err := os.ReadFile(s.cfg.LibPasswd)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("%s not found, build the image first", s.cfg.LibPasswd)
	}
	libEntries, err := etcfiles.ParsePasswd(libData)
	if err != nil {
		return 0, 0, 0, err
	}

	libNames := make(map[string]struct{}, len(libEntries))
	for _, e := range libEntries {
		libNames[e.Name] = struct{}{}
	}

	etcData, err := os.ReadFile(s.cfg.EtcPasswd)
	if err != nil {
		return 0, 0, 0, err
	}
	etcEntries, err := etcfiles.ParsePasswd(etcData)
	if err != nil {
		return 0, 0, 0, err
	}

	var cleaned []etcfiles.PasswdEntry
	for _, e := range etcEntries {
		if _, inLib := libNames[e.Name]; !inLib {
			cleaned = append(cleaned, e)
		}
	}

	cleaned, removed = removeUIDConflicts(cleaned, libEntries)

	info, err := os.Stat(s.cfg.EtcPasswd)
	if err != nil {
		return 0, 0, 0, err
	}

	return len(cleaned), len(libEntries), removed, os.WriteFile(s.cfg.EtcPasswd, etcfiles.FormatPasswd(cleaned), info.Mode().Perm())
}

// cleanEtcGroup удаляет из /etc/group записи, которые уже есть в /usr/lib/group.
// Сохраняет записи у которых есть member-оверлейды (пользователи, отсутствующие в /usr/lib)
func (s *Service) cleanEtcGroup() (etcCount int, libCount int, removed int, err error) {
	libData, err := os.ReadFile(s.cfg.LibGroup)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("%s not found, build the image first", s.cfg.LibGroup)
	}
	libEntries, err := etcfiles.ParseGroup(libData)
	if err != nil {
		return 0, 0, 0, err
	}

	libMap := make(map[string]etcfiles.GroupEntry, len(libEntries))
	for _, e := range libEntries {
		libMap[e.Name] = e
	}

	etcData, err := os.ReadFile(s.cfg.EtcGroup)
	if err != nil {
		return 0, 0, 0, err
	}
	etcEntries, err := etcfiles.ParseGroup(etcData)
	if err != nil {
		return 0, 0, 0, err
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

	cleaned, removed = removeGIDConflicts(cleaned, libMap)

	info, err := os.Stat(s.cfg.EtcGroup)
	if err != nil {
		return 0, 0, 0, err
	}

	return len(cleaned), len(libEntries), removed, os.WriteFile(s.cfg.EtcGroup, etcfiles.FormatGroup(cleaned), info.Mode().Perm())
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

// joinPasswdFiles сливает /usr/lib/passwd в /etc/passwd и очищает lib.
func (s *Service) joinPasswdFiles() ([]etcfiles.PasswdEntry, error) {
	etcData, err := os.ReadFile(s.cfg.EtcPasswd)
	if err != nil {
		return nil, err
	}
	etcEntries, err := etcfiles.ParsePasswd(etcData)
	if err != nil {
		return nil, err
	}

	var libEntries []etcfiles.PasswdEntry
	if libData, readErr := os.ReadFile(s.cfg.LibPasswd); readErr == nil {
		libEntries, _ = etcfiles.ParsePasswd(libData)
	}

	// /etc выигрывает на конфликте имён.
	merged := etcfiles.MergePasswd(libEntries, etcEntries)

	info, err := os.Stat(s.cfg.EtcPasswd)
	if err != nil {
		return nil, err
	}
	perm := info.Mode().Perm()

	if err = os.WriteFile(s.cfg.EtcPasswd, etcfiles.FormatPasswd(merged), perm); err != nil {
		return nil, err
	}
	// Очищаем lib.
	if err = os.WriteFile(s.cfg.LibPasswd, nil, perm); err != nil {
		return nil, err
	}

	return merged, nil
}

// joinGroupFiles сливает /usr/lib/group в /etc/group и очищает lib.
func (s *Service) joinGroupFiles() ([]etcfiles.GroupEntry, error) {
	etcData, err := os.ReadFile(s.cfg.EtcGroup)
	if err != nil {
		return nil, err
	}
	etcEntries, err := etcfiles.ParseGroup(etcData)
	if err != nil {
		return nil, err
	}

	var libEntries []etcfiles.GroupEntry
	if libData, readErr := os.ReadFile(s.cfg.LibGroup); readErr == nil {
		libEntries, _ = etcfiles.ParseGroup(libData)
	}

	merged := etcfiles.MergeGroup(libEntries, etcEntries)

	info, err := os.Stat(s.cfg.EtcGroup)
	if err != nil {
		return nil, err
	}
	perm := info.Mode().Perm()

	if err = os.WriteFile(s.cfg.EtcGroup, etcfiles.FormatGroup(merged), perm); err != nil {
		return nil, err
	}
	if err = os.WriteFile(s.cfg.LibGroup, nil, perm); err != nil {
		return nil, err
	}

	return merged, nil
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

// resolveUsers возвращает список пользователей для синхронизации по селекторам.
func (s *Service) resolveUsers(selectors []UserSelector) ([]string, error) {
	if len(selectors) > 0 {
		return s.selectUsers(selectors)
	}
	return s.defaultWheelUsers()
}

// selectUsers резолвит селекторы в имена реально существующих пользователей.
func (s *Service) selectUsers(selectors []UserSelector) ([]string, error) {
	entries, err := s.existingUsers()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var result []string
	add := func(name string) {
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}

	for _, sel := range selectors {
		switch {
		case sel.Name != "":
			for _, e := range entries {
				if e.Name == sel.Name {
					add(e.Name)
					break
				}
			}
		case sel.UID != nil:
			for _, e := range entries {
				if e.UID == *sel.UID {
					add(e.Name)
				}
			}
		case sel.UIDRange != nil:
			lo, hi, err := sel.UIDRange.Bounds()
			if err != nil {
				return nil, err
			}
			for _, e := range entries {
				if e.UID >= lo && e.UID <= hi {
					add(e.Name)
				}
			}
		}
	}
	return result, nil
}

// defaultWheelUsers возвращает членов wheel, являющихся обычными
// пользователями (UID 1000-60000), исключая системные аккаунты.
func (s *Service) defaultWheelUsers() ([]string, error) {
	regular, err := s.regularUserNames()
	if err != nil {
		return nil, err
	}

	wheelMembers, err := s.getWheelMembers()
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(wheelMembers))
	for _, m := range wheelMembers {
		if _, ok := regular[m]; ok {
			result = append(result, m)
		}
	}
	return result, nil
}

// existingUsers собирает записи из /etc/passwd и /usr/lib/passwd,
// при совпадении имён приоритет у /etc.
func (s *Service) existingUsers() ([]etcfiles.PasswdEntry, error) {
	etcEntries, err := s.readPasswd(s.cfg.EtcPasswd)
	if err != nil {
		return nil, err
	}

	names := make(map[string]struct{}, len(etcEntries))
	for _, e := range etcEntries {
		names[e.Name] = struct{}{}
	}

	all := etcEntries
	if libEntries, errLib := s.readPasswd(s.cfg.LibPasswd); errLib == nil {
		for _, e := range libEntries {
			if _, ok := names[e.Name]; !ok {
				all = append(all, e)
			}
		}
	}
	return all, nil
}

// regularUserNames собирает обычных пользователей (UID 1000-60000) из /etc/passwd.
func (s *Service) regularUserNames() (map[string]struct{}, error) {
	etcEntries, err := s.readPasswd(s.cfg.EtcPasswd)
	if err != nil {
		return nil, err
	}

	names := map[string]struct{}{}
	for _, e := range etcEntries {
		if etcfiles.IsRegularUser(e.UID) && e.UID != 0 {
			names[e.Name] = struct{}{}
		}
	}
	return names, nil
}

// readPasswd читает и парсит passwd-файл.
func (s *Service) readPasswd(path string) ([]etcfiles.PasswdEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return etcfiles.ParsePasswd(data)
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

// removeUIDConflicts удаляет локальных системных пользователей, чей UID занят
// в /usr/lib/passwd пользователем с другим именем: приоритет у пользователей образа.
func removeUIDConflicts(entries []etcfiles.PasswdEntry, libEntries []etcfiles.PasswdEntry) (kept []etcfiles.PasswdEntry, removed int) {
	libUIDs := make(map[int]struct{}, len(libEntries))
	for _, e := range libEntries {
		libUIDs[e.UID] = struct{}{}
	}

	for _, e := range entries {
		_, uidTaken := libUIDs[e.UID]
		if uidTaken && !etcfiles.IsRegularUser(e.UID) {
			removed++
			continue
		}
		kept = append(kept, e)
	}
	return kept, removed
}

// removeGIDConflicts удаляет локальные системные группы, чей GID занят
// в /usr/lib/group группой с другим именем: приоритет у групп образа.
func removeGIDConflicts(entries []etcfiles.GroupEntry, libMap map[string]etcfiles.GroupEntry) (kept []etcfiles.GroupEntry, removed int) {
	libGIDs := make(map[int]struct{}, len(libMap))
	for _, e := range libMap {
		libGIDs[e.GID] = struct{}{}
	}

	for _, e := range entries {
		_, sameName := libMap[e.Name]
		_, gidTaken := libGIDs[e.GID]
		if gidTaken && !sameName && !etcfiles.IsRegularGroup(e.Name, e.GID) {
			removed++
			continue
		}
		kept = append(kept, e)
	}
	return kept, removed
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
