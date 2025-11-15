package models

import (
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/thediveo/osrelease"
)

var cpeNameRegex = regexp.MustCompile(`:\d+$`)
var usrLibOsRelease = "/usr/lib/os-release"
var etcOsRelease = "/etc/os-release"
var plymouthThemesDir = "/usr/share/plymouth/themes"
var plymouthConfigFile = "/etc/plymouth/plymouthd.conf"
var plymouthKargsPath = "/usr/lib/bootc/kargs.d/00-plymouth.toml"
var plymouthDracutConfPath = "/usr/lib/dracut/dracut.conf.d/00-plymouth.conf"

type BrandingBody struct {
	// Имя брендинга для пакетов
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Тема плимут
	PlymouthTheme string `yaml:"plymouth-theme,omitempty" json:"plymouth-theme,omitempty"`

	// Тема плимут
	BuildType string `yaml:"build-type,omitempty" json:"build-type,omitempty"`
}

func (b *BrandingBody) Check() error {
	return nil
}

func (b *BrandingBody) Execute(ctx context.Context, svc Service) error {
	if b.Name != "" {
		filters := map[string]any{
			"name": fmt.Sprintf("branding-%s-", b.Name),
		}
		packages, err := svc.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
		if err != nil {
			return err
		}
		if len(packages) == 0 {
			return fmt.Errorf("no branding packages found for %s", b.Name)
		}

		var pkgsNames []string
		for _, pkg := range packages {
			pkgsNames = append(pkgsNames, pkg.Name)
		}

		packagesBody := &PackagesBody{Install: pkgsNames}

		if err = packagesBody.Execute(ctx, svc); err != nil {
			return err
		}

		info, err := os.Stat(usrLibOsRelease)
		if err != nil {
			return err
		}
		vars := osrelease.NewFromName(usrLibOsRelease)

		now := time.Now()
		curVer := now.Format("20060102")
		prettyCurVer := now.Format("02.01.2006")

		prettyType := ""
		prettyNamePostfix := ""
		releaseType := ""

		bType := b.BuildType
		switch bType {
		case "stable":
			prettyType = osutils.Capitalize(bType)
			releaseType = bType
		case "nightly":
			prettyType = osutils.Capitalize(bType)
			prettyNamePostfix = fmt.Sprintf(" %s", prettyType)
			releaseType = "development"
		}

		for name, value := range vars {
			switch name {
			case "VERSION":
				vars[name] = fmt.Sprintf("%s %s", prettyCurVer, prettyType)
			case "VERSION_ID":
				vars[name] = fmt.Sprintf("%s-%s", curVer, bType)
			case "RELEASE_TYPE":
				vars[name] = releaseType
			case "PRETTY_NAME":
				vars[name] = value + prettyNamePostfix
			case "CPE_NAME":
				vars[name] = cpeNameRegex.ReplaceAllString(value, fmt.Sprintf(":%s:%s", bType, curVer))
			}
		}

		vars["IMAGE_ID"] = vars["ID"]
		vars["IMAGE_VERSION"] = vars["VERSION_ID"]

		var newLines []string
		for name, value := range vars {
			newLines = append(newLines, fmt.Sprintf("%s=\"%s\"", name, value))
		}

		newOsReleaseContent := strings.Join(newLines, "\n") + "\n"
		if err = os.WriteFile(usrLibOsRelease, []byte(newOsReleaseContent), info.Mode().Perm()); err != nil {
			return err
		}

		linkBody := &LinkBody{
			Target:      etcOsRelease,
			Destination: usrLibOsRelease,
			Replace:     true,
		}

		if err = linkBody.Execute(ctx, svc); err != nil {
			return err
		}
	}

	if b.PlymouthTheme != "" {
		var themes []string
		if osutils.IsExists(plymouthThemesDir) {
			files, err := os.ReadDir(plymouthThemesDir)
			if err != nil {
				return err
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
				return err
			}
			if len(packages) == 0 {
				return fmt.Errorf("no plymouth theme found for %s", b.PlymouthTheme)
			}

			var pkgsNames []string
			for _, pkg := range packages {
				pkgsNames = append(pkgsNames, pkg.Name)
			}

			packagesBody := &PackagesBody{Install: pkgsNames}

			if err := packagesBody.Execute(ctx, svc); err != nil {
				return err
			}
		}

		plymouthPaths := []string{plymouthKargsPath, plymouthDracutConfPath}

		if b.PlymouthTheme == "disabled" {
			if err := os.WriteFile(plymouthConfigFile, []byte(""), 0644); err != nil {
				return err
			}
			for _, p := range plymouthPaths {
				if err := os.RemoveAll(p); err != nil {
					return err
				}
			}
		} else {
			plymouthConfig := strings.Join([]string{
				"[Daemon]",
				fmt.Sprintf("Theme=%s", b.PlymouthTheme),
				"ShowDelay=0",
				"DeviceTimeout=10",
			}, "\n") + "\n"

			if err := os.WriteFile(plymouthConfigFile, []byte(plymouthConfig), 0644); err != nil {
				return err
			}

			for _, p := range plymouthPaths {
				if err := os.MkdirAll(path.Dir(p), 0644); err != nil {
					return err
				}
			}

			if err := os.WriteFile(plymouthKargsPath, []byte(`kargs = ["rhgb", "quiet", "splash", "plymouth.enable=1", "rd.plymouth=1"]`+"\n"), 0644); err != nil {
				return err
			}
			if err := os.WriteFile(plymouthDracutConfPath, []byte(`add_dracutmodules+=" plymouth "`+"\n"), 0644); err != nil {
				return err
			}
		}
	}

	return nil
}
