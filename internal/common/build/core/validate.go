package core

import (
	"apm/internal/common/app"
	"apm/internal/common/build/models"
	"apm/internal/common/osutils"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ValidationService рекурсивно валидирует конфиг включая все include файлы.
type ValidationService struct {
	visited   map[string]bool
	pathStack []string
}

// NewValidationService создаёт новый сервис валидации.
func NewValidationService() *ValidationService {
	return &ValidationService{
		visited:   make(map[string]bool),
		pathStack: []string{},
	}
}

// Validate рекурсивно валидирует модули и все include.
func (v *ValidationService) Validate(modules *[]Module, basePath string) error {
	for _, module := range *modules {
		if module.Type == TypeInclude {
			if err := v.validateInclude(&module, basePath); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *ValidationService) validateInclude(module *Module, basePath string) error {
	body, ok := module.Body.(*models.IncludeBody)
	if !ok {
		return nil
	}

	for _, target := range body.Targets {
		resolvedPath := v.resolvePath(target, basePath)

		if v.visited[resolvedPath] {
			return v.wrapError(fmt.Errorf(app.T_("circular include detected: %s"), resolvedPath))
		}
		v.visited[resolvedPath] = true
		v.pathStack = append(v.pathStack, resolvedPath)

		if err := v.validateTarget(resolvedPath); err != nil {
			return err
		}

		v.pathStack = v.pathStack[:len(v.pathStack)-1]
	}
	return nil
}

func (v *ValidationService) validateTarget(path string) error {
	if osutils.IsURL(path) {
		return v.validateFile(path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return v.wrapError(fmt.Errorf(app.T_("include target not found: %s"), path))
	}

	if info.IsDir() {
		return v.validateDir(path)
	}
	return v.validateFile(path)
}

func (v *ValidationService) validateDir(dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return v.wrapError(err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			if err = v.validateFile(filepath.Join(dir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *ValidationService) validateFile(path string) error {
	modules, err := ReadAndParseModules(path)
	if err != nil {
		return v.wrapError(err)
	}

	return v.Validate(modules, getBasePath(path))
}

func (v *ValidationService) resolvePath(target, basePath string) string {
	if osutils.IsURL(target) || filepath.IsAbs(target) {
		return target
	}
	return filepath.Join(basePath, target)
}

func (v *ValidationService) wrapError(err error) error {
	if len(v.pathStack) == 0 {
		return err
	}
	return fmt.Errorf("%s: %w", strings.Join(v.pathStack, " → "), err)
}

func getBasePath(path string) string {
	if !osutils.IsURL(path) {
		return filepath.Dir(path)
	}

	u, err := url.Parse(path)
	if err != nil {
		return ""
	}

	lastSlash := strings.LastIndex(u.Path, "/")
	if lastSlash > 0 {
		u.Path = u.Path[:lastSlash]
	}

	return u.String()
}

// ValidateConfigRecursive выполняет рекурсивную валидацию конфига включая все include файлы.
func ValidateConfigRecursive(cfg *Config, basePath string, _ map[string]bool) error {
	return NewValidationService().Validate(&cfg.Modules, basePath)
}
