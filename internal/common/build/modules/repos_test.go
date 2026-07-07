package modules

import (
	"context"
	"testing"

	reposervice "altlinux.space/alt-atomic/apm/pkg/aptrepo"
)

func TestRepoEntries(t *testing.T) {
	repos := []reposervice.Repository{{Entry: "deb a"}, {Entry: "deb b"}}
	if got := repoEntries(repos, ", "); got != "deb a, deb b" {
		t.Errorf("repoEntries = %q", got)
	}
	if got := repoEntries(nil, ","); got != "" {
		t.Errorf("repoEntries(nil) = %q, want empty", got)
	}
}

func TestReposBody_UpdatesByDefault(t *testing.T) {
	d := &fakeDomain{repoMgr: &fakeRepoManager{}}

	if _, err := (&ReposBody{}).Execute(context.Background(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if !d.updateCalled {
		t.Error("expected package DB update by default")
	}
	if len(d.installCalls) != 1 || len(d.installCalls[0]) != 1 || d.installCalls[0][0] != "ca-certificates" {
		t.Errorf("installCalls = %v, want [[ca-certificates]]", d.installCalls)
	}
}

func TestReposBody_NoUpdateSkips(t *testing.T) {
	d := &fakeDomain{repoMgr: &fakeRepoManager{}}

	if _, err := (&ReposBody{NoUpdate: true}).Execute(context.Background(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if d.updateCalled || len(d.installCalls) != 0 {
		t.Error("no-update must skip update and ca-certificates install")
	}
}

func TestReposBody_UnknownBranch(t *testing.T) {
	d := &fakeDomain{repoMgr: &fakeRepoManager{branches: []string{"sisyphus"}}}

	if _, err := (&ReposBody{Branch: "p10"}).Execute(context.Background(), d); err == nil {
		t.Fatal("expected unknown branch error")
	}
}

func TestReposBody_CleanAndSetBranch(t *testing.T) {
	mgr := &fakeRepoManager{branches: []string{"p11"}}
	d := &fakeDomain{repoMgr: mgr}
	b := &ReposBody{Clean: true, Branch: "p11", Date: "latest", NoUpdate: true}

	if _, err := b.Execute(context.Background(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if !mgr.removeAll {
		t.Error("clean should remove all repos")
	}
	if mgr.setBranch != "p11" {
		t.Errorf("setBranch = %q, want p11", mgr.setBranch)
	}
}

func TestReposBody_CleanTemporary(t *testing.T) {
	mgr := &fakeRepoManager{}
	d := &fakeDomain{repoMgr: mgr}

	if _, err := (&ReposBody{CleanTemporary: true, NoUpdate: true}).Execute(context.Background(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if !mgr.cleanCalled {
		t.Error("clean-temporary should call CleanTemporary")
	}
}

func TestReposBody_CustomAndTasks(t *testing.T) {
	mgr := &fakeRepoManager{}
	d := &fakeDomain{repoMgr: mgr}
	b := &ReposBody{Custom: []string{"deb x"}, Tasks: []string{"12345"}, NoUpdate: true}

	if _, err := b.Execute(context.Background(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if len(mgr.addedArgs) != 2 {
		t.Errorf("AddRepository calls = %v, want 2", mgr.addedArgs)
	}
}
