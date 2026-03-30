package altfiles

import (
	"fmt"
	"os"
)

const (
	EtcPasswd   = "/etc/passwd"
	EtcGroup    = "/etc/group"
	EtcNsswitch = "/etc/nsswitch.conf"
	LibPasswd   = "/usr/lib/passwd"
	LibGroup    = "/usr/lib/group"
)

// ApplyResult содержит статистику выполнения
type ApplyResult struct {
	EtcPasswdCount int
	LibPasswdCount int
	EtcGroupCount  int
	LibGroupCount  int
}

// ApplyBuild разделяет passwd/group и патчит nsswitch.conf (для сборки в контейнере)
func ApplyBuild() (*ApplyResult, error) {
	etcPasswd, libPasswd, err := splitPasswdFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to split passwd: %w", err)
	}

	etcGroup, libGroup, err := splitGroupFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to split group: %w", err)
	}

	if err = patchNsswitchFile(); err != nil {
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
func ApplyFix() (*ApplyResult, error) {
	etcPasswdCount, libPasswdCount, err := cleanEtcPasswd()
	if err != nil {
		return nil, fmt.Errorf("failed to clean /etc/passwd: %w", err)
	}

	etcGroupCount, libGroupCount, err := cleanEtcGroup()
	if err != nil {
		return nil, fmt.Errorf("failed to clean /etc/group: %w", err)
	}

	if err = patchNsswitchFile(); err != nil {
		return nil, fmt.Errorf("failed to patch nsswitch.conf: %w", err)
	}

	return &ApplyResult{
		EtcPasswdCount: etcPasswdCount,
		LibPasswdCount: libPasswdCount,
		EtcGroupCount:  etcGroupCount,
		LibGroupCount:  libGroupCount,
	}, nil
}

// cleanEtcPasswd удаляет из /etc/passwd записи, которые уже есть в /usr/lib/passwd
func cleanEtcPasswd() (etcCount int, libCount int, err error) {
	libData, err := os.ReadFile(LibPasswd)
	if err != nil {
		return 0, 0, fmt.Errorf("%s not found, build the image first", LibPasswd)
	}
	libEntries, err := ParsePasswd(libData)
	if err != nil {
		return 0, 0, err
	}

	libNames := make(map[string]struct{}, len(libEntries))
	for _, e := range libEntries {
		libNames[e.Name] = struct{}{}
	}

	etcData, err := os.ReadFile(EtcPasswd)
	if err != nil {
		return 0, 0, err
	}
	etcEntries, err := ParsePasswd(etcData)
	if err != nil {
		return 0, 0, err
	}

	var cleaned []PasswdEntry
	for _, e := range etcEntries {
		if _, inLib := libNames[e.Name]; !inLib {
			cleaned = append(cleaned, e)
		}
	}

	info, err := os.Stat(EtcPasswd)
	if err != nil {
		return 0, 0, err
	}

	return len(cleaned), len(libEntries), os.WriteFile(EtcPasswd, FormatPasswd(cleaned), info.Mode().Perm())
}

// cleanEtcGroup удаляет из /etc/group записи, которые уже есть в /usr/lib/group
func cleanEtcGroup() (etcCount int, libCount int, err error) {
	libData, err := os.ReadFile(LibGroup)
	if err != nil {
		return 0, 0, fmt.Errorf("%s not found, build the image first", LibGroup)
	}
	libEntries, err := ParseGroup(libData)
	if err != nil {
		return 0, 0, err
	}

	libNames := make(map[string]struct{}, len(libEntries))
	for _, e := range libEntries {
		libNames[e.Name] = struct{}{}
	}

	etcData, err := os.ReadFile(EtcGroup)
	if err != nil {
		return 0, 0, err
	}
	etcEntries, err := ParseGroup(etcData)
	if err != nil {
		return 0, 0, err
	}

	var cleaned []GroupEntry
	for _, e := range etcEntries {
		if _, inLib := libNames[e.Name]; !inLib {
			cleaned = append(cleaned, e)
		}
	}

	info, err := os.Stat(EtcGroup)
	if err != nil {
		return 0, 0, err
	}

	return len(cleaned), len(libEntries), os.WriteFile(EtcGroup, FormatGroup(cleaned), info.Mode().Perm())
}

// splitPasswdFiles для сборки: мержит /etc и /usr/lib, сплитит, пишет оба файла
func splitPasswdFiles() (etc []PasswdEntry, lib []PasswdEntry, err error) {
	etcData, err := os.ReadFile(EtcPasswd)
	if err != nil {
		return nil, nil, err
	}
	etcEntries, err := ParsePasswd(etcData)
	if err != nil {
		return nil, nil, err
	}

	var libEntries []PasswdEntry
	if libData, readErr := os.ReadFile(LibPasswd); readErr == nil {
		libEntries, _ = ParsePasswd(libData)
	}

	merged := MergePasswd(etcEntries, libEntries)
	newEtc, newLib := SplitPasswd(merged)

	info, err := os.Stat(EtcPasswd)
	if err != nil {
		return nil, nil, err
	}
	perm := info.Mode().Perm()

	if err = os.WriteFile(LibPasswd, FormatPasswd(newLib), perm); err != nil {
		return nil, nil, err
	}
	if err = os.WriteFile(EtcPasswd, FormatPasswd(newEtc), perm); err != nil {
		return nil, nil, err
	}

	return newEtc, newLib, nil
}

// splitGroupFiles для сборки: мержит /etc и /usr/lib, сплитит, пишет оба файла
func splitGroupFiles() (etc []GroupEntry, lib []GroupEntry, err error) {
	etcData, err := os.ReadFile(EtcGroup)
	if err != nil {
		return nil, nil, err
	}
	etcEntries, err := ParseGroup(etcData)
	if err != nil {
		return nil, nil, err
	}

	var libEntries []GroupEntry
	if libData, readErr := os.ReadFile(LibGroup); readErr == nil {
		libEntries, _ = ParseGroup(libData)
	}

	merged := MergeGroup(etcEntries, libEntries)
	newEtc, newLib := SplitGroup(merged)

	info, err := os.Stat(EtcGroup)
	if err != nil {
		return nil, nil, err
	}
	perm := info.Mode().Perm()

	if err = os.WriteFile(LibGroup, FormatGroup(newLib), perm); err != nil {
		return nil, nil, err
	}
	if err = os.WriteFile(EtcGroup, FormatGroup(newEtc), perm); err != nil {
		return nil, nil, err
	}

	return newEtc, newLib, nil
}

func patchNsswitchFile() error {
	data, err := os.ReadFile(EtcNsswitch)
	if err != nil {
		return err
	}

	info, err := os.Stat(EtcNsswitch)
	if err != nil {
		return err
	}

	patched := PatchNsswitch(data)
	return os.WriteFile(EtcNsswitch, patched, info.Mode().Perm())
}
