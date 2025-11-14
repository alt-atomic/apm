package execute

import (
	"apm/internal/common/app"
	"apm/internal/common/build/core"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"path/filepath"
)

func Copy(_ context.Context, _ Service, b *core.Body) error {
	replaceText := ""
	if b.Replace {
		replaceText = " with replacing"
	}
	app.Log.Info(fmt.Sprintf("Copying %s to %s%s", b.Target, b.Destination, replaceText))

	if !filepath.IsAbs(b.Destination) {
		return fmt.Errorf("destination in move type must be absolute path")
	}

	return osutils.Copy(b.Target, b.Destination, b.Replace)
}
