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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"altlinux.space/alt-atomic/apm/pkg/osutils"
)

// Progress event states (values match reply.State*).
const (
	StateBefore = "BEFORE"
	StateAfter  = "AFTER"
)

// Engine is the neutral recipe executor: module loop, expr/env, includes, and stage hooks.
type Engine struct {
	version     Version
	eventPrefix string

	hooks     map[Stage][]HookFunc
	moduleSeq int
	collected []string
}

// NewEngine creates an engine; eventPrefix prefixes emitted module event names.
func NewEngine(ver Version, eventPrefix string) *Engine {
	return &Engine{
		version:     ver,
		eventPrefix: eventPrefix,
	}
}

// Hook registers a stage hook.
func (e *Engine) Hook(stage Stage, fn HookFunc) {
	if e.hooks == nil {
		e.hooks = map[Stage][]HookFunc{}
	}
	e.hooks[stage] = append(e.hooks[stage], fn)
}

// runStageHooks runs a stage's hooks in order, returning the first error.
func (e *Engine) runStageHooks(ctx context.Context, stage Stage) error {
	for _, fn := range e.hooks[stage] {
		if err := fn(ctx); err != nil {
			return err
		}
	}
	return nil
}

// CollectOutput accumulates module output for the final report.
func (e *Engine) CollectOutput(text string) {
	if trimmed := strings.TrimSpace(text); trimmed != "" {
		e.collected = append(e.collected, trimmed)
	}
}

// CollectedOutput returns the accumulated module output.
func (e *Engine) CollectedOutput() string {
	return strings.Join(e.collected, "\n")
}

// Build runs the modules wrapped by stage hooks: PreModules → modules → PostModules → PostBuild.
func (e *Engine) Build(ctx context.Context, bc RuntimeContext, modules []Module) error {
	if err := e.runModules(ctx, bc, modules); err != nil {
		return err
	}

	return e.runStageHooks(ctx, PostBuild)
}

// runModules wraps module execution in the PreModules/PostModules hooks.
func (e *Engine) runModules(ctx context.Context, bc RuntimeContext, modules []Module) (err error) {
	if err = e.runStageHooks(ctx, PreModules); err != nil {
		return err
	}

	defer func() {
		if postErr := e.runStageHooks(ctx, PostModules); postErr != nil {
			logger.Error(fmt.Sprintf("post-modules stage failed: %v", postErr))
			if err == nil {
				err = postErr
			}
		}
	}()

	_, err = e.executeModules(ctx, bc, modules)
	return err
}

func (e *Engine) ExecuteModule(ctx context.Context, bc RuntimeContext, module Module, modulesMap map[string]*MapModule) (*MapModule, error) {
	exprData := newExprData(e.version)
	exprData.Modules = modulesMap

	moduleResolvedEnvMap, err := ResolveExprMap(module.Env, exprData)
	if err != nil {
		return nil, err
	}

	shouldExecute := true
	if module.If != "" {
		shouldExecute, err = ExtractExprResultBool(module.If, exprData)
		if err != nil {
			return nil, err
		}
	}

	existedEnvMap := map[string]string{}
	for key, value := range moduleResolvedEnvMap {
		if currentValue, ok := os.LookupEnv(key); ok {
			existedEnvMap[key] = currentValue
		}

		if err = os.Setenv(key, value); err != nil {
			return nil, err
		}
	}

	if !shouldExecute {
		var outputModule *MapModule = nil

		if module.Id != "" {
			outputModule = &MapModule{
				Name:   module.Name,
				Type:   module.Type,
				Id:     module.Id,
				If:     false,
				Output: map[string]string{},
			}
		}

		return outputModule, nil
	}

	body := module.Body
	if body == nil {
		return nil, fmt.Errorf("module %s has no body", module.Type)
	}

	e.moduleSeq++
	eventName := fmt.Sprintf("%s.%d", e.eventPrefix, e.moduleSeq)
	eventView := module.Name
	if eventView == "" {
		eventView = fmt.Sprintf("%v", module.GetLabel())
	}
	bc.EmitEvent(ctx, StateBefore, eventName, eventView)
	defer bc.EmitEvent(ctx, StateAfter, eventName, eventView)

	exprData.Env = osutils.GetEnvMap()

	// Resolve env expressions in the body via reflection.
	if err = ResolveStruct(body, exprData); err != nil {
		return nil, fmt.Errorf("failed to resolve env variables: %w", err)
	}

	output := map[string]string{}

	if out, err := body.Execute(ctx, bc); err != nil {
		return nil, fmt.Errorf("module '%s': %w", module.GetLabel(), err)
	} else {
		if out == nil && len(module.Output) > 0 {
			logger.Warn(fmt.Sprintf(T_("'%s' type doesn't support output"), module.Type))
		} else if out != nil {
			output, err = ResolveExprMap(module.Output, out)
			if err != nil {
				return nil, err
			}
		}
	}

	for key := range moduleResolvedEnvMap {
		if oldValue, ok := existedEnvMap[key]; ok {
			if err = os.Setenv(key, oldValue); err != nil {
				return nil, err
			}
		} else {
			if err = os.Unsetenv(key); err != nil {
				return nil, err
			}
		}
	}

	var outputModule *MapModule = nil

	if module.Id != "" {
		outputModule = &MapModule{
			Name:   module.Name,
			Type:   module.Type,
			Id:     module.Id,
			If:     true,
			Output: output,
		}
	}

	return outputModule, nil
}

func (e *Engine) ExecuteInclude(ctx context.Context, bc RuntimeContext, target string) (map[string]*MapModule, error) {
	if osutils.IsURL(target) {
		return e.executeIncludeFile(ctx, bc, target)
	}

	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return nil, e.executeIncludeDir(ctx, bc, target)
	}

	return e.executeIncludeFileWithCD(ctx, bc, target)
}

// executeIncludeDir runs every recipe file in the directory.
func (e *Engine) executeIncludeDir(ctx context.Context, bc RuntimeContext, dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		path := filepath.Join(dir, file.Name())
		if isRecipeFile(path) {
			// Ignore the dir so include ids stay scoped to a single file.
			if _, err = e.executeIncludeFileWithCD(ctx, bc, path); err != nil {
				return err
			}
		}
	}

	return nil
}

// executeIncludeFileWithCD runs a file with the working dir set to its folder.
func (e *Engine) executeIncludeFileWithCD(ctx context.Context, bc RuntimeContext, filePath string) (map[string]*MapModule, error) {
	originalWd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Always restore the original working directory.
	defer func() {
		if chErr := os.Chdir(originalWd); chErr != nil {
			logger.Error(fmt.Sprintf("failed to restore working directory: %v", chErr))
		}
	}()

	includeDir := filepath.Dir(filePath)
	includeName := filepath.Base(filePath)

	if includeDir != originalWd {
		if err = os.Chdir(includeDir); err != nil {
			return nil, fmt.Errorf("failed to change directory to %s: %w", includeDir, err)
		}
	}

	return e.executeIncludeFile(ctx, bc, includeName)
}

// executeIncludeFile reads and runs a modules file (YAML or JSON).
func (e *Engine) executeIncludeFile(ctx context.Context, bc RuntimeContext, path string) (map[string]*MapModule, error) {
	modules, err := ReadAndParseModules(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse modules from %s: %w", path, err)
	}

	return e.executeModules(ctx, bc, *modules)
}

func (e *Engine) executeModules(ctx context.Context, bc RuntimeContext, modules []Module) (map[string]*MapModule, error) {
	modulesMap := map[string]*MapModule{}

	for _, module := range modules {
		if output, err := e.ExecuteModule(ctx, bc, module, modulesMap); err != nil {
			return nil, err
		} else {
			if output != nil {
				if value, found := modulesMap[module.Id]; found {
					oldLabel := value.GetLabel()
					newLabel := module.GetLabel()

					oldLabelText := ""
					newLabelText := ""

					if value.Name != "" {
						oldLabelText = fmt.Sprintf(" (%s)", oldLabel)
					}
					if module.Name != "" {
						newLabelText = fmt.Sprintf(" with %s", newLabel)
					}

					logger.Warn(fmt.Sprintf(T_("module with id='%s'%s will be overriding%s"), module.Id, oldLabelText, newLabelText))
				}

				modulesMap[module.Id] = output
			}
		}
	}

	return modulesMap, nil
}
