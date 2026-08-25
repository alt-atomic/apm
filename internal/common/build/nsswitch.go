package build

import (
	"context"
	"fmt"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/build/altfiles"
	"altlinux.space/alt-atomic/apm/internal/common/helper"
)

const altFilesPkg = "libnss-altfiles"

// altFilesManaged: true, если в контейнере и установлен libnss-altfiles.
func (cfgService *ConfigService) altFilesManaged(ctx context.Context) bool {
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

// revertNssAltFiles сливает /usr/lib в /etc, если система в split-режиме.
func (cfgService *ConfigService) revertNssAltFiles() error {
	if !helper.IsRunningInContainer() {
		return nil
	}

	svc := altfiles.NewDefault()
	split, err := svc.IsSplit()
	if err != nil {
		return err
	}
	if !split {
		return nil
	}

	app.Log.Info("nss-altfiles: joining /usr/lib back into /etc for imperative tools")

	result, err := svc.ApplyJoin()
	if err != nil {
		return err
	}

	app.Log.Info(fmt.Sprintf("nss-altfiles: joined /etc/passwd=%d, /etc/group=%d",
		result.EtcPasswdCount, result.EtcGroupCount))
	return nil
}

// splitNssAltFiles формирует altfiles, если установлен libnss-altfiles.
func (cfgService *ConfigService) splitNssAltFiles(ctx context.Context) error {
	if !cfgService.altFilesManaged(ctx) {
		return nil
	}

	app.Log.Info("Configuring nss-altfiles: splitting passwd/group for atomic system")

	result, err := altfiles.NewDefault().ApplyBuild()
	if err != nil {
		return err
	}

	app.Log.Info(fmt.Sprintf("nss-altfiles: /etc/passwd=%d, /usr/lib/passwd=%d, /etc/group=%d, /usr/lib/group=%d",
		result.EtcPasswdCount, result.LibPasswdCount, result.EtcGroupCount, result.LibGroupCount))
	return nil
}
