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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// FlatModule представляет раскрытый модуль с контекстом
type FlatModule struct {
	Module     Module
	SourceFile string
	BaseDir    string
}

// FlattenModules рекурсивно раскрывает все include модули в плоский список
func FlattenModules(modules []Module, baseDir string, sourceFile string) ([]FlatModule, error) {
	var result []FlatModule

	for _, module := range modules {
		if module.Type == TypeInclude {
			includeBody, ok := module.Body.(*models.IncludeBody)
			if !ok {
				continue
			}

			for _, target := range includeBody.Targets {
				subModules, subBaseDir, err := loadIncludeTarget(target, baseDir)
				if err != nil {
					return nil, err
				}

				flat, err := FlattenModules(subModules, subBaseDir, target)
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
			})
		}
	}

	return result, nil
}

// loadIncludeTarget загружает модули из target (файл, директория или URL)
func loadIncludeTarget(target string, currentBaseDir string) ([]Module, string, error) {
	if osutils.IsURL(target) {
		modules, err := ReadAndParseModulesYamlUrl(target)
		if err != nil {
			return nil, "", err
		}
		return *modules, currentBaseDir, nil
	}

	targetPath := target
	if !filepath.IsAbs(target) {
		targetPath = filepath.Join(currentBaseDir, target)
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, "", err
	}

	if info.IsDir() {
		return loadIncludeDir(targetPath)
	}

	return loadIncludeFile(targetPath)
}

// loadIncludeFile загружает модули из файла
func loadIncludeFile(filePath string) ([]Module, string, error) {
	modules, err := ReadAndParseModules(filePath)
	if err != nil {
		return nil, "", err
	}

	baseDir := filepath.Dir(filePath)
	return *modules, baseDir, nil
}

// loadIncludeDir загружает модули из всех yml файлов в директории
func loadIncludeDir(dirPath string) ([]Module, string, error) {
	var allModules []Module

	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, "", err
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
		modules, err := ReadAndParseModules(filePath)
		if err != nil {
			return nil, "", err
		}

		allModules = append(allModules, *modules...)
	}

	return allModules, dirPath, nil
}

// HasModuleDependencies проверяет использует ли модуль ${{ Modules. }}
func HasModuleDependencies(module Module) bool {
	data, err := json.Marshal(module)
	if err != nil {
		return true
	}
	str := string(data)
	return strings.Contains(str, "${{ Modules.") || strings.Contains(str, "${{Modules.")
}

// HasCondition проверяет есть ли у модуля условие if
func HasCondition(module Module) bool {
	return module.If != ""
}

// IsCacheable проверяет можно ли кэшировать модуль
func IsCacheable(module Module) bool {
	// Не кэшируем если есть условия или зависимости от других модулей
	if HasCondition(module) {
		return false
	}
	if HasModuleDependencies(module) {
		return false
	}
	return true
}
