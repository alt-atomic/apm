package execute

import (
	"apm/internal/common/app"
	"apm/internal/common/build/core"
	"apm/internal/common/osutils"
	"context"
	"fmt"
)

func Shell(ctx context.Context, svc Service, b *core.Body) error {
	for _, cmdSh := range b.GetCommands() {
		app.Log.Info(fmt.Sprintf("Executing `%s`", cmdSh))
		if err := osutils.ExecSh(ctx, cmdSh, svc.ResourcesDir(), true); err != nil {
			return err
		}
	}
	return nil
}
