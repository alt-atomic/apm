package modules

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"altlinux.space/alt-atomic/apm/pkg/build"
)

func TestIncludeBody_IncludeTargets(t *testing.T) {
	b := &IncludeBody{Targets: []string{"a.yml", "b.yml"}}
	if got := b.IncludeTargets(); !reflect.DeepEqual(got, []string{"a.yml", "b.yml"}) {
		t.Errorf("IncludeTargets() = %v", got)
	}
}

func TestIncludeBody_SingleTargetReturnsRaw(t *testing.T) {
	want := map[string]*build.MapModule{"mod": {Id: "mod"}}
	rt := &fakeRuntime{includeFn: func(_ context.Context, target string) (map[string]*build.MapModule, error) {
		return want, nil
	}}

	b := &IncludeBody{Targets: []string{"one.yml"}}
	out, err := b.Execute(context.Background(), rt)
	if err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	// single target returns the include output directly
	got, ok := out.(map[string]*build.MapModule)
	if !ok {
		t.Fatalf("output type = %T, want map[string]*build.MapModule", out)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("output = %v, want %v", got, want)
	}
}

func TestIncludeBody_MultipleTargetsKeyedByTarget(t *testing.T) {
	rt := &fakeRuntime{includeFn: func(_ context.Context, target string) (map[string]*build.MapModule, error) {
		return map[string]*build.MapModule{target: {Id: target}}, nil
	}}

	b := &IncludeBody{Targets: []string{"a.yml", "b.yml"}}
	out, err := b.Execute(context.Background(), rt)
	if err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	got, ok := out.(map[string]map[string]*build.MapModule)
	if !ok {
		t.Fatalf("output type = %T", out)
	}
	if len(got) != 2 || got["a.yml"] == nil || got["b.yml"] == nil {
		t.Errorf("output = %v, want keys a.yml and b.yml", got)
	}
}

func TestIncludeBody_ErrorPropagates(t *testing.T) {
	rt := &fakeRuntime{includeFn: func(_ context.Context, _ string) (map[string]*build.MapModule, error) {
		return nil, errors.New("include failed")
	}}

	b := &IncludeBody{Targets: []string{"x.yml"}}
	if _, err := b.Execute(context.Background(), rt); err == nil {
		t.Fatal("expected include error to propagate")
	}
}
