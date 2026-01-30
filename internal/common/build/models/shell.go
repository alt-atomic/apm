package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

type ShellBody struct {
	// Команды на выполнение
	Command string `yaml:"command,omitempty" json:"command,omitempty" required:""`

	// Quiet command output
	Quiet bool `yaml:"quiet,omitempty" json:"quiet,omitempty"`
}

func (b *ShellBody) Execute(ctx context.Context, _ Service) (any, error) {
	app.Log.Debug(fmt.Sprintf("Executing `%s`", b.Command))

	if err := osutils.ExecSh(ctx, b.Command, "", b.Quiet); err != nil {
		return nil, err
	}

	return nil, nil
}

func (b *ShellBody) Hash(baseDir string, env map[string]string) string {
	h := hashWithEnv(b, env)

	// Раскрываем env переменные в command
	resolvedCommand := resolveEnvPlaceholders(b.Command, env)

	// Ищем файлы скриптов в command и хэшируем их содержимое
	words := strings.Fields(resolvedCommand)
	for _, word := range words {
		// Пропускаем флаги и переменные
		if strings.HasPrefix(word, "-") || strings.HasPrefix(word, "$") {
			continue
		}

		// Проверяем пути к скриптам (.sh файлы или пути с /)
		if strings.HasSuffix(word, ".sh") || strings.Contains(word, "/") {
			scriptPath := word
			if !filepath.IsAbs(word) {
				scriptPath = filepath.Join(baseDir, word)
			}
			if fileHash, err := hashPath(scriptPath); err == nil {
				combined := h + fileHash
				h = hashJSON(combined)
			}
		}
	}

	return h
}
