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

func Remove(_ context.Context, _ Service, b *core.Body) error {
	for _, pathTarget := range b.GetTargets() {
		if !filepath.IsAbs(pathTarget) {
			return fmt.Errorf("target in remove type must be absolute path")
		}

		if b.Inside {
			app.Log.Info(fmt.Sprintf("Cleaning %s", strings.Join(b.GetTargets(), ", ")))
			if err := osutils.Clean(pathTarget); err != nil {
				return err
			}
		} else {
			app.Log.Info(fmt.Sprintf("Removing %s", strings.Join(b.GetTargets(), ", ")))
			if err := os.RemoveAll(pathTarget); err != nil {
				return err
			}
		}
	}
	return nil
}
