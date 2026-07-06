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
// Copyright (C) 2026 The APM Authors
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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

func parseSymbolicMode(s string) (os.FileMode, error) {
	if len(s) != 9 {
		return 0, fmt.Errorf(T_("invalid length: expected 9 characters, got %d"), len(s))
	}

	var mode os.FileMode

	// Bit masks for regular permissions (positions 1–9 in the string)
	bitMasks := []os.FileMode{
		0400, // 1: owner — read
		0200, // 2: owner — write
		0100, // 3: owner — execute
		0040, // 4: group — read
		0020, // 5: group — write
		0010, // 6: group — execute
		0004, // 7: other — read
		0002, // 8: other — write
		0001, // 9: other — execute
	}

	for i := range 9 {
		char := rune(s[i])
		switch char {
		case 'r', 'w', 'x':
			mode |= bitMasks[i]
		}
	}

	// setuid check
	switch rune(s[2]) {
	case 's', 'S':
		mode |= 04000 // setuid
	}

	// setgid check
	switch rune(s[5]) {
	case 's', 'S':
		mode |= 02000 // setgid
	}

	// sticky check
	switch rune(s[8]) {
	case 't', 'T':
		mode |= 01000 // sticky
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
		return parseSymbolicMode(s)
	}

	return 0, errors.New(T_("Wrong permission format"))
}

func Clean(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		// File doesn't exist, do nothing
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
	defer func() { _ = src.Close() }()

	dst, err := os.OpenFile(destPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, perm)
	if err != nil {
		return err
	}
	defer func() { _ = dst.Close() }()

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	return dst.Close()
}

func PrependFile(sourcePath, destPath string, perm fs.FileMode) error {
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	destData, err := os.ReadFile(destPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		destData = nil
	}

	newContent := append(sourceData, destData...)

	err = os.WriteFile(destPath, newContent, perm)
	if err != nil {
		return err
	}

	return nil
}

func Move(sourcePath, destPath string, replace bool) error {
	// Copy the source to the destination
	err := Copy(sourcePath, destPath, replace)
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
	if _, err := os.Stat(destPath); err == nil && replace {
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
	defer func() { _ = sourceFile.Close() }()

	destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, sourceInfo.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Close()
}
