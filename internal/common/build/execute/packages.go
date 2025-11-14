package execute

import (
	"apm/internal/common/app"
	"apm/internal/common/build/core"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"strings"
)

func Packages(ctx context.Context, svc Service, b *core.Body) error {
	var text []string
	if len(b.Install) != 0 {
		text = append(text, fmt.Sprintf("installing %s", strings.Join(b.Install, ", ")))
	}
	if len(b.Remove) != 0 {
		text = append(text, fmt.Sprintf("removing %s", strings.Join(b.Remove, ", ")))
	}
	if len(text) != 0 {
		app.Log.Info(osutils.Capitalize(strings.Join(text, " and ")))
	}

	var ops []string
	for _, p := range b.Install {
		ops = append(ops, p+"+")
	}
	for _, p := range b.Remove {
		ops = append(ops, p+"-")
	}

	return svc.CombineInstallRemovePackages(ctx, ops, false, false)
}
