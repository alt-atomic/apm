package execute

import (
	"apm/internal/common/build/core"
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

func Branding(ctx context.Context, svc Service) error {
	branding := svc.Config().Branding

	if branding.Name != "" {
		filters := map[string]any{
			"name": fmt.Sprintf("branding-%s-", branding.Name),
		}
		packages, err := svc.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
		if err != nil {
			return err
		}
		if len(packages) == 0 {
			return fmt.Errorf("no branding packages found for %s", branding.Name)
		}

		var pkgsNames []string
		for _, pkg := range packages {
			pkgsNames = append(pkgsNames, pkg.Name)
		}
		if err = Packages(ctx, svc, &core.Body{Install: pkgsNames}); err != nil {
			return err
		}

		info, err := os.Stat(core.UsrLibOsRelease)
		if err != nil {
			return err
		}
		vars := osrelease.NewFromName(core.UsrLibOsRelease)

		now := time.Now()
		curVer := now.Format("20060102")
		prettyCurVer := now.Format("02.01.2006")

		prettyType := ""
		prettyNamePostfix := ""
		releaseType := ""

		bType := svc.Config().BuildType
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
		if err = os.WriteFile(core.UsrLibOsRelease, []byte(newOsReleaseContent), info.Mode().Perm()); err != nil {
			return err
		}

		if err = Link(ctx, svc, &core.Body{
			Target:      core.EtcOsRelease,
			Destination: core.UsrLibOsRelease,
			Replace:     true,
		}); err != nil {
			return err
		}
	}

	if branding.PlymouthTheme != "" {
		var themes []string
		if osutils.IsExists(core.PlymouthThemesDir) {
			files, err := os.ReadDir(core.PlymouthThemesDir)
			if err != nil {
				return err
			}
			for _, file := range files {
				themes = append(themes, file.Name())
			}
		}

		if !slices.Contains(themes, branding.PlymouthTheme) {
			filters := map[string]any{
				"name": fmt.Sprintf("plymouth-theme-%s", branding.PlymouthTheme),
			}
			packages, err := svc.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
			if err != nil {
				return err
			}
			if len(packages) == 0 {
				return fmt.Errorf("no plymouth theme found for %s", branding.PlymouthTheme)
			}

			var pkgsNames []string
			for _, pkg := range packages {
				pkgsNames = append(pkgsNames, pkg.Name)
			}
			if err := Packages(ctx, svc, &core.Body{Install: pkgsNames}); err != nil {
				return err
			}
		}

		plymouthPaths := []string{core.PlymouthKargsPath, core.PlymouthDracutConfPath}

		if branding.PlymouthTheme == "disabled" {
			if err := os.WriteFile(core.PlymouthConfigFile, []byte(""), 0644); err != nil {
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
				fmt.Sprintf("Theme=%s", branding.PlymouthTheme),
				"ShowDelay=0",
				"DeviceTimeout=10",
			}, "\n") + "\n"

			if err := os.WriteFile(core.PlymouthConfigFile, []byte(plymouthConfig), 0644); err != nil {
				return err
			}

			for _, p := range plymouthPaths {
				if err := os.MkdirAll(path.Dir(p), 0644); err != nil {
					return err
				}
			}

			if err := os.WriteFile(core.PlymouthKargsPath, []byte(`kargs = ["rhgb", "quiet", "splash", "plymouth.enable=1", "rd.plymouth=1"]`+"\n"), 0644); err != nil {
				return err
			}
			if err := os.WriteFile(core.PlymouthDracutConfPath, []byte(`add_dracutmodules+=" plymouth "`+"\n"), 0644); err != nil {
				return err
			}
		}
	}

	return nil
}
