// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
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

package helper

import (
	"apm/lib"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const TransactionKey contextKey = "transaction"

// RunCommand выполняет команду и возвращает stdout, stderr и ошибку.
func RunCommand(ctx context.Context, command string) (string, string, error) {
	lib.Log.Debug("run command: ", command)
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// IsRunningInContainer проверка, запущен ли apm внутри контейнера
func IsRunningInContainer() bool {
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	if len(os.Getenv("container")) > 0 {
		return true
	}

	return false
}

func IsRegularFileAndIsPackage(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".rpm" && ext != ".deb" {
		return false, nil
	}
	return true, nil
}

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
