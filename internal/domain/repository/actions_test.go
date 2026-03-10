package repository

import (
	"apm/internal/common/apmerr"
	_package "apm/internal/common/apt/package"
	aptLib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/testutil"
	"apm/internal/domain/repository/service"
	"context"
	"errors"
	"testing"
)

type mockRepoService struct {
	getReposResult     []service.Repository
	getReposErr        error
	addResult          []service.Repository
	addErr             error
	removeResult       []service.Repository
	removeErr          error
	setBranchAdded     []service.Repository
	setBranchRemoved   []service.Repository
	setBranchErr       error
	cleanResult        []service.Repository
	cleanErr           error
	branches           []string
	taskPackagesResult []string
	taskPackagesErr    error
	simulateAddResult  []service.Repository
	simulateAddErr     error
	simulateRemResult  []service.Repository
	simulateRemErr     error
}

func (m *mockRepoService) GetRepositories(_ context.Context, _ bool) ([]service.Repository, error) {
	return m.getReposResult, m.getReposErr
}
func (m *mockRepoService) AddRepository(_ context.Context, _ []string, _ string) ([]service.Repository, error) {
	return m.addResult, m.addErr
}
func (m *mockRepoService) RemoveRepository(_ context.Context, _ []string, _ string, _ bool) ([]service.Repository, error) {
	return m.removeResult, m.removeErr
}
func (m *mockRepoService) SetBranch(_ context.Context, _ string, _ string) ([]service.Repository, []service.Repository, error) {
	return m.setBranchAdded, m.setBranchRemoved, m.setBranchErr
}
func (m *mockRepoService) CleanTemporary(_ context.Context) ([]service.Repository, error) {
	return m.cleanResult, m.cleanErr
}
func (m *mockRepoService) GetBranches() []string { return m.branches }
func (m *mockRepoService) GetTaskPackages(_ context.Context, _ string) ([]string, error) {
	return m.taskPackagesResult, m.taskPackagesErr
}
func (m *mockRepoService) SimulateAdd(_ context.Context, _ []string, _ string, _ bool) ([]service.Repository, error) {
	return m.simulateAddResult, m.simulateAddErr
}
func (m *mockRepoService) SimulateRemove(_ context.Context, _ []string, _ string, _ bool) ([]service.Repository, error) {
	return m.simulateRemResult, m.simulateRemErr
}

type mockAptActions struct {
	updateErr    error
	findInstall  []string
	findRemove   []string
	findChanges  *aptLib.PackageChanges
	findErr      error
	combineErr   error
}

func (m *mockAptActions) Update(_ context.Context, _ ...bool) ([]_package.Package, error) {
	return nil, m.updateErr
}
func (m *mockAptActions) FindPackage(_ context.Context, _ []string, _ []string, _ bool, _ bool, _ bool) ([]string, []string, []_package.Package, *aptLib.PackageChanges, error) {
	return m.findInstall, m.findRemove, nil, m.findChanges, m.findErr
}
func (m *mockAptActions) CombineInstallRemovePackages(_ context.Context, _ []string, _ []string, _ bool, _ bool, _ bool) error {
	return m.combineErr
}

func newTestActions(repo *mockRepoService, apt *mockAptActions) *Actions {
	if repo == nil {
		repo = &mockRepoService{}
	}
	if apt == nil {
		apt = &mockAptActions{}
	}
	return &Actions{
		appConfig:         testutil.DefaultAppConfig(),
		repoService:       repo,
		serviceAptActions: apt,
	}
}

func TestList(t *testing.T) {
	repos := []service.Repository{
		{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch", Active: true},
		{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch", Arch: "noarch", Active: true},
	}

	t.Run("returns active repositories", func(t *testing.T) {
		actions := newTestActions(&mockRepoService{getReposResult: repos}, nil)

		resp, err := actions.List(context.Background(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Count != 2 {
			t.Errorf("expected count=2, got %d", resp.Count)
		}
		if len(resp.Repositories) != 2 {
			t.Errorf("expected 2 repos, got %d", len(resp.Repositories))
		}
	})

	t.Run("returns all repositories including inactive", func(t *testing.T) {
		allRepos := []service.Repository{
			{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch", Active: true},
			{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/Sisyphus", Active: false},
		}
		actions := newTestActions(&mockRepoService{getReposResult: allRepos}, nil)

		resp, err := actions.List(context.Background(), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Count != 2 {
			t.Errorf("expected count=2, got %d", resp.Count)
		}
	})

	t.Run("empty list returns zero count", func(t *testing.T) {
		actions := newTestActions(&mockRepoService{getReposResult: []service.Repository{}}, nil)

		resp, err := actions.List(context.Background(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Count != 0 {
			t.Errorf("expected count=0, got %d", resp.Count)
		}
	})

	t.Run("service error propagates", func(t *testing.T) {
		actions := newTestActions(&mockRepoService{getReposErr: errors.New("read error")}, nil)

		_, err := actions.List(context.Background(), false)
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeRepository)
	})
}

func TestAdd(t *testing.T) {
	t.Run("success adds repositories", func(t *testing.T) {
		repo := &mockRepoService{addResult: []service.Repository{
			{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch", Arch: "x86_64", Key: "p11", Components: []string{"classic"}, Active: true, File: "/etc/apt/sources.list", Entry: "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic"},
		}}
		actions := newTestActions(repo, nil)

		resp, err := actions.Add(context.Background(), []string{"p11"}, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Added) != 1 {
			t.Errorf("expected 1 added, got %d", len(resp.Added))
		}
	})

	t.Run("empty args returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, nil)

		_, err := actions.Add(context.Background(), []string{}, "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("all already exist returns no operation", func(t *testing.T) {
		repo := &mockRepoService{addResult: []service.Repository{}}
		actions := newTestActions(repo, nil)

		_, err := actions.Add(context.Background(), []string{"p11"}, "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNoOperation)
	})

	t.Run("service error propagates", func(t *testing.T) {
		repo := &mockRepoService{addErr: errors.New("permission denied")}
		actions := newTestActions(repo, nil)

		_, err := actions.Add(context.Background(), []string{"p11"}, "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeRepository)
	})
}

func TestCheckAdd(t *testing.T) {
	t.Run("success returns simulation", func(t *testing.T) {
		repo := &mockRepoService{simulateAddResult: []service.Repository{
			{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch", Arch: "x86_64", Key: "p11", Components: []string{"classic"}, Active: true, Entry: "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic"},
		}}
		actions := newTestActions(repo, nil)

		resp, err := actions.CheckAdd(context.Background(), []string{"p11"}, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.WillAdd) != 1 {
			t.Errorf("expected 1 willAdd, got %d", len(resp.WillAdd))
		}
	})

	t.Run("empty args returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, nil)

		_, err := actions.CheckAdd(context.Background(), []string{}, "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("nothing to add returns no operation", func(t *testing.T) {
		repo := &mockRepoService{simulateAddResult: []service.Repository{}}
		actions := newTestActions(repo, nil)

		_, err := actions.CheckAdd(context.Background(), []string{"p11"}, "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNoOperation)
	})

	t.Run("service error propagates", func(t *testing.T) {
		repo := &mockRepoService{simulateAddErr: errors.New("parse error")}
		actions := newTestActions(repo, nil)

		_, err := actions.CheckAdd(context.Background(), []string{"invalid"}, "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeRepository)
	})
}

func TestRemove(t *testing.T) {
	t.Run("success removes repositories", func(t *testing.T) {
		repo := &mockRepoService{removeResult: []service.Repository{
			{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch", Arch: "x86_64", Key: "p11", Components: []string{"classic"}, Active: false, File: "/etc/apt/sources.list", Entry: "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic"},
		}}
		actions := newTestActions(repo, nil)

		resp, err := actions.Remove(context.Background(), []string{"p11"}, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Removed) != 1 {
			t.Errorf("expected 1 removed, got %d", len(resp.Removed))
		}
	})

	t.Run("empty args returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, nil)

		_, err := actions.Remove(context.Background(), []string{}, "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("nothing found returns no operation", func(t *testing.T) {
		repo := &mockRepoService{removeResult: []service.Repository{}}
		actions := newTestActions(repo, nil)

		_, err := actions.Remove(context.Background(), []string{"p11"}, "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNoOperation)
	})

	t.Run("service error propagates", func(t *testing.T) {
		repo := &mockRepoService{removeErr: errors.New("permission denied")}
		actions := newTestActions(repo, nil)

		_, err := actions.Remove(context.Background(), []string{"p11"}, "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeRepository)
	})
}

func TestCheckRemove(t *testing.T) {
	t.Run("success returns simulation", func(t *testing.T) {
		repo := &mockRepoService{simulateRemResult: []service.Repository{
			{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch", Arch: "x86_64", Key: "p11", Components: []string{"classic"}, Active: true, Entry: "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic"},
		}}
		actions := newTestActions(repo, nil)

		resp, err := actions.CheckRemove(context.Background(), []string{"p11"}, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.WillRemove) != 1 {
			t.Errorf("expected 1 willRemove, got %d", len(resp.WillRemove))
		}
	})

	t.Run("empty args returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, nil)

		_, err := actions.CheckRemove(context.Background(), []string{}, "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("nothing to remove returns no operation", func(t *testing.T) {
		repo := &mockRepoService{simulateRemResult: []service.Repository{}}
		actions := newTestActions(repo, nil)

		_, err := actions.CheckRemove(context.Background(), []string{"p11"}, "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNoOperation)
	})

	t.Run("service error propagates", func(t *testing.T) {
		repo := &mockRepoService{simulateRemErr: errors.New("io error")}
		actions := newTestActions(repo, nil)

		_, err := actions.CheckRemove(context.Background(), []string{"p11"}, "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeRepository)
	})
}

func TestSet(t *testing.T) {
	t.Run("success sets branch", func(t *testing.T) {
		repo := &mockRepoService{
			setBranchAdded: []service.Repository{
				{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch", Arch: "x86_64", Key: "p11", Components: []string{"classic"}, Active: true, Entry: "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic"},
			},
			setBranchRemoved: []service.Repository{
				{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/p10/branch", Arch: "x86_64", Key: "p10", Components: []string{"classic"}, Active: false, Entry: "rpm [p10] http://ftp.altlinux.org/pub/distributions/ALTLinux/p10/branch x86_64 classic"},
			},
		}
		actions := newTestActions(repo, nil)

		resp, err := actions.Set(context.Background(), "p11", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Branch != "p11" {
			t.Errorf("expected branch p11, got %s", resp.Branch)
		}
		if len(resp.Added) != 1 {
			t.Errorf("expected 1 added, got %d", len(resp.Added))
		}
		if len(resp.Removed) != 1 {
			t.Errorf("expected 1 removed, got %d", len(resp.Removed))
		}
	})

	t.Run("branch with date shows combined name", func(t *testing.T) {
		repo := &mockRepoService{setBranchAdded: []service.Repository{
			{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/archive/p11/date/2025/01/01", Arch: "x86_64", Key: "p11", Components: []string{"classic"}, Active: true, Entry: "rpm [p11] http://ftp.altlinux.org/pub/distributions/archive/p11/date/2025/01/01 x86_64 classic"},
		}}
		actions := newTestActions(repo, nil)

		resp, err := actions.Set(context.Background(), "p11", "20250101")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Branch != "p11 20250101" {
			t.Errorf("expected branch 'p11 20250101', got '%s'", resp.Branch)
		}
	})

	t.Run("empty branch returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, nil)

		_, err := actions.Set(context.Background(), "  ", "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("service error propagates", func(t *testing.T) {
		repo := &mockRepoService{setBranchErr: errors.New("unknown branch")}
		actions := newTestActions(repo, nil)

		_, err := actions.Set(context.Background(), "p11", "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeRepository)
	})
}

func TestCheckSet(t *testing.T) {
	t.Run("success returns simulation", func(t *testing.T) {
		repo := &mockRepoService{
			simulateRemResult: []service.Repository{
				{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/p10/branch", Arch: "x86_64", Key: "p10", Components: []string{"classic"}, Active: true, Entry: "rpm [p10] http://ftp.altlinux.org/pub/distributions/ALTLinux/p10/branch x86_64 classic"},
			},
			simulateAddResult: []service.Repository{
				{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch", Arch: "x86_64", Key: "p11", Components: []string{"classic"}, Active: true, Entry: "rpm [p11] http://ftp.altlinux.org/pub/distributions/ALTLinux/p11/branch x86_64 classic"},
			},
		}
		actions := newTestActions(repo, nil)

		resp, err := actions.CheckSet(context.Background(), "p11", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.WillAdd) != 1 {
			t.Errorf("expected 1 willAdd, got %d", len(resp.WillAdd))
		}
		if len(resp.WillRemove) != 1 {
			t.Errorf("expected 1 willRemove, got %d", len(resp.WillRemove))
		}
	})

	t.Run("empty branch returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, nil)

		_, err := actions.CheckSet(context.Background(), "", "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("simulate remove error propagates", func(t *testing.T) {
		repo := &mockRepoService{simulateRemErr: errors.New("fail")}
		actions := newTestActions(repo, nil)

		_, err := actions.CheckSet(context.Background(), "p11", "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeRepository)
	})

	t.Run("simulate add error propagates", func(t *testing.T) {
		repo := &mockRepoService{
			simulateRemResult: []service.Repository{
				{Type: "rpm", URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/p10/branch", Arch: "x86_64", Key: "p10", Components: []string{"classic"}, Active: true, Entry: "rpm [p10] http://ftp.altlinux.org/pub/distributions/ALTLinux/p10/branch x86_64 classic"},
			},
			simulateAddErr: errors.New("fail"),
		}
		actions := newTestActions(repo, nil)

		_, err := actions.CheckSet(context.Background(), "p11", "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeRepository)
	})
}

func TestClean(t *testing.T) {
	t.Run("success removes temporary repos", func(t *testing.T) {
		repo := &mockRepoService{cleanResult: []service.Repository{
			{Type: "rpm", URL: "cdrom:[ALT Linux p11] /media/ALTLinux", Arch: "x86_64", Components: []string{"classic"}, Active: false, Entry: "rpm cdrom:[ALT Linux p11] /media/ALTLinux x86_64 classic"},
		}}
		actions := newTestActions(repo, nil)

		resp, err := actions.Clean(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Removed) != 1 {
			t.Errorf("expected 1 removed, got %d", len(resp.Removed))
		}
	})

	t.Run("nothing to clean returns no operation", func(t *testing.T) {
		repo := &mockRepoService{cleanResult: []service.Repository{}}
		actions := newTestActions(repo, nil)

		_, err := actions.Clean(context.Background())
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNoOperation)
	})

	t.Run("service error propagates", func(t *testing.T) {
		repo := &mockRepoService{cleanErr: errors.New("io error")}
		actions := newTestActions(repo, nil)

		_, err := actions.Clean(context.Background())
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeRepository)
	})
}

func TestCheckClean(t *testing.T) {
	t.Run("finds cdrom and task repos", func(t *testing.T) {
		repos := []service.Repository{
			{URL: "cdrom:[ALT Linux p11] /media/ALTLinux", Entry: "rpm cdrom:[ALT Linux p11] /media/ALTLinux x86_64 classic", Active: true},
			{URL: "http://git.altlinux.org/repo/370123/", Components: []string{"task"}, Entry: "rpm http://git.altlinux.org/repo/370123/ x86_64 task", Active: true},
			{URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/Sisyphus", Components: []string{"classic"}, Entry: "rpm [alt] http://ftp.altlinux.org/pub/distributions/ALTLinux/Sisyphus x86_64 classic", Active: true},
		}
		actions := newTestActions(&mockRepoService{getReposResult: repos}, nil)

		resp, err := actions.CheckClean(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.WillRemove) != 2 {
			t.Errorf("expected 2 willRemove, got %d", len(resp.WillRemove))
		}
	})

	t.Run("nothing to clean returns no operation", func(t *testing.T) {
		repos := []service.Repository{
			{URL: "http://ftp.altlinux.org/pub/distributions/ALTLinux/Sisyphus", Components: []string{"classic"}, Active: true},
		}
		actions := newTestActions(&mockRepoService{getReposResult: repos}, nil)

		_, err := actions.CheckClean(context.Background())
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNoOperation)
	})

	t.Run("service error propagates", func(t *testing.T) {
		actions := newTestActions(&mockRepoService{getReposErr: errors.New("fail")}, nil)

		_, err := actions.CheckClean(context.Background())
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeRepository)
	})
}

func TestGetBranches(t *testing.T) {
	t.Run("returns branches from service", func(t *testing.T) {
		repo := &mockRepoService{branches: []string{"sisyphus", "p11", "p10", "p9", "c10f2"}}
		actions := newTestActions(repo, nil)

		resp, err := actions.GetBranches(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Branches) != 5 {
			t.Errorf("expected 5 branches, got %d", len(resp.Branches))
		}
	})
}

func TestGetTaskPackages(t *testing.T) {
	t.Run("success returns packages", func(t *testing.T) {
		repo := &mockRepoService{taskPackagesResult: []string{"vim", "curl", "bash"}}
		actions := newTestActions(repo, nil)

		resp, err := actions.GetTaskPackages(context.Background(), "123456")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Count != 3 {
			t.Errorf("expected count=3, got %d", resp.Count)
		}
		if resp.TaskNum != "123456" {
			t.Errorf("expected taskNum=123456, got %s", resp.TaskNum)
		}
	})

	t.Run("empty task number returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, nil)

		_, err := actions.GetTaskPackages(context.Background(), "  ")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("service error propagates", func(t *testing.T) {
		repo := &mockRepoService{taskPackagesErr: errors.New("task not found")}
		actions := newTestActions(repo, nil)

		_, err := actions.GetTaskPackages(context.Background(), "999")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeRepository)
	})
}

func TestTestTask(t *testing.T) {
	t.Run("empty task number returns validation error", func(t *testing.T) {
		actions := newTestActions(nil, nil)

		_, err := actions.TestTask(context.Background(), "")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeValidation)
	})

	t.Run("no packages in task returns not found", func(t *testing.T) {
		repo := &mockRepoService{taskPackagesResult: []string{}}
		actions := newTestActions(repo, nil)

		_, err := actions.TestTask(context.Background(), "123")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNotFound)
	})

	t.Run("get task packages error propagates", func(t *testing.T) {
		repo := &mockRepoService{taskPackagesErr: errors.New("not found")}
		actions := newTestActions(repo, nil)

		_, err := actions.TestTask(context.Background(), "123")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeRepository)
	})

	t.Run("add repository error propagates", func(t *testing.T) {
		repo := &mockRepoService{
			taskPackagesResult: []string{"vim"},
			addErr:             errors.New("permission denied"),
		}
		actions := newTestActions(repo, nil)

		_, err := actions.TestTask(context.Background(), "123")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeRepository)
	})

	t.Run("apt update error propagates", func(t *testing.T) {
		repo := &mockRepoService{
			taskPackagesResult: []string{"vim"},
			addResult:          []service.Repository{{Type: "rpm", URL: "http://git.altlinux.org/repo/370123/", Arch: "x86_64", Components: []string{"task"}, Active: true, Entry: "rpm http://git.altlinux.org/repo/370123/ x86_64 task"}},
		}
		apt := &mockAptActions{updateErr: errors.New("apt update failed")}
		actions := newTestActions(repo, apt)

		_, err := actions.TestTask(context.Background(), "123")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})

	t.Run("find package error propagates", func(t *testing.T) {
		repo := &mockRepoService{
			taskPackagesResult: []string{"vim"},
			addResult:          []service.Repository{{Type: "rpm", URL: "http://git.altlinux.org/repo/370123/", Arch: "x86_64", Components: []string{"task"}, Active: true, Entry: "rpm http://git.altlinux.org/repo/370123/ x86_64 task"}},
		}
		apt := &mockAptActions{findErr: errors.New("dependency conflict")}
		actions := newTestActions(repo, apt)

		_, err := actions.TestTask(context.Background(), "123")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})

	t.Run("no changes returns no operation", func(t *testing.T) {
		repo := &mockRepoService{
			taskPackagesResult: []string{"vim"},
			addResult:          []service.Repository{{Type: "rpm", URL: "http://git.altlinux.org/repo/370123/", Arch: "x86_64", Components: []string{"task"}, Active: true, Entry: "rpm http://git.altlinux.org/repo/370123/ x86_64 task"}},
		}
		apt := &mockAptActions{
			findChanges: &aptLib.PackageChanges{NewInstalledCount: 0, UpgradedCount: 0},
		}
		actions := newTestActions(repo, apt)

		_, err := actions.TestTask(context.Background(), "123")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeNoOperation)
	})

	t.Run("success installs task packages", func(t *testing.T) {
		repo := &mockRepoService{
			taskPackagesResult: []string{"vim"},
			addResult:          []service.Repository{{Type: "rpm", URL: "http://git.altlinux.org/repo/370123/", Arch: "x86_64", Components: []string{"task"}, Active: true, Entry: "rpm http://git.altlinux.org/repo/370123/ x86_64 task"}},
		}
		apt := &mockAptActions{
			findInstall: []string{"vim"},
			findChanges: &aptLib.PackageChanges{NewInstalledCount: 1},
		}
		actions := newTestActions(repo, apt)

		resp, err := actions.TestTask(context.Background(), "123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.TaskNum != "123" {
			t.Errorf("expected taskNum=123, got %s", resp.TaskNum)
		}
		if resp.Info.NewInstalledCount != 1 {
			t.Errorf("expected 1 new install, got %d", resp.Info.NewInstalledCount)
		}
	})

	t.Run("combine error propagates", func(t *testing.T) {
		repo := &mockRepoService{
			taskPackagesResult: []string{"vim"},
			addResult:          []service.Repository{{Type: "rpm", URL: "http://git.altlinux.org/repo/370123/", Arch: "x86_64", Components: []string{"task"}, Active: true, Entry: "rpm http://git.altlinux.org/repo/370123/ x86_64 task"}},
		}
		apt := &mockAptActions{
			findInstall: []string{"vim"},
			findChanges: &aptLib.PackageChanges{NewInstalledCount: 1},
			combineErr:  errors.New("install failed"),
		}
		actions := newTestActions(repo, apt)

		_, err := actions.TestTask(context.Background(), "123")
		testutil.AssertAPMError(t, err, apmerr.ErrorTypeApt)
	})
}
