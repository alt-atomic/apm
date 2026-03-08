package _package

import (
	aptLib "apm/internal/common/binding/apt/lib"
	"testing"
)

func TestExtractLastMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single entry",
			input:    "* Mon Jan 15 2024 maintainer@alt\n- updated to 1.0\n- fixed bug #123",
			expected: "* Mon Jan 15 2024 maintainer@alt\n- updated to 1.0\n- fixed bug #123",
		},
		{
			name: "two entries returns first",
			input: "* Mon Jan 15 2024 maintainer@alt\n- updated to 1.0\n\n" +
				"* Fri Dec 01 2023 maintainer@alt\n- initial release",
			expected: "* Mon Jan 15 2024 maintainer@alt\n- updated to 1.0",
		},
		{
			name: "three entries returns first",
			input: "* Mon Jan 15 2024 a@b\n- v3\n\n" +
				"* Fri Dec 01 2023 a@b\n- v2\n\n" +
				"* Wed Nov 01 2023 a@b\n- v1",
			expected: "* Mon Jan 15 2024 a@b\n- v3",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no header",
			input:    "just some text\nanother line",
			expected: "",
		},
		{
			name:     "blank lines between entries",
			input:    "\n\n* Mon Jan 15 2024 a@b\n\n- change1\n\n- change2\n\n* Fri Dec 01 2023 a@b\n- old",
			expected: "* Mon Jan 15 2024 a@b\n- change1\n- change2",
		},
		{
			name:     "header only no body",
			input:    "* Mon Jan 15 2024 maintainer@alt",
			expected: "* Mon Jan 15 2024 maintainer@alt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLastMessage(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestIsChangelogHeader(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"* Mon Jan 15 2024 maintainer@alt", true},
		{"* Fri Dec 01 2023 someone", true},
		{"* a b c 2025 rest", true},
		{"- just a change line", false},
		{"not a header at all", false},
		{"* too few 2024", false},
		{"* Mon Jan 15 abcd maintainer@alt", false},
		{"*Mon Jan 15 2024 maintainer@alt", false},
		{"", false},
		{"* Mon Jan 15 0999 maintainer@alt", true},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := isChangelogHeader(tt.line)
			if got != tt.want {
				t.Errorf("isChangelogHeader(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestConvertAptPackage(t *testing.T) {
	ap := &aptLib.PackageInfo{
		Name:             "firefox",
		Version:          "1:115.0-alt1",
		Description:      "A fast web browser",
		ShortDescription: "Web browser",
		Section:          "Networking",
		Architecture:     "x86_64",
		Maintainer:       "someone@alt",
		Depends:          "libgtk3, libdbus, libgtk3",
		Provides:         "www-browser, x-www-browser",
		InstalledSize:    204800,
		DownloadSize:     65536,
		Filename:         "RPMS.main/firefox-115.0-alt1.x86_64.rpm",
		Aliases:          []string{"ff", "mozilla-firefox"},
		Files:            []string{"/usr/bin/firefox", "/usr/share/firefox"},
	}

	pkg := convertAptPackage(ap)

	if pkg.Name != "firefox" {
		t.Errorf("Name: got %q, want %q", pkg.Name, "firefox")
	}
	if pkg.Architecture != "x86_64" {
		t.Errorf("Architecture: got %q, want %q", pkg.Architecture, "x86_64")
	}
	if pkg.Section != "Networking" {
		t.Errorf("Section: got %q, want %q", pkg.Section, "Networking")
	}
	if pkg.InstalledSize != 204800 {
		t.Errorf("InstalledSize: got %d, want %d", pkg.InstalledSize, 204800)
	}
	if pkg.Size != 65536 {
		t.Errorf("Size: got %d, want %d", pkg.Size, 65536)
	}
	if pkg.Installed != false {
		t.Error("Installed should be false for new package")
	}
	if pkg.TypePackage != int(PackageTypeSystem) {
		t.Errorf("TypePackage: got %d, want %d", pkg.TypePackage, PackageTypeSystem)
	}

	if pkg.Description != "A fast web browser" {
		t.Errorf("Description: got %q", pkg.Description)
	}
	if pkg.Summary != "Web browser" {
		t.Errorf("Summary: got %q", pkg.Summary)
	}

	seen := make(map[string]bool)
	for _, d := range pkg.Depends {
		if seen[d] {
			t.Errorf("duplicate in Depends: %q", d)
		}
		seen[d] = true
	}

	if len(pkg.Aliases) != 2 {
		t.Errorf("Aliases: got %d, want 2", len(pkg.Aliases))
	}

	if len(pkg.Files) != 2 {
		t.Errorf("Files: got %d, want 2", len(pkg.Files))
	}

	if pkg.Version == "" {
		t.Error("Version should not be empty")
	}
	if pkg.VersionRaw != "1:115.0-alt1" {
		t.Errorf("VersionRaw: got %q, want %q", pkg.VersionRaw, "1:115.0-alt1")
	}
}

func TestConvertAptPackage_DescriptionFallback(t *testing.T) {
	ap := &aptLib.PackageInfo{
		Name:             "test",
		Version:          "1.0",
		ShortDescription: "Short desc only",
	}

	pkg := convertAptPackage(ap)
	if pkg.Description != "Short desc only" {
		t.Errorf("Description should fallback to summary, got %q", pkg.Description)
	}
}

func TestConvertAptPackage_EmptyDepends(t *testing.T) {
	ap := &aptLib.PackageInfo{
		Name:    "test",
		Version: "1.0",
	}

	pkg := convertAptPackage(ap)
	if pkg.Depends != nil {
		t.Errorf("Depends should be nil for empty input, got %v", pkg.Depends)
	}
	if pkg.Provides != nil {
		t.Errorf("Provides should be nil for empty input, got %v", pkg.Provides)
	}
}
