package system

import (
	"context"
	"errors"
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/swcat"
	"altlinux.space/alt-atomic/apm/internal/common/testutil"
)

func TestInfo(t *testing.T) {
	vim := _package.Package{Name: "vim", Version: "9.0", Summary: "Text editor"}
	neovim := _package.Package{Name: "neovim", Version: "0.9"}
	nano := _package.Package{Name: "nano", Version: "7.0"}

	tests := []struct {
		name        string
		packageName string
		db          *mockAptDB
		wantErr     bool
		wantErrType string
		wantPkg     string
	}{
		{
			name:        "found directly by name",
			packageName: "vim",
			db:          &mockAptDB{getByNameResult: vim},
			wantPkg:     "vim",
		},
		{
			name:        "not found, one alternative via provides",
			packageName: "vi",
			db: &mockAptDB{
				getByNameErr: errors.New("not found"),
				queryResult:  []_package.Package{vim},
			},
			wantPkg: "vim",
		},
		{
			name:        "not found, multiple alternatives suggests them in error",
			packageName: "editor",
			db: &mockAptDB{
				getByNameErr: errors.New("not found"),
				queryResult:  []_package.Package{neovim, nano},
			},
			wantErr:     true,
			wantErrType: apmerr.ErrorTypeNotFound,
		},
		{
			name:        "not found, no alternatives at all",
			packageName: "nonexistent",
			db: &mockAptDB{
				getByNameErr: errors.New("not found"),
				queryResult:  []_package.Package{},
			},
			wantErr:     true,
			wantErrType: apmerr.ErrorTypeNotFound,
		},
		{
			name:        "empty package name",
			packageName: "  ",
			db:          &mockAptDB{},
			wantErr:     true,
			wantErrType: apmerr.ErrorTypeValidation,
		},
		{
			name:        "alternatives query fails",
			packageName: "broken",
			db: &mockAptDB{
				getByNameErr: errors.New("not found"),
				queryErr:     errors.New("db connection lost"),
			},
			wantErr:     true,
			wantErrType: apmerr.ErrorTypeDatabase,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := newTestActions(nil, tt.db, nil)

			resp, err := actions.Info(context.Background(), tt.packageName)

			if tt.wantErr {
				testutil.AssertAPMError(t, err, tt.wantErrType)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.PackageInfo.Name != tt.wantPkg {
				t.Errorf("expected package %s, got %s", tt.wantPkg, resp.PackageInfo.Name)
			}
		})
	}
}

func TestMultiInfo(t *testing.T) {
	vim := _package.Package{Name: "vim", Version: "9.0"}
	curl := _package.Package{Name: "curl", Version: "8.0"}

	t.Run("all found directly", func(t *testing.T) {
		db := &mockAptDB{getByNamesResult: []_package.Package{vim, curl}}
		actions := newTestActions(nil, db, nil)

		resp, err := actions.MultiInfo(context.Background(), []string{"vim", "curl"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Packages) != 2 {
			t.Errorf("expected 2 packages, got %d", len(resp.Packages))
		}
		if len(resp.NotFound) != 0 {
			t.Errorf("expected empty notFound, got %v", resp.NotFound)
		}
	})

	t.Run("missing package found via provides fallback", func(t *testing.T) {
		db := &mockAptDB{
			getByNamesResult: []_package.Package{vim},
			queryResult:      []_package.Package{curl},
		}
		actions := newTestActions(nil, db, nil)

		resp, err := actions.MultiInfo(context.Background(), []string{"vim", "wget"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Packages) != 2 {
			t.Errorf("expected 2 packages (vim + curl via provides), got %d", len(resp.Packages))
		}
	})

	t.Run("missing package not found anywhere goes to notFound", func(t *testing.T) {
		db := &mockAptDB{
			getByNamesResult: []_package.Package{vim},
			queryResult:      []_package.Package{},
		}
		actions := newTestActions(nil, db, nil)

		resp, err := actions.MultiInfo(context.Background(), []string{"vim", "nonexistent"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Packages) != 1 {
			t.Errorf("expected 1 package, got %d", len(resp.Packages))
		}
		if len(resp.NotFound) != 1 || resp.NotFound[0] != "nonexistent" {
			t.Errorf("expected notFound=[nonexistent], got %v", resp.NotFound)
		}
	})

	t.Run("empty list returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, &mockAptDB{}, nil)
		_, err := actions.MultiInfo(context.Background(), []string{})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("all whitespace names returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, &mockAptDB{}, nil)
		_, err := actions.MultiInfo(context.Background(), []string{"  ", "", " "})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("GetPackagesByNames DB error propagates", func(t *testing.T) {
		db := &mockAptDB{getByNamesErr: errors.New("db failure")}
		actions := newTestActions(nil, db, nil)
		_, err := actions.MultiInfo(context.Background(), []string{"vim"})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeDatabase)
	})
}

func TestSearch(t *testing.T) {
	pkgs := []_package.Package{
		{Name: "vim", Summary: "Text editor"},
		{Name: "vim-enhanced", Summary: "Enhanced vim"},
	}

	tests := []struct {
		name        string
		query       string
		db          *mockAptDB
		wantErr     bool
		wantErrType string
		wantCount   int
	}{
		{
			name:      "found packages",
			query:     "vim",
			db:        &mockAptDB{searchResult: pkgs},
			wantCount: 2,
		},
		{
			name:        "nothing found returns not found",
			query:       "zzzzz",
			db:          &mockAptDB{searchResult: []_package.Package{}},
			wantErr:     true,
			wantErrType: apmerr.ErrorTypeNotFound,
		},
		{
			name:        "empty query returns validation error",
			query:       "  ",
			db:          &mockAptDB{},
			wantErr:     true,
			wantErrType: apmerr.ErrorTypeValidation,
		},
		{
			name:        "database error propagates",
			query:       "vim",
			db:          &mockAptDB{searchErr: errors.New("connection lost")},
			wantErr:     true,
			wantErrType: apmerr.ErrorTypeDatabase,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := newTestActions(nil, tt.db, nil)
			resp, err := actions.Search(context.Background(), tt.query, false)

			if tt.wantErr {
				testutil.AssertAPMError(t, err, tt.wantErrType)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(resp.Packages) != tt.wantCount {
				t.Errorf("expected %d packages, got %d", tt.wantCount, len(resp.Packages))
			}
		})
	}
}

func TestList(t *testing.T) {
	pkgs := []_package.Package{
		{Name: "bash", Installed: true},
		{Name: "zsh", Installed: false},
	}

	t.Run("returns packages with total count", func(t *testing.T) {
		db := &mockAptDB{countResult: 100, queryResult: pkgs}
		actions := newTestActions(nil, db, nil)

		resp, err := actions.List(context.Background(), ListParams{Limit: 10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Packages) != 2 {
			t.Errorf("expected 2 packages, got %d", len(resp.Packages))
		}
		if resp.TotalCount != 100 {
			t.Errorf("expected totalCount=100, got %d", resp.TotalCount)
		}
	})

	t.Run("empty result returns not found", func(t *testing.T) {
		db := &mockAptDB{queryResult: []_package.Package{}}
		actions := newTestActions(nil, db, nil)

		_, err := actions.List(context.Background(), ListParams{})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNotFound)
	})

	t.Run("forceUpdate triggers apt update before query", func(t *testing.T) {
		db := &mockAptDB{countResult: 1, queryResult: []_package.Package{{Name: "test"}}}
		actions := newTestActions(&mockAptActions{}, db, nil)

		resp, err := actions.List(context.Background(), ListParams{ForceUpdate: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Packages) != 1 {
			t.Errorf("expected 1 package, got %d", len(resp.Packages))
		}
	})

	t.Run("forceUpdate apt error stops execution", func(t *testing.T) {
		apt := &mockAptActions{updateErr: errors.New("apt update failed")}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		_, err := actions.List(context.Background(), ListParams{ForceUpdate: true})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})

	t.Run("count error propagates", func(t *testing.T) {
		db := &mockAptDB{countErr: errors.New("count failed")}
		actions := newTestActions(nil, db, nil)

		_, err := actions.List(context.Background(), ListParams{})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeDatabase)
	})
}

func TestSections(t *testing.T) {
	t.Run("returns sections from DB", func(t *testing.T) {
		db := &mockAptDB{sectionsResult: []string{"editors", "utils", "libs"}}
		actions := newTestActions(nil, db, nil)

		resp, err := actions.Sections(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Sections) != 3 {
			t.Errorf("expected 3 sections, got %d", len(resp.Sections))
		}
	})

	t.Run("database error propagates", func(t *testing.T) {
		db := &mockAptDB{sectionsErr: errors.New("db error")}
		actions := newTestActions(nil, db, nil)

		_, err := actions.Sections(context.Background())
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeDatabase)
	})
}

func TestFormatPackageOutput(t *testing.T) {
	pkg := _package.Package{
		Name:       "vim",
		Summary:    "Text editor",
		Version:    "9.0",
		Installed:  true,
		Maintainer: "packager@alt",
		Size:       12345,
		Section:    "editors",
	}
	actions := newTestActions(nil, &mockAptDB{}, nil)

	t.Run("single full preserves all fields", func(t *testing.T) {
		result := actions.FormatPackageOutput(pkg, true)
		full, ok := result.(_package.Package)
		if !ok {
			t.Fatalf("expected Package, got %T", result)
		}
		if full.Section != "editors" || full.Size != 12345 {
			t.Error("full output should preserve all fields")
		}
	})

	t.Run("single short strips extra fields", func(t *testing.T) {
		result := actions.FormatPackageOutput(pkg, false)
		short, ok := result.(ShortPackageResponse)
		if !ok {
			t.Fatalf("expected ShortPackageResponse, got %T", result)
		}
		if short.Name != "vim" || short.Version != "9.0" || !short.Installed {
			t.Errorf("wrong short values: %+v", short)
		}
	})

	t.Run("slice short converts all", func(t *testing.T) {
		pkgs := []_package.Package{pkg, {Name: "nano", Version: "7.0"}}
		result := actions.FormatPackageOutput(pkgs, false)
		short, ok := result.([]ShortPackageResponse)
		if !ok {
			t.Fatalf("expected []ShortPackageResponse, got %T", result)
		}
		if len(short) != 2 || short[1].Name != "nano" {
			t.Errorf("unexpected: %+v", short)
		}
	})

	t.Run("unknown type returns nil", func(t *testing.T) {
		if actions.FormatPackageOutput("string", false) != nil {
			t.Error("expected nil for unknown type")
		}
	})
}

func TestEnrichWithAppStream(t *testing.T) {
	comp := swcat.Component{
		Type:    "desktop-application",
		ID:      "org.vim.Vim",
		PkgName: "vim",
	}

	t.Run("enriches packages when format is json and HasAppStream is true", func(t *testing.T) {
		appStream := &mockAppStream{
			result: map[string][]swcat.Component{"vim": {comp}},
		}
		actions := newTestActions(nil, &mockAptDB{
			getByNameResult: _package.Package{Name: "vim", HasAppStream: true},
		}, nil)
		actions.appConfig = testutil.JsonAppConfig()
		actions.serviceAppStreamDB = appStream

		resp, err := actions.Info(context.Background(), "vim")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.PackageInfo.AppStream) != 1 {
			t.Errorf("expected 1 appstream component, got %d", len(resp.PackageInfo.AppStream))
		}
		if resp.PackageInfo.AppStream[0].ID != "org.vim.Vim" {
			t.Errorf("expected component ID org.vim.Vim, got %s", resp.PackageInfo.AppStream[0].ID)
		}
	})

	t.Run("skips enrichment when format is text", func(t *testing.T) {
		appStream := &mockAppStream{
			result: map[string][]swcat.Component{"vim": {comp}},
		}
		actions := newTestActions(nil, &mockAptDB{
			getByNameResult: _package.Package{Name: "vim", HasAppStream: true},
		}, nil)
		actions.serviceAppStreamDB = appStream

		resp, err := actions.Info(context.Background(), "vim")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.PackageInfo.AppStream) != 0 {
			t.Errorf("expected no appstream in text format, got %d", len(resp.PackageInfo.AppStream))
		}
	})

	t.Run("skips packages without HasAppStream flag", func(t *testing.T) {
		appStream := &mockAppStream{
			result: map[string][]swcat.Component{"vim": {comp}},
		}
		actions := newTestActions(nil, &mockAptDB{
			getByNameResult: _package.Package{Name: "vim", HasAppStream: false},
		}, nil)
		actions.appConfig = testutil.JsonAppConfig()
		actions.serviceAppStreamDB = appStream

		resp, err := actions.Info(context.Background(), "vim")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.PackageInfo.AppStream) != 0 {
			t.Errorf("expected no appstream when HasAppStream=false, got %d", len(resp.PackageInfo.AppStream))
		}
	})

	t.Run("appstream error does not fail the request", func(t *testing.T) {
		appStream := &mockAppStream{err: errors.New("db error")}
		actions := newTestActions(nil, &mockAptDB{
			getByNameResult: _package.Package{Name: "vim", HasAppStream: true},
		}, nil)
		actions.appConfig = testutil.JsonAppConfig()
		actions.serviceAppStreamDB = appStream

		resp, err := actions.Info(context.Background(), "vim")
		if err != nil {
			t.Fatalf("expected no error even when appstream fails, got: %v", err)
		}
		if resp.PackageInfo.Name != "vim" {
			t.Errorf("expected package vim, got %s", resp.PackageInfo.Name)
		}
	})
}
