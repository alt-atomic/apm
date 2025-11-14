package execute

import (
	"apm/internal/common/app"
	"apm/internal/common/build/core"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"path/filepath"
)

func Merge(_ context.Context, _ Service, b *core.Body) error {
	app.Log.Info(fmt.Sprintf("Merging %s with %s", b.Target, b.Destination))

	if !filepath.IsAbs(b.Target) {
		return fmt.Errorf("target in merge type must be absolute path")
	}

	mode, err := osutils.StringToFileMode(b.Perm)
	if err != nil {
		return err
	}
	return osutils.AppendFile(b.Target, b.Destination, mode)
}
