package icon

import (
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"apm/cmd/common/helper"
	"apm/lib"
)

// SwCatIconService — сервис для работы с XML-файлами SWCatalog.
type SwCatIconService struct {
	path          string
	containerName string
}

// NewSwCatIconService — конструктор сервиса.
func NewSwCatIconService(path string, containerName string) *SwCatIconService {
	return &SwCatIconService{
		path:          path,
		containerName: containerName,
	}
}

// Component – исходная структура из XML.
type Component struct {
	XMLName xml.Name `xml:"component"`
	PkgName string   `xml:"pkgname"`
	Icons   []Icon   `xml:"icon"`
}

// Icon – структура для иконок.
type Icon struct {
	Type   string `xml:"type,attr" json:"type"`
	Width  int    `xml:"width,attr" json:"width"`
	Height int    `xml:"height,attr" json:"height"`
	Value  string `xml:",chardata" json:"value"`
}

// SWCatalog – структура, соответствующая корневому элементу XML.
type SWCatalog struct {
	XMLName    xml.Name    `xml:"components"`
	Components []Component `xml:"component"`
}

// PackageIconsSwCat – итоговая структура для каждого пакета.
type PackageIconsSwCat struct {
	PkgName string `json:"pkgName"`
	Icons   []Icon `json:"icons"`
}

// copyDirFromContainer копирует каталог src из контейнера в dst на хосте.
func (s *SwCatIconService) copyDirFromContainer(src, dst string) error {
	// Создаем целевую директорию на хосте.
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	// Команда копирования из контейнера.
	cmdStr := fmt.Sprintf("%s distrobox enter %s -- cp -r %s/. %s", lib.Env.CommandPrefix, s.containerName, src, dst)
	_, stderr, err := helper.RunCommand(cmdStr)
	if err != nil {
		return fmt.Errorf("ошибка копирования из контейнера: %v, stderr: %s", err, stderr)
	}
	return nil
}

// prepareTempIconDirs копирует исходные каталоги с иконками из контейнера во временную директорию на хосте.
func (s *SwCatIconService) prepareTempIconDirs(cachedSource, stockSource string) (tempCached, tempStock string, cleanup func(), err error) {
	// Если контейнер не указан, возвращаем переданные пути.
	if s.containerName == "" {
		return cachedSource, stockSource, func() {}, nil
	}
	// Формируем базовую временную директорию на хосте, например: /tmp/apm/<container>
	baseTemp := filepath.Join("/tmp/apm", s.containerName)
	// Добавляем дополнительный уровень "icons" для конечного пути.
	tempCached = filepath.Join(baseTemp, "cached", "icons")
	if len(stockSource) > 0 {
		tempStock = filepath.Join(baseTemp, "stock", "icons")
	} else {
		tempStock = ""
	}

	// Создаем временные директории.
	if err = os.MkdirAll(tempCached, 0755); err != nil {
		return "", "", nil, err
	}
	if tempStock != "" {
		if err = os.MkdirAll(tempStock, 0755); err != nil {
			return "", "", nil, err
		}
	}

	// Копируем каталоги из контейнера во временные.
	if err = s.copyDirFromContainer(cachedSource, tempCached); err != nil {
		return "", "", nil, err
	}
	if len(stockSource) > 0 {
		if err = s.copyDirFromContainer(stockSource, tempStock); err != nil {
			return "", "", nil, err
		}
	}

	cleanup = func() {
		err = os.RemoveAll(filepath.Join("/tmp", "apm"))
		if err != nil {
			lib.Log.Error(err)
		}
	}
	return tempCached, tempStock, cleanup, nil
}

// listSubdirs возвращает список поддиректорий в заданном каталоге.
// Так как во временной директории на хосте мы уже работаем локально, используется os.ReadDir.
func (s *SwCatIconService) listSubdirs(base string) ([]string, error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(base, entry.Name()))
		}
	}
	return dirs, nil
}

// tryReadFile читает файл локально (во временной директории на хосте).
func (s *SwCatIconService) tryReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// searchFileInDirs ищет файл fileName в поддиректориях верхнего уровня каталога baseDir.
// Если resolution не пустой, ищет только в поддиректории с этим именем (например, "128x128").
func (s *SwCatIconService) searchFileInDirs(baseDir, resolution, fileName string) (string, error) {
	topDirs, err := s.listSubdirs(baseDir)
	if err != nil {
		return "", err
	}
	for _, topDir := range topDirs {
		var searchDir string
		if resolution != "" {
			searchDir = filepath.Join(topDir, resolution)
		} else {
			searchDir = topDir
		}
		fullPath := filepath.Join(searchDir, fileName)
		if _, err = os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}
	return "", fmt.Errorf("файл %s не найден в %s", fileName, baseDir)
}

// getIconFromPackage пытается получить содержимое иконки для заданного пакета.
// Приоритет: cached (сначала 128x128, затем любая) -> stock -> remote.
func (s *SwCatIconService) getIconFromPackage(pkg PackageIconsSwCat, cachedIconsBase string, stockIconsBase string) ([]byte, error) {
	readFile := func(path string) ([]byte, error) {
		return s.tryReadFile(path)
	}

	// 1. Ищем cached-иконку с разрешением 128x128.
	for _, icon := range pkg.Icons {
		if strings.ToLower(icon.Type) == "cached" && icon.Width == 128 && icon.Height == 128 {
			fileName := strings.TrimSpace(icon.Value)
			path, err := s.searchFileInDirs(cachedIconsBase, fmt.Sprintf("%dx%d", 128, 128), fileName)
			if err == nil {
				return readFile(path)
			}
		}
	}

	// 2. Ищем любую cached-иконку.
	for _, icon := range pkg.Icons {
		if strings.ToLower(icon.Type) == "cached" {
			fileName := strings.TrimSpace(icon.Value)
			path, err := s.searchFileInDirs(cachedIconsBase, "", fileName)
			if err == nil {
				return readFile(path)
			}
		}
	}

	// 3. Если cached не найдены, ищем stock-иконку.
	if stockIconsBase != "" {
		for _, icon := range pkg.Icons {
			if strings.ToLower(icon.Type) == "stock" {
				baseName := strings.TrimSpace(icon.Value)
				// Пробуем PNG.
				path, err := s.searchFileInDirs(stockIconsBase, "", baseName+".png")
				if err == nil {
					return readFile(path)
				}
				// Пробуем SVG.
				path, err = s.searchFileInDirs(stockIconsBase, "", baseName+".svg")
				if err == nil {
					return readFile(path)
				}
			}
		}
	}

	// 4. Если stock не найдены, ищем remote-иконку.
	for _, icon := range pkg.Icons {
		if strings.ToLower(icon.Type) == "remote" {
			url := strings.TrimSpace(icon.Value)
			resp, err := http.Get(url)
			if err != nil {
				continue
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				continue
			}
			data, err := io.ReadAll(resp.Body)
			if err == nil {
				return data, nil
			}
		}
	}

	return nil, fmt.Errorf("иконка не найдена для пакета %s", pkg.PkgName)
}

// LoadSWCatalogs загружает все XML-файлы из директории, объединяет данные по пакетам (без дублей),
// распаковывает файлы с расширением .gz, выводит результат в консоль и возвращает срез PackageIconsSwCat.
func (s *SwCatIconService) LoadSWCatalogs() ([]PackageIconsSwCat, error) {
	var allComponents []Component
	var fileNames []string

	// Получаем список имен файлов из s.path.
	if s.containerName == "" {
		entries, err := os.ReadDir(s.path)
		if err != nil {
			return nil, fmt.Errorf("не удалось прочитать директорию %s: %w", s.path, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				fileNames = append(fileNames, entry.Name())
			}
		}
	} else {
		// Для контейнера используем команду find для файлов 1-го уровня.
		cmdStr := fmt.Sprintf("%s distrobox enter %s -- find %s -maxdepth 1 -type f", lib.Env.CommandPrefix, s.containerName, s.path)
		stdout, stderr, err := helper.RunCommand(cmdStr)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения файлов в %s: %v, stderr: %s", s.path, err, stderr)
		}
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		for _, line := range lines {
			if line != "" {
				fileNames = append(fileNames, filepath.Base(line))
			}
		}
	}

	// Обрабатываем каждый XML-файл.
	for _, fileName := range fileNames {
		if !(strings.HasSuffix(fileName, ".xml") || strings.HasSuffix(fileName, ".xml.gz")) {
			continue
		}
		fullPath := filepath.Join(s.path, fileName)
		var data []byte
		var err error
		if s.containerName == "" {
			data, err = os.ReadFile(fullPath)
			if err != nil {
				return nil, fmt.Errorf("не удалось прочитать файл %s: %w", fullPath, err)
			}
		} else {
			cmdStr := fmt.Sprintf("%s distrobox enter %s -- cat %s", lib.Env.CommandPrefix, s.containerName, fullPath)
			stdout, stderr, err := helper.RunCommand(cmdStr)
			if err != nil {
				return nil, fmt.Errorf("ошибка выполнения команды для файла %s: %v, stderr: %s", fullPath, err, stderr)
			}
			data = []byte(stdout)
		}
		if strings.HasSuffix(fileName, ".gz") {
			data, err = s.decompressGzip(data)
			if err != nil {
				return nil, fmt.Errorf("не удалось распаковать файл %s: %w", fullPath, err)
			}
		}
		var catalog SWCatalog
		err = xml.Unmarshal(data, &catalog)
		if err != nil {
			return nil, fmt.Errorf("ошибка парсинга XML файла %s: %w", fullPath, err)
		}
		allComponents = append(allComponents, catalog.Components...)
	}

	// Объединяем компоненты по PkgName.
	pkgMap := make(map[string][]Icon)
	for _, comp := range allComponents {
		if len(comp.Icons) > 0 {
			pkgMap[comp.PkgName] = append(pkgMap[comp.PkgName], comp.Icons...)
		}
	}

	var packages []PackageIconsSwCat
	for pkgName, icons := range pkgMap {
		uniqueIcons := s.removeDuplicateIcons(icons)
		var filteredIcons []Icon
		var hasCached bool
		for _, icon := range uniqueIcons {
			if strings.ToLower(icon.Type) == "cached" {
				hasCached = true
				break
			}
		}
		if hasCached {
			for _, icon := range uniqueIcons {
				if strings.ToLower(icon.Type) == "cached" {
					filteredIcons = append(filteredIcons, icon)
				}
			}
		} else {
			filteredIcons = uniqueIcons
		}
		packages = append(packages, PackageIconsSwCat{PkgName: pkgName, Icons: filteredIcons})
	}

	return packages, nil
}

// decompressGzip распаковывает данные из среза байт.
func (s *SwCatIconService) decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// removeDuplicateIcons удаляет дубликаты иконок внутри одного пакета.
func (s *SwCatIconService) removeDuplicateIcons(icons []Icon) []Icon {
	seen := make(map[string]bool)
	var result []Icon
	for _, icon := range icons {
		key := fmt.Sprintf("%s-%d-%d-%s", icon.Type, icon.Width, icon.Height, icon.Value)
		if !seen[key] {
			seen[key] = true
			result = append(result, icon)
		}
	}
	return result
}
