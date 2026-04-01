package lint

import (
	"apm/internal/common/reply"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

var containerRuntimeFiles = []string{
	"/run/.containerenv",
	"/run/systemd/resolve/stub-resolv.conf",
}

var ignoredPrefixes = []string{
	"/run/secrets",
	"/tmp/apm-src",
	"/tmp/go-cache",
	"/tmp/go-build",
	"/tmp/go-mod",
}

var runtimeOnlyDirs = []string{"run", "tmp"}

type runTmpAnalysis struct {
	Unexpected []string
}

func (a *runTmpAnalysis) Analyze(ctx context.Context, rootfs string) error {
	reply.CreateEventNotification(ctx, reply.StateBefore, reply.WithEventName(reply.EventSystemLintRunTmp))
	defer reply.CreateEventNotification(ctx, reply.StateAfter, reply.WithEventName(reply.EventSystemLintRunTmp))
	for _, dir := range runtimeOnlyDirs {
		dirPath := filepath.Join(rootfs, dir)
		if _, err := os.Lstat(dirPath); os.IsNotExist(err) {
			continue
		}

		paths, err := collectPaths(rootfs, dirPath)
		if err != nil {
			return fmt.Errorf("walking /%s: %w", dir, err)
		}

		paths = pruneKnownPaths(paths, containerRuntimeFiles)
		for _, p := range paths {
			if !hasIgnoredPrefix(p) {
				a.Unexpected = append(a.Unexpected, p)
			}
		}
	}

	sort.Strings(a.Unexpected)
	return nil
}

func hasIgnoredPrefix(path string) bool {
	for _, prefix := range ignoredPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func collectPaths(rootfs, dir string) ([]string, error) {
	var paths []string

	var dirSt syscall.Stat_t
	_ = syscall.Lstat(dir, &dirSt)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsPermission(err) || os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())

		if isMount(fullPath, dirSt.Dev) {
			continue
		}

		relPath, err := filepath.Rel(rootfs, fullPath)
		if err != nil {
			return nil, err
		}

		absPath := "/" + relPath
		paths = append(paths, absPath)

		if entry.Type().IsDir() {
			sub, err := collectPaths(rootfs, fullPath)
			if err != nil {
				return nil, err
			}
			paths = append(paths, sub...)
		}
	}
	return paths, nil
}

func isMount(path string, parentDev uint64) bool {
	var st syscall.Stat_t
	if err := syscall.Lstat(path, &st); err != nil {
		return false
	}
	return st.Dev != parentDev
}

func pruneKnownPaths(paths []string, known []string) []string {
	remove := make(map[string]bool, len(known))
	for _, kf := range known {
		remove[kf] = true
	}

	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	for _, kf := range known {
		if !pathSet[kf] {
			continue
		}
		for dir := filepath.Dir(kf); dir != "/" && dir != "."; dir = filepath.Dir(dir) {
			hasOther := false
			for _, p := range paths {
				if p != dir && filepath.Dir(p) == dir && !remove[p] {
					hasOther = true
					break
				}
			}
			if hasOther {
				break
			}
			remove[dir] = true
		}
	}

	var result []string
	for _, p := range paths {
		if !remove[p] {
			result = append(result, p)
		}
	}
	return result
}
