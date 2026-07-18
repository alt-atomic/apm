package modules

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	pkgbuild "altlinux.space/alt-atomic/apm/pkg/build"
	builtin "altlinux.space/alt-atomic/apm/pkg/build/modules"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/filter"
	"altlinux.space/alt-atomic/apm/pkg/osutils"

	"github.com/thediveo/osrelease"
)

const usrLibOsRelease = "/usr/lib/os-release"
const etcOsRelease = "/etc/os-release"

type BrandingBody struct {
	// Имя брендинга для пакетов
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Подпакеты брендинга, которые нужно поставить. Если пуст, поставятся все
	Subpackages []string `yaml:"subpackages,omitempty" json:"subpackages,omitempty" needs:"Name"`

	// Словарь переопределения или добавления полей в os-release.
	ReleaseOverrides map[string]string `yaml:"release-overrides,omitempty" json:"release-overrides,omitempty"`

	// Тип сборки, нужен для os-release
	BuildType string `yaml:"build-type,omitempty" json:"build-type,omitempty"`
}

func (b *BrandingBody) Execute(ctx context.Context, bc pkgbuild.RuntimeContext) (any, error) {
	return withDomain(ctx, bc, b.run)
}

func (b *BrandingBody) run(ctx context.Context, svc DomainContext) (any, error) {
	if len(b.ReleaseOverrides) != 0 && !svc.IsAtomic() {
		app.Log.Warn("release-overrides doesn't work in non-atomic builds. It will be overwritten on branding package update")
	}

	var info os.FileInfo
	var err error

	if b.Name != "" {
		var brandingPackagesPrefix = fmt.Sprintf("branding-%s-", b.Name)
		var brandingSubpackages []string

		if len(b.Subpackages) != 0 {
			brandingSubpackages = b.Subpackages
		} else {
			filters := []filter.Filter{
				{Field: "name", Op: filter.OpLike, Value: brandingPackagesPrefix},
			}
			var err error
			packages, err := svc.QueryHostImagePackages(ctx, filter.AndGroups(filters), "version", "DESC", 0, 0)
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

		// Исправить release файл для атомарной системы
		if slices.Contains(brandingSubpackages, "release") && svc.IsAtomic() {
			info, err = os.Stat(usrLibOsRelease)
			if err != nil {
				return nil, err
			}
			vars := osrelease.NewFromName(usrLibOsRelease)

			maps.Copy(vars, b.ReleaseOverrides)

			curVer := vars["VERSION"]
			prettyCurVer := vars["VERSION"]

			if _, err := time.Parse("20060102", vars["VERSION"]); err == nil {
				now := time.Now()
				curVer = now.Format("20060102")
				prettyCurVer = now.Format("02.01.2006")
			}

			bType := b.BuildType

			if bType == "" {
				bType = "stable"
			}

			prettyType := ""
			prettyNameSuffix := ""
			releaseType := ""
			versionId := fmt.Sprintf("%s-%s", curVer, bType)

			prettyType = osutils.Capitalize(bType)

			switch bType {
			case "stable":
				releaseType = bType
			case "nightly":
				prettyNameSuffix = " " + prettyType
				releaseType = "development"
			default:
				prettyNameSuffix = " " + prettyType
			}

			if value, ok := vars["PRETTY_NAME"]; ok {
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

			return nil, saveOsRelease(ctx, svc, vars, info.Mode().Perm())
		}

	} else if len(b.ReleaseOverrides) != 0 && svc.IsAtomic() {
		info, err = os.Stat(usrLibOsRelease)
		if err != nil {
			return nil, err
		}
		vars := osrelease.NewFromName(usrLibOsRelease)

		maps.Copy(vars, b.ReleaseOverrides)

		return nil, saveOsRelease(ctx, svc, vars, info.Mode().Perm())
	}

	return nil, nil
}

func saveOsRelease(ctx context.Context, svc DomainContext, vars map[string]string, perm fs.FileMode) error {
	newOsReleaseContent := renderOsRelease(vars, osReleaseKeyOrder(usrLibOsRelease))
	if err := os.WriteFile(usrLibOsRelease, []byte(newOsReleaseContent), perm); err != nil {
		return err
	}

	// We must create link only in atomic builds bacause of classic systems uses
	// /etc/os-release dedicate of /usr/lib/os-release for keeping BUILD_ID var
	if svc.IsAtomic() {
		linkBody := &builtin.LinkBody{
			Target:  etcOsRelease,
			To:      usrLibOsRelease,
			Replace: true,
		}

		if _, err := linkBody.Execute(ctx, svc); err != nil {
			return err
		}
	}

	return nil
}

// renderOsRelease собирает os-release в стабильном порядке.
func renderOsRelease(vars map[string]string, fileOrder []string) string {
	written := make(map[string]bool, len(vars))
	var lines []string

	for _, name := range fileOrder {
		if value, ok := vars[name]; ok && !written[name] {
			written[name] = true
			lines = append(lines, name+"="+quoteOsReleaseValue(value))
		}
	}

	var restKeys []string
	for name := range vars {
		if !written[name] {
			restKeys = append(restKeys, name)
		}
	}
	slices.Sort(restKeys)
	for _, name := range restKeys {
		lines = append(lines, name+"="+quoteOsReleaseValue(vars[name]))
	}

	return strings.Join(lines, "\n") + "\n"
}

// quoteOsReleaseValue экранирует спецсимволы.
func quoteOsReleaseValue(v string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`, "`", "\\`")
	return `"` + r.Replace(v) + `"`
}

// osReleaseKeyOrder возвращает порядок ключей из файла os-release.
func osReleaseKeyOrder(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var keys []string
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if key, _, found := strings.Cut(line, "="); found {
			keys = append(keys, key)
		}
	}
	return keys
}

func init() { pkgbuild.Register(TypeBranding, func() pkgbuild.Body { return &BrandingBody{} }) }
