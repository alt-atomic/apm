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
	"apm/internal/common/app"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const TransactionKey contextKey = "transaction"

// GenerateTransactionID генерирует уникальный ID транзакции
func GenerateTransactionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
}

// RunCommand выполняет команду и возвращает stdout, stderr и ошибку.
func RunCommand(ctx context.Context, args []string) (string, string, error) {
	app.Log.Debug("run command: ", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// BuildDistroboxArgs строит массив аргументов из commandPrefix и дополнительных аргументов.
func BuildDistroboxArgs(commandPrefix string, args ...string) []string {
	var result []string
	if commandPrefix != "" {
		result = append(result, strings.Fields(commandPrefix)...)
	}
	result = append(result, args...)
	return result
}

// FilterLines фильтрует строки вывода, оставляя только содержащие substr.
func FilterLines(output, substr string) string {
	var result []string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, substr) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

// FilterLinesPrefix фильтрует строки, оставляя начинающиеся с prefix.
func FilterLinesPrefix(output, prefix string) string {
	var result []string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
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

// AppDescription возвращает описание приложения
func AppDescription() string {
	return app.T_("Universal package manager for ALT Linux") + "\n" +
		app.T_("Manages system packages via APT, distrobox containers, atomic images and kernels") + "\n" +
		app.T_("Supports repository management") + "\n" +
		app.T_("Works as CLI tool, D-Bus service (system/session) or HTTP API server") + "\n" +
		app.T_("Output formats: text (default, types: tree/plain via -ft) and json (-f json)")
}

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// FilterDescription генерирует общее описание фильтра для команд.
func FilterDescription(examples string, notes ...string) string {
	result := app.T_("Filtering for the list method:") + "\n\n" +
		app.T_("Format: key[op]=value or key=value (each field has a default operator)") + "\n\n" +
		app.T_("Available operators:") + "\n" +
		"  eq       - " + app.T_("exact match (=)") + "\n" +
		"  ne       - " + app.T_("not equal (<>)") + "\n" +
		"  like     - " + app.T_("pattern match (LIKE), use % as wildcard") + "\n" +
		"  gt       - " + app.T_("greater than (>)") + "\n" +
		"  gte      - " + app.T_("greater than or equal (>=)") + "\n" +
		"  lt       - " + app.T_("less than (<)") + "\n" +
		"  lte      - " + app.T_("less than or equal (<=)") + "\n" +
		"  contains - " + app.T_("contains value (for JSON/array fields)") + "\n\n" +
		app.T_("OR: use \"|\" to combine values: key[op]=value1|value2") + "\n\n" +
		app.T_("Examples:") + "\n" +
		"  " + examples
	for _, note := range notes {
		result += "\n\n" + note
	}
	return result
}
