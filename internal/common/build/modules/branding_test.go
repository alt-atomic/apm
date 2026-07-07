package modules

import (
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
