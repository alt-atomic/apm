package version

import (
	"testing"
)

func TestParseVersion_Full(t *testing.T) {
	v, err := ParseVersion("1.2.3")
	if err != nil {
		t.Fatal(err)
	}

	if v.Major != 1 || v.Minor != 2 || v.Patch != 3 {
		t.Errorf("expected 1.2.3, got %d.%d.%d", v.Major, v.Minor, v.Patch)
	}
	if v.Commits != 0 {
		t.Errorf("expected 0 commits, got %d", v.Commits)
	}
	if v.Value != "1.2.3" {
		t.Errorf("expected value '1.2.3', got '%s'", v.Value)
	}
}

func TestParseVersion_WithVPrefix(t *testing.T) {
	v, err := ParseVersion("v2.0.1")
	if err != nil {
		t.Fatal(err)
	}

	if v.Major != 2 || v.Minor != 0 || v.Patch != 1 {
		t.Errorf("expected 2.0.1, got %d.%d.%d", v.Major, v.Minor, v.Patch)
	}
}

func TestParseVersion_WithCommits(t *testing.T) {
	v, err := ParseVersion("1.5.0+42")
	if err != nil {
		t.Fatal(err)
	}

	if v.Major != 1 || v.Minor != 5 || v.Patch != 0 {
		t.Errorf("expected 1.5.0, got %d.%d.%d", v.Major, v.Minor, v.Patch)
	}
	if v.Commits != 42 {
		t.Errorf("expected 42 commits, got %d", v.Commits)
	}
	if v.Value != "1.5.0+42" {
		t.Errorf("expected value '1.5.0+42', got '%s'", v.Value)
	}
}

func TestParseVersion_WithVAndCommits(t *testing.T) {
	v, err := ParseVersion("v0.9.1+7")
	if err != nil {
		t.Fatal(err)
	}

	if v.Major != 0 || v.Minor != 9 || v.Patch != 1 {
		t.Errorf("expected 0.9.1, got %d.%d.%d", v.Major, v.Minor, v.Patch)
	}
	if v.Commits != 7 {
		t.Errorf("expected 7 commits, got %d", v.Commits)
	}
}

func TestParseVersion_InvalidFormat(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"two parts", "1.2"},
		{"one part", "1"},
		{"non-numeric major", "a.2.3"},
		{"non-numeric minor", "1.b.3"},
		{"non-numeric patch", "1.2.c"},
		{"non-numeric commits", "1.2.3+abc"},
		{"empty string", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseVersion(c.input)
			if err == nil {
				t.Errorf("ParseVersion(%q) should return error", c.input)
			}
		})
	}
}
