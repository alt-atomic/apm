package build

import (
	"apm/internal/common/app"
	"apm/internal/common/build/altfiles"
	"apm/internal/common/helper"
	"context"
	"fmt"
)

const altFilesPkg = "libnss-altfiles"

// altFilesManaged: true, если атомарная сборка в контейнере с установленным libnss-altfiles.
func (cfgService *ConfigService) altFilesManaged(ctx context.Context) bool {
	if !cfgService.IsAtomic() {
		return false
	}

	if !helper.IsRunningInContainer() {
		app.Log.Info("Not running in container, skipping nss-altfiles setup")
		return false
	}

	pkg, err := cfgService.GetPackageByName(ctx, altFilesPkg)
	if err != nil || pkg == nil || !pkg.Installed {
		app.Log.Info(fmt.Sprintf("Package %s is not installed, skipping nss-altfiles setup", altFilesPkg))
		return false
	}
	return true
}

// revertNssAltFiles сливает /usr/lib в /etc. Возвращает true, если откат выполнен.
func (cfgService *ConfigService) revertNssAltFiles(ctx context.Context) (bool, error) {
	if !cfgService.altFilesManaged(ctx) {
		return false, nil
	}

	app.Log.Info("nss-altfiles: joining /usr/lib back into /etc for imperative tools")

	result, err := altfiles.NewDefault().ApplyJoin()
	if err != nil {
		return false, err
	}

	app.Log.Info(fmt.Sprintf("nss-altfiles: joined /etc/passwd=%d, /etc/group=%d",
		result.EtcPasswdCount, result.EtcGroupCount))
	return true, nil
}

// splitNssAltFiles разделяет passwd/group по /etc и /usr/lib и патчит nsswitch.conf.
func (cfgService *ConfigService) splitNssAltFiles() error {
	app.Log.Info("Configuring nss-altfiles: splitting passwd/group for atomic system")

	result, err := altfiles.NewDefault().ApplyBuild()
	if err != nil {
		return err
	}

	app.Log.Info(fmt.Sprintf("nss-altfiles: /etc/passwd=%d, /usr/lib/passwd=%d, /etc/group=%d, /usr/lib/group=%d",
		result.EtcPasswdCount, result.LibPasswdCount, result.EtcGroupCount, result.LibGroupCount))
	return nil
}
