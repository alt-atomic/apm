package wire

import (
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type testNested struct {
	Reason string `json:"reason"`
}

type testJSONNumbers struct{}

func (testJSONNumbers) MarshalJSON() ([]byte, error) {
	return []byte(`{"integer":9007199254740993,"fraction":1.5}`), nil
}

type testJSONArrayWithNull struct{}

func (testJSONArrayWithNull) MarshalJSON() ([]byte, error) {
	return []byte(`[1,null,2]`), nil
}

type testDTO struct {
	Name       string            `json:"name"`
	Hidden     string            `json:"-"`
	unexported string            //nolint:unused
	Count      int               `json:"count"`
	Size       uint64            `json:"size"`
	Ratio      float64           `json:"ratio"`
	Installed  bool              `json:"installed"`
	Optional   string            `json:"optional,omitempty"`
	Result     *string           `json:"result"`
	Items      []string          `json:"items"`
	Nested     testNested        `json:"nested"`
	Children   []testNested      `json:"children"`
	Env        map[string]string `json:"env,omitempty"`
	Extra      map[string]any    `json:"extra,omitempty"`
	Body       any               `json:"body,omitempty"`
}

func TestStructDict(t *testing.T) {
	d, err := StructDict(testDTO{
		Name:      "vim",
		Hidden:    "secret",
		Count:     -2,
		Size:      12345,
		Ratio:     0.5,
		Installed: true,
		Result:    new("done"),
		Items:     []string{"a", "b"},
		Nested:    testNested{Reason: "essential"},
		Children:  []testNested{{Reason: "x"}, {Reason: "y"}},
		Env:       map[string]string{"K": "V"},
		Extra:     map[string]any{"n": 7, "s": "str"},
		Body:      testNested{Reason: "dynamic"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := d["Hidden"]; ok {
		t.Error("json \"-\" field must be skipped")
	}
	if _, ok := d["optional"]; ok {
		t.Error("empty omitempty field must be skipped")
	}
	if got := d["name"].Value().(string); got != "vim" {
		t.Errorf("name = %v", got)
	}
	if got := d["count"].Value().(int64); got != -2 {
		t.Errorf("count = %v (int must stay integer)", d["count"].Value())
	}
	if got := d["size"].Value().(uint64); got != 12345 {
		t.Errorf("size = %v", got)
	}
	if got := d["result"].Value().(string); got != "done" {
		t.Errorf("result pointer = %v", got)
	}
	if got := d["items"].Value().([]string); len(got) != 2 {
		t.Errorf("items = %v", got)
	}
	nested := d["nested"].Value().(Dict)
	if nested["reason"].Value().(string) != "essential" {
		t.Errorf("nested = %v", nested)
	}
	children := d["children"].Value().([]Dict)
	if len(children) != 2 || children[1]["reason"].Value().(string) != "y" {
		t.Errorf("children = %v", children)
	}
	if got := d["env"].Value().(map[string]string); got["K"] != "V" {
		t.Errorf("env = %v", got)
	}
	extra := d["extra"].Value().(Dict)
	if extra["n"].Value().(int64) != 7 || extra["s"].Value().(string) != "str" {
		t.Errorf("extra = %v (any values must keep runtime types)", extra)
	}
	body := d["body"].Value().(Dict)
	if body["reason"].Value().(string) != "dynamic" {
		t.Errorf("body = %v (interface must unwrap dynamic type)", body)
	}
}

func TestStructDictOmitEmptyMatchesJSON(t *testing.T) {
	// непустой указатель на пустой срез: omitempty должен опустить, как encoding/json
	type dto struct {
		Items []string          `json:"items,omitempty"`
		Env   map[string]string `json:"env,omitempty"`
	}
	d, err := StructDict(dto{Items: make([]string, 0), Env: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d["items"]; ok {
		t.Error("empty non-nil slice must be omitted")
	}
	if _, ok := d["env"]; ok {
		t.Error("empty non-nil map must be omitted")
	}
}

func TestStructDictNilPointerOmitted(t *testing.T) {
	d, err := StructDict(testDTO{Name: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d["result"]; ok {
		t.Error("nil pointer field must be absent")
	}
	if _, ok := d["body"]; ok {
		t.Error("nil interface field must be absent")
	}
	if items, ok := d["items"]; !ok || len(items.Value().([]string)) != 0 {
		t.Error("empty slice without omitempty must be present and empty")
	}
}

func TestStructDictSignature(t *testing.T) {
	d, err := StructDict(testDTO{Name: "x", Extra: map[string]any{"n": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if sig := dbus.MakeVariant(d).Signature().String(); sig != "a{sv}" {
		t.Errorf("signature = %s", sig)
	}
}

func TestStructDicts(t *testing.T) {
	rows, err := StructDicts([]testNested{{Reason: "a"}, {Reason: "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0]["reason"].Value().(string) != "a" {
		t.Errorf("rows = %v", rows)
	}
}

// enumKey повторяет форму ключа-перечисления из фильтров (map[PackageType]string).
type enumKey uint8

func TestStructDictNonStringMapKeys(t *testing.T) {
	type dto struct {
		Enum  map[enumKey]string `json:"enum"`
		Ints  map[int]int        `json:"ints"`
		Extra map[string]any     `json:"extra"`
	}
	d, err := StructDict(dto{
		Enum:  map[enumKey]string{0: "System package"},
		Ints:  map[int]int{-7: 1},
		Extra: map[string]any{"info": map[enumKey]string{1: "Stplr package"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := d["enum"].Value().(map[string]string); got["0"] != "System package" {
		t.Errorf("enum = %v, want key \"0\"", got)
	}
	if got := d["ints"].Value().(Dict); got["-7"].Value().(int64) != 1 {
		t.Errorf("ints = %v, want key \"-7\"", got)
	}
	// вложенная карта внутри any: тот же кейс, что ломал FilterFields
	nested := d["extra"].Value().(Dict)["info"].Value().(map[string]string)
	if nested["1"] != "Stplr package" {
		t.Errorf("extra.info = %v", nested)
	}
}

func TestStructDictRejectsNonStruct(t *testing.T) {
	if _, err := StructDict("plain"); err == nil {
		t.Error("expected error for non-struct")
	}
}

func TestStructDictKeepsZeroStructWithOmitEmpty(t *testing.T) {
	type dto struct {
		Nested testNested `json:"nested,omitempty"`
	}

	d, err := StructDict(dto{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d["nested"]; !ok {
		t.Error("zero struct must not be omitted by omitempty")
	}
}

func TestStructDictByteSliceUsesDBusByteArray(t *testing.T) {
	type dto struct {
		Data []byte `json:"data"`
	}

	d, err := StructDict(dto{Data: []byte{1, 2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	if sig := d["data"].Signature().String(); sig != "ay" {
		t.Fatalf("signature = %s, want ay", sig)
	}
	if got := d["data"].Value().([]byte); len(got) != 3 || got[2] != 3 {
		t.Errorf("data = %v", got)
	}
}

func TestStructDictRejectsPointerScalarSlice(t *testing.T) {
	type dto struct {
		Values []*string `json:"values"`
	}
	if _, err := StructDict(dto{Values: []*string{new("one"), new("two")}}); err == nil {
		t.Fatal("expected error for pointer scalar array")
	}
}

func TestStructDictRejectsNilSliceElement(t *testing.T) {
	type dto struct {
		Values []*string `json:"values"`
	}
	if _, err := StructDict(dto{Values: []*string{new("one"), nil}}); err == nil {
		t.Fatal("expected error for nil array element")
	}
}

func TestStructDictRejectsNonObjectElementJSONMarshaler(t *testing.T) {
	type dto struct {
		Times []time.Time `json:"times"`
	}
	value := time.Date(2026, time.August, 25, 12, 30, 0, 0, time.UTC)

	if _, err := StructDict(dto{Times: []time.Time{value}}); err == nil {
		t.Fatal("expected error for array element marshaled as string")
	}
	if _, err := StructDict(dto{Times: []time.Time{}}); err == nil {
		t.Fatal("expected the same error for an empty array")
	}
}

func TestStructDictPreservesJSONInteger(t *testing.T) {
	type dto struct {
		Numbers testJSONNumbers `json:"numbers"`
	}

	d, err := StructDict(dto{})
	if err != nil {
		t.Fatal(err)
	}
	numbers := d["numbers"].Value().(Dict)
	if got := numbers["integer"].Value().(int64); got != 9007199254740993 {
		t.Errorf("integer = %d", got)
	}
	if got := numbers["fraction"].Value().(float64); got != 1.5 {
		t.Errorf("fraction = %v", got)
	}
}

func TestStructDictRejectsJSONNullArrayElement(t *testing.T) {
	type dto struct {
		Values testJSONArrayWithNull `json:"values"`
	}

	if _, err := StructDict(dto{}); err == nil {
		t.Fatal("expected error for JSON null array element")
	}
}
