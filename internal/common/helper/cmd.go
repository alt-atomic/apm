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
	"reflect"
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

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ClearCLIHiddenFields обнуляет поля структуры с тегом cli:"hidden".
func ClearCLIHiddenFields(v interface{}) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).Tag.Get("cli") == "hidden" {
			rv.Field(i).Set(reflect.Zero(rt.Field(i).Type))
		}
	}
}
