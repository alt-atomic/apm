package altfiles

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// PasswdEntry представляет одну строку из /etc/passwd
type PasswdEntry struct {
	Name     string
	Password string
	UID      int
	GID      int
	Gecos    string
	Home     string
	Shell    string
}

func (e PasswdEntry) String() string {
	return fmt.Sprintf("%s:%s:%d:%d:%s:%s:%s", e.Name, e.Password, e.UID, e.GID, e.Gecos, e.Home, e.Shell)
}

// ParsePasswd парсит содержимое файла passwd
func ParsePasswd(data []byte) ([]PasswdEntry, error) {
	var entries []PasswdEntry
	for _, line := range bytes.Split(data, []byte("\n")) {
		s := strings.TrimSpace(string(line))
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}

		fields := strings.SplitN(s, ":", 7)
		if len(fields) != 7 {
			return nil, fmt.Errorf("invalid passwd line: %s", s)
		}

		uid, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("invalid UID in passwd line: %s", s)
		}

		gid, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil, fmt.Errorf("invalid GID in passwd line: %s", s)
		}

		entries = append(entries, PasswdEntry{
			Name:     fields[0],
			Password: fields[1],
			UID:      uid,
			GID:      gid,
			Gecos:    fields[4],
			Home:     fields[5],
			Shell:    fields[6],
		})
	}
	return entries, nil
}

// FormatPasswd сериализует записи обратно в формат passwd
func FormatPasswd(entries []PasswdEntry) []byte {
	var buf bytes.Buffer
	for _, e := range entries {
		buf.WriteString(e.String())
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// isRegularUser возвращает true для root (UID 0) и реальных пользователей (UID 1000-60000)
func isRegularUser(uid int) bool {
	return uid == 0 || (uid >= 1000 && uid <= 60000)
}

// SplitPasswd разделяет записи passwd на /etc (root + UID 1000-60000) и /usr/lib (остальные)
func SplitPasswd(entries []PasswdEntry) (etc []PasswdEntry, lib []PasswdEntry) {
	for _, e := range entries {
		if isRegularUser(e.UID) {
			etc = append(etc, e)
		} else {
			lib = append(lib, e)
		}
	}
	return
}

// MergePasswd объединяет записи из нескольких источников, дедуплицируя по имени
func MergePasswd(sources ...[]PasswdEntry) []PasswdEntry {
	seen := map[string]int{}
	var result []PasswdEntry
	for _, entries := range sources {
		for _, e := range entries {
			if idx, ok := seen[e.Name]; ok {
				result[idx] = e
			} else {
				seen[e.Name] = len(result)
				result = append(result, e)
			}
		}
	}
	return result
}
