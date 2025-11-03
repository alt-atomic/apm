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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"unicode"
)

func IsExists(path string) bool {
	_, err := os.Stat(path)
	return os.IsExist(err)
}

func Capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func AppendFile(sourcePath, destPath string) error {
	src, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_APPEND|os.O_WRONLY, 0644)
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

func ExecSh(command string, chDir string) error {
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = chDir
	_, err := cmd.Output()
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
