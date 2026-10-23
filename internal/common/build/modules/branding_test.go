package modules

import (
	"maps"
	"strings"
	"testing"
)

func TestRenderOsReleaseStableOrder(t *testing.T) {
	vars := map[string]string{
		"NAME":       "Selenite",
		"ID":         "selenite",
		"VERSION_ID": "1.0",
		"BUILD_ID":   "Selenite 1.0",
		"VARIANT":    "Cloud",
	}
	fileOrder := []string{"NAME", "VERSION_ID", "ID"}

	expected := strings.Join([]string{
		`NAME="Selenite"`,
		`VERSION_ID="1.0"`,
		`ID="selenite"`,
		`BUILD_ID="Selenite 1.0"`,
		`VARIANT="Cloud"`,
	}, "\n") + "\n"

	for i := range 50 {
		if got := renderOsRelease(vars, fileOrder); got != expected {
			t.Fatalf("unstable render on run %d:\n%s\nwant:\n%s", i, got, expected)
		}
	}
}

func TestRenderOsReleaseNoFileOrder(t *testing.T) {
	vars := map[string]string{"B": "2", "A": "1", "C": "3"}

	expected := "A=\"1\"\nB=\"2\"\nC=\"3\"\n"
	if got := renderOsRelease(vars, nil); got != expected {
		t.Fatalf("got:\n%s\nwant:\n%s", got, expected)
	}
}

func TestQuoteOsReleaseValue(t *testing.T) {
	cases := map[string]string{
		`plain`:        `"plain"`,
		`with "quote"`: `"with \"quote\""`,
		`back\slash`:   `"back\\slash"`,
		`dollar $VAR`:  `"dollar \$VAR"`,
		"back`tick":    "\"back\\`tick\"",
	}
	for in, want := range cases {
		if got := quoteOsReleaseValue(in); got != want {
			t.Fatalf("quote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestApplyReleaseOverridesDeletesEmptyValues(t *testing.T) {
	vars := map[string]string{
		"ID":         "onyx",
		"NAME":       "ALT Atomic",
		"VARIANT":    "Onyx",
		"VARIANT_ID": "onyx",
	}

	applyReleaseOverrides(vars, map[string]string{
		"NAME":       "ALT Atomic Onyx",
		"VARIANT":    "",
		"VARIANT_ID": "",
	})

	want := map[string]string{
		"ID":   "onyx",
		"NAME": "ALT Atomic Onyx",
	}
	if !maps.Equal(vars, want) {
		t.Fatalf("got %#v, want %#v", vars, want)
	}
}

func TestNormalizeAtomicOsRelease(t *testing.T) {
	tests := []struct {
		name      string
		buildType string
		imageID   string
		variant   string
		variantID string
		want      map[string]string
	}{
		{
			name:      "stable default",
			buildType: "stable",
			imageID:   "alt-atomic-onyx",
			want: map[string]string{
				"ID":            "onyx",
				"VERSION":       "16.10.2026",
				"VERSION_ID":    "20261016",
				"RELEASE_TYPE":  "stable",
				"PRETTY_NAME":   "ALT Atomic Onyx",
				"CPE_NAME":      "cpe:/o:alt-atomic:onyx:20261016:stable",
				"IMAGE_ID":      "alt-atomic-onyx",
				"IMAGE_VERSION": "20261016",
			},
		},
		{
			name:      "nightly default",
			buildType: "nightly",
			imageID:   "alt-atomic-onyx",
			want: map[string]string{
				"ID":            "onyx-nightly",
				"VERSION":       "16.10.2026",
				"VERSION_ID":    "20261016",
				"RELEASE_TYPE":  "development",
				"PRETTY_NAME":   "ALT Atomic Onyx Nightly",
				"CPE_NAME":      "cpe:/o:alt-atomic:onyx:20261016:nightly",
				"IMAGE_ID":      "alt-atomic-onyx-nightly",
				"IMAGE_VERSION": "20261016",
			},
		},
		{
			name:      "stable NVIDIA",
			buildType: "stable",
			imageID:   "alt-atomic-onyx-nvidia",
			variant:   "NVIDIA",
			variantID: "nvidia",
			want: map[string]string{
				"ID":            "onyx",
				"VERSION":       "16.10.2026 (NVIDIA)",
				"VERSION_ID":    "20261016-nvidia",
				"RELEASE_TYPE":  "stable",
				"PRETTY_NAME":   "ALT Atomic Onyx (NVIDIA)",
				"CPE_NAME":      "cpe:/o:alt-atomic:onyx:20261016:stable:nvidia",
				"IMAGE_ID":      "alt-atomic-onyx-nvidia",
				"IMAGE_VERSION": "20261016-nvidia",
			},
		},
		{
			name:      "nightly NVIDIA",
			buildType: "nightly",
			imageID:   "alt-atomic-onyx-nvidia",
			variant:   "NVIDIA",
			variantID: "nvidia",
			want: map[string]string{
				"ID":            "onyx-nightly",
				"VERSION":       "16.10.2026 (NVIDIA)",
				"VERSION_ID":    "20261016-nvidia",
				"RELEASE_TYPE":  "development",
				"PRETTY_NAME":   "ALT Atomic Onyx Nightly (NVIDIA)",
				"CPE_NAME":      "cpe:/o:alt-atomic:onyx:20261016:nightly:nvidia",
				"IMAGE_ID":      "alt-atomic-onyx-nvidia-nightly",
				"IMAGE_VERSION": "20261016-nvidia",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := map[string]string{
				"NAME":        "ALT Atomic Onyx",
				"VERSION":     "20261016",
				"ID":          "onyx",
				"ID_LIKE":     "alt-atomic altlinux",
				"PRETTY_NAME": "stale value",
				"CPE_NAME":    "cpe:/o:alt-atomic:onyx:old:stable",
				"IMAGE_ID":    tt.imageID,
				"BUILD_ID":    "stale value",
			}
			if tt.variant != "" {
				vars["VARIANT"] = tt.variant
				vars["VARIANT_ID"] = tt.variantID
			}

			if err := normalizeAtomicOsRelease(vars, tt.buildType); err != nil {
				t.Fatal(err)
			}

			for key, want := range tt.want {
				if got := vars[key]; got != want {
					t.Errorf("%s = %q, want %q", key, got, want)
				}
			}
			if _, ok := vars["BUILD_ID"]; ok {
				t.Error("BUILD_ID must be unset")
			}
			if tt.variantID == "" {
				if _, ok := vars["VARIANT"]; ok {
					t.Error("VARIANT must be unset for the default image")
				}
				if _, ok := vars["VARIANT_ID"]; ok {
					t.Error("VARIANT_ID must be unset for the default image")
				}
			}
		})
	}
}

func TestNormalizeAtomicOsReleasePreservesBrandNamespace(t *testing.T) {
	vars := map[string]string{
		"NAME":       "Example Workstation",
		"VERSION":    "20261016",
		"ID":         "workstation",
		"ID_LIKE":    "example linux",
		"CPE_NAME":   "cpe:/o:example:workstation:old:stable",
		"IMAGE_ID":   "example-workstation-enterprise",
		"VARIANT":    "Enterprise",
		"VARIANT_ID": "enterprise",
	}

	if err := normalizeAtomicOsRelease(vars, "edge"); err != nil {
		t.Fatal(err)
	}

	if got, want := vars["ID_LIKE"], "example linux"; got != want {
		t.Errorf("ID_LIKE = %q, want %q", got, want)
	}
	if got, want := vars["CPE_NAME"], "cpe:/o:example:workstation:20261016:edge:enterprise"; got != want {
		t.Errorf("CPE_NAME = %q, want %q", got, want)
	}
	if got, want := vars["IMAGE_ID"], "example-workstation-enterprise-edge"; got != want {
		t.Errorf("IMAGE_ID = %q, want %q", got, want)
	}
}
