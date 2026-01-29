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

package build

import (
	"apm/internal/common/app"
	"apm/internal/common/build/common_types"
	"apm/internal/common/build/core"
	"apm/internal/common/build/models"
	"apm/internal/common/osutils"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BuildahOptions опции для сборки через buildah
type BuildahOptions struct {
	Tag           string
	BaseImage     string
	ConfigPath    string
	ResourcesPath string
	CacheDir      string
}

// LayerCache простой кэш слоёв
type LayerCache struct {
	cacheDir string
	entries  map[string]LayerCacheEntry
}

// LayerCacheEntry запись в кэше
type LayerCacheEntry struct {
	ModuleHash string    `json:"module_hash"`
	LayerID    string    `json:"layer_id"`
	BaseImage  string    `json:"base_image"`
	CreatedAt  time.Time `json:"created_at"`
}

// BuildahBuilder управляет сборкой через buildah
type BuildahBuilder struct {
	appConfig   *app.Configuration
	config      *Config
	options     BuildahOptions
	cache       *LayerCache
	containerID string
}

// NewBuildahBuilder создаёт новый BuildahBuilder
func NewBuildahBuilder(appConfig *app.Configuration, config *Config, opts BuildahOptions) *BuildahBuilder {
	// Определяем директорию кэша
	cacheDir := opts.CacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(filepath.Dir(appConfig.PathDBSQLSystem), "buildah-cache")
	}
	_ = os.MkdirAll(cacheDir, 0755)

	// Определяем пути
	if opts.ConfigPath == "" {
		opts.ConfigPath = appConfig.PathImageFile
	}
	if opts.ResourcesPath == "" {
		opts.ResourcesPath = appConfig.PathResourcesDir
	}

	return &BuildahBuilder{
		appConfig: appConfig,
		config:    config,
		options:   opts,
		cache:     newLayerCache(cacheDir),
	}
}

// newLayerCache создаёт кэш слоёв
func newLayerCache(cacheDir string) *LayerCache {
	cache := &LayerCache{
		cacheDir: cacheDir,
		entries:  make(map[string]LayerCacheEntry),
	}
	cache.load()
	return cache
}

func (c *LayerCache) load() {
	cacheFile := filepath.Join(c.cacheDir, "layers.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &c.entries)
}

func (c *LayerCache) save() error {
	cacheFile := filepath.Join(c.cacheDir, "layers.json")
	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cacheFile, data, 0644)
}

func (c *LayerCache) get(hash string) (string, bool) {
	entry, ok := c.entries[hash]
	if !ok {
		return "", false
	}
	return entry.LayerID, true
}

func (c *LayerCache) set(hash, layerID, baseImage string) {
	c.entries[hash] = LayerCacheEntry{
		ModuleHash: hash,
		LayerID:    layerID,
		BaseImage:  baseImage,
		CreatedAt:  time.Now(),
	}
	_ = c.save()
}

// Build выполняет послойную сборку образа
func (b *BuildahBuilder) Build(ctx context.Context) (string, error) {
	if _, err := exec.LookPath("buildah"); err != nil {
		return "", fmt.Errorf("buildah not found: %w", err)
	}

	baseImage := b.config.Image
	if b.options.BaseImage != "" {
		baseImage = b.options.BaseImage
	}
	if baseImage == "" {
		return "", errors.New("base image is required")
	}

	// Раскрываем все include модули
	resourcesDir := b.options.ResourcesPath
	flatModules, err := core.FlattenModules(b.config.Modules, resourcesDir, b.options.ConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to flatten modules: %w", err)
	}

	app.Log.Info(fmt.Sprintf("Building image from %s with %d modules (flattened)", baseImage, len(flatModules)))

	// Находим точку начала сборки (первый незакэшированный модуль)
	startIdx, startImage, prevHash := b.findCacheBreakpoint(flatModules, baseImage)

	if startIdx >= len(flatModules) {
		app.Log.Info("All modules cached, tagging final image")
		if err := b.tagImage(ctx, startImage, b.options.Tag); err != nil {
			return "", err
		}
		return startImage, nil
	}

	// Создаём контейнер
	containerID, err := b.createContainer(ctx, startImage)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}
	b.containerID = containerID
	defer b.cleanup()

	app.Log.Info(fmt.Sprintf("Starting from module %d/%d", startIdx+1, len(flatModules)))

	// Выполняем модули
	var lastLayerID string
	for i := startIdx; i < len(flatModules); i++ {
		fm := flatModules[i]
		moduleHash := b.computeModuleHash(fm, prevHash)
		cacheable := core.IsCacheable(fm.Module)

		label := fm.Module.GetLabel()
		if !cacheable {
			app.Log.Info(fmt.Sprintf("[%d/%d] Building (not cacheable): %s", i+1, len(flatModules), label))
		} else {
			app.Log.Info(fmt.Sprintf("[%d/%d] Building: %s", i+1, len(flatModules), label))
		}

		// Выполняем модуль
		if err = b.runModule(ctx, fm, i); err != nil {
			return "", fmt.Errorf("module %d (%s) failed: %w", i, label, err)
		}

		// Коммитим слой
		lastLayerID, err = b.commitLayer(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to commit layer %d: %w", i, err)
		}

		// Сохраняем в кэш только если модуль кэшируемый
		if cacheable {
			b.cache.set(moduleHash, lastLayerID, baseImage)
		}

		// Обновляем prevHash для следующего модуля в цепочке
		prevHash = moduleHash
	}

	// Финальный коммит с тегом
	imageID, err := b.commitFinal(ctx, b.options.Tag)
	if err != nil {
		return "", fmt.Errorf("failed to commit final image: %w", err)
	}

	cached := startIdx
	built := len(flatModules) - startIdx
	app.Log.Info(fmt.Sprintf("Build complete: %s (cached: %d, built: %d)", b.options.Tag, cached, built))

	return imageID, nil
}

// findCacheBreakpoint находит первый незакэшированный модуль
// Возвращает: (startIdx, startImage, prevHash)
// prevHash нужен для продолжения цепочки хешей в Build
func (b *BuildahBuilder) findCacheBreakpoint(modules []core.FlatModule, baseImage string) (int, string, string) {
	lastCachedImage := baseImage
	prevHash := baseImage

	for i, fm := range modules {
		if !core.IsCacheable(fm.Module) {
			return i, lastCachedImage, prevHash
		}

		moduleHash := b.computeModuleHash(fm, prevHash)
		layerID, found := b.cache.get(moduleHash)
		if !found {
			return i, lastCachedImage, prevHash
		}

		app.Log.Info(fmt.Sprintf("[%d/%d] Cache hit: %s", i+1, len(modules), fm.Module.GetLabel()))
		prevHash = moduleHash
		lastCachedImage = layerID
	}

	return len(modules), lastCachedImage, prevHash
}

// computeModuleHash вычисляет хеш модуля
func (b *BuildahBuilder) computeModuleHash(fm core.FlatModule, baseImage string) string {
	data := struct {
		Type        string `json:"type"`
		Body        any    `json:"body"`
		Env         any    `json:"env"`
		BaseImage   string `json:"base_image"`
		Source      string `json:"source"`
		ContentHash string `json:"content_hash,omitempty"`
	}{
		Type:      fm.Module.Type,
		Body:      fm.Module.Body,
		Env:       fm.Module.Env,
		BaseImage: baseImage,
		Source:    fm.SourceFile,
	}

	// Добавляем хэш содержимого для copy и shell модулей
	data.ContentHash = b.computeContentHash(fm)

	jsonData, _ := json.Marshal(data)

	// Раскрываем ${{ Env.X }} выражения
	exprData := common_types.ExprData{
		Env:     osutils.GetEnvMap(),
		Modules: map[string]*common_types.MapModule{},
	}
	resolved, err := core.ResolveExpr(string(jsonData), exprData)
	if err != nil {
		resolved = string(jsonData)
	}

	hash := sha256.Sum256([]byte(resolved))
	return hex.EncodeToString(hash[:])
}

// computeContentHash вычисляет хэш содержимого файлов для copy/shell модулей
func (b *BuildahBuilder) computeContentHash(fm core.FlatModule) string {
	switch fm.Module.Type {
	case core.TypeCopy:
		return b.hashCopySource(fm)
	case core.TypeShell:
		return b.hashShellScript(fm)
	default:
		return ""
	}
}

// hashCopySource вычисляет хэш source для copy модуля
func (b *BuildahBuilder) hashCopySource(fm core.FlatModule) string {
	body, ok := fm.Module.Body.(*models.CopyBody)
	if !ok || body.Source == "" {
		return ""
	}

	sourcePath := body.Source
	if !filepath.IsAbs(sourcePath) {
		sourcePath = filepath.Join(fm.BaseDir, sourcePath)
	}

	hash, err := hashPath(sourcePath)
	if err != nil {
		return ""
	}
	return hash
}

// hashShellScript вычисляет хэш скрипта для shell модуля
func (b *BuildahBuilder) hashShellScript(fm core.FlatModule) string {
	body, ok := fm.Module.Body.(*models.ShellBody)
	if !ok || body.Command == "" {
		return ""
	}

	// Проверяем является ли command путём к файлу
	cmd := body.Command

	// Если команда начинается с ./ или / считаем это путём к скрипту
	scriptPath := ""
	if strings.HasPrefix(cmd, "./") || strings.HasPrefix(cmd, "/") {
		parts := strings.Fields(cmd)
		if len(parts) > 0 {
			scriptPath = parts[0]
		}
	} else if !strings.Contains(cmd, " ") && !strings.Contains(cmd, "\n") {
		scriptPath = cmd
	}

	if scriptPath == "" {
		return ""
	}

	// Резолвим относительный путь
	if !filepath.IsAbs(scriptPath) {
		scriptPath = filepath.Join(fm.BaseDir, scriptPath)
	}

	// Проверяем существует ли файл
	info, err := os.Stat(scriptPath)
	if err != nil || info.IsDir() {
		return ""
	}

	hash, err := hashFile(scriptPath)
	if err != nil {
		return ""
	}
	return hash
}

// hashPath вычисляет хэш файла или директории
func hashPath(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		return hashDir(path)
	}
	return hashFile(path)
}

// hashFile вычисляет хэш содержимого файла
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashDir вычисляет хэш директории (все файлы рекурсивно)
func hashDir(dir string) (string, error) {
	var hashes []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(dir, path)

		fileHash, err := hashFile(path)
		if err != nil {
			return err
		}

		hashes = append(hashes, relPath+":"+fileHash)
		return nil
	})

	if err != nil {
		return "", err
	}

	// Сортируем для детерминированности
	sort.Strings(hashes)

	combined := strings.Join(hashes, "\n")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:]), nil
}

// runModule выполняет модуль внутри контейнера
func (b *BuildahBuilder) runModule(ctx context.Context, fm core.FlatModule, index int) error {
	args := []string{"run"}

	args = append(args,
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/etc/apm/image.yml,ro", b.options.ConfigPath),
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/etc/apm/resources,ro", b.options.ResourcesPath),
	)

	// Монтируем APM бинарник с хоста
	if apmPath, err := exec.LookPath("apm"); err == nil {
		args = append(args, "--mount", fmt.Sprintf("type=bind,src=%s,dst=/usr/bin/apm,ro", apmPath))
	}

	// Монтируем системные includes если они существуют
	if _, err := os.Stat("/usr/share/apm"); err == nil {
		args = append(args, "--mount", "type=bind,src=/usr/share/apm,dst=/usr/share/apm,ro")
	}

	if fm.BaseDir != "" {
		relPath, err := filepath.Rel(b.options.ResourcesPath, fm.BaseDir)
		if err == nil && !strings.HasPrefix(relPath, "..") {
			workDir := filepath.Join("/etc/apm/resources", relPath)
			args = append(args, "--workingdir", workDir)
		}
	}

	args = append(args,
		b.containerID,
		"--",
		"apm", "system", "image", "build",
		"--config", "/etc/apm/image.yml",
		"--resources", "/etc/apm/resources",
		"--flat-index", fmt.Sprintf("%d", index),
	)

	cmd := exec.CommandContext(ctx, "buildah", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	return cmd.Run()
}

// createContainer создаёт рабочий контейнер
func (b *BuildahBuilder) createContainer(ctx context.Context, image string) (string, error) {
	cmd := exec.CommandContext(ctx, "buildah", "from", image)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// commitLayer коммитит текущее состояние
func (b *BuildahBuilder) commitLayer(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "buildah", "commit", "--rm=false", b.containerID)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%w: %s", err, string(exitErr.Stderr))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// commitFinal создаёт финальный образ с тегом
func (b *BuildahBuilder) commitFinal(ctx context.Context, tag string) (string, error) {
	cmd := exec.CommandContext(ctx, "buildah", "commit", b.containerID, tag)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%w: %s", err, string(exitErr.Stderr))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// tagImage присваивает тег существующему образу
func (b *BuildahBuilder) tagImage(ctx context.Context, imageID, tag string) error {
	cmd := exec.CommandContext(ctx, "buildah", "tag", imageID, tag)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}

// cleanup удаляет рабочий контейнер
func (b *BuildahBuilder) cleanup() {
	if b.containerID != "" {
		_ = exec.Command("buildah", "rm", b.containerID).Run()
		b.containerID = ""
	}
}

// ClearCache очищает кэш слоёв
func (b *BuildahBuilder) ClearCache() error {
	b.cache.entries = make(map[string]LayerCacheEntry)
	return b.cache.save()
}
