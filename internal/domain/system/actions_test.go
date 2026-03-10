package system

import (
	"apm/internal/common/apmerr"
	_package "apm/internal/common/apt/package"
	aptLib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/build"
	"apm/internal/common/filter"
	"apm/internal/common/swcat"
	"apm/internal/common/testutil"
	"apm/internal/domain/system/service"
	"context"
	"errors"
	"syscall"
	"testing"
)

type mockAptActions struct {
	overrides       map[string]string
	checkRemoveRes  *aptLib.PackageChanges
	checkRemoveErr  error
	checkUpgradeRes *aptLib.PackageChanges
	checkUpgradeErr error
	prepareInstall  []string
	prepareRemove   []string
	prepareErr      error
	findChanges     *aptLib.PackageChanges
	findErr         error
	updateErr       error
}

func (m *mockAptActions) SetAptConfigOverrides(o map[string]string) { m.overrides = o }
func (m *mockAptActions) GetAptConfigOverrides() map[string]string  { return m.overrides }
func (m *mockAptActions) CheckRemove(_ context.Context, _ []string, _ bool, _ bool) (*aptLib.PackageChanges, error) {
	return m.checkRemoveRes, m.checkRemoveErr
}
func (m *mockAptActions) CheckUpgrade(_ context.Context) (*aptLib.PackageChanges, error) {
	return m.checkUpgradeRes, m.checkUpgradeErr
}
func (m *mockAptActions) PrepareInstallPackages(_ context.Context, _ []string) ([]string, []string, error) {
	return m.prepareInstall, m.prepareRemove, m.prepareErr
}
func (m *mockAptActions) FindPackage(_ context.Context, _ []string, _ []string, _ bool, _ bool, _ bool) ([]string, []string, []_package.Package, *aptLib.PackageChanges, error) {
	return nil, nil, nil, m.findChanges, m.findErr
}
func (m *mockAptActions) Remove(_ context.Context, _ []string, _ bool, _ bool) error { return nil }
func (m *mockAptActions) CombineInstallRemovePackages(_ context.Context, _ []string, _ []string, _ bool, _ bool, _ bool) error {
	return nil
}
func (m *mockAptActions) Update(_ context.Context, _ ...bool) ([]_package.Package, error) {
	return nil, m.updateErr
}
func (m *mockAptActions) UpdateDBOnly(_ context.Context, _ ...bool) ([]_package.Package, error) {
	return nil, nil
}
func (m *mockAptActions) AptUpdate(_ context.Context, _ ...bool) error { return nil }
func (m *mockAptActions) GetInstalledPackages(_ context.Context, _ ...bool) (map[string]string, error) {
	return nil, nil
}
func (m *mockAptActions) Upgrade(_ context.Context, _ bool) error               { return nil }
func (m *mockAptActions) ReinstallPackages(_ context.Context, _ []string) error { return nil }
func (m *mockAptActions) Install(_ context.Context, _ []string, _ bool) error   { return nil }

type mockAptDB struct {
	dbExistErr       error
	getByNameResult  _package.Package
	getByNameErr     error
	getByNamesResult []_package.Package
	getByNamesErr    error
	queryResult      []_package.Package
	queryErr         error
	countResult      int64
	countErr         error
	searchResult     []_package.Package
	searchErr        error
	sectionsResult   []string
	sectionsErr      error
}

func (m *mockAptDB) PackageDatabaseExist(_ context.Context) error { return m.dbExistErr }
func (m *mockAptDB) GetPackageByName(_ context.Context, _ string) (_package.Package, error) {
	return m.getByNameResult, m.getByNameErr
}
func (m *mockAptDB) GetPackagesByNames(_ context.Context, _ []string) ([]_package.Package, error) {
	return m.getByNamesResult, m.getByNamesErr
}
func (m *mockAptDB) QueryHostImagePackages(_ context.Context, _ []filter.Filter, _ string, _ string, _ int, _ int) ([]_package.Package, error) {
	return m.queryResult, m.queryErr
}
func (m *mockAptDB) CountHostImagePackages(_ context.Context, _ []filter.Filter) (int64, error) {
	return m.countResult, m.countErr
}
func (m *mockAptDB) SearchPackagesByNameLike(_ context.Context, _ string, _ bool) ([]_package.Package, error) {
	return m.searchResult, m.searchErr
}
func (m *mockAptDB) SearchPackagesMultiLimit(_ context.Context, _ string, _ int, _ bool) ([]_package.Package, error) {
	return m.searchResult, m.searchErr
}
func (m *mockAptDB) SyncPackageInstallationInfo(_ context.Context, _ map[string]string) error {
	return nil
}
func (m *mockAptDB) UpdateAppStreamLinks(_ context.Context) error { return nil }
func (m *mockAptDB) GetSections(_ context.Context) ([]string, error) {
	return m.sectionsResult, m.sectionsErr
}

type mockHostDB struct {
	historyResult []build.ImageHistory
	historyErr    error
	countResult   int
	countErr      error
}

func (m *mockHostDB) GetImageHistoriesFiltered(_ context.Context, _ string, _ int, _ int) ([]build.ImageHistory, error) {
	return m.historyResult, m.historyErr
}
func (m *mockHostDB) CountImageHistoriesFiltered(_ context.Context, _ string) (int, error) {
	return m.countResult, m.countErr
}

type mockHostImage struct{}

func (m *mockHostImage) EnableOverlay() error { return nil }
func (m *mockHostImage) GetHostImage() (build.HostImage, error) {
	return build.HostImage{}, nil
}
func (m *mockHostImage) CheckAndUpdateBaseImage(_ context.Context, _ bool, _ bool, _ build.Config) error {
	return nil
}
func (m *mockHostImage) SwitchImage(_ context.Context, _ string, _ bool) error { return nil }
func (m *mockHostImage) BuildAndSwitch(_ context.Context, _ bool, _ bool, _ build.SwitchableConfig) error {
	return nil
}

type mockHostConfig struct {
	config  *build.Config
	loadErr error
	saveErr error
}

func (m *mockHostConfig) LoadConfig() error                               { return m.loadErr }
func (m *mockHostConfig) GetConfigEnvVars() (map[string]string, error)    { return nil, nil }
func (m *mockHostConfig) SaveConfig() error                               { return m.saveErr }
func (m *mockHostConfig) GenerateDockerfile(_ bool) error                 { return nil }
func (m *mockHostConfig) AddInstallPackage(_ string) error                { return nil }
func (m *mockHostConfig) AddRemovePackage(_ string) error                 { return nil }
func (m *mockHostConfig) GetConfig() *build.Config                        { return m.config }
func (m *mockHostConfig) SetConfig(c *build.Config)                       { m.config = c }
func (m *mockHostConfig) ConfigIsChanged(_ context.Context) (bool, error) { return false, nil }
func (m *mockHostConfig) SaveConfigToDB(_ context.Context) error          { return nil }

type mockTempConfig struct {
	config *service.TemporaryConfig
}

func (m *mockTempConfig) LoadConfig() error                   { return nil }
func (m *mockTempConfig) SaveConfig() error                   { return nil }
func (m *mockTempConfig) AddInstallPackage(_ string) error    { return nil }
func (m *mockTempConfig) AddRemovePackage(_ string) error     { return nil }
func (m *mockTempConfig) DeleteFile() error                   { return nil }
func (m *mockTempConfig) GetConfig() *service.TemporaryConfig { return m.config }

type mockAppStream struct {
	result map[string][]swcat.Component
	err    error
}

func (m *mockAppStream) GetByPkgNames(_ context.Context, _ []string) (map[string][]swcat.Component, error) {
	return m.result, m.err
}

func newTestActions(aptAct *mockAptActions, aptDB *mockAptDB, hostDB *mockHostDB) *Actions {
	if aptAct == nil {
		aptAct = &mockAptActions{}
	}
	if aptDB == nil {
		aptDB = &mockAptDB{}
	}
	if hostDB == nil {
		hostDB = &mockHostDB{}
	}
	return &Actions{
		appConfig:              testutil.DefaultAppConfig(),
		serviceAptActions:      aptAct,
		serviceAptDatabase:     aptDB,
		serviceHostDatabase:    hostDB,
		serviceHostImage:       &mockHostImage{},
		serviceHostConfig:      &mockHostConfig{},
		serviceTemporaryConfig: &mockTempConfig{},
		serviceAppStreamDB:     &mockAppStream{},
	}
}


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

func TestCheckInstall(t *testing.T) {
	t.Run("success returns package changes", func(t *testing.T) {
		changes := &aptLib.PackageChanges{
			NewInstalledCount:    1,
			NewInstalledPackages: []string{"vim"},
		}
		apt := &mockAptActions{
			prepareInstall: []string{"vim"},
			findChanges:    changes,
		}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		resp, err := actions.CheckInstall(context.Background(), []string{"vim"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Info.NewInstalledCount != 1 {
			t.Errorf("expected 1 new install, got %d", resp.Info.NewInstalledCount)
		}
	})

	t.Run("empty packages returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, &mockAptDB{}, nil)
		_, err := actions.CheckInstall(context.Background(), []string{})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("prepare error returns apt error", func(t *testing.T) {
		apt := &mockAptActions{prepareErr: errors.New("bad package spec")}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		_, err := actions.CheckInstall(context.Background(), []string{"bad+"})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})

	t.Run("find error returns apt error", func(t *testing.T) {
		apt := &mockAptActions{
			prepareInstall: []string{"vim"},
			findErr:        errors.New("dependency conflict"),
		}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		_, err := actions.CheckInstall(context.Background(), []string{"vim"})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})
}

func TestCheckRemove(t *testing.T) {
	t.Run("success shows removal candidates with dependencies", func(t *testing.T) {
		changes := &aptLib.PackageChanges{
			RemovedCount:    2,
			RemovedPackages: []string{"vim", "vim-common"},
		}
		apt := &mockAptActions{checkRemoveRes: changes}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		resp, err := actions.CheckRemove(context.Background(), []string{"vim"}, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Info.RemovedCount != 2 {
			t.Errorf("expected 2 removed, got %d", resp.Info.RemovedCount)
		}
	})

	t.Run("apt error propagates", func(t *testing.T) {
		apt := &mockAptActions{checkRemoveErr: errors.New("cannot remove essential")}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		_, err := actions.CheckRemove(context.Background(), []string{"glibc"}, false, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})
}

func TestCheckUpgrade(t *testing.T) {
	t.Run("success shows available upgrades", func(t *testing.T) {
		changes := &aptLib.PackageChanges{UpgradedCount: 15}
		apt := &mockAptActions{checkUpgradeRes: changes}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		resp, err := actions.CheckUpgrade(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Info.UpgradedCount != 15 {
			t.Errorf("expected 15 upgrades, got %d", resp.Info.UpgradedCount)
		}
	})

	t.Run("apt error propagates", func(t *testing.T) {
		apt := &mockAptActions{checkUpgradeErr: errors.New("repo unreachable")}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		_, err := actions.CheckUpgrade(context.Background())
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})
}

func TestCheckReinstall(t *testing.T) {
	t.Run("empty packages returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, &mockAptDB{}, nil)
		_, err := actions.CheckReinstall(context.Background(), []string{})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("success returns reinstall changes", func(t *testing.T) {
		changes := &aptLib.PackageChanges{NewInstalledCount: 1}
		apt := &mockAptActions{
			prepareInstall: []string{"bash"},
			findChanges:    changes,
		}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		resp, err := actions.CheckReinstall(context.Background(), []string{"bash"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Info.NewInstalledCount != 1 {
			t.Errorf("expected 1 reinstall, got %d", resp.Info.NewInstalledCount)
		}
	})
}

func TestImageHistory(t *testing.T) {
	history := []build.ImageHistory{
		{ImageName: "alt:p11"},
		{ImageName: "alt:p11"},
	}

	t.Run("returns history with total count", func(t *testing.T) {
		hostDB := &mockHostDB{historyResult: history, countResult: 10}
		actions := newTestActions(nil, &mockAptDB{}, hostDB)

		resp, err := actions.ImageHistory(context.Background(), "", 10, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.History) != 2 {
			t.Errorf("expected 2 entries, got %d", len(resp.History))
		}
		if resp.TotalCount != 10 {
			t.Errorf("expected totalCount=10, got %d", resp.TotalCount)
		}
	})

	t.Run("history query error propagates", func(t *testing.T) {
		hostDB := &mockHostDB{historyErr: errors.New("db error")}
		actions := newTestActions(nil, &mockAptDB{}, hostDB)

		_, err := actions.ImageHistory(context.Background(), "", 10, 0)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeDatabase)
	})

	t.Run("count query error propagates", func(t *testing.T) {
		hostDB := &mockHostDB{historyResult: history, countErr: errors.New("count error")}
		actions := newTestActions(nil, &mockAptDB{}, hostDB)

		_, err := actions.ImageHistory(context.Background(), "", 10, 0)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeDatabase)
	})
}

func TestImageGetConfig(t *testing.T) {
	t.Run("returns loaded config", func(t *testing.T) {
		cfg := &build.Config{Image: "alt:p11"}
		actions := newTestActions(nil, &mockAptDB{}, nil)
		actions.serviceHostConfig = &mockHostConfig{config: cfg}

		resp, err := actions.ImageGetConfig(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Config.Image != "alt:p11" {
			t.Errorf("expected image alt:p11, got %s", resp.Config.Image)
		}
	})

	t.Run("load error propagates as image error", func(t *testing.T) {
		actions := newTestActions(nil, &mockAptDB{}, nil)
		actions.serviceHostConfig = &mockHostConfig{loadErr: errors.New("file not found")}

		_, err := actions.ImageGetConfig(context.Background())
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeImage)
	})
}

func TestImageSaveConfig(t *testing.T) {
	t.Run("replaces config and saves", func(t *testing.T) {
		hcfg := &mockHostConfig{config: &build.Config{Image: "old"}}
		actions := newTestActions(nil, &mockAptDB{}, nil)
		actions.serviceHostConfig = hcfg

		newCfg := build.Config{Image: "alt:p12"}
		resp, err := actions.ImageSaveConfig(context.Background(), newCfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Config.Image != "alt:p12" {
			t.Errorf("expected saved image alt:p12, got %s", resp.Config.Image)
		}
		if hcfg.config.Image != "alt:p12" {
			t.Error("SetConfig should update the stored config")
		}
	})

	t.Run("save error propagates as image error", func(t *testing.T) {
		actions := newTestActions(nil, &mockAptDB{}, nil)
		actions.serviceHostConfig = &mockHostConfig{
			config:  &build.Config{},
			saveErr: errors.New("disk full"),
		}

		_, err := actions.ImageSaveConfig(context.Background(), build.Config{Image: "new"})
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeImage)
	})
}

func TestSetGetAptConfigOverrides(t *testing.T) {
	t.Run("set and get overrides roundtrip", func(t *testing.T) {
		apt := &mockAptActions{}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		overrides := map[string]string{"Acquire::http::Proxy": "http://proxy:8080"}
		resp, err := actions.SetAptConfigOverrides(overrides)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Options["Acquire::http::Proxy"] != "http://proxy:8080" {
			t.Error("SetAptConfigOverrides should return the passed options")
		}

		resp, err = actions.GetAptConfigOverrides()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Options["Acquire::http::Proxy"] != "http://proxy:8080" {
			t.Error("GetAptConfigOverrides should return previously set options")
		}
	})

	t.Run("get returns empty map when nil", func(t *testing.T) {
		apt := &mockAptActions{overrides: nil}
		actions := newTestActions(apt, &mockAptDB{}, nil)

		resp, err := actions.GetAptConfigOverrides()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Options == nil {
			t.Error("should return empty map, not nil")
		}
	})
}

func TestValidateDB_DBEmpty(t *testing.T) {
	apt := &mockAptActions{updateErr: errors.New("update failed")}
	db := &mockAptDB{dbExistErr: errors.New("empty database")}
	actions := newTestActions(apt, db, nil)

	_, err := actions.Search(context.Background(), "vim", false)
	if syscall.Geteuid() == 0 {
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeDatabase)
	} else {
		testutil.AssertAPMError(t, err, apmerr.ErrorTypePermission)
	}
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
