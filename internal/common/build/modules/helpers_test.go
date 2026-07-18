package modules

import (
	"context"

	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/filter"
	"altlinux.space/alt-atomic/apm/internal/domain/kernel/service"
	reposervice "altlinux.space/alt-atomic/apm/pkg/aptrepo"
	pkgbuild "altlinux.space/alt-atomic/apm/pkg/build"
	"altlinux.space/alt-atomic/apm/pkg/command"
)

// fakeDomain implements DomainContext for domain-module tests.
type fakeDomain struct {
	runner command.Runner

	// GetPackageByName lookup table and forced error
	packages map[string]*_package.Package
	getErr   error

	// captured domain calls
	combineCalls   [][]string
	combineDepends bool
	combineErr     error
	updateCalled   bool
	upgradeCalled  bool
	updateErr      error
	upgradeErr     error
	overrideCalls  []map[string]string
	collected      []string

	installCalls [][]string
	queryResult  []_package.Package
	queryErr     error

	repoMgr   RepoManager
	kernelMgr KernelManager
}

var _ DomainContext = (*fakeDomain)(nil)

func (f *fakeDomain) Runner() command.Runner                            { return f.runner }
func (f *fakeDomain) CollectOutput(text string)                         { f.collected = append(f.collected, text) }
func (f *fakeDomain) EmitEvent(context.Context, string, string, string) {}
func (f *fakeDomain) ExecuteInclude(context.Context, string) (map[string]*pkgbuild.MapModule, error) {
	return nil, nil
}

func (f *fakeDomain) IsAtomic() bool { return false }

func (f *fakeDomain) SetAptConfigOverrides(overrides map[string]string) {
	f.overrideCalls = append(f.overrideCalls, overrides)
}

func (f *fakeDomain) ValidateDB(context.Context) error { return nil }

func (f *fakeDomain) CombineInstallRemovePackages(_ context.Context, packages []string, _, depends, _ bool) error {
	f.combineCalls = append(f.combineCalls, packages)
	f.combineDepends = depends
	return f.combineErr
}

func (f *fakeDomain) InstallPackages(_ context.Context, packages []string) error {
	f.installCalls = append(f.installCalls, packages)
	return nil
}

func (f *fakeDomain) QueryHostImagePackages(context.Context, []filter.FilterGroup, string, string, int, int) ([]_package.Package, error) {
	return f.queryResult, f.queryErr
}

func (f *fakeDomain) GetPackageByName(_ context.Context, name string) (*_package.Package, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if p, ok := f.packages[name]; ok {
		return p, nil
	}
	return &_package.Package{Name: name}, nil
}

func (f *fakeDomain) UpdatePackages(context.Context) error {
	f.updateCalled = true
	return f.updateErr
}

func (f *fakeDomain) UpgradePackages(context.Context) error {
	f.upgradeCalled = true
	return f.upgradeErr
}

func (f *fakeDomain) KernelManager() KernelManager { return f.kernelMgr }
func (f *fakeDomain) RepoService() RepoManager     { return f.repoMgr }

// fakeRepoManager implements RepoManager with canned results and call capture.
type fakeRepoManager struct {
	branches     []string
	addResult    []reposervice.Repository
	removeResult []reposervice.Repository
	cleanResult  []reposervice.Repository
	setAdded     []reposervice.Repository

	addErr, removeErr, setErr, cleanErr error

	addedArgs   [][]string
	setBranch   string
	removeAll   bool
	cleanCalled bool
}

func (m *fakeRepoManager) GetBranches() []string { return m.branches }

func (m *fakeRepoManager) AddRepository(_ context.Context, args []string, _ string) ([]reposervice.Repository, error) {
	m.addedArgs = append(m.addedArgs, args)
	return m.addResult, m.addErr
}

func (m *fakeRepoManager) RemoveRepository(_ context.Context, _ []string, _ string, _ bool) ([]reposervice.Repository, error) {
	m.removeAll = true
	return m.removeResult, m.removeErr
}

func (m *fakeRepoManager) SetBranch(_ context.Context, branch, _ string) ([]reposervice.Repository, []reposervice.Repository, error) {
	m.setBranch = branch
	return m.setAdded, nil, m.setErr
}

func (m *fakeRepoManager) CleanTemporary(context.Context) ([]reposervice.Repository, error) {
	m.cleanCalled = true
	return m.cleanResult, m.cleanErr
}

// fakeKernelManager implements KernelManager with canned results and call capture.
type fakeKernelManager struct {
	latest      *service.Info
	latestErr   error
	autoPkgs    []string
	autoErr     error
	parseResult *service.Info

	autoGotCurrent *service.Info
	installCalled  bool
	installedInfo  *service.Info
	installModules []string
}

func (m *fakeKernelManager) FindLatestKernel(_ context.Context, flavour string) (*service.Info, error) {
	if m.latestErr != nil {
		return nil, m.latestErr
	}
	if m.latest != nil {
		return m.latest, nil
	}
	return &service.Info{Flavour: flavour}, nil
}

func (m *fakeKernelManager) AutoSelectHeadersAndFirmware(_ context.Context, _ *service.Info, currentKernel *service.Info, _ bool) ([]string, error) {
	m.autoGotCurrent = currentKernel
	return m.autoPkgs, m.autoErr
}

func (m *fakeKernelManager) RemoveKernel(*service.Info, bool) error { return nil }

func (m *fakeKernelManager) InstallKernel(_ context.Context, kernel *service.Info, modules []string, _, _ bool) error {
	m.installCalled = true
	m.installedInfo = kernel
	m.installModules = modules
	return nil
}

func (m *fakeKernelManager) ParseKernelPackageFromDB(_package.Package) *service.Info {
	return m.parseResult
}

// fakeRunner records every Run invocation and returns a preset error.
type fakeRunner struct {
	err   error
	calls [][]string
}

func (r *fakeRunner) Run(_ context.Context, args []string, _ ...command.Option) (string, string, error) {
	r.calls = append(r.calls, args)
	return "", "", r.err
}

// bareRuntime implements only pkgbuild.RuntimeContext (not DomainContext),
// used to check that domain modules reject a non-domain context.
type bareRuntime struct{}

func (bareRuntime) Runner() command.Runner                            { return nil }
func (bareRuntime) CollectOutput(string)                              {}
func (bareRuntime) EmitEvent(context.Context, string, string, string) {}
func (bareRuntime) ExecuteInclude(context.Context, string) (map[string]*pkgbuild.MapModule, error) {
	return nil, nil
}
