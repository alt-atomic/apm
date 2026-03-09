package kernel

import (
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	_package "apm/internal/common/apt/package"
	aptlib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/testutil"
	"apm/internal/domain/kernel/service"
	"context"
	"errors"
	"syscall"
	"testing"
)

type mockAptActions struct {
	updateResult     []_package.Package
	updateErr        error
	aptUpdateErr     error
	installedPkgs    map[string]string
	installedPkgsErr error
}

func (m *mockAptActions) Update(_ context.Context, _ ...bool) ([]_package.Package, error) {
	return m.updateResult, m.updateErr
}
func (m *mockAptActions) AptUpdate(_ context.Context, _ ...bool) error {
	return m.aptUpdateErr
}
func (m *mockAptActions) GetInstalledPackages(_ context.Context, _ ...bool) (map[string]string, error) {
	return m.installedPkgs, m.installedPkgsErr
}

type mockAptDatabase struct {
	dbExistErr error
	syncErr    error
}

func (m *mockAptDatabase) PackageDatabaseExist(_ context.Context) error { return m.dbExistErr }
func (m *mockAptDatabase) SyncPackageInstallationInfo(_ context.Context, _ map[string]string) error {
	return m.syncErr
}

type mockKernelManager struct {
	listKernelsResult   []*service.Info
	listKernelsErr      error
	currentKernel       *service.Info
	currentKernelErr    error
	findLatestResult    *service.Info
	findLatestErr       error
	inheritModules      []string
	inheritModulesErr   error
	autoSelectResult    []string
	autoSelectErr       error
	simulateResult      *service.UpgradePreview
	simulateErr         error
	installKernelErr    error
	findNextFlavours    []string
	findNextFlavoursErr error
	rpmKernels          []*service.Info
	rpmKernelsErr       error
	backupKernel        *service.Info
	backupKernelErr     error
	groupResult         map[string][]*service.Info
	removeResult        *aptlib.PackageChanges
	removeErr           error
	detectFlavour       string
	detectFlavourErr    error
	availableModules    []service.ModuleInfo
	availableModulesErr error
	fullPkgName         string
	installModResult    *aptlib.PackageChanges
	installModErr       error
	simplePkgName       string
}

func (m *mockKernelManager) ListKernels(_ context.Context, _ string) ([]*service.Info, error) {
	return m.listKernelsResult, m.listKernelsErr
}
func (m *mockKernelManager) GetCurrentKernel(_ context.Context) (*service.Info, error) {
	return m.currentKernel, m.currentKernelErr
}
func (m *mockKernelManager) FindLatestKernel(_ context.Context, _ string) (*service.Info, error) {
	return m.findLatestResult, m.findLatestErr
}
func (m *mockKernelManager) InheritModulesFromKernel(_ *service.Info, _ *service.Info) ([]string, error) {
	return m.inheritModules, m.inheritModulesErr
}
func (m *mockKernelManager) AutoSelectHeadersAndFirmware(_ context.Context, _ *service.Info, _ bool) ([]string, error) {
	return m.autoSelectResult, m.autoSelectErr
}
func (m *mockKernelManager) SimulateUpgrade(_ *service.Info, _ []string, _ bool) (*service.UpgradePreview, error) {
	return m.simulateResult, m.simulateErr
}
func (m *mockKernelManager) InstallKernel(_ context.Context, _ *service.Info, _ []string, _ bool, _ bool) error {
	return m.installKernelErr
}
func (m *mockKernelManager) FindNextFlavours(_ string) ([]string, error) {
	return m.findNextFlavours, m.findNextFlavoursErr
}
func (m *mockKernelManager) ListInstalledKernelsFromRPM(_ context.Context) ([]*service.Info, error) {
	return m.rpmKernels, m.rpmKernelsErr
}
func (m *mockKernelManager) GetBackupKernel(_ context.Context) (*service.Info, error) {
	return m.backupKernel, m.backupKernelErr
}
func (m *mockKernelManager) GroupKernelsByFlavour(_ []*service.Info) map[string][]*service.Info {
	return m.groupResult
}
func (m *mockKernelManager) RemovePackages(_ context.Context, _ []string, _ bool) (*aptlib.PackageChanges, error) {
	return m.removeResult, m.removeErr
}
func (m *mockKernelManager) DetectCurrentFlavour(_ context.Context) (string, error) {
	return m.detectFlavour, m.detectFlavourErr
}
func (m *mockKernelManager) FindAvailableModules(_ *service.Info) ([]service.ModuleInfo, error) {
	return m.availableModules, m.availableModulesErr
}
func (m *mockKernelManager) GetFullPackageNameForModule(packageName string) string {
	if m.fullPkgName != "" {
		return m.fullPkgName
	}
	return packageName
}
func (m *mockKernelManager) InstallModules(_ context.Context, _ []string, _ bool) (*aptlib.PackageChanges, error) {
	return m.installModResult, m.installModErr
}
func (m *mockKernelManager) GetSimplePackageNameForModule(packageName string) string {
	if m.simplePkgName != "" {
		return m.simplePkgName
	}
	return packageName
}
func (m *mockKernelManager) BuildFullKernelInfo(info *service.Info) service.FullKernelInfo {
	return service.FullKernelInfo{
		PackageName: info.PackageName,
		Flavour:     info.Flavour,
		Version:     info.Version,
		FullVersion: info.FullVersion,
		IsInstalled: info.IsInstalled,
		IsRunning:   info.IsRunning,
	}
}

func newTestActions(km *mockKernelManager, apt *mockAptActions, db *mockAptDatabase) *Actions {
	if km == nil {
		km = &mockKernelManager{}
	}
	if apt == nil {
		apt = &mockAptActions{}
	}
	if db == nil {
		db = &mockAptDatabase{}
	}
	return &Actions{
		appConfig:          testutil.DefaultAppConfig(),
		kernelManager:      km,
		serviceAptActions:  apt,
		serviceAptDatabase: db,
	}
}

func testContext() context.Context {
	cfg := testutil.DefaultAppConfig()
	return context.WithValue(context.Background(), app.AppConfigKey, cfg)
}

func testKernel(flavour, version, fullVersion string) *service.Info {
	return &service.Info{
		PackageName:      "kernel-image-" + flavour,
		Flavour:          flavour,
		Version:          version,
		VersionInstalled: version,
		Release:          "alt1",
		FullVersion:      fullVersion,
		IsInstalled:      true,
	}
}

func TestListKernels(t *testing.T) {
	kernels := []*service.Info{
		{PackageName: "kernel-image-6.12", Flavour: "6.12", Version: "6.12.10", FullVersion: "kernel-image-6.12#6.12.10-alt1", IsInstalled: true},
		{PackageName: "kernel-image-6.12", Flavour: "6.12", Version: "6.12.5", FullVersion: "kernel-image-6.12#6.12.5-alt1", IsInstalled: true},
	}

	t.Run("returns all kernels", func(t *testing.T) {
		actions := newTestActions(&mockKernelManager{listKernelsResult: kernels}, nil, nil)

		resp, err := actions.ListKernels(testContext(), "6.12", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Kernels) != 2 {
			t.Errorf("expected 2 kernels, got %d", len(resp.Kernels))
		}
	})

	t.Run("filters installed only", func(t *testing.T) {
		mixed := []*service.Info{
			{PackageName: "kernel-image-6.12", Flavour: "6.12", Version: "6.12.10", FullVersion: "kernel-image-6.12#6.12.10-alt1", IsInstalled: true},
			{PackageName: "kernel-image-6.12", Flavour: "6.12", Version: "6.12.5", FullVersion: "kernel-image-6.12#6.12.5-alt1", IsInstalled: false},
		}
		actions := newTestActions(&mockKernelManager{listKernelsResult: mixed}, nil, nil)

		resp, err := actions.ListKernels(testContext(), "6.12", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Kernels) != 1 {
			t.Errorf("expected 1 kernel, got %d", len(resp.Kernels))
		}
	})

	t.Run("no kernels returns not found", func(t *testing.T) {
		actions := newTestActions(&mockKernelManager{listKernelsResult: []*service.Info{}}, nil, nil)

		_, err := actions.ListKernels(testContext(), "unknown", false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNotFound)
	})

	t.Run("service error propagates", func(t *testing.T) {
		actions := newTestActions(&mockKernelManager{listKernelsErr: errors.New("db error")}, nil, nil)

		_, err := actions.ListKernels(testContext(), "", false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeKernel)
	})

	t.Run("db validation error propagates", func(t *testing.T) {
		actions := newTestActions(nil,
			&mockAptActions{updateErr: errors.New("update failed")},
			&mockAptDatabase{dbExistErr: errors.New("no db")},
		)

		_, err := actions.ListKernels(testContext(), "", false)
		if syscall.Geteuid() == 0 {
			testutil.AssertAPMError(t, err, apmerr.ErrorTypeDatabase)
		} else {
			testutil.AssertAPMError(t, err, apmerr.ErrorTypePermission)
		}
	})
}

func TestGetCurrentKernel(t *testing.T) {
	current := testKernel("6.12", "6.12.10", "kernel-image-6.12#6.12.10-alt1")

	t.Run("success returns current kernel", func(t *testing.T) {
		actions := newTestActions(&mockKernelManager{currentKernel: current}, nil, nil)

		resp, err := actions.GetCurrentKernel(testContext())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Kernel.Flavour != "6.12" {
			t.Errorf("expected flavour 6.12, got %s", resp.Kernel.Flavour)
		}
	})

	t.Run("service error propagates", func(t *testing.T) {
		actions := newTestActions(&mockKernelManager{currentKernelErr: errors.New("uname failed")}, nil, nil)

		_, err := actions.GetCurrentKernel(testContext())
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeKernel)
	})
}

func TestInstallKernel(t *testing.T) {
	latest := testKernel("6.12", "6.12.10", "kernel-image-6.12#6.12.10-alt1")

	t.Run("empty flavour returns validation error", func(t *testing.T) {
		actions := newTestActions(&mockKernelManager{findLatestResult: latest}, nil, nil)

		_, err := actions.InstallKernel(testContext(), "  ", nil, false, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("apt update error propagates", func(t *testing.T) {
		actions := newTestActions(nil, &mockAptActions{aptUpdateErr: errors.New("apt failed")}, nil)

		_, err := actions.InstallKernel(testContext(), "6.12", nil, false, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})

	t.Run("find latest error propagates", func(t *testing.T) {
		actions := newTestActions(&mockKernelManager{findLatestErr: errors.New("not found")}, nil, nil)

		_, err := actions.InstallKernel(testContext(), "6.12", nil, false, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeKernel)
	})

	t.Run("missing modules returns not found", func(t *testing.T) {
		km := &mockKernelManager{
			findLatestResult: latest,
			simulateResult: &service.UpgradePreview{
				MissingModules: []string{"nonexistent"},
			},
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.InstallKernel(testContext(), "6.12", []string{"nonexistent"}, false, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNotFound)
	})

	t.Run("already installed returns success message", func(t *testing.T) {
		km := &mockKernelManager{
			findLatestResult: latest,
			simulateResult: &service.UpgradePreview{
				Changes: &aptlib.PackageChanges{},
			},
		}
		actions := newTestActions(km, nil, nil)

		resp, err := actions.InstallKernel(testContext(), "6.12", nil, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Preview != nil {
			t.Error("expected nil preview for already installed")
		}
	})

	t.Run("dry run returns preview", func(t *testing.T) {
		changes := &aptlib.PackageChanges{
			NewInstalledPackages: []string{"kernel-image-6.12"},
			NewInstalledCount:    1,
		}
		km := &mockKernelManager{
			findLatestResult: latest,
			simulateResult: &service.UpgradePreview{
				Changes: changes,
			},
		}
		actions := newTestActions(km, nil, nil)

		resp, err := actions.InstallKernel(testContext(), "6.12", nil, false, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Preview == nil {
			t.Error("expected preview for dry run")
		}
	})

	t.Run("install error propagates", func(t *testing.T) {
		changes := &aptlib.PackageChanges{
			NewInstalledPackages: []string{"kernel-image-6.12"},
			NewInstalledCount:    1,
		}
		km := &mockKernelManager{
			findLatestResult: latest,
			simulateResult: &service.UpgradePreview{
				Changes: changes,
			},
			installKernelErr: errors.New("install failed"),
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.InstallKernel(testContext(), "6.12", nil, false, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeKernel)
	})

	t.Run("success installs kernel", func(t *testing.T) {
		changes := &aptlib.PackageChanges{
			NewInstalledPackages: []string{"kernel-image-6.12"},
			NewInstalledCount:    1,
		}
		km := &mockKernelManager{
			findLatestResult: latest,
			simulateResult: &service.UpgradePreview{
				Changes: changes,
			},
		}
		apt := &mockAptActions{installedPkgs: map[string]string{}}
		actions := newTestActions(km, apt, nil)

		resp, err := actions.InstallKernel(testContext(), "6.12", nil, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Kernel.Flavour != "6.12" {
			t.Errorf("expected flavour 6.12, got %s", resp.Kernel.Flavour)
		}
	})
}

func TestUpdateKernel(t *testing.T) {
	latest := testKernel("6.12", "6.12.10", "kernel-image-6.12#6.12.10-alt1")
	current := &service.Info{
		PackageName:      "kernel-image-6.12",
		Flavour:          "6.12",
		Version:          "6.12.5",
		VersionInstalled: "6.12.5",
		Release:          "alt1",
		FullVersion:      "kernel-image-6.12#6.12.5-alt1",
		IsInstalled:      true,
	}

	t.Run("apt update error propagates", func(t *testing.T) {
		actions := newTestActions(nil, &mockAptActions{updateErr: errors.New("update failed")}, nil)

		_, err := actions.UpdateKernel(testContext(), "", nil, false, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})

	t.Run("no kernels found returns not found", func(t *testing.T) {
		km := &mockKernelManager{
			detectFlavour: "6.12",
			findLatestErr: errors.New("not found"),
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.UpdateKernel(testContext(), "6.12", nil, false, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNotFound)
	})

	t.Run("already up to date returns success", func(t *testing.T) {
		upToDate := testKernel("6.12", "6.12.10", "kernel-image-6.12#6.12.10-alt1")
		km := &mockKernelManager{
			detectFlavour:    "6.12",
			findLatestResult: upToDate,
			currentKernel:    upToDate,
		}
		actions := newTestActions(km, nil, nil)

		resp, err := actions.UpdateKernel(testContext(), "", nil, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Preview != nil {
			t.Error("expected nil preview for up to date")
		}
	})

	t.Run("get current kernel error propagates", func(t *testing.T) {
		km := &mockKernelManager{
			detectFlavour:    "6.12",
			findLatestResult: latest,
			currentKernelErr: errors.New("cannot get current"),
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.UpdateKernel(testContext(), "", nil, false, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeKernel)
	})

	t.Run("update available calls install", func(t *testing.T) {
		changes := &aptlib.PackageChanges{
			UpgradedPackages: []string{"kernel-image-6.12"},
			UpgradedCount:    1,
		}
		km := &mockKernelManager{
			detectFlavour:    "6.12",
			findLatestResult: latest,
			currentKernel:    current,
			simulateResult: &service.UpgradePreview{
				Changes: changes,
			},
		}
		apt := &mockAptActions{installedPkgs: map[string]string{}}
		actions := newTestActions(km, apt, nil)

		resp, err := actions.UpdateKernel(testContext(), "", nil, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Kernel.Flavour != "6.12" {
			t.Errorf("expected flavour 6.12, got %s", resp.Kernel.Flavour)
		}
	})
}

func TestCleanOldKernels(t *testing.T) {
	current := &service.Info{
		PackageName: "kernel-image-6.12",
		Flavour:     "6.12",
		Version:     "6.12.10",
		Release:     "alt1",
		FullVersion: "kernel-image-6.12#6.12.10-alt1",
		IsInstalled: true,
	}
	old := &service.Info{
		PackageName: "kernel-image-6.12",
		Flavour:     "6.12",
		Version:     "6.12.3",
		Release:     "alt1",
		FullVersion: "kernel-image-6.12#6.12.3-alt1",
		IsInstalled: true,
	}

	t.Run("no kernels found returns not found", func(t *testing.T) {
		km := &mockKernelManager{
			rpmKernels:    []*service.Info{},
			currentKernel: current,
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.CleanOldKernels(testContext(), false, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNotFound)
	})

	t.Run("rpm query error propagates", func(t *testing.T) {
		km := &mockKernelManager{rpmKernelsErr: errors.New("rpm error")}
		actions := newTestActions(km, nil, nil)

		_, err := actions.CleanOldKernels(testContext(), false, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeKernel)
	})

	t.Run("get current kernel error propagates", func(t *testing.T) {
		km := &mockKernelManager{
			rpmKernels:       []*service.Info{current},
			currentKernelErr: errors.New("uname error"),
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.CleanOldKernels(testContext(), false, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeKernel)
	})

	t.Run("no old kernels returns no operation", func(t *testing.T) {
		km := &mockKernelManager{
			rpmKernels:    []*service.Info{current},
			currentKernel: current,
			groupResult: map[string][]*service.Info{
				"6.12": {current},
			},
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.CleanOldKernels(testContext(), true, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNoOperation)
	})

	t.Run("dry run returns preview", func(t *testing.T) {
		km := &mockKernelManager{
			rpmKernels:    []*service.Info{current, old},
			currentKernel: current,
			removeResult:  &aptlib.PackageChanges{RemovedCount: 1},
			groupResult: map[string][]*service.Info{
				"6.12": {current, old},
			},
		}
		actions := newTestActions(km, nil, nil)

		resp, err := actions.CleanOldKernels(testContext(), true, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.RemoveKernels) != 1 {
			t.Errorf("expected 1 kernel to remove, got %d", len(resp.RemoveKernels))
		}
		if resp.Preview == nil {
			t.Error("expected preview for dry run")
		}
	})

	t.Run("success removes old kernels", func(t *testing.T) {
		km := &mockKernelManager{
			rpmKernels:    []*service.Info{current, old},
			currentKernel: current,
			groupResult: map[string][]*service.Info{
				"6.12": {current, old},
			},
		}
		actions := newTestActions(km, nil, nil)

		resp, err := actions.CleanOldKernels(testContext(), true, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.RemoveKernels) != 1 {
			t.Errorf("expected 1 kernel removed, got %d", len(resp.RemoveKernels))
		}
	})
}

func TestListKernelModules(t *testing.T) {
	latest := testKernel("6.12", "6.12.10", "kernel-image-6.12#6.12.10-alt1")
	modules := []service.ModuleInfo{
		{Name: "drm", IsInstalled: true, PackageName: "kernel-modules-drm-6.12"},
		{Name: "v4l", IsInstalled: false, PackageName: "kernel-modules-v4l-6.12"},
	}

	t.Run("success returns modules", func(t *testing.T) {
		km := &mockKernelManager{
			findLatestResult: latest,
			availableModules: modules,
		}
		actions := newTestActions(km, nil, nil)

		resp, err := actions.ListKernelModules(testContext(), "6.12")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Modules) != 2 {
			t.Errorf("expected 2 modules, got %d", len(resp.Modules))
		}
	})

	t.Run("auto detect flavour", func(t *testing.T) {
		km := &mockKernelManager{
			detectFlavour:    "6.12",
			findLatestResult: latest,
			availableModules: modules,
		}
		actions := newTestActions(km, nil, nil)

		resp, err := actions.ListKernelModules(testContext(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Modules) != 2 {
			t.Errorf("expected 2 modules, got %d", len(resp.Modules))
		}
	})

	t.Run("detect flavour error propagates", func(t *testing.T) {
		km := &mockKernelManager{detectFlavourErr: errors.New("no current kernel")}
		actions := newTestActions(km, nil, nil)

		_, err := actions.ListKernelModules(testContext(), "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeKernel)
	})

	t.Run("find latest error propagates", func(t *testing.T) {
		km := &mockKernelManager{findLatestErr: errors.New("not found")}
		actions := newTestActions(km, nil, nil)

		_, err := actions.ListKernelModules(testContext(), "6.12")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeKernel)
	})

	t.Run("find modules error propagates", func(t *testing.T) {
		km := &mockKernelManager{
			findLatestResult: latest,
			availableModulesErr: errors.New("db error"),
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.ListKernelModules(testContext(), "6.12")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeKernel)
	})
}

func TestInstallKernelModules(t *testing.T) {
	latest := testKernel("6.12", "6.12.10", "kernel-image-6.12#6.12.10-alt1")
	modules := []service.ModuleInfo{
		{Name: "drm", IsInstalled: false, PackageName: "kernel-modules-drm-6.12"},
		{Name: "v4l", IsInstalled: false, PackageName: "kernel-modules-v4l-6.12"},
	}

	t.Run("empty modules returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, nil, nil)

		_, err := actions.InstallKernelModules(testContext(), "6.12", []string{}, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("module not available returns not found", func(t *testing.T) {
		km := &mockKernelManager{
			findLatestResult: latest,
			availableModules: modules,
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.InstallKernelModules(testContext(), "6.12", []string{"nonexistent"}, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNotFound)
	})

	t.Run("already installed returns no operation", func(t *testing.T) {
		installed := []service.ModuleInfo{
			{Name: "drm", IsInstalled: true, PackageName: "kernel-modules-drm-6.12"},
		}
		km := &mockKernelManager{
			findLatestResult: latest,
			availableModules: installed,
			currentKernel: &service.Info{
				Flavour: "6.12",
			},
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.InstallKernelModules(testContext(), "6.12", []string{"drm"}, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNoOperation)
	})

	t.Run("dry run returns preview", func(t *testing.T) {
		km := &mockKernelManager{
			findLatestResult: latest,
			availableModules: modules,
			currentKernel:    testKernel("6.12", "6.12.10", "kernel-image-6.12#6.12.10-alt1"),
			installModResult: &aptlib.PackageChanges{NewInstalledCount: 1},
		}
		actions := newTestActions(km, nil, nil)

		resp, err := actions.InstallKernelModules(testContext(), "6.12", []string{"drm"}, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Preview == nil {
			t.Error("expected preview for dry run")
		}
	})

	t.Run("install error propagates", func(t *testing.T) {
		km := &mockKernelManager{
			findLatestResult: latest,
			availableModules: modules,
			currentKernel:    testKernel("6.12", "6.12.10", "kernel-image-6.12#6.12.10-alt1"),
			installModErr:    errors.New("install failed"),
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.InstallKernelModules(testContext(), "6.12", []string{"drm"}, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeKernel)
	})

	t.Run("success installs modules", func(t *testing.T) {
		km := &mockKernelManager{
			findLatestResult: latest,
			availableModules: modules,
			currentKernel:    testKernel("6.12", "6.12.10", "kernel-image-6.12#6.12.10-alt1"),
		}
		apt := &mockAptActions{installedPkgs: map[string]string{}}
		actions := newTestActions(km, apt, nil)

		resp, err := actions.InstallKernelModules(testContext(), "6.12", []string{"drm"}, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Kernel.Flavour != "6.12" {
			t.Errorf("expected flavour 6.12, got %s", resp.Kernel.Flavour)
		}
	})
}

func TestRemoveKernelModules(t *testing.T) {
	latest := testKernel("6.12", "6.12.10", "kernel-image-6.12#6.12.10-alt1")
	installed := []service.ModuleInfo{
		{Name: "drm", IsInstalled: true, PackageName: "kernel-modules-drm-6.12"},
		{Name: "v4l", IsInstalled: true, PackageName: "kernel-modules-v4l-6.12"},
	}

	t.Run("empty modules returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, nil, nil)

		_, err := actions.RemoveKernelModules(testContext(), "6.12", []string{}, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("module not found returns not found", func(t *testing.T) {
		km := &mockKernelManager{
			findLatestResult: latest,
			availableModules: installed,
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.RemoveKernelModules(testContext(), "6.12", []string{"nonexistent"}, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNotFound)
	})

	t.Run("module not installed returns no operation", func(t *testing.T) {
		notInstalled := []service.ModuleInfo{
			{Name: "drm", IsInstalled: false, PackageName: "kernel-modules-drm-6.12"},
		}
		km := &mockKernelManager{
			findLatestResult: latest,
			availableModules: notInstalled,
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.RemoveKernelModules(testContext(), "6.12", []string{"drm"}, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNoOperation)
	})

	t.Run("dry run returns preview", func(t *testing.T) {
		km := &mockKernelManager{
			findLatestResult: latest,
			availableModules: installed,
			removeResult:     &aptlib.PackageChanges{RemovedCount: 1},
		}
		actions := newTestActions(km, nil, nil)

		resp, err := actions.RemoveKernelModules(testContext(), "6.12", []string{"drm"}, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Preview == nil {
			t.Error("expected preview for dry run")
		}
	})

	t.Run("remove error propagates", func(t *testing.T) {
		km := &mockKernelManager{
			findLatestResult: latest,
			availableModules: installed,
			removeErr:        errors.New("remove failed"),
		}
		actions := newTestActions(km, nil, nil)

		_, err := actions.RemoveKernelModules(testContext(), "6.12", []string{"drm"}, false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeKernel)
	})

	t.Run("success removes modules", func(t *testing.T) {
		km := &mockKernelManager{
			findLatestResult: latest,
			availableModules: installed,
			currentKernel:    testKernel("6.12", "6.12.10", "kernel-image-6.12#6.12.10-alt1"),
		}
		apt := &mockAptActions{installedPkgs: map[string]string{}}
		actions := newTestActions(km, apt, nil)

		resp, err := actions.RemoveKernelModules(testContext(), "6.12", []string{"drm"}, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Kernel.Flavour != "6.12" {
			t.Errorf("expected flavour 6.12, got %s", resp.Kernel.Flavour)
		}
	})

	t.Run("auto detect flavour", func(t *testing.T) {
		km := &mockKernelManager{
			detectFlavour:    "6.12",
			findLatestResult: latest,
			availableModules: installed,
			currentKernel:    testKernel("6.12", "6.12.10", "kernel-image-6.12#6.12.10-alt1"),
		}
		apt := &mockAptActions{installedPkgs: map[string]string{}}
		actions := newTestActions(km, apt, nil)

		resp, err := actions.RemoveKernelModules(testContext(), "", []string{"drm"}, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Kernel.Flavour != "6.12" {
			t.Errorf("expected flavour 6.12, got %s", resp.Kernel.Flavour)
		}
	})
}
