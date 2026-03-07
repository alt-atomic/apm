package _package

import (
	"testing"
)

func TestToDBModelAndBack(t *testing.T) {
	original := Package{
		Name:             "vim",
		Architecture:     "x86_64",
		Section:          "Editors",
		InstalledSize:    10240,
		Maintainer:       "maintainer@alt",
		Version:          "9.0",
		VersionRaw:       "2:9.0-alt1",
		VersionInstalled: "9.0",
		Depends:          []string{"libncurses", "libgpm"},
		Aliases:          []string{"vi", "vim-enhanced"},
		Provides:         []string{"editor"},
		Size:             4096,
		Filename:         "RPMS.main/vim-9.0-alt1.x86_64.rpm",
		Summary:          "Vi IMproved",
		Description:      "Vim is a text editor",
		Changelog:        "* Mon Jan 15 2024 a@b\n- updated",
		Installed:        true,
		TypePackage:      int(PackageTypeSystem),
		Files:            []string{"/usr/bin/vim", "/usr/bin/vimdiff"},
	}

	dbModel := original.toDBModel()
	restored := dbModel.fromDBModel()

	// Scalar fields
	if restored.Name != original.Name {
		t.Errorf("Name: got %q, want %q", restored.Name, original.Name)
	}
	if restored.Architecture != original.Architecture {
		t.Errorf("Architecture mismatch")
	}
	if restored.Section != original.Section {
		t.Errorf("Section mismatch")
	}
	if restored.InstalledSize != original.InstalledSize {
		t.Errorf("InstalledSize: got %d, want %d", restored.InstalledSize, original.InstalledSize)
	}
	if restored.Maintainer != original.Maintainer {
		t.Errorf("Maintainer mismatch")
	}
	if restored.Version != original.Version {
		t.Errorf("Version mismatch")
	}
	if restored.VersionRaw != original.VersionRaw {
		t.Errorf("VersionRaw mismatch")
	}
	if restored.VersionInstalled != original.VersionInstalled {
		t.Errorf("VersionInstalled mismatch")
	}
	if restored.Size != original.Size {
		t.Errorf("Size mismatch")
	}
	if restored.Filename != original.Filename {
		t.Errorf("Filename mismatch")
	}
	if restored.Summary != original.Summary {
		t.Errorf("Summary mismatch")
	}
	if restored.Description != original.Description {
		t.Errorf("Description mismatch")
	}
	if restored.Changelog != original.Changelog {
		t.Errorf("Changelog mismatch")
	}
	if restored.Installed != original.Installed {
		t.Errorf("Installed: got %v, want %v", restored.Installed, original.Installed)
	}
	if restored.TypePackage != original.TypePackage {
		t.Errorf("TypePackage: got %d, want %d", restored.TypePackage, original.TypePackage)
	}

	assertSliceEqual(t, "Depends", restored.Depends, original.Depends)
	assertSliceEqual(t, "Aliases", restored.Aliases, original.Aliases)
	assertSliceEqual(t, "Provides", restored.Provides, original.Provides)
	assertSliceEqual(t, "Files", restored.Files, original.Files)
}

func TestToDBModel_EmptySlices(t *testing.T) {
	pkg := Package{Name: "empty", Version: "1.0"}
	dbModel := pkg.toDBModel()

	if dbModel.Depends != "" {
		t.Errorf("Depends should be empty, got %q", dbModel.Depends)
	}
	if dbModel.Aliases != "" {
		t.Errorf("Aliases should be empty, got %q", dbModel.Aliases)
	}
	if dbModel.Provides != "" {
		t.Errorf("Provides should be empty, got %q", dbModel.Provides)
	}
	if dbModel.Files != "" {
		t.Errorf("Files should be empty, got %q", dbModel.Files)
	}
}

func TestFromDBModel_EmptyStrings(t *testing.T) {
	dbPkg := DBPackage{Name: "empty", Version: "1.0"}
	pkg := dbPkg.fromDBModel()

	if pkg.Depends != nil {
		t.Errorf("Depends should be nil, got %v", pkg.Depends)
	}
	if pkg.Aliases != nil {
		t.Errorf("Aliases should be nil, got %v", pkg.Aliases)
	}
	if pkg.Provides != nil {
		t.Errorf("Provides should be nil, got %v", pkg.Provides)
	}
	if pkg.Files != nil {
		t.Errorf("Files should be nil, got %v", pkg.Files)
	}
}

func TestFromDBModel_WhitespaceOnly(t *testing.T) {
	dbPkg := DBPackage{
		Name:    "test",
		Version: "1.0",
		Depends: "   ",
		Aliases: "  ",
	}
	pkg := dbPkg.fromDBModel()

	if pkg.Depends != nil {
		t.Errorf("Depends should be nil for whitespace, got %v", pkg.Depends)
	}
	if pkg.Aliases != nil {
		t.Errorf("Aliases should be nil for whitespace, got %v", pkg.Aliases)
	}
}

func TestFromDBModel_HasAppStream(t *testing.T) {
	id := uint(42)
	dbPkg := DBPackage{Name: "app", Version: "1.0", IDAppStream: &id}
	pkg := dbPkg.fromDBModel()

	if !pkg.HasAppStream {
		t.Error("HasAppStream should be true when IDAppStream is set")
	}

	dbPkg2 := DBPackage{Name: "lib", Version: "1.0"}
	pkg2 := dbPkg2.fromDBModel()

	if pkg2.HasAppStream {
		t.Error("HasAppStream should be false when IDAppStream is nil")
	}
}

func assertSliceEqual(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: len got %d, want %d (%v vs %v)", name, len(got), len(want), got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %q, want %q", name, i, got[i], want[i])
		}
	}
}
