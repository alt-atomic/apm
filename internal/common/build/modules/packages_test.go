package modules

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestPackagesBody_BuildsOps(t *testing.T) {
	d := &fakeDomain{}
	b := &PackagesBody{Install: []string{"foo", "bar"}, Remove: []string{"baz"}, Depends: true}

	if _, err := b.Execute(t.Context(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	want := []string{"foo+", "bar+", "baz-"}
	if len(d.combineCalls) != 1 || !reflect.DeepEqual(d.combineCalls[0], want) {
		t.Errorf("combineCalls = %v, want [%v]", d.combineCalls, want)
	}
	if !d.combineDepends {
		t.Error("depends flag not forwarded")
	}
}

func TestPackagesBody_OptionsAreRequestScoped(t *testing.T) {
	d := &fakeDomain{}
	b := &PackagesBody{Options: map[string]string{"K": "V"}, Install: []string{"x"}}

	if _, err := b.Execute(t.Context(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if !reflect.DeepEqual(d.combineConfig, map[string]string{"K": "V"}) {
		t.Fatalf("APT config = %v, want map[K:V]", d.combineConfig)
	}
}

func TestPackagesBody_UpdateUpgradeGating(t *testing.T) {
	d := &fakeDomain{}
	b := &PackagesBody{Update: true, Upgrade: true}

	if _, err := b.Execute(context.Background(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if !d.updateCalled || !d.upgradeCalled {
		t.Errorf("update=%v upgrade=%v, want both true", d.updateCalled, d.upgradeCalled)
	}
	// no install/remove means no combine call
	if len(d.combineCalls) != 0 {
		t.Errorf("combineCalls = %v, want none", d.combineCalls)
	}
}

func TestPackagesBody_NoopWhenEmpty(t *testing.T) {
	d := &fakeDomain{}
	b := &PackagesBody{}

	if _, err := b.Execute(context.Background(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if len(d.combineCalls) != 0 || d.updateCalled || d.upgradeCalled {
		t.Error("empty body should do nothing")
	}
}

func TestPackagesBody_CombineErrorPropagates(t *testing.T) {
	d := &fakeDomain{combineErr: errors.New("apt failed")}
	b := &PackagesBody{Install: []string{"x"}}

	if _, err := b.Execute(context.Background(), d); err == nil {
		t.Fatal("expected combine error to propagate")
	}
}

func TestPackagesBody_RejectsNonDomainContext(t *testing.T) {
	b := &PackagesBody{Install: []string{"x"}}
	if _, err := b.Execute(context.Background(), bareRuntime{}); err == nil {
		t.Fatal("expected error for non-domain context")
	}
}
