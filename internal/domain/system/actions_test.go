package system

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/filter"
	"altlinux.space/alt-atomic/apm/internal/common/imagesvc"
	"altlinux.space/alt-atomic/apm/internal/common/swcat"
	"altlinux.space/alt-atomic/apm/internal/common/testutil"
	"altlinux.space/alt-atomic/apm/internal/domain/system/temporary"
	aptLib "altlinux.space/alt-atomic/apm/pkg/apt/lib"
)

type mockAptActions struct {
	overrides       map[string]string
	checkRemoveRes  *aptLib.PackageChanges
	checkRemoveErr  error
	checkUpgradeRes *aptLib.PackageChanges
	checkUpgradeErr error
	prepareInstall  []string
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
	return m.prepareInstall, nil, m.prepareErr
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
func (m *mockAptActions) AptUpdateIfStale(_ context.Context, _ time.Duration, _ ...bool) error {
	return nil
}
func (m *mockAptActions) GetInstalledPackages(_ context.Context, _ ...bool) (map[string]string, error) {
	return nil, nil
}
func (m *mockAptActions) RpmIsPackageInstalled(_ string) (bool, error)          { return false, nil }
func (m *mockAptActions) Upgrade(_ context.Context, _ bool) error               { return nil }
func (m *mockAptActions) ReinstallPackages(_ context.Context, _ []string) error { return nil }
func (m *mockAptActions) Install(_ context.Context, _ []string, _ bool) error   { return nil }
func (m *mockAptActions) DownloadSource(_ context.Context, _ []string, _ string) ([]aptLib.SourcePackage, error) {
	return nil, nil
}
func (m *mockAptActions) InstallSourcePackages(_ context.Context, _ []string) error { return nil }

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
func (m *mockAptDB) QueryHostImagePackages(_ context.Context, _ []filter.FilterGroup, _ string, _ string, _ int, _ int) ([]_package.Package, error) {
	return m.queryResult, m.queryErr
}
func (m *mockAptDB) CountHostImagePackages(_ context.Context, _ []filter.FilterGroup) (int64, error) {
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
	historyResult []imagesvc.ImageHistory
	historyErr    error
	countResult   int
	countErr      error
}

func (m *mockHostDB) GetImageHistoriesFiltered(_ context.Context, _ string, _ int, _ int) ([]imagesvc.ImageHistory, error) {
	return m.historyResult, m.historyErr
}
func (m *mockHostDB) CountImageHistoriesFiltered(_ context.Context, _ string) (int, error) {
	return m.countResult, m.countErr
}

type mockHostImage struct {
	isActive    bool
	switchErr   error
	switchCalls int
	buildCalls  int
}

func (m *mockHostImage) EnableOverlay() error { return nil }
func (m *mockHostImage) GetHostImage() (imagesvc.HostImage, error) {
	return imagesvc.HostImage{}, nil
}
func (m *mockHostImage) CheckAndUpdateBaseImage(_ context.Context, _ bool, _ bool, _ imagesvc.Config) error {
	return nil
}
func (m *mockHostImage) SwitchImage(_ context.Context, _ string, _ bool) error {
	m.switchCalls++
	return m.switchErr
}
func (m *mockHostImage) BuildAndSwitch(_ context.Context, _ bool, _ bool, _ imagesvc.SwitchableConfig) error {
	m.buildCalls++
	return nil
}
func (m *mockHostImage) VerifyRemoteImage(_ context.Context, _ string) error { return nil }
func (m *mockHostImage) IsImageActive(_ string, _ bool) (bool, error) {
	return m.isActive, nil
}

type mockHostConfig struct {
	config    *imagesvc.Config
	loadErr   error
	saveErr   error
	saveCalls int
}

func (m *mockHostConfig) LoadConfig() error                            { return m.loadErr }
func (m *mockHostConfig) GetConfigEnvVars() (map[string]string, error) { return nil, nil }
func (m *mockHostConfig) SaveConfig() error {
	m.saveCalls++
	return m.saveErr
}
func (m *mockHostConfig) SetImage(image string)                           { m.config.Image = image }
func (m *mockHostConfig) GenerateDockerfile(_ bool) error                 { return nil }
func (m *mockHostConfig) AddInstallPackage(_ string) error                { return nil }
func (m *mockHostConfig) AddRemovePackage(_ string) error                 { return nil }
func (m *mockHostConfig) GetConfig() *imagesvc.Config                     { return m.config }
func (m *mockHostConfig) SetConfig(c *imagesvc.Config)                    { m.config = c }
func (m *mockHostConfig) ConfigIsChanged(_ context.Context) (bool, error) { return false, nil }
func (m *mockHostConfig) SaveConfigToDB(_ context.Context) error          { return nil }
func (m *mockHostConfig) ApplyPathOverrides(_, _ string) error            { return nil }

type mockTempConfig struct {
	config *temporary.Config
}

func (m *mockTempConfig) LoadConfig() error                { return nil }
func (m *mockTempConfig) SaveConfig() error                { return nil }
func (m *mockTempConfig) AddInstallPackage(_ string) error { return nil }
func (m *mockTempConfig) AddRemovePackage(_ string) error  { return nil }
func (m *mockTempConfig) DeleteFile() error                { return nil }
func (m *mockTempConfig) GetConfig() *temporary.Config     { return m.config }

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
