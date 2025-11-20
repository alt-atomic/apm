// SPDX-License-Identifier: GPL-3.0-or-later
//
// This file was originally part of the project "LURE - Linux User REpository",
// created by Elara Musayelyan.
// It was later modified as part of "ALR - Any Linux Repository" by the ALR Authors.
// This version has been further modified as part of "Stapler" by Maxim Slipenko and other Stapler Authors.
//
// Copyright (C) Elara Musayelyan (LURE)
// Copyright (C) 2025 The ALR Authors
// Copyright (C) 2025 The Stapler Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package osutils

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

func parseSymbolicMode(s string) (os.FileMode, error) {
	if len(s) != 9 {
		return 0, fmt.Errorf("expected 9 characters, got %d", len(s))
	}

	var mode os.FileMode

	// Сопоставляем позиции и биты
	for i, char := range s {
		switch i {
		case 0:
			if char == 'r' {
				mode |= 0400
			} // владелец: чтение
		case 1:
			if char == 'w' {
				mode |= 0200
			} // владелец: запись
		case 2:
			if char == 'x' {
				mode |= 0100
			} // владелец: исполнение
		case 3:
			if char == 'r' {
				mode |= 0040
			} // группа: чтение
		case 4:
			if char == 'w' {
				mode |= 0020
			} // группа: запись
		case 5:
			if char == 'x' {
				mode |= 0010
			} // группа: исполнение
		case 6:
			if char == 'r' {
				mode |= 0004
			} // другие: чтение
		case 7:
			if char == 'w' {
				mode |= 0002
			} // другие: запись
		case 8:
			if char == 'x' {
				mode |= 0001
			} // другие: исполнение
		}
	}

	return mode, nil
}

func IsURL(str string) bool {
	u, err := url.Parse(str)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}

func StringToFileMode(s string) (os.FileMode, error) {
	if len(s) == 9 {
		mode, err := parseSymbolicMode(s)
		if err != nil {
			return 0, err
		}
		return mode, nil
	}

	mode, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(mode), nil
}

func Clean(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		// File doesn't exists, do nothing
		return nil
	}

	switch {
	case info.IsDir():
		return cleanDir(path)
	case info.Mode().IsRegular():
		return cleanFile(path)
	default:
		return nil
	}
}

func cleanFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	return os.WriteFile(path, []byte(""), info.Mode().Perm())
}

func cleanDir(dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		err = os.RemoveAll(path.Join(dir, file.Name()))
		if err != nil {
			return err
		}
	}

	return nil
}

func GetEnvMap() map[string]string {
	envMap := make(map[string]string)
	for _, envLine := range os.Environ() {
		parts := strings.SplitN(envLine, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	return envMap
}

func IsExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func AppendFile(sourcePath, destPath string, perm fs.FileMode) error {
	src, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, perm)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	return nil
}

func ExecShOutput(ctx context.Context, command string, chDir string, std bool) (string, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	if chDir != "" {
		cmd.Dir = chDir
	}
	if std {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	var out, err = cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.Trim(string(out), "\n"), nil
}

func ExecSh(ctx context.Context, command string, chDir string, std bool, quite bool) error {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	if chDir != "" {
		cmd.Dir = chDir
	}
	if std {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// Move attempts to use os.Rename and if that fails (such as for a cross-device move),
// it instead copies the source to the destination and then removes the source.
func Move(sourcePath, destPath string, replace bool) error {
	if IsExists(destPath) && replace {
		err := os.RemoveAll(destPath)
		if err != nil {
			return err
		}
	}

	// Try to rename the source to the destination
	err := os.Rename(sourcePath, destPath)
	if err == nil {
		return nil // Successful move
	}

	// Rename failed, so copy the source to the destination
	err = Copy(sourcePath, destPath, replace)
	if err != nil {
		return err
	}

	// Copy successful, remove the original source
	err = os.RemoveAll(sourcePath)
	if err != nil {
		return err
	}

	return nil
}

func Copy(sourcePath, destPath string, replace bool) error {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	switch {
	case sourceInfo.IsDir():
		return copyDir(sourcePath, destPath, replace)
	case sourceInfo.Mode().IsRegular():
		return copyFile(sourcePath, destPath, replace)
	default:
		return nil
	}
}

func copyDir(sourcePath, destPath string, replace bool) error {
	if IsExists(destPath) && replace {
		err := os.RemoveAll(destPath)
		if err != nil {
			return err
		}
	}

	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	err = os.MkdirAll(destPath, sourceInfo.Mode())
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		sourceEntry := filepath.Join(sourcePath, entry.Name())
		destEntry := filepath.Join(destPath, entry.Name())

		err = Copy(sourceEntry, destEntry, replace)
		if err != nil {
			return err
		}
	}

	return nil
}

func copyFile(sourcePath, destPath string, replace bool) error {
	if IsExists(destPath) && replace {
		err := os.RemoveAll(destPath)
		if err != nil {
			return err
		}
	}

	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, sourceInfo.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return nil
}
