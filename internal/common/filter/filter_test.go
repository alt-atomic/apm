package filter

import (
	"testing"
)

func TestSplitFilterString(t *testing.T) {
	tests := []struct {
		input     string
		wantField string
		wantOp    Op
		wantValue string
		wantErr   bool
	}{
		{"name=zip", "name", "", "zip", false},
		{"name[eq]=zip", "name", OpEq, "zip", false},
		{"name[like]=zip", "name", OpLike, "zip", false},
		{"size[gt]=1000", "size", OpGt, "1000", false},
		{"size[gte]=1000", "size", OpGte, "1000", false},
		{"size[lt]=500", "size", OpLt, "500", false},
		{"size[lte]=500", "size", OpLte, "500", false},
		{"name[ne]=test", "name", OpNe, "test", false},
		{"depends[contains]=libgtk", "depends", OpContains, "libgtk", false},
		{"  name[eq] = zip ", "name", OpEq, "zip", false},
		// errors
		{"noequals", "", "", "", true},
		{"name=", "", "", "", true},
		{"=value", "", "", "", true},
		{"name[eq=value", "", "", "", true},
		// SQL injection attempt
		{"field); DROP TABLE--[eq]=x", "field); DROP TABLE--", OpEq, "x", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			field, op, value, err := splitFilterString(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if field != tt.wantField {
				t.Errorf("field: got %q, want %q", field, tt.wantField)
			}
			if op != tt.wantOp {
				t.Errorf("op: got %q, want %q", op, tt.wantOp)
			}
			if value != tt.wantValue {
				t.Errorf("value: got %q, want %q", value, tt.wantValue)
			}
		})
	}
}

func TestConfigParse(t *testing.T) {
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"name":      {DefaultOp: OpLike},
			"installed": {DefaultOp: OpEq, AllowedOps: []Op{OpEq}},
			"size":      {DefaultOp: OpEq},
			"depends":   {DefaultOp: OpContains, AllowedOps: []Op{OpContains, OpLike}},
		},
	}

	t.Run("default op", func(t *testing.T) {
		filters, err := cfg.Parse([]string{"name=zip"})
		if err != nil {
			t.Fatal(err)
		}
		if len(filters) != 1 {
			t.Fatalf("expected 1 filter, got %d", len(filters))
		}
		if filters[0].Op != OpLike {
			t.Errorf("expected op %q, got %q", OpLike, filters[0].Op)
		}
		if filters[0].Value != "zip" {
			t.Errorf("expected value %q, got %q", "zip", filters[0].Value)
		}
	})

	t.Run("explicit op", func(t *testing.T) {
		filters, err := cfg.Parse([]string{"name[eq]=zip"})
		if err != nil {
			t.Fatal(err)
		}
		if filters[0].Op != OpEq {
			t.Errorf("expected op %q, got %q", OpEq, filters[0].Op)
		}
	})

	t.Run("disallowed op", func(t *testing.T) {
		_, err := cfg.Parse([]string{"installed[like]=true"})
		if err == nil {
			t.Fatal("expected error for disallowed op")
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		_, err := cfg.Parse([]string{"unknown=value"})
		if err == nil {
			t.Fatal("expected error for unknown field")
		}
	})

	t.Run("multiple filters", func(t *testing.T) {
		filters, err := cfg.Parse([]string{"name=zip", "installed=true", "size[gt]=100"})
		if err != nil {
			t.Fatal(err)
		}
		if len(filters) != 3 {
			t.Fatalf("expected 3 filters, got %d", len(filters))
		}
	})

	t.Run("empty and whitespace", func(t *testing.T) {
		filters, err := cfg.Parse([]string{"", "  ", "name=zip"})
		if err != nil {
			t.Fatal(err)
		}
		if len(filters) != 1 {
			t.Fatalf("expected 1 filter, got %d", len(filters))
		}
	})

	t.Run("unknown operator", func(t *testing.T) {
		_, err := cfg.Parse([]string{"name[foo]=bar"})
		if err == nil {
			t.Fatal("expected error for unknown operator")
		}
	})

	t.Run("sql injection in field name", func(t *testing.T) {
		_, err := cfg.Parse([]string{"name; DROP TABLE--=value"})
		if err == nil {
			t.Fatal("expected error for unsafe field name")
		}
	})

	t.Run("default op fallback to eq", func(t *testing.T) {
		cfgNoDefault := &Config{
			Fields: map[string]FieldConfig{
				"test": {},
			},
		}
		filters, err := cfgNoDefault.Parse([]string{"test=value"})
		if err != nil {
			t.Fatal(err)
		}
		if filters[0].Op != OpEq {
			t.Errorf("expected fallback op %q, got %q", OpEq, filters[0].Op)
		}
	})
}

func TestConfigParseWithPrefixes(t *testing.T) {
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"name": {DefaultOp: OpLike},
		},
		Prefixes: map[string]FieldConfig{
			"app.": {DefaultOp: OpLike},
		},
	}

	t.Run("prefix field", func(t *testing.T) {
		filters, err := cfg.Parse([]string{"app.name=test"})
		if err != nil {
			t.Fatal(err)
		}
		if filters[0].Field != "app.name" {
			t.Errorf("expected field %q, got %q", "app.name", filters[0].Field)
		}
		if filters[0].Op != OpLike {
			t.Errorf("expected op %q, got %q", OpLike, filters[0].Op)
		}
	})

	t.Run("prefix field with explicit op", func(t *testing.T) {
		filters, err := cfg.Parse([]string{"app.categories[eq]=Game"})
		if err != nil {
			t.Fatal(err)
		}
		if filters[0].Op != OpEq {
			t.Errorf("expected op %q, got %q", OpEq, filters[0].Op)
		}
	})

	t.Run("prefix sql injection blocked", func(t *testing.T) {
		_, err := cfg.Parse([]string{"app.'); DROP TABLE--=x"})
		if err == nil {
			t.Fatal("expected error for unsafe prefix field name")
		}
	})
}

func TestEscapeLike(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"100%", "100\\%"},
		{"under_score", "under\\_score"},
		{"back\\slash", "back\\\\slash"},
		{"%_\\", "\\%\\_\\\\"},
	}
	for _, tt := range tests {
		got := escapeLike(tt.input)
		if got != tt.want {
			t.Errorf("escapeLike(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidateSortField(t *testing.T) {
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"name": {Sortable: true},
			"size": {Sortable: false},
		},
	}

	if err := cfg.ValidateSortField("name"); err != nil {
		t.Errorf("expected valid sort field: %v", err)
	}
	if err := cfg.ValidateSortField("size"); err == nil {
		t.Error("expected error for non-sortable field")
	}
	if err := cfg.ValidateSortField("unknown"); err == nil {
		t.Error("expected error for unknown field")
	}
	if err := cfg.ValidateSortField("name; DROP TABLE--"); err == nil {
		t.Error("expected error for unsafe sort field name")
	}
}

func TestSplitOrValues(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"zip", []string{"zip"}},
		{"zip|rar", []string{"zip", "rar"}},
		{"zip|rar|tar", []string{"zip", "rar", "tar"}},
		{"zip | rar", []string{"zip", "rar"}},
		{"zip||rar", []string{"zip", "rar"}},
		{"|", []string{"|"}},
		{"no-pipe", []string{"no-pipe"}},
	}
	for _, tt := range tests {
		got := SplitOrValues(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("SplitOrValues(%q): got %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("SplitOrValues(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestConfigParseOrValue(t *testing.T) {
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"name":    {DefaultOp: OpLike},
			"section": {DefaultOp: OpEq},
		},
	}

	t.Run("or value preserved in filter", func(t *testing.T) {
		filters, err := cfg.Parse([]string{"name[like]=zip|rar"})
		if err != nil {
			t.Fatal(err)
		}
		if filters[0].Value != "zip|rar" {
			t.Errorf("expected value %q, got %q", "zip|rar", filters[0].Value)
		}
	})

	t.Run("or with default op", func(t *testing.T) {
		filters, err := cfg.Parse([]string{"section=games|education"})
		if err != nil {
			t.Fatal(err)
		}
		if filters[0].Op != OpEq {
			t.Errorf("expected op %q, got %q", OpEq, filters[0].Op)
		}
		if filters[0].Value != "games|education" {
			t.Errorf("expected value %q, got %q", "games|education", filters[0].Value)
		}
	})
}

func TestIsSafeFieldName(t *testing.T) {
	safe := []string{"name", "app.name", "installed_size", "a1", "_field"}
	for _, s := range safe {
		if !IsSafeFieldName(s) {
			t.Errorf("expected %q to be safe", s)
		}
	}
	unsafe := []string{"'; DROP TABLE", "field); --", "a b", "name=1", "", "123abc", "a,b"}
	for _, s := range unsafe {
		if IsSafeFieldName(s) {
			t.Errorf("expected %q to be unsafe", s)
		}
	}
}
