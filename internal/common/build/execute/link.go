package execute

import (
	"apm/internal/common/app"
	"apm/internal/common/build/core"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

func Link(_ context.Context, _ Service, b *core.Body) error {
	if !filepath.IsAbs(b.Target) {
		return fmt.Errorf("target in link type must be absolute path")
	}

	app.Log.Info(fmt.Sprintf("Linking %s to %s", b.Target, b.Destination))
	if b.Replace {
		if err := os.RemoveAll(b.Target); err != nil {
			return err
		}
	}

	if filepath.IsAbs(b.Destination) {
		relativePath, err := filepath.Rel(path.Dir(b.Target), b.Destination)
		if err != nil {
			relativePath = b.Destination
		}
		return os.Symlink(relativePath, b.Target)
	}
	return os.Symlink(b.Destination, b.Target)
}
