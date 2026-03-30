package build

import (
	"apm/internal/common/altfiles"
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"context"
	"fmt"
)

const altFilesPkg = "libnss-altfiles"

// applyNssAltfiles выполняет настройку nss-altfiles для атомарных систем
func (cfgService *ConfigService) applyNssAltfiles(ctx context.Context) error {
	if !helper.IsRunningInContainer() {
		app.Log.Info("Not running in container, skipping nss-altfiles setup")
		return nil
	}

	pkg, err := cfgService.GetPackageByName(ctx, altFilesPkg)
	if err != nil || pkg == nil || !pkg.Installed {
		app.Log.Info(fmt.Sprintf("Package %s is not installed, skipping nss-altfiles setup", altFilesPkg))
		return nil
	}

	app.Log.Info("Configuring nss-altfiles: splitting passwd/group for atomic system")

	result, err := altfiles.ApplyBuild()
	if err != nil {
		return err
	}

	app.Log.Info(fmt.Sprintf("nss-altfiles: /etc/passwd=%d, /usr/lib/passwd=%d, /etc/group=%d, /usr/lib/group=%d",
		result.EtcPasswdCount, result.LibPasswdCount, result.EtcGroupCount, result.LibGroupCount))
	return nil
}
