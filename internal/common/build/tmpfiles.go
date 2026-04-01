package build

import (
	"apm/internal/common/app"
	"apm/internal/common/build/lint"
	"apm/internal/common/helper"
	"context"
	"fmt"
)

// fixTmpFiles выполняет анализ и фикс tmpfiles.d после сборки образа
func (cfgService *ConfigService) fixTmpFiles(ctx context.Context) error {
	if !helper.IsRunningInContainer() {
		app.Log.Info("Not running in container, skipping tmpfiles.d fix")
		return nil
	}

	app.Log.Info("Fixing tmpfiles.d: analyzing /var and /etc coverage")

	svc := lint.New("/")
	result, written, err := svc.AnalyzeTmpFiles(ctx, true)
	if err != nil {
		return fmt.Errorf("tmpfiles.d fix: %w", err)
	}

	if result == nil {
		app.Log.Info("tmpfiles.d: no missing entries")
		return nil
	}

	if len(result.Unsupported) > 0 {
		app.Log.Warn(fmt.Sprintf("tmpfiles.d: %d unsupported paths (not regular files, dirs, or symlinks): %v", len(result.Unsupported), result.Unsupported))
	}

	app.Log.Info(fmt.Sprintf("tmpfiles.d: fixed %d missing entries, written %s", len(result.Missing), written))
	return nil
}
