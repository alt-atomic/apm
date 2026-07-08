package build

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"altlinux.space/alt-atomic/apm/pkg/osutils"
)

// ValidationService recursively validates a config and all its includes.
type ValidationService struct {
	pathStack []string
	validated map[string]bool
}

// NewValidationService creates a validation service.
func NewValidationService() *ValidationService {
	return &ValidationService{
		pathStack: []string{},
		validated: make(map[string]bool),
	}
}

// inStack reports whether the path is already on the stack (a cycle).
func (v *ValidationService) inStack(path string) bool {
	return slices.Contains(v.pathStack, path)
}

// Validate recursively validates modules and their includes.
func (v *ValidationService) Validate(modules *[]Module, basePath string) error {
	for _, module := range *modules {
		if _, ok := module.Body.(includeTargeter); !ok {
			continue
		}
		if err := v.validateInclude(&module, basePath); err != nil {
			return err
		}
	}
	return nil
}

// includeTargeter lets the engine validate nested files without knowing the module type.
type includeTargeter interface {
	IncludeTargets() []string
}

func (v *ValidationService) validateInclude(module *Module, basePath string) error {
	body, ok := module.Body.(includeTargeter)
	if !ok {
		return nil
	}

	for _, target := range body.IncludeTargets() {
		resolvedPath := v.resolvePath(target, basePath)

		if v.inStack(resolvedPath) {
			return v.wrapError(fmt.Errorf(T_("circular include detected: %s"), resolvedPath))
		}

		if v.validated[resolvedPath] {
			continue
		}

		v.pathStack = append(v.pathStack, resolvedPath)

		if err := v.validateTarget(resolvedPath); err != nil {
			return err
		}

		v.pathStack = v.pathStack[:len(v.pathStack)-1]
		v.validated[resolvedPath] = true
	}
	return nil
}

func (v *ValidationService) validateTarget(path string) error {
	if osutils.IsURL(path) {
		return v.validateFile(path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return v.wrapError(fmt.Errorf(T_("include target not found: %s"), path))
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
		if isRecipeFile(name) {
			filePath := filepath.Join(dir, name)

			// Replace the dir on the stack with the concrete file.
			if len(v.pathStack) > 0 {
				v.pathStack[len(v.pathStack)-1] = filePath
			}

			if err = v.validateFile(filePath); err != nil {
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

// ValidateConfigRecursive validates a config and all its includes recursively.
func ValidateConfigRecursive(cfg *Config, basePath string) error {
	return NewValidationService().Validate(&cfg.Modules, basePath)
}
