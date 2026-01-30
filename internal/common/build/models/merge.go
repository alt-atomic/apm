package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
)

type MergeBody struct {
	// Путь до файла, содеримое которого нужно взять
	Source string `yaml:"source,omitempty" json:"source,omitempty" required:""`

	// Путь до файла, куда нужно добавить содержимое
	Destination string `yaml:"destination,omitempty" json:"destination,omitempty" required:""`

	// Права для создания файла в формате rwxrwxrwx, если он не существует
	CreateFilePerm string `yaml:"create-file-perm,omitempty" json:"create-file-perm,omitempty"`

	// Добавлять ли содержимое в начало файла
	Prepend bool `yaml:"prepend,omitempty" json:"prepend,omitempty"`
}

func (b *MergeBody) Execute(_ context.Context, _ Service) (any, error) {
	app.Log.Info(fmt.Sprintf("Merging %s with %s", b.Source, b.Destination))

	if !filepath.IsAbs(b.Destination) {
		return nil, fmt.Errorf("destination in merge type must be absolute path")
	}

	var mode fs.FileMode = 0
	if b.CreateFilePerm != "" {
		var err error
		mode, err = osutils.StringToFileMode(b.CreateFilePerm)
		if err != nil {
			return nil, err
		}
	}

	if b.Prepend {
		return nil, osutils.PrependFile(b.Source, b.Destination, mode)
	} else {
		return nil, osutils.AppendFile(b.Source, b.Destination, mode)
	}
}

func (b *MergeBody) Hash(baseDir string, env map[string]string) string {
	h := hashWithEnv(b, env)

	// Раскрываем env переменные в путях
	resolvedSource := resolveEnvPlaceholders(b.Source, env)

	// Определяем путь к source
	sourcePath := resolvedSource
	if !filepath.IsAbs(resolvedSource) {
		sourcePath = filepath.Join(baseDir, resolvedSource)
	}

	// Хэшируем содержимое файла если он существует
	if fileHash, err := hashPath(sourcePath); err == nil {
		combined := h + fileHash
		return hashJSON(combined)
	}

	return h
}
