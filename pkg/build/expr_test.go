package build

import (
	"reflect"
	"testing"
)

func testData() ExprData {
	return ExprData{
		Env:     map[string]string{"FOO": "bar"},
		Version: Version{Major: 5, Minor: 2, Patch: 1},
	}
}

func TestResolveExpr_Placeholders(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no placeholder", "plain text", "plain text"},
		{"version field", "v${{ Version.Major }}", "v5"},
		{"env access", "x=${{ Env.FOO }}", "x=bar"},
		{"multiple", "${{ Version.Major }}.${{ Version.Minor }}", "5.2"},
		{"whitespace inside", "${{   Version.Patch   }}", "1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ResolveExpr(c.in, testData())
			if err != nil {
				t.Fatalf("ResolveExpr(%q) = %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ResolveExpr(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestResolveExpr_UnknownVarErrors(t *testing.T) {
	if _, err := ResolveExpr("${{ Nonexistent }}", testData()); err == nil {
		t.Fatal("expected error for unknown variable")
	}
}

func TestResolveExpr_FirstErrorPropagates(t *testing.T) {
	// second placeholder is broken; the error must not be swallowed
	if _, err := ResolveExpr("${{ Version.Major }} ${{ Nope }}", testData()); err == nil {
		t.Fatal("expected error from the second placeholder")
	}
}

func TestResolveExprMap(t *testing.T) {
	got, err := ResolveExprMap(map[string]string{
		"ver":    "${{ Version.Major }}",
		"static": "plain",
	}, testData())
	if err != nil {
		t.Fatalf("ResolveExprMap() = %v", err)
	}
	if got["ver"] != "5" || got["static"] != "plain" {
		t.Errorf("ResolveExprMap() = %v", got)
	}
}

func TestResolveExprMap_ErrorPropagates(t *testing.T) {
	if _, err := ResolveExprMap(map[string]string{"bad": "${{ Nope }}"}, testData()); err == nil {
		t.Fatal("expected error to propagate")
	}
}

type sampleBody struct {
	Name string
	Tags []string
	Vars map[string]string
	// unexported field must be skipped, not panic
	hidden string
}

func TestResolveStruct(t *testing.T) {
	b := &sampleBody{
		Name: "v${{ Version.Major }}",
		Tags: []string{"${{ Env.FOO }}", "static"},
		Vars: map[string]string{"k": "${{ Version.Minor }}"},
	}
	if err := ResolveStruct(b, testData()); err != nil {
		t.Fatalf("ResolveStruct() = %v", err)
	}
	if b.Name != "v5" {
		t.Errorf("Name = %q, want v5", b.Name)
	}
	if !reflect.DeepEqual(b.Tags, []string{"bar", "static"}) {
		t.Errorf("Tags = %v", b.Tags)
	}
	if b.Vars["k"] != "2" {
		t.Errorf("Vars[k] = %q, want 2", b.Vars["k"])
	}
}

func TestResolveStruct_RequiresPointer(t *testing.T) {
	if err := ResolveStruct(sampleBody{}, testData()); err == nil {
		t.Fatal("expected error for non-pointer argument")
	}
}

func TestExtractExprResult_Types(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"1 + 1", "2"},
		{"true", "true"},
		{`"hi"`, "hi"},
		{"Version.Major * 2", "10"},
	}
	for _, c := range cases {
		got, err := ExtractExprResult(c.expr, testData())
		if err != nil {
			t.Errorf("ExtractExprResult(%q) = %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("ExtractExprResult(%q) = %q, want %q", c.expr, got, c.want)
		}
	}
}

func TestExtractExprResult_UnsupportedType(t *testing.T) {
	// a map result is not a supported scalar output
	if _, err := ExtractExprResult("Env", testData()); err == nil {
		t.Fatal("expected error for unsupported output type")
	}
}

func TestExtractExprResultBool(t *testing.T) {
	cases := []struct {
		expr string
		want bool
	}{
		{"1 == 1", true},
		{"Version.Major > 10", false},
	}
	for _, c := range cases {
		got, err := ExtractExprResultBool(c.expr, testData())
		if err != nil {
			t.Errorf("ExtractExprResultBool(%q) = %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("ExtractExprResultBool(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestExtractExprResultBool_CompileError(t *testing.T) {
	if _, err := ExtractExprResultBool("1 +", testData()); err == nil {
		t.Fatal("expected compile error")
	}
}
