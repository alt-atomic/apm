package lint

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const factoryDir = "usr/share/factory"

// WriteFactory копирует файлы C-записей в rootfs/usr/share/factory с сохранением прав и владельца.
func (a *tmpFilesAnalysis) WriteFactory(rootfs string) ([]string, error) {
	var written []string
	for _, e := range a.Missing {
		if e.Type != "C" {
			continue
		}
		dst := filepath.Join(rootfs, factoryDir, e.Path)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return written, fmt.Errorf("creating factory dir for %s: %w", e.Path, err)
		}
		if err := copyFactoryFile(filepath.Join(rootfs, e.Path), dst); err != nil {
			return written, fmt.Errorf("copying %s to factory: %w", e.Path, err)
		}
		written = append(written, e.Path)
	}
	return written, nil
}

func copyFactoryFile(srcPath, dst string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	info, err := src.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, src); err != nil {
		_ = out.Close()
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	// chown до chmod: ядро сбрасывает suid/sgid при смене владельца
	uid, gid := ownerIDs(info)
	if err = os.Chown(dst, uid, gid); err != nil && os.Geteuid() == 0 {
		return err
	}
	if err = os.Chmod(dst, info.Mode()); err != nil {
		return err
	}
	return nil
}

// CleanFactory удаляет из factory только файлы, записанные прошлым apm-lint.conf.
func (a *tmpFilesAnalysis) CleanFactory(rootfs string) error {
	conf := filepath.Join(rootfs, "usr", "lib", "tmpfiles.d", confName)
	f, err := os.Open(conf)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] != 'C' {
			continue
		}
		p := extractPath(line)
		if p == "" {
			continue
		}
		target := filepath.Join(rootfs, factoryDir, p)
		if err = os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing factory file %s: %w", target, err)
		}
		pruneEmptyDirs(filepath.Dir(target), filepath.Join(rootfs, factoryDir))
	}
	return scanner.Err()
}

func pruneEmptyDirs(dir, stop string) {
	for dir != stop && strings.HasPrefix(dir, stop) {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
