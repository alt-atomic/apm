package models

import (
	_package "apm/internal/common/apt/package"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/thediveo/osrelease"
)

var usrLibOsRelease = "/usr/lib/os-release"
var etcOsRelease = "/etc/os-release"
var plymouthThemesDir = "/usr/share/plymouth/themes"
var plymouthConfigFile = "/etc/plymouth/plymouthd.conf"
var plymouthKargsPath = "/usr/lib/bootc/kargs.d/00-plymouth.toml"
var plymouthDracutConfPath = "/usr/lib/dracut/dracut.conf.d/00-plymouth.conf"

type BrandingBody struct {
	// Имя брендинга для пакетов
	Name string `yaml:"name,omitempty" json:"name,omitempty" needs:"BuildType"`

	// Тема плимут
	PlymouthTheme string `yaml:"plymouth-theme,omitempty" json:"plymouth-theme,omitempty"`

	// Тип сборки, нужен для os-release
	BuildType string `yaml:"build-type,omitempty" json:"build-type,omitempty" needs:"Name"`
}

func (b *BrandingBody) Execute(ctx context.Context, svc Service) (any, error) {
	if b.Name != "" {
		var brandingPackagesPrefix = fmt.Sprintf("branding-%s-", b.Name)

		filters := map[string]any{
			"name": brandingPackagesPrefix,
		}
		packages, err := svc.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
		if err != nil {
			return nil, err
		}
		if len(packages) == 0 {
			return nil, fmt.Errorf("no branding packages found for %s", b.Name)
		}

		var brandingMap = map[string]_package.Package{}

		var pkgsNames []string
		for _, pkg := range packages {
			brandingMap[pkg.Name[len(brandingPackagesPrefix):len(pkg.Name)]] = pkg
			pkgsNames = append(pkgsNames, pkg.Name)
		}

		packagesBody := &PackagesBody{Install: pkgsNames}

		if _, err = packagesBody.Execute(ctx, svc); err != nil {
			return nil, err
		}

		if _, ok := brandingMap["release"]; ok {
			info, err := os.Stat(usrLibOsRelease)
			if err != nil {
				return nil, err
			}
			vars := osrelease.NewFromName(usrLibOsRelease)

			now := time.Now()
			curVer := now.Format("20060102")
			prettyCurVer := now.Format("02.01.2006")

			bType := b.BuildType
			prettyType := ""
			prettyNameSuffix := ""
			releaseType := ""
			versionId := fmt.Sprintf("%s-%s", curVer, bType)

			switch bType {
			case "stable":
				prettyType = osutils.Capitalize(bType)
				releaseType = bType
			case "nightly":
				prettyType = osutils.Capitalize(bType)
				prettyNameSuffix = fmt.Sprintf(" %s", prettyType)
				releaseType = "development"
			}

			if value, ok := vars["PRETTY_NAME"]; ok {
				// Package was not installed, but installed now
				if !strings.HasSuffix(value, prettyNameSuffix) {
					vars["PRETTY_NAME"] = value + prettyNameSuffix
				}
			}
			if value, ok := vars["ID"]; ok {
				// Package was not installed, but installed now
				if value == "altlinux" {
					vars["ID_LIKE"] = value
				}
				if !strings.HasSuffix(value, fmt.Sprintf("-%s", bType)) {
					vars["ID"] = fmt.Sprintf("%s-%s", value, bType)
				}
			} else {
				vars["ID"] = "unknown-distro"
			}
			vars["RELEASE_TYPE"] = releaseType
			vars["VERSION"] = fmt.Sprintf("%s %s", prettyCurVer, prettyType)
			vars["VERSION_ID"] = versionId
			vars["CPE_NAME"] = fmt.Sprintf("cpe:/o:%s:%s", strings.ReplaceAll(vars["ID"], "-", ":"), curVer)
			vars["IMAGE_ID"] = vars["ID"]
			vars["IMAGE_VERSION"] = vars["VERSION_ID"]

			var newLines []string
			for name, value := range vars {
				newLines = append(newLines, fmt.Sprintf("%s=\"%s\"", name, value))
			}

			newOsReleaseContent := strings.Join(newLines, "\n") + "\n"
			if err = os.WriteFile(usrLibOsRelease, []byte(newOsReleaseContent), info.Mode().Perm()); err != nil {
				return nil, err
			}

			linkBody := &LinkBody{
				Target:  etcOsRelease,
				To:      usrLibOsRelease,
				Replace: true,
			}

			if _, err = linkBody.Execute(ctx, svc); err != nil {
				return nil, err
			}
		}
	}

	if b.PlymouthTheme != "" {
		plymouthPaths := []string{plymouthKargsPath, plymouthDracutConfPath}

		if b.PlymouthTheme == "disabled" {
			if osutils.IsExists(plymouthConfigFile) {
				if err := os.WriteFile(plymouthConfigFile, []byte(""), 0644); err != nil {
					return nil, err
				}
				for _, p := range plymouthPaths {
					if err := os.RemoveAll(p); err != nil {
						return nil, err
					}
				}
			}
		} else {
			var themes []string
			if osutils.IsExists(plymouthThemesDir) {
				files, err := os.ReadDir(plymouthThemesDir)
				if err != nil {
					return nil, err
				}
				for _, file := range files {
					themes = append(themes, file.Name())
				}
			}

			if !slices.Contains(themes, b.PlymouthTheme) {
				filters := map[string]any{
					"name": fmt.Sprintf("plymouth-theme-%s", b.PlymouthTheme),
				}
				packages, err := svc.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
				if err != nil {
					return nil, err
				}
				if len(packages) == 0 {
					return nil, fmt.Errorf("no plymouth theme found for %s", b.PlymouthTheme)
				}

				var pkgsNames []string
				for _, pkg := range packages {
					pkgsNames = append(pkgsNames, pkg.Name)
				}

				packagesBody := &PackagesBody{Install: pkgsNames}

				if _, err = packagesBody.Execute(ctx, svc); err != nil {
					return nil, err
				}
			}

			plymouthConfig := strings.Join([]string{
				"[Daemon]",
				fmt.Sprintf("Theme=%s", b.PlymouthTheme),
				"ShowDelay=0",
				"DeviceTimeout=10",
			}, "\n") + "\n"

			if err := os.MkdirAll(path.Dir(plymouthConfigFile), 0644); err != nil {
				return nil, err
			}
			if err := os.WriteFile(plymouthConfigFile, []byte(plymouthConfig), 0644); err != nil {
				return nil, err
			}

			for _, p := range plymouthPaths {
				if err := os.MkdirAll(path.Dir(p), 0644); err != nil {
					return nil, err
				}
			}

			if err := os.WriteFile(plymouthKargsPath, []byte(`kargs = ["rhgb", "quiet", "splash", "plymouth.enable=1", "rd.plymouth=1"]`+"\n"), 0644); err != nil {
				return nil, err
			}
			if err := os.WriteFile(plymouthDracutConfPath, []byte(`add_dracutmodules+=" plymouth "`+"\n"), 0644); err != nil {
				return nil, err
			}
		}
	}

	return nil, nil
}
