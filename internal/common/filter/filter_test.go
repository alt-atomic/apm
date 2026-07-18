package filter

import (
	"sort"
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
		// оператор допустим только в конце ключа
		{"name[eq]|summary[like]=zip", "", "", "", true},
		{"name[eq]junk=zip", "", "", "", true},
		{"name|summary[like]=zip", "name|summary", OpLike, "zip", false},
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

func TestColOpToSQL(t *testing.T) {
	tests := []struct {
		name    string
		col     string
		op      Op
		value   string
		wantCol string
		wantOp  string
		wantVal string
	}{
		{"eq", "name", OpEq, "test", "name", "= ?", "test"},
		{"ne", "name", OpNe, "test", "name", "<> ?", "test"},
		{"like", "name", OpLike, "test", "name", "LIKE ? ESCAPE '\\'", "%test%"},
		{"like with special chars", "name", OpLike, "100%", "name", "LIKE ? ESCAPE '\\'", "%100\\%%"},
		{"gt", "size", OpGt, "100", "size", "> ?", "100"},
		{"gte", "size", OpGte, "100", "size", ">= ?", "100"},
		{"lt", "size", OpLt, "100", "size", "< ?", "100"},
		{"lte", "size", OpLte, "100", "size", "<= ?", "100"},
		{"contains", "depends", OpContains, "libgtk", "(',' || depends || ',')", "LIKE ? ESCAPE '\\'", "%,libgtk,%"},
		{"contains with underscore", "tags", OpContains, "my_tag", "(',' || tags || ',')", "LIKE ? ESCAPE '\\'", "%,my\\_tag,%"},
		{"default fallback", "name", "unknown", "test", "name", "= ?", "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col, op, val := ColOpToSQL(tt.col, tt.op, tt.value)
			if col != tt.wantCol {
				t.Errorf("col: got %q, want %q", col, tt.wantCol)
			}
			if op != tt.wantOp {
				t.Errorf("op: got %q, want %q", op, tt.wantOp)
			}
			if val != tt.wantVal {
				t.Errorf("val: got %q, want %q", val, tt.wantVal)
			}
		})
	}
}

func TestConfigFieldsInfo(t *testing.T) {
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"name": {DefaultOp: OpLike, AllowedOps: []Op{OpEq, OpLike}, Sortable: true, Extra: map[string]any{"type": "STRING"}},
			"size": {DefaultOp: OpEq, Sortable: true},
		},
	}

	info := cfg.FieldsInfo()
	if len(info) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(info))
	}

	byName := make(map[string]FieldInfo)
	for _, fi := range info {
		byName[fi.Name] = fi
	}

	nameInfo := byName["name"]
	if nameInfo.DefaultOp != OpLike {
		t.Errorf("name.DefaultOp: got %q, want %q", nameInfo.DefaultOp, OpLike)
	}
	if len(nameInfo.AllowedOps) != 2 {
		t.Errorf("name.AllowedOps: got %d, want 2", len(nameInfo.AllowedOps))
	}
	if !nameInfo.Sortable {
		t.Error("name should be sortable")
	}
	if nameInfo.Extra["type"] != "STRING" {
		t.Errorf("name.Extra[type]: got %v, want STRING", nameInfo.Extra["type"])
	}

	sizeInfo := byName["size"]
	if len(sizeInfo.AllowedOps) != len(AllOps) {
		t.Errorf("size.AllowedOps should default to AllOps, got %d", len(sizeInfo.AllowedOps))
	}
}

func TestConfigAllowedFields(t *testing.T) {
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"name":    {},
			"size":    {},
			"version": {},
		},
	}

	fields := cfg.AllowedFields()
	sort.Strings(fields)
	expected := []string{"name", "size", "version"}
	if len(fields) != len(expected) {
		t.Fatalf("expected %d fields, got %d", len(expected), len(fields))
	}
	for i := range fields {
		if fields[i] != expected[i] {
			t.Errorf("field[%d]: got %q, want %q", i, fields[i], expected[i])
		}
	}
}

func TestConfigSortableFields(t *testing.T) {
	cfg := &Config{
		Fields: map[string]FieldConfig{
			"name":      {Sortable: true},
			"size":      {Sortable: true},
			"installed": {Sortable: false},
		},
	}

	fields := cfg.SortableFields()
	sort.Strings(fields)
	if len(fields) != 2 {
		t.Fatalf("expected 2 sortable fields, got %d", len(fields))
	}
	expected := []string{"name", "size"}
	for i := range fields {
		if fields[i] != expected[i] {
			t.Errorf("field[%d]: got %q, want %q", i, fields[i], expected[i])
		}
	}
}

func TestIsValidOp(t *testing.T) {
	for _, op := range AllOps {
		if !isValidOp(op) {
			t.Errorf("expected %q to be valid", op)
		}
	}
	if isValidOp("invalid") {
		t.Error("expected 'invalid' to be invalid op")
	}
}

func groupsTestConfig() *Config {
	return &Config{
		Fields: map[string]FieldConfig{
			"name":        {DefaultOp: OpLike},
			"description": {DefaultOp: OpLike},
			"installed":   {DefaultOp: OpEq, AllowedOps: []Op{OpEq, OpNe}},
			"size":        {DefaultOp: OpEq},
		},
	}
}

func TestValidateMultiField(t *testing.T) {
	cfg := groupsTestConfig()

	t.Run("valid multi-field", func(t *testing.T) {
		filters, err := cfg.Validate([]Filter{{Field: "name|description", Op: OpLike, Value: "x"}})
		if err != nil {
			t.Fatal(err)
		}
		if filters[0].Field != "name|description" {
			t.Errorf("expected normalized field, got %q", filters[0].Field)
		}
	})

	t.Run("normalizes spaces", func(t *testing.T) {
		filters, err := cfg.Validate([]Filter{{Field: "name | description", Op: OpLike, Value: "x"}})
		if err != nil {
			t.Fatal(err)
		}
		if filters[0].Field != "name|description" {
			t.Errorf("expected normalized field, got %q", filters[0].Field)
		}
	})

	t.Run("uniform default op", func(t *testing.T) {
		filters, err := cfg.Validate([]Filter{{Field: "name|description", Value: "x"}})
		if err != nil {
			t.Fatal(err)
		}
		if filters[0].Op != OpLike {
			t.Errorf("expected op %q, got %q", OpLike, filters[0].Op)
		}
	})

	t.Run("differing default ops require explicit op", func(t *testing.T) {
		if _, err := cfg.Validate([]Filter{{Field: "name|size", Value: "x"}}); err == nil {
			t.Fatal("expected error: name defaults to like, size to eq")
		}
		filters, err := cfg.Validate([]Filter{{Field: "name|size", Op: OpEq, Value: "x"}})
		if err != nil {
			t.Fatal(err)
		}
		if filters[0].Op != OpEq {
			t.Errorf("expected explicit op kept, got %q", filters[0].Op)
		}
	})

	t.Run("unknown subfield rejected", func(t *testing.T) {
		if _, err := cfg.Validate([]Filter{{Field: "name|unknown", Op: OpLike, Value: "x"}}); err == nil {
			t.Fatal("expected error for unknown subfield")
		}
	})

	t.Run("op disallowed for one subfield rejected", func(t *testing.T) {
		if _, err := cfg.Validate([]Filter{{Field: "name|installed", Op: OpLike, Value: "x"}}); err == nil {
			t.Fatal("expected error: like is not allowed for installed")
		}
	})

	t.Run("unsafe subfield rejected", func(t *testing.T) {
		if _, err := cfg.Validate([]Filter{{Field: "name|na me", Op: OpLike, Value: "x"}}); err == nil {
			t.Fatal("expected error for unsafe subfield")
		}
	})
}

func TestMakeGroups(t *testing.T) {
	filters := []Filter{{Field: "installed", Op: OpEq, Value: "true"}, {Field: "size", Op: OpGt, Value: "1"}}
	orFilters := []Filter{{Field: "name", Op: OpLike, Value: "x"}, {Field: "description", Op: OpLike, Value: "x"}}

	groups := MakeGroups(filters, orFilters)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if len(groups[0]) != 1 || len(groups[1]) != 1 {
		t.Errorf("expected AND filters as single-element groups, got %v", groups)
	}
	if len(groups[2]) != 2 {
		t.Errorf("expected orFilters as one group of 2, got %v", groups[2])
	}

	if got := MakeGroups(nil, nil); len(got) != 0 {
		t.Errorf("expected no groups, got %v", got)
	}
	if got := AndGroups(filters); len(got) != 2 {
		t.Errorf("expected 2 groups, got %v", got)
	}
}

func TestConfigParseGroups(t *testing.T) {
	cfg := groupsTestConfig()

	t.Run("filters and or-filters", func(t *testing.T) {
		groups, err := cfg.ParseGroups(
			[]string{"installed=true"},
			[]string{"name[like]=office", "description[like]=office"},
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(groups) != 2 {
			t.Fatalf("expected 2 groups, got %d", len(groups))
		}
		if len(groups[0]) != 1 || groups[0][0].Field != "installed" {
			t.Errorf("unexpected AND group: %v", groups[0])
		}
		if len(groups[1]) != 2 {
			t.Errorf("expected OR group of 2, got %v", groups[1])
		}
	})

	t.Run("or-filters only", func(t *testing.T) {
		groups, err := cfg.ParseGroups(nil, []string{"name=zip", "size[gt]=1000"})
		if err != nil {
			t.Fatal(err)
		}
		if len(groups) != 1 || len(groups[0]) != 2 {
			t.Fatalf("expected single OR group of 2, got %v", groups)
		}
	})

	t.Run("invalid or-filter", func(t *testing.T) {
		if _, err := cfg.ParseGroups(nil, []string{"unknown=x"}); err == nil {
			t.Fatal("expected error for unknown field in or-filter")
		}
	})
}

func TestParseJSONBody(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		body, err := ParseJSONBody("")
		if err != nil {
			t.Fatal(err)
		}
		if len(body.Filters) != 0 || len(body.OrFilters) != 0 {
			t.Errorf("expected empty body, got %+v", body)
		}
	})

	t.Run("object", func(t *testing.T) {
		body, err := ParseJSONBody(`{"filters":[{"field":"installed","value":"true"}],"orFilters":[{"field":"name","op":"like","value":"x"},{"field":"size","op":"gt","value":"1"}]}`)
		if err != nil {
			t.Fatal(err)
		}
		if len(body.Filters) != 1 || len(body.OrFilters) != 2 {
			t.Errorf("unexpected body: %+v", body)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		if _, err := ParseJSONBody("{broken"); err == nil {
			t.Fatal("expected error for invalid json")
		}
	})
}

func TestValidateBody(t *testing.T) {
	cfg := groupsTestConfig()

	groups, err := cfg.ValidateBody(ListBody{
		Filters:   []Filter{{Field: "installed", Value: "true"}},
		OrFilters: []Filter{{Field: "name", Value: "x"}, {Field: "description", Value: "x"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[1][0].Op != OpLike {
		t.Errorf("expected default op filled in OR group, got %q", groups[1][0].Op)
	}

	if _, err = cfg.ValidateBody(ListBody{OrFilters: []Filter{{Field: "unknown", Value: "x"}}}); err == nil {
		t.Fatal("expected error for unknown field in orFilters")
	}
}
