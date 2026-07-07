package modules

import (
	"testing"

	"altlinux.space/alt-atomic/apm/pkg/build"
)

// each module type registers a working body factory
func TestRegisteredTypes(t *testing.T) {
	types := []string{
		TypeCopy, TypeInclude, TypeLink, TypeMerge, TypeMkdir,
		TypeMove, TypeNetwork, TypeRemove, TypeReplace, TypeShell, TypeSystemd,
	}
	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			body, err := build.NewBody(typ)
			if err != nil {
				t.Fatalf("NewBody(%q) = %v", typ, err)
			}
			if body == nil {
				t.Fatalf("NewBody(%q) returned nil body", typ)
			}
		})
	}
}
