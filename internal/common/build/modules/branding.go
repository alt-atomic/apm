package modules

import (
	"context"
	"fmt"
	"io/fs"
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

			applyReleaseOverrides(vars, b.ReleaseOverrides)
			if err := normalizeAtomicOsRelease(vars, b.BuildType); err != nil {
				return nil, err
			}

			return nil, saveOsRelease(ctx, svc, vars, info.Mode().Perm())
		}

	} else if len(b.ReleaseOverrides) != 0 && svc.IsAtomic() {
		info, err = os.Stat(usrLibOsRelease)
		if err != nil {
			return nil, err
		}
		vars := osrelease.NewFromName(usrLibOsRelease)

		applyReleaseOverrides(vars, b.ReleaseOverrides)
		if b.BuildType != "" {
			if err := normalizeAtomicOsRelease(vars, b.BuildType); err != nil {
				return nil, err
			}
		}

		return nil, saveOsRelease(ctx, svc, vars, info.Mode().Perm())
	}

	return nil, nil
}

func applyReleaseOverrides(vars, overrides map[string]string) {
	for name, value := range overrides {
		if value == "" {
			delete(vars, name)
			continue
		}
		vars[name] = value
	}
}

func normalizeAtomicOsRelease(vars map[string]string, buildType string) error {
	name := strings.TrimSpace(vars["NAME"])
	if name == "" {
		name = "No Name"
	}

	baseID := strings.TrimSpace(vars["ID"])
	if baseID == "" {
		baseID = "org.example.NoName"
	}

	version := strings.TrimSpace(vars["VERSION"])
	if version == "" {
		version = "0.1"
	}

	buildType = strings.ToLower(strings.TrimSpace(buildType))
	if buildType == "" {
		buildType = "stable"
	}

	releaseType := "development"
	buildName := osutils.Capitalize(buildType)
	switch buildType {
	case "stable":
		releaseType = "stable"
		buildName = ""
	}

	baseID = strings.TrimSuffix(baseID, "-"+buildType)

	variant := strings.TrimSpace(vars["VARIANT"])
	variantID := strings.TrimSpace(vars["VARIANT_ID"])

	if variantID == "" {
		delete(vars, "VARIANT")
		delete(vars, "VARIANT_ID")
	} else {
		vars["VARIANT"] = variant
		vars["VARIANT_ID"] = variantID
	}

	prettyVersion := version
	if parsed, err := time.Parse("20060102", version); err == nil {
		prettyVersion = parsed.Format("02.01.2006")
	}

	versionID := version
	if variantID != "" {
		versionID += "-" + variantID
		prettyVersion += " (" + variant + ")"
	}

	prettyName := name
	if buildName != "" {
		prettyName += " " + buildName
	}
	if variant != "" {
		prettyName += " (" + variant + ")"
	}

	id := baseID
	if buildType != "stable" {
		id += "-" + buildType
	}

	baseImageID := strings.TrimSpace(vars["IMAGE_ID"])
	if baseImageID == "" {
		baseImageID = baseID
	}
	imageParts := []string{baseImageID}
	if buildType != "stable" {
		imageParts = append(imageParts, buildType)
	}

	cpeName := strings.TrimSpace(vars["CPE_NAME"])
	if cpeName != "" {
		var err error
		cpeName, err = normalizeCPEName(cpeName, version, buildType, variantID)
		if err != nil {
			return err
		}
	}

	vars["ID"] = id
	vars["VERSION"] = prettyVersion
	vars["VERSION_ID"] = versionID
	vars["RELEASE_TYPE"] = releaseType
	vars["PRETTY_NAME"] = prettyName
	if cpeName != "" {
		vars["CPE_NAME"] = cpeName
	}
	vars["IMAGE_ID"] = strings.Join(imageParts, "-")
	vars["IMAGE_VERSION"] = versionID
	delete(vars, "BUILD_ID")

	return nil
}

func normalizeCPEName(cpeName, version, buildType, variantID string) (string, error) {
	const prefix = "cpe:/"
	if !strings.HasPrefix(cpeName, prefix) {
		return "", fmt.Errorf("unsupported CPE_NAME %q", cpeName)
	}

	parts := strings.SplitN(strings.TrimPrefix(cpeName, prefix), ":", 4)
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", fmt.Errorf("unsupported CPE_NAME %q", cpeName)
	}

	parts = append(parts[:3], version, buildType)
	if variantID != "" {
		parts = append(parts, variantID)
	}

	return prefix + strings.Join(parts, ":"), nil
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
