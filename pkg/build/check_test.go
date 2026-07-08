package build

import "testing"

func TestKebabPascalRoundTrip(t *testing.T) {
	cases := []struct{ kebab, pascal string }{
		{"create-file-perm", "CreateFilePerm"},
		{"source", "Source"},
		{"", ""},
	}
	for _, c := range cases {
		if got := KebabToPascal(c.kebab); got != c.pascal {
			t.Errorf("KebabToPascal(%q) = %q, want %q", c.kebab, got, c.pascal)
		}
	}
}

func TestBodyTypeToType(t *testing.T) {
	if got := BodyTypeToType("CopyBody"); got != "copy" {
		t.Errorf("BodyTypeToType(CopyBody) = %q, want copy", got)
	}
}

type requiredBody struct {
	Source string `required:""`
}

func TestCheckBody_Required(t *testing.T) {
	if err := CheckBody(&requiredBody{}); err == nil {
		t.Error("expected error for missing required field")
	}
	if err := CheckBody(&requiredBody{Source: "x"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

type conflictBody struct {
	Enabled bool `conflicts:"Masked"`
	Masked  bool `conflicts:"Enabled"`
}

func TestCheckBody_Conflicts(t *testing.T) {
	if err := CheckBody(&conflictBody{Enabled: true, Masked: true}); err == nil {
		t.Error("expected error when conflicting fields are both set")
	}
	if err := CheckBody(&conflictBody{Enabled: true}); err != nil {
		t.Errorf("unexpected error for single field: %v", err)
	}
}

type needsBody struct {
	CreateLink  bool `needs:"Destination"`
	Destination string
}

func TestCheckBody_Needs(t *testing.T) {
	if err := CheckBody(&needsBody{CreateLink: true}); err == nil {
		t.Error("expected error when needed field is missing")
	}
	if err := CheckBody(&needsBody{CreateLink: true, Destination: "/x"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// needs is ignored when the field itself is empty
	if err := CheckBody(&needsBody{}); err != nil {
		t.Errorf("unexpected error for empty needs field: %v", err)
	}
}

func TestCheckBody_PanicsOnNonPointer(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for non-pointer argument")
		}
	}()
	_ = CheckBody(requiredBody{})
}
