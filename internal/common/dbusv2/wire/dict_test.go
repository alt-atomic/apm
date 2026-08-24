package wire

import (
	"errors"
	"strings"
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
)

func TestParseOptions(t *testing.T) {
	var flag bool
	var text string
	var num uint32

	opts := Dict{
		"flag": V(true),
		"text": V("hello"),
		"num":  V(uint32(7)),
	}
	spec := map[string]any{"flag": &flag, "text": &text, "num": &num}

	if err := ParseOptions(opts, spec); err != nil {
		t.Fatalf("ParseOptions: %v", err)
	}
	if !flag || text != "hello" || num != 7 {
		t.Errorf("parsed values: flag=%v text=%q num=%d", flag, text, num)
	}
}

func TestParseOptionsUnknownKey(t *testing.T) {
	err := ParseOptions(Dict{"bogus": V(true)}, map[string]any{})
	assertValidation(t, err, "bogus")
}

func TestParseOptionsTypeMismatch(t *testing.T) {
	var flag bool
	err := ParseOptions(Dict{"flag": V("not a bool")}, map[string]any{"flag": &flag})
	assertValidation(t, err, "flag")
}

func TestParseOptionsEmpty(t *testing.T) {
	if err := ParseOptions(nil, map[string]any{}); err != nil {
		t.Fatalf("ParseOptions(nil): %v", err)
	}
}

func assertValidation(t *testing.T, err error, substr string) {
	t.Helper()
	if apmErr, ok := errors.AsType[apmerr.APMError](err); !ok || apmErr.Type != apmerr.ErrorTypeValidation {
		t.Fatalf("err = %v, want VALIDATION", err)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Errorf("error %q does not mention %q", err, substr)
	}
}
