package execute

import (
	"apm/internal/common/app"
	"apm/internal/common/build/core"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Mkdir(_ context.Context, _ Service, b *core.Body) error {
	app.Log.Info(fmt.Sprintf("Creating dirs at %s", strings.Join(b.GetTargets(), ", ")))
	for _, pathTarget := range b.GetTargets() {
		if !filepath.IsAbs(pathTarget) {
			return fmt.Errorf("target in mkdir type must be absolute path")
		}

		mode, err := osutils.StringToFileMode(b.Perm)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(pathTarget, mode); err != nil {
			return err
		}
	}
	return nil
}
