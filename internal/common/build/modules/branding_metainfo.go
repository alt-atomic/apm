package modules

import (
	"fmt"
	"maps"

	"altlinux.space/alt-atomic/apm/internal/common/metainfo"
)

// BrandingMetainfo describes the optional fields of an operating-system component.
type BrandingMetainfo struct {
	Name          map[string]string  `yaml:"name,omitempty" json:"name,omitempty"`
	Summary       map[string]string  `yaml:"summary,omitempty" json:"summary,omitempty"`
	Description   map[string]string  `yaml:"description,omitempty" json:"description,omitempty"`
	Variants      []metainfo.Variant `yaml:"variants,omitempty" json:"variants,omitempty"`
	LatestRelease *metainfo.Release  `yaml:"latest-release,omitempty" json:"latest-release,omitempty"`
}

func brandingComponent(id string, vars map[string]string, cfg *BrandingMetainfo) metainfo.OperatingSystem {
	if cfg == nil {
		cfg = &BrandingMetainfo{}
	}
	withDefault := func(values map[string]string, fallback string) map[string]string {
		values = maps.Clone(values)
		if _, exists := values["C"]; !exists && fallback != "" {
			if values == nil {
				values = make(map[string]string)
			}
			values["C"] = fallback
		}
		return values
	}
	component := metainfo.OperatingSystem{
		ID:          id,
		Name:        withDefault(cfg.Name, vars["NAME"]),
		Summary:     withDefault(cfg.Summary, vars["NAME"]),
		Description: cfg.Description,
		Variants:    cfg.Variants,
		Homepage:    vars["HOME_URL"],
	}
	if name := vars["VENDOR_NAME"]; name != "" {
		vendorID := ""
		if home := vars["VENDOR_URL"]; home != "" {
			vendorID = metainfo.ReverseDNS(home, "")
		}
		component.Developer = &metainfo.Developer{ID: vendorID, Name: name}
	}
	if cfg.LatestRelease == nil {
		return component
	}
	release := *cfg.LatestRelease
	if release.Version == "" && release.Date == "" && len(release.Description) == 0 {
		return component
	}
	if release.Version == "" {
		release.Version = vars["VERSION_ID"]
	}
	if release.Version != "" {
		component.Release = &release
	}
	return component
}

func saveBrandingMetainfo(root, id string, found bool, vars map[string]string, cfg *BrandingMetainfo) error {
	if found {
		if cfg != nil {
			return fmt.Errorf("operating-system component %q already exists; branding.metainfo must be omitted", id)
		}
		return nil
	}
	return metainfo.WriteOperatingSystem(root, brandingComponent(id, vars, cfg))
}
