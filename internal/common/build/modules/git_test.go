package modules

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
)

func TestGitBody_SelectsBuildDeps(t *testing.T) {
	d := &fakeDomain{
		runner: &fakeRunner{},
		packages: map[string]*_package.Package{
			"libfoo": {Name: "libfoo", Installed: false},
			"gcc":    {Name: "gcc", Installed: false},
		},
	}
	b := &GitBody{Url: "u", Command: "make", Deps: []string{"libfoo"}, BuildDeps: []string{"gcc"}}

	if _, err := b.Execute(context.Background(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	// first combine installs both deps, second removes only the build-only dep
	if len(d.combineCalls) != 2 {
		t.Fatalf("combineCalls = %v, want 2", d.combineCalls)
	}
	if !reflect.DeepEqual(d.combineCalls[0], []string{"libfoo+", "gcc+"}) {
		t.Errorf("install ops = %v", d.combineCalls[0])
	}
	if !reflect.DeepEqual(d.combineCalls[1], []string{"gcc-"}) {
		t.Errorf("remove ops = %v, want [gcc-]", d.combineCalls[1])
	}
}

func TestGitBody_SkipsInstalledDeps(t *testing.T) {
	d := &fakeDomain{
		runner:   &fakeRunner{},
		packages: map[string]*_package.Package{"libfoo": {Name: "libfoo", Installed: true}},
	}
	b := &GitBody{Url: "u", Command: "make", Deps: []string{"libfoo"}}

	if _, err := b.Execute(context.Background(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	// already-installed dep triggers no package operations
	if len(d.combineCalls) != 0 {
		t.Errorf("combineCalls = %v, want none", d.combineCalls)
	}
}

func TestGitBody_ClonesWithRevision(t *testing.T) {
	runner := &fakeRunner{}
	d := &fakeDomain{runner: runner}
	b := &GitBody{Url: "https://x/y.git", Command: "make", Rev: "abc123"}

	if _, err := b.Execute(context.Background(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("runner calls = %v, want clone + command", runner.calls)
	}
	// clone carries the pinned revision and the url
	clone := runner.calls[0][0]
	if !strings.Contains(clone, "clone --revision=abc123") || !strings.Contains(clone, "https://x/y.git") {
		t.Errorf("clone command = %q", clone)
	}
	// the build command runs afterwards
	if !reflect.DeepEqual(runner.calls[1], []string{"make"}) {
		t.Errorf("build command = %v", runner.calls[1])
	}
}

func TestGitBody_GetPackageErrorPropagates(t *testing.T) {
	d := &fakeDomain{runner: &fakeRunner{}, getErr: errors.New("db down")}
	b := &GitBody{Url: "u", Command: "make", Deps: []string{"libfoo"}}

	if _, err := b.Execute(context.Background(), d); err == nil {
		t.Fatal("expected GetPackageByName error to propagate")
	}
}
