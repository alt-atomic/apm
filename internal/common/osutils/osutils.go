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
	"apm/internal/common/app"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

func parseSymbolicMode(s string) (os.FileMode, error) {
	if len(s) != 9 {
		return 0, fmt.Errorf(app.T_("invalid length: expected 9 characters, got %d"), len(s))
	}

	var mode os.FileMode

	// Битовые маски для обычных прав (позиции 1–9 в строке, индексы 1–9 в s)
	bitMasks := []os.FileMode{
		0400, // 1: owner — чтение
		0200, // 2: owner — запись
		0100, // 3: owner — исполнение
		0040, // 4: group — чтение
		0020, // 5: group — запись
		0010, // 6: group — исполнение
		0004, // 7: other — чтение
		0002, // 8: other — запись
		0001, // 9: other — исполнение
	}

	for i := range 9 {
		char := rune(s[i])
		switch char {
		case 'r', 'w', 'x':
			mode |= bitMasks[i]
		}
	}

	// Проверка setuid
	switch rune(s[2]) {
	case 's', 'S':
		mode |= 04000 // setuid
	}

	// Проверка setgid
	switch rune(s[5]) {
	case 's', 'S':
		mode |= 02000 // setgid
	}

	// Проверка sticky
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

	return 0, errors.New(app.T_("Wrong permission format"))
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

type Writer struct {
	RealWriter       io.Writer
	RealOutputWriter io.Writer
	Divider          string
	dividerPassed    bool
}

func (w *Writer) Write(p []byte) (n int, err error) {
	if w.dividerPassed {
		n, err = w.RealOutputWriter.Write(p)
		if err != nil {
			return
		}
	} else {
		if strings.Contains(string(p), w.Divider) {
			w.dividerPassed = true

			parts := strings.SplitN(string(p), w.Divider, 2)

			n, err = w.RealWriter.Write([]byte(parts[0]))
			if err != nil {
				return
			}
			n, err = w.RealOutputWriter.Write([]byte(parts[1]))
			if err != nil {
				return
			}
			n = len(p)
			err = nil

		} else {
			n, err = w.RealWriter.Write(p)
			if err != nil {
				return
			}
		}
	}

	return
}

func ExecShWithDivider(
	ctx context.Context,
	command string,
	commandOutput string,
	divider string,
	quiet bool,
) (string, string, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", "set -e\n"+command+fmt.Sprintf("\necho '%s'\n%s", divider, commandOutput))

	// Если нужен вывод в консоль И в переменную
	var cmdout bytes.Buffer
	var cmdoutOutput bytes.Buffer
	if quiet {
		cmd.Stdout = &Writer{
			RealWriter:       &cmdout,
			RealOutputWriter: &cmdoutOutput,
			Divider:          divider,
		}
	} else {
		cmd.Stdout = &Writer{
			RealWriter:       io.MultiWriter(os.Stdout, &cmdout),
			RealOutputWriter: &cmdoutOutput,
			Divider:          divider,
		}
	}

	cmd.Stderr = os.Stderr
	err := cmd.Run()
	result := cmdout.Bytes()
	resultOutput := cmdoutOutput.Bytes()

	if cmd.ProcessState.ExitCode() != 0 {
		return "", "", fmt.Errorf("command '%s' failed with exit code %d", command, cmd.ProcessState.ExitCode())
	}

	if err != nil {
		return "", "", err
	}

	return string(result), string(resultOutput), nil
}

func ExecShWithOutput(
	ctx context.Context,
	command string,
	chDir string,
	quiet bool,
) (string, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	if chDir != "" {
		cmd.Dir = chDir
	}

	// Если нужен вывод в консоль И в переменную
	var stdout bytes.Buffer
	if quiet {
		cmd.Stdout = io.MultiWriter(&stdout)
	} else {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	}

	cmd.Stderr = os.Stderr
	err := cmd.Run()
	output := stdout.Bytes()

	if cmd.ProcessState.ExitCode() != 0 {
		return "", fmt.Errorf("command '%s' failed with exit code %d", command, cmd.ProcessState.ExitCode())
	}

	if err != nil {
		return "", err
	}

	return string(output), nil
}

// Copies the source to the destination and then removes the source.
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
