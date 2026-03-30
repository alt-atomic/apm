package altfiles

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// GroupEntry представляет одну строку из /etc/group
type GroupEntry struct {
	Name     string
	Password string
	GID      int
	Members  string
}

func (e GroupEntry) String() string {
	return fmt.Sprintf("%s:%s:%d:%s", e.Name, e.Password, e.GID, e.Members)
}

// ParseGroup парсит содержимое файла group
func ParseGroup(data []byte) ([]GroupEntry, error) {
	var entries []GroupEntry
	for _, line := range bytes.Split(data, []byte("\n")) {
		s := strings.TrimSpace(string(line))
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}

		fields := strings.SplitN(s, ":", 4)
		if len(fields) != 4 {
			return nil, fmt.Errorf("invalid group line: %s", s)
		}

		gid, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("invalid GID in group line: %s", s)
		}

		entries = append(entries, GroupEntry{
			Name:     fields[0],
			Password: fields[1],
			GID:      gid,
			Members:  fields[3],
		})
	}
	return entries, nil
}

// FormatGroup сериализует записи обратно в формат group
func FormatGroup(entries []GroupEntry) []byte {
	var buf bytes.Buffer
	for _, e := range entries {
		buf.WriteString(e.String())
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// isRegularGroup возвращает true для root (GID 0), wheel и реальных групп (GID 1000-60000)
func isRegularGroup(name string, gid int) bool {
	if gid == 0 || name == "wheel" {
		return true
	}
	return gid >= 1000 && gid <= 60000
}

// SplitGroup разделяет записи group на /etc (root + wheel + GID 1000-60000) и /usr/lib (остальные)
func SplitGroup(entries []GroupEntry) (etc []GroupEntry, lib []GroupEntry) {
	for _, e := range entries {
		if isRegularGroup(e.Name, e.GID) {
			etc = append(etc, e)
		} else {
			lib = append(lib, e)
		}
	}
	return
}

// MergeGroup объединяет записи из нескольких источников, дедуплицируя по имени
func MergeGroup(sources ...[]GroupEntry) []GroupEntry {
	seen := map[string]int{}
	var result []GroupEntry
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
