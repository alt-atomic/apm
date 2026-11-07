package modules

import (
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
	data, err := renderBrandingMetainfo("onyx", map[string]string{"ID": "onyx", "NAME": "Onyx", "VERSION_ID": "0.6"}, b.Metainfo)
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
	if _, err := renderBrandingMetainfo("onyx", map[string]string{"ID": "onyx"}, b.Metainfo); err == nil {
		t.Fatal("invalid XML fragment accepted")
	}
}

func TestCurrentDistroComponentID(t *testing.T) {
	id, err := currentOSComponentID()
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("libappstream returned an empty OS component ID")
	}
}

func TestBrandingComponentMissing(t *testing.T) {
	found, err := сomponentExists("org.example.apm-test-nonexistent-system-component")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("unexpected operating-system component")
	}
}

func TestBrandingMetainfoPartialRelease(t *testing.T) {
	data, err := renderBrandingMetainfo("onyx", map[string]string{"ID": "onyx", "NAME": "A & B", "VERSION_ID": "0.6"}, &BrandingMetainfo{LatestRelease: &BrandingRelease{Date: "2026-11-07"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `version="0.6" date="2026-11-07"`) || !strings.Contains(string(data), `<name>A &amp; B</name>`) {
		t.Fatalf("missing fallbacks/escaping: %s", data)
	}
}

// This checks the generated document against the installed AppStream validator
// when it is available, in addition to the dependency-free regression tests.
func TestBrandingMetainfoAppStream(t *testing.T) {
	cli, err := exec.LookPath("appstreamcli")
	if err != nil {
		t.Skip("appstreamcli is not installed")
	}
	data, err := renderBrandingMetainfo("org.alt_atomic.onyx", map[string]string{"ID": "onyx", "NAME": "ALT Atomic Onyx", "HOME_URL": "https://alt-atomic.org/", "VENDOR_URL": "https://alt-atomic.org/", "VENDOR_NAME": "ALT Atomic Team", "VERSION_ID": "0.6"}, &BrandingMetainfo{
		Summary:       map[string]string{"C": "Atomic desktop operating system"},
		Description:   map[string]string{"C": "<p>An atomic desktop operating system with the GNOME desktop environment.</p>", "ru": "<p>Атомарная операционная система с рабочим столом GNOME.</p>"},
		LatestRelease: &BrandingRelease{Date: "2026-10-10", Description: map[string]string{"C": "<p>Updated system components.</p>", "ru": "<p>Обновлены компоненты системы.</p>"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "org.alt_atomic.onyx.metainfo.xml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(cli, "validate", "--no-net", path).CombinedOutput(); err != nil {
		t.Fatalf("AppStream validation failed: %v\n%s\n%s", err, output, data)
	}
}
