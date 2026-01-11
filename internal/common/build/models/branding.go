package models

import (
	"apm/internal/common/app"
	"apm/internal/common/osutils"
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/thediveo/osrelease"
)

var usrLibOsRelease = "/usr/lib/os-release"
var etcOsRelease = "/etc/os-release"

type BrandingBody struct {
	// Имя брендинга для пакетов
	Name string `yaml:"name,omitempty" json:"name,omitempty" needs:"BuildType"`

	// Подпакетоы брендинга, которые нужно поставить. Если пуст, поставятся все
	Subpackages []string `yaml:"subpackages,omitempty" json:"subpackages,omitempty" needs:"Name"`

	// Словарь переопределения или добавления полей в os-release.
	ReleaseOverrides map[string]string `yaml:"release-overrides,omitempty" json:"release-overrides,omitempty" needs:"Name"`

	// Тип сборки, нужен для os-release
	BuildType string `yaml:"build-type,omitempty" json:"build-type,omitempty" needs:"Name"`
}

func (b *BrandingBody) Execute(ctx context.Context, svc Service) (any, error) {
	if b.Name != "" {
		var brandingPackagesPrefix = fmt.Sprintf("branding-%s-", b.Name)
		var brandingSubpackages = []string{}

		if len(b.Subpackages) != 0 {
			brandingSubpackages = b.Subpackages
		} else {
			filters := map[string]any{
				"name": brandingPackagesPrefix,
			}
			var err error
			packages, err := svc.QueryHostImagePackages(ctx, filters, "version", "DESC", 0, 0)
			if err != nil {
				return nil, err
			}

			if len(packages) == 0 {
				return nil, fmt.Errorf("no branding packages found for %s", b.Name)
			}

			for _, p := range packages {
				brandingSubpackages = append(brandingSubpackages, strings.TrimPrefix(p.Name, brandingPackagesPrefix))
			}
		}

		var pkgsNames []string
		for _, subpkg := range brandingSubpackages {
			pkgsNames = append(pkgsNames, brandingPackagesPrefix+subpkg)
		}

		packagesBody := &PackagesBody{Install: pkgsNames}

		if _, err := packagesBody.Execute(ctx, svc); err != nil {
			return nil, err
		}

		info, err := os.Stat(usrLibOsRelease)
		if err != nil {
			return nil, err
		}
		vars := osrelease.NewFromName(usrLibOsRelease)

		maps.Copy(vars, b.ReleaseOverrides)

		// Исправить release файл для атомарной системы
		if slices.Contains(brandingSubpackages, "release") && svc.IsAtomic() {
			curVer := vars["VERSION"]
			prettyCurVer := vars["VERSION"]

			if _, err := time.Parse("20060102", vars["VERSION"]); err != nil {
				app.Log.Warn("Couldn't parse os-release VERSION")
			} else {
				now := time.Now()
				curVer = now.Format("20060102")
				prettyCurVer = now.Format("02.01.2006")
			}

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
				prettyNameSuffix = " " + prettyType
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
				vars["ID"] = "linux"
			}
			vars["RELEASE_TYPE"] = releaseType
			vars["VERSION"] = fmt.Sprintf("%s %s", prettyCurVer, prettyType)
			vars["VERSION_ID"] = versionId
			vars["CPE_NAME"] = fmt.Sprintf("cpe:/o:%s:%s", strings.ReplaceAll(vars["ID"], "-", ":"), curVer)
			vars["IMAGE_ID"] = vars["ID"]
			vars["IMAGE_VERSION"] = vars["VERSION_ID"]
			if variant, ok := vars["VARIANT"]; ok {
				vars["BUILD_ID"] = fmt.Sprintf("%s %s", variant, vars["VERSION_ID"])
			} else {
				vars["BUILD_ID"] = fmt.Sprintf("%s %s", vars["NAME"], vars["VERSION_ID"])
			}

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

	return nil, nil
}
