package execute

import (
	"apm/internal/common/app"
	"apm/internal/common/build/core"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

func Move(ctx context.Context, svc Service, b *core.Body) error {
	var withText []string
	if b.CreateLink {
		withText = append(withText, "with linking")
	}
	if b.Replace {
		withText = append(withText, "with replacing")
	}
	app.Log.Info(fmt.Sprintf("Moving %s to %s%s", b.Target, b.Destination, " "+strings.Join(withText, " and ")))

	if !filepath.IsAbs(b.Target) {
		return fmt.Errorf("target in move type must be absolute path")
	}
	if !filepath.IsAbs(b.Destination) {
		return fmt.Errorf("destination in move type must be absolute path")
	}

	if err := osutils.Move(b.Target, b.Destination, b.Replace); err != nil {
		return err
	}

	if b.CreateLink {
		if err := Link(ctx, svc, &core.Body{Target: b.Target, Destination: b.Destination}); err != nil {
			return err
		}
	}
	return nil
}
