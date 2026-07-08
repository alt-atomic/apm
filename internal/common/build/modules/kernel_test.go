package modules

import (
	"context"
	"slices"
	"testing"
)

func TestKernelInfoIsEmpty(t *testing.T) {
	if !(&KernelInfo{}).IsEmpty() {
		t.Error("empty KernelInfo should be empty")
	}
	if (&KernelInfo{Flavour: "un-def"}).IsEmpty() {
		t.Error("KernelInfo with flavour is not empty")
	}
}

func TestInitrdIsEmpty(t *testing.T) {
	if !(&Initrd{}).IsEmpty() {
		t.Error("empty Initrd should be empty")
	}
	if (&Initrd{Method: "dracut"}).IsEmpty() {
		t.Error("Initrd with method is not empty")
	}
}

func TestKernelBody_Validate(t *testing.T) {
	// unknown method is rejected
	if err := (&KernelBody{Initrd: Initrd{Method: "bogus"}}).Validate(); err == nil {
		t.Error("expected error for unknown initrd method")
	}
	// known methods pass
	for _, m := range []string{"auto", "dracut", "make-initrd", "none"} {
		if err := (&KernelBody{Initrd: Initrd{Method: m}}).Validate(); err != nil {
			t.Errorf("method %q rejected: %v", m, err)
		}
	}
	// empty initrd skips the check entirely
	if err := (&KernelBody{}).Validate(); err != nil {
		t.Errorf("empty body should validate: %v", err)
	}
}

func TestKernelBody_ValidateErrorPropagates(t *testing.T) {
	b := &KernelBody{Initrd: Initrd{Method: "bogus"}}
	if _, err := b.Execute(context.Background(), &fakeDomain{}); err == nil {
		t.Fatal("expected validate error from Execute")
	}
}

func TestKernelBody_NoopWhenEmpty(t *testing.T) {
	d := &fakeDomain{}
	if _, err := (&KernelBody{}).Execute(context.Background(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if d.updateCalled {
		t.Error("empty kernel body should do nothing")
	}
}

func TestKernelBody_InitrdNoneIsNoop(t *testing.T) {
	d := &fakeDomain{}
	b := &KernelBody{Initrd: Initrd{Method: "none"}}
	if _, err := b.Execute(context.Background(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
}

func TestKernelBody_InstallsKernel(t *testing.T) {
	km := &fakeKernelManager{}
	d := &fakeDomain{kernelMgr: km}
	b := &KernelBody{
		KernelInfo: KernelInfo{Flavour: "un-def", Modules: []string{"kvm"}},
		Initrd:     Initrd{Method: "none"},
	}

	if _, err := b.Execute(context.Background(), d); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if !km.installCalled {
		t.Fatal("InstallKernel was not called")
	}
	if km.installedInfo == nil || km.installedInfo.Flavour != "un-def" {
		t.Errorf("installed flavour = %v, want un-def", km.installedInfo)
	}
	if !slices.Contains(km.installModules, "kvm") {
		t.Errorf("install modules = %v, want to contain kvm", km.installModules)
	}
	// package DB is refreshed after install
	if !d.updateCalled {
		t.Error("expected package DB update after kernel install")
	}
}
