package build

import (
	"apm/internal/common/altfiles"
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"context"
	"fmt"
	"os"
)

const (
	etcPasswd   = "/etc/passwd"
	etcGroup    = "/etc/group"
	etcNssWitch = "/etc/nsswitch.conf"
	libPasswd   = "/usr/lib/passwd"
	libGroup    = "/usr/lib/group"
	altFilesPkg = "libnss-altfiles"
)

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

	if err = splitPasswdFiles(); err != nil {
		return fmt.Errorf("failed to split passwd: %w", err)
	}

	if err = splitGroupFiles(); err != nil {
		return fmt.Errorf("failed to split group: %w", err)
	}

	if err = patchNssWitchFile(); err != nil {
		return fmt.Errorf("failed to patch nsswitch.conf: %w", err)
	}

	app.Log.Info("nss-altfiles configuration complete")
	return nil
}

// splitPasswdFiles читает /etc/passwd и (если есть) /usr/lib/passwd,
// объединяет записи, разделяет заново и записывает оба файла.
func splitPasswdFiles() error {
	etcData, err := os.ReadFile(etcPasswd)
	if err != nil {
		return err
	}
	etcEntries, err := altfiles.ParsePasswd(etcData)
	if err != nil {
		return err
	}

	var libEntries []altfiles.PasswdEntry
	if libData, err := os.ReadFile(libPasswd); err == nil {
		libEntries, _ = altfiles.ParsePasswd(libData)
	}

	merged := altfiles.MergePasswd(etcEntries, libEntries)
	newEtc, newLib := altfiles.SplitPasswd(merged)

	info, err := os.Stat(etcPasswd)
	if err != nil {
		return err
	}
	perm := info.Mode().Perm()

	if err = os.WriteFile(libPasswd, altfiles.FormatPasswd(newLib), perm); err != nil {
		return err
	}
	return os.WriteFile(etcPasswd, altfiles.FormatPasswd(newEtc), perm)
}

// splitGroupFiles читает /etc/group и (если есть) /usr/lib/group,
// объединяет записи, разделяет заново и записывает оба файла.
func splitGroupFiles() error {
	etcData, err := os.ReadFile(etcGroup)
	if err != nil {
		return err
	}
	etcEntries, err := altfiles.ParseGroup(etcData)
	if err != nil {
		return err
	}

	var libEntries []altfiles.GroupEntry
	if libData, err := os.ReadFile(libGroup); err == nil {
		libEntries, _ = altfiles.ParseGroup(libData)
	}

	merged := altfiles.MergeGroup(etcEntries, libEntries)
	newEtc, newLib := altfiles.SplitGroup(merged)

	info, err := os.Stat(etcGroup)
	if err != nil {
		return err
	}
	perm := info.Mode().Perm()

	if err = os.WriteFile(libGroup, altfiles.FormatGroup(newLib), perm); err != nil {
		return err
	}
	return os.WriteFile(etcGroup, altfiles.FormatGroup(newEtc), perm)
}

func patchNssWitchFile() error {
	data, err := os.ReadFile(etcNssWitch)
	if err != nil {
		return err
	}

	info, err := os.Stat(etcNssWitch)
	if err != nil {
		return err
	}

	patched := altfiles.PatchNsswitch(data)
	return os.WriteFile(etcNssWitch, patched, info.Mode().Perm())
}
