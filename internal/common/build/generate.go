package build

import (
	"apm/internal/common/app"
	"apm/internal/common/build/common_types"
	"apm/internal/common/build/core"
	"apm/internal/common/build/models"
	"apm/internal/common/osutils"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	markerStart = "#apm-generated:start"
	markerEnd   = "#apm-generated:end"
)

// GenerateOptions параметры генерации Containerfile.
type GenerateOptions struct {
	Output          string
	ResourcesSource string
	ExpandRemote    bool
	CacheBust       string
}

// FlatStep развёрнутый модуль с хэшем для слоя.
type FlatStep struct {
	Module  core.Module
	BaseDir string
	Hash    string
}

// Generate строит Containerfile из загруженной конфигурации.
func (cfgService *ConfigService) Generate(opts GenerateOptions) (string, error) {
	cfg := cfgService.serviceHostConfig.GetConfig()
	if cfg == nil {
		return "", errors.New(app.T_("Configuration not loaded. Load config first"))
	}

	version := cfgService.appConfig.ConfigManager.GetParsedVersion().Value

	// top-level env конфига становится глобальным ENV для всех шагов.
	configEnv, err := core.ResolveExprMap(cfg.Env, common_types.ExprData{
		Env:     osutils.GetEnvMap(),
		Version: *cfgService.appConfig.ConfigManager.GetParsedVersion(),
	})
	if err != nil {
		return "", err
	}

	block, err := RenderBlock(cfg.Modules, cfgService.ResourcesDir(), version, configEnv, opts)
	if err != nil {
		return "", err
	}

	content, err := WriteWithMarkers(opts.Output, block, cfg.Image)
	if err != nil {
		return "", err
	}

	return content, nil
}

// RenderBlock разворачивает модули и рендерит блок между маркерами.
func RenderBlock(modules []core.Module, resourcesDir, version string, env map[string]string, opts GenerateOptions) (string, error) {
	steps, err := FlattenModules(modules, resourcesDir, "", nil, opts.ExpandRemote)
	if err != nil {
		return "", err
	}

	bad, err := DetectCrossState(steps)
	if err != nil {
		return "", err
	}
	if len(bad) > 0 {
		return "", fmt.Errorf(app.T_("generate mode does not support cross-module references (${{ Modules.* }}): %s"), strings.Join(bad, ", "))
	}

	resourcesSource := opts.ResourcesSource
	if resourcesSource == "" {
		resourcesSource = "./src/resources"
	}

	// Ресурсы монтируются на каждый шаг, если каталог существует — паритет с монолитом.
	resourcesMount := ""
	if info, statErr := os.Stat(resourcesDir); statErr == nil && info.IsDir() {
		resourcesMount = fmt.Sprintf(" --mount=type=bind,source=%s,target=/etc/apm/resources", resourcesSource)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "ENV APM_GENERATED_VERSION=%s\n", version)
	if opts.CacheBust != "" {
		fmt.Fprintf(&b, "ENV APM_CACHE_BUST=%q\n", opts.CacheBust)
	}
	for _, k := range slices.Sorted(maps.Keys(env)) {
		fmt.Fprintf(&b, "ENV %s=%q\n", k, env[k])
	}
	b.WriteString("RUN apm system image build --phase pre-modules\n")

	for _, step := range steps {
		raw, err := marshalModule(step.Module)
		if err != nil {
			return "", err
		}

		stepH, err := step.Module.Body.CacheHash(step.BaseDir)
		if err != nil {
			return "", err
		}

		// Флаги RUN (--mount) обязаны идти сразу после RUN, до APM_STEP.
		cmd := "APM_STEP=" + stepH + " apm system image build --step-inline '" + escapeSingleQuotes(string(raw)) + "'"

		if resourcesMount == "" {
			b.WriteString("RUN " + cmd + "\n")
		} else {
			b.WriteString("RUN" + resourcesMount + " \\\n  " + cmd + "\n")
		}
	}

	b.WriteString("RUN apm system image build --phase post-modules")
	return b.String(), nil
}

// FlattenModules разворачивает include в плоский список шагов, перенося if и env на детей.
func FlattenModules(modules []core.Module, baseDir, parentIf string, parentEnv map[string]string, expandRemote bool) ([]FlatStep, error) {
	return flattenModulesSeen(modules, baseDir, parentIf, parentEnv, expandRemote, nil)
}

// flattenTarget разворачивает один include-таргет (файл, директорию или URL).
func flattenTarget(target, baseDir, parentIf string, parentEnv map[string]string, expandRemote bool, seen []string) ([]FlatStep, error) {
	if osutils.IsURL(target) {
		if slices.Contains(seen, target) {
			return nil, fmt.Errorf(app.T_("circular include detected: %s"), target)
		}
		seen = append(slices.Clone(seen), target)

		mods, err := core.ReadAndParseModulesYamlUrl(target)
		if err != nil {
			return nil, err
		}
		return flattenModulesSeen(*mods, baseDir, parentIf, parentEnv, expandRemote, seen)
	}

	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, target)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if slices.Contains(seen, abs) {
		return nil, fmt.Errorf(app.T_("circular include detected: %s"), abs)
	}
	seen = append(slices.Clone(seen), abs)

	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return flattenDir(abs, parentIf, parentEnv, expandRemote, seen)
	}

	mods, err := core.ReadAndParseModulesYamlFile(abs)
	if err != nil {
		return nil, err
	}
	return flattenModulesSeen(*mods, filepath.Dir(abs), parentIf, parentEnv, expandRemote, seen)
}

// flattenDir разворачивает все yml-файлы директории.
func flattenDir(dir, parentIf string, parentEnv map[string]string, expandRemote bool, seen []string) ([]FlatStep, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)

	var out []FlatStep
	for _, name := range names {
		sub, err := flattenTarget(name, dir, parentIf, parentEnv, expandRemote, seen)
		if err != nil {
			return nil, err
		}
		out = append(out, sub...)
	}
	return out, nil
}

// flattenModulesSeen — FlattenModules с продолжением учёта циклов.
func flattenModulesSeen(modules []core.Module, baseDir, parentIf string, parentEnv map[string]string, expandRemote bool, seen []string) ([]FlatStep, error) {
	var out []FlatStep

	for _, m := range modules {
		effIf := combineIf(parentIf, m.If)
		effEnv := mergeEnv(parentEnv, m.Env)

		if m.Type != core.TypeInclude {
			m.If = effIf
			m.Env = effEnv
			out = append(out, FlatStep{Module: m, BaseDir: baseDir})
			continue
		}

		body, ok := m.Body.(*models.IncludeBody)
		if !ok {
			return nil, errors.New(app.T_("include module has invalid body"))
		}

		for _, target := range body.Targets {
			if osutils.IsURL(target) && !expandRemote {
				m.If = effIf
				m.Env = effEnv
				out = append(out, FlatStep{Module: singleInclude(m, target), BaseDir: baseDir})
				continue
			}
			sub, err := flattenTarget(target, baseDir, effIf, effEnv, expandRemote, seen)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		}
	}

	return out, nil
}

// singleInclude возвращает include с единственным таргетом.
func singleInclude(m core.Module, target string) core.Module {
	m.Body = &models.IncludeBody{Targets: []string{target}}
	return m
}

// DetectCrossState возвращает метки шагов со ссылками на выводы других модулей.
func DetectCrossState(steps []FlatStep) ([]string, error) {
	var bad []string
	for _, step := range steps {
		uses, err := stepUsesModules(step.Module)
		if err != nil {
			return nil, err
		}
		if uses {
			bad = append(bad, fmt.Sprintf("%v", step.Module.GetLabel()))
		}
	}
	return bad, nil
}

// stepUsesModules сообщает, ссылается ли модуль на переменную Modules в if или ${{ }}.
func stepUsesModules(m core.Module) (bool, error) {
	uses, err := core.ExprUsesVar(m.If, "Modules")
	if err != nil || uses {
		return uses, err
	}

	raw, err := marshalNoHTML(m.Body)
	if err != nil {
		return false, err
	}

	for _, expression := range core.PlaceholderExprs(string(raw)) {
		uses, err := core.ExprUsesVar(expression, "Modules")
		if err != nil {
			return false, err
		}
		if uses {
			return true, nil
		}
	}

	return false, nil
}

// combineIf объединяет родительское и дочернее условия через &&.
func combineIf(parent, child string) string {
	switch {
	case parent == "":
		return child
	case child == "":
		return parent
	default:
		return "(" + parent + ") && (" + child + ")"
	}
}

// mergeEnv объединяет env, дочерние значения важнее.
func mergeEnv(parent, child map[string]string) map[string]string {
	if len(parent) == 0 && len(child) == 0 {
		return nil
	}
	out := map[string]string{}
	maps.Copy(out, parent)
	maps.Copy(out, child)
	return out
}

// escapeSingleQuotes экранирует одинарные кавычки для shell.
func escapeSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}

// marshalModule сериализует модуль в JSON без HTML-экранирования.
func marshalModule(m core.Module) ([]byte, error) {
	return marshalNoHTML(m)
}

// marshalNoHTML сериализует значение в JSON без HTML-экранирования.
func marshalNoHTML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// WriteWithMarkers вставляет блок между маркерами, возвращает итоговый файл.
func WriteWithMarkers(path, block, image string) (string, error) {
	var content string

	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		content, err = replaceRegion(string(existing), block)
		if err != nil {
			return "", err
		}
	case os.IsNotExist(err):
		content = renderSkeleton(block, image)
	default:
		return "", err
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return content, nil
}

// replaceRegion заменяет содержимое между маркерами или дописывает блок.
func replaceRegion(existing, block string) (string, error) {
	startIdx := strings.Index(existing, markerStart)
	endIdx := strings.Index(existing, markerEnd)

	region := markerStart + "\n" + block + "\n" + markerEnd

	if startIdx >= 0 && endIdx > startIdx {
		prefix := existing[:startIdx]
		suffix := existing[endIdx+len(markerEnd):]
		return prefix + region + suffix, nil
	}

	if !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	return existing + region + "\n", nil
}

// renderSkeleton создаёт новый Containerfile с блоком.
func renderSkeleton(block, image string) string {
	ref, args := convertImageRef(image)

	var b strings.Builder
	for _, a := range args {
		b.WriteString("ARG " + a + "\n")
	}
	b.WriteString("FROM " + ref + "\n")
	b.WriteString(markerStart + "\n")
	b.WriteString(block + "\n")
	b.WriteString(markerEnd + "\n")
	return b.String()
}

// convertImageRef превращает ${{ Env.X }} в Dockerfile ARG-ссылку.
func convertImageRef(image string) (string, []string) {
	var args []string
	ref := core.ReplacePlaceholders(image, func(expression string) string {
		if name, ok := strings.CutPrefix(expression, "Env."); ok {
			args = append(args, name)
			return "${" + name + "}"
		}
		return "${{ " + expression + " }}"
	})
	if ref == "" {
		ref = "scratch"
	}
	return ref, args
}
