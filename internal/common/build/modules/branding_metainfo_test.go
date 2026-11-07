package modules

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/metainfo"
	"github.com/goccy/go-yaml"
)

func TestBrandingMetainfoPresence(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, configured := range []bool{false, true} {
			t.Run(strings.Join([]string{map[bool]string{true: "existing", false: "missing"}[existing], map[bool]string{true: "configured", false: "default"}[configured]}, "/"), func(t *testing.T) {
				root := t.TempDir()
				vars := map[string]string{"ID": "onyx", "HOME_URL": "https://www.alt-atomic.org/", "NAME": "ALT Atomic Onyx", "VERSION_ID": "0.6", "VENDOR_URL": "https://alt-atomic.org/", "VENDOR_NAME": "ALT Atomic Team"}
				path := filepath.Join(root, "usr/share/metainfo/org.alt_atomic.onyx.metainfo.xml")
				original := `<component type="operating-system"><id>org.alt_atomic.onyx</id><name>Existing</name><summary>Existing system</summary><metadata_license>FSFAP</metadata_license></component>`
				if existing {
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(original), 0644); err != nil {
						t.Fatal(err)
					}
				}
				var cfg *BrandingMetainfo
				if configured {
					cfg = &BrandingMetainfo{Summary: map[string]string{"C": "Custom summary"}}
				}
				err := saveBrandingMetainfo(root, "org.alt_atomic.onyx", existing, vars, cfg)
				if existing && configured {
					if err == nil || !strings.Contains(err.Error(), "branding.metainfo must be omitted") {
						t.Fatalf("expected component conflict, got %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if existing {
					if string(data) != original {
						t.Fatal("existing component was overwritten")
					}
					return
				}
				var doc struct {
					XMLName xml.Name
					ID      string `xml:"id"`
					Name    string `xml:"name"`
					Version string `xml:"releases>release>unused"`
				}
				if err := xml.Unmarshal(data, &doc); err != nil {
					t.Fatal(err)
				}
				if doc.XMLName.Local != "component" || doc.ID != "org.alt_atomic.onyx" || doc.Name != vars["NAME"] {
					t.Fatalf("unexpected component: %s", data)
				}
				for _, want := range []string{`type="operating-system"`, `version="0.6"`, `<metadata_license>FSFAP</metadata_license>`, `<developer id="org.alt_atomic">`} {
					if !strings.Contains(string(data), want) {
						t.Errorf("missing %s in %s", want, data)
					}
				}
			})
		}
	}
}

func TestBrandingMetainfoYAML(t *testing.T) {
	var b BrandingBody
	err := yaml.Unmarshal([]byte(`metainfo:
  name:
    ru: Оникс
  description:
    C: '<p>A &amp; B</p>'
    ru: '<p>Описание</p>'
  x-variants:
    - image: registry.example.org/onyx:latest
      name:
        C: Onyx
  latest-release:
    version: "0.5"
    date: "2026-10-10"
    description:
      C: '<p>Release notes</p>'
`), &b)
	if err != nil {
		t.Fatal(err)
	}
	data, err := metainfo.RenderOperatingSystem(brandingComponent("onyx", map[string]string{"ID": "onyx", "NAME": "Onyx", "VERSION_ID": "0.6"}, b.Metainfo))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<name>Onyx</name>`, `xml:lang="ru"`, `<p>A &amp; B</p>`, `<x-variants>`, `<image>registry.example.org/onyx:latest</image>`, `version="0.5" date="2026-10-10"`, `<p>Release notes</p>`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %s in %s", want, data)
		}
	}
	if strings.Contains(string(data), `<description xml:lang=`) || !strings.Contains(string(data), `<p xml:lang="ru">Описание</p>`) {
		t.Errorf("translations must be on description paragraphs: %s", data)
	}

	if strings.Contains(string(data), "&lt;p&gt;") {
		t.Fatal("description markup escaped")
	}
	b.Metainfo.Description = map[string]string{"C": "</description><id>injected</id>"}
	if _, err := metainfo.RenderOperatingSystem(brandingComponent("onyx", map[string]string{"ID": "onyx"}, b.Metainfo)); err == nil {
		t.Fatal("invalid XML fragment accepted")
	}
}

func TestBrandingMetainfoPartialRelease(t *testing.T) {
	data, err := metainfo.RenderOperatingSystem(brandingComponent("onyx", map[string]string{"ID": "onyx", "NAME": "A & B", "VERSION_ID": "0.6"}, &BrandingMetainfo{LatestRelease: &metainfo.Release{Date: "2026-11-07"}}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `version="0.6" date="2026-11-07"`) || !strings.Contains(string(data), `<name>A &amp; B</name>`) {
		t.Fatalf("missing fallbacks/escaping: %s", data)
	}
}

func TestBrandingComponentDoesNotRetainDefaults(t *testing.T) {
	cfg := &BrandingMetainfo{
		Name:          map[string]string{"ru": "Оникс"},
		Summary:       map[string]string{"C": "Custom summary"},
		LatestRelease: &metainfo.Release{Date: "2026-11-07"},
	}
	first := brandingComponent("org.example.First", map[string]string{"NAME": "First", "VERSION_ID": "1"}, cfg)
	second := brandingComponent("org.example.Second", map[string]string{"NAME": "Second", "VERSION_ID": "2"}, cfg)
	if first.Name["C"] != "First" || second.Name["C"] != "Second" {
		t.Fatalf("defaults leaked between components: %v, %v", first.Name, second.Name)
	}
	if first.Release.Version != "1" || second.Release.Version != "2" {
		t.Fatalf("release defaults leaked between components: %v, %v", first.Release, second.Release)
	}
	if second.Name["ru"] != "Оникс" || second.Summary["C"] != "Custom summary" {
		t.Fatalf("explicit metadata was replaced: %+v", second)
	}
	if _, exists := cfg.Name["C"]; exists || cfg.LatestRelease.Version != "" {
		t.Fatal("resolving defaults modified the branding configuration")
	}
}
