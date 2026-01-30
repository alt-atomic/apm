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

package core

import (
	"apm/internal/common/build/models"
	"apm/internal/common/osutils"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// FlatModule представляет раскрытый модуль с контекстом
type FlatModule struct {
	Module     Module
	SourceFile string
	BaseDir    string
	Env        map[string]string
}

// FlattenModules рекурсивно раскрывает все include модули в плоский список
func FlattenModules(modules []Module, baseDir string, sourceFile string) ([]FlatModule, error) {
	return flattenModulesWithEnv(modules, baseDir, sourceFile, nil)
}

// flattenModulesWithEnv рекурсивно раскрывает модули с накоплением env контекста
func flattenModulesWithEnv(modules []Module, baseDir string, sourceFile string, parentEnv map[string]string) ([]FlatModule, error) {
	var result []FlatModule

	for _, module := range modules {
		if module.Type == TypeInclude {
			includeBody, ok := module.Body.(*models.IncludeBody)
			if !ok {
				continue
			}

			for _, target := range includeBody.Targets {
				subModules, subBaseDir, subEnv, err := loadIncludeTargetWithEnv(target, baseDir)
				if err != nil {
					return nil, err
				}

				// Накапливаем env: parent -> include file env
				mergedEnv := mergeEnv(parentEnv, subEnv)

				flat, err := flattenModulesWithEnv(subModules, subBaseDir, target, mergedEnv)
				if err != nil {
					return nil, err
				}

				result = append(result, flat...)
			}
		} else {
			result = append(result, FlatModule{
				Module:     module,
				SourceFile: sourceFile,
				BaseDir:    baseDir,
				Env:        parentEnv,
			})
		}
	}

	return result, nil
}

// mergeEnv объединяет env maps, второй переопределяет первый
func mergeEnv(base, override map[string]string) map[string]string {
	if base == nil && override == nil {
		return nil
	}
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}

// loadIncludeTargetWithEnv загружает модули и env из target (файл, директория или URL)
func loadIncludeTargetWithEnv(target string, currentBaseDir string) ([]Module, string, map[string]string, error) {
	if osutils.IsURL(target) {
		cfg, err := readAndParseConfigYamlUrl(target)
		if err != nil {
			return nil, "", nil, err
		}
		return cfg.Modules, currentBaseDir, cfg.Env, nil
	}

	targetPath := target
	if !filepath.IsAbs(target) {
		targetPath = filepath.Join(currentBaseDir, target)
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, "", nil, err
	}

	if info.IsDir() {
		return loadIncludeDirWithEnv(targetPath)
	}

	return loadIncludeFileWithEnv(targetPath)
}

// loadIncludeFileWithEnv загружает модули и env из файла
func loadIncludeFileWithEnv(filePath string) ([]Module, string, map[string]string, error) {
	cfg, err := ReadAndParseConfigYamlFile(filePath)
	if err != nil {
		return nil, "", nil, err
	}

	baseDir := filepath.Dir(filePath)
	return cfg.Modules, baseDir, cfg.Env, nil
}

// loadIncludeDirWithEnv загружает модули из всех yml файлов в директории
// Env из разных файлов объединяются
func loadIncludeDirWithEnv(dirPath string) ([]Module, string, map[string]string, error) {
	var allModules []Module
	var allEnv map[string]string

	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, "", nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}

		filePath := filepath.Join(dirPath, name)
		cfg, err := ReadAndParseConfigYamlFile(filePath)
		if err != nil {
			return nil, "", nil, err
		}

		allModules = append(allModules, cfg.Modules...)
		allEnv = mergeEnv(allEnv, cfg.Env)
	}

	return allModules, dirPath, allEnv, nil
}

// readAndParseConfigYamlUrl загружает и парсит конфиг из URL
func readAndParseConfigYamlUrl(url string) (Config, error) {
	resp, err := http.Get(url)
	if err != nil {
		return Config{}, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Config{}, err
	}

	return ParseYamlConfigData(data)
}
