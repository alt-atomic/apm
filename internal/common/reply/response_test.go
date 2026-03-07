package reply

import (
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestFilterFields_SimpleTopLevel(t *testing.T) {
	data := map[string]interface{}{
		"name":    "firefox",
		"version": "120.0",
		"size":    1024,
		"section": "browsers",
	}
	result := filterFields(data, []string{"name", "version"})

	if result["name"] != "firefox" {
		t.Errorf("expected name=firefox, got %v", result["name"])
	}
	if result["version"] != "120.0" {
		t.Errorf("expected version=120.0, got %v", result["version"])
	}
	if _, ok := result["size"]; ok {
		t.Error("size should be filtered out")
	}
	if _, ok := result["section"]; ok {
		t.Error("section should be filtered out")
	}
}

func TestFilterFields_PreservesMessage(t *testing.T) {
	data := map[string]interface{}{
		"message": "Package info",
		"name":    "vim",
		"version": "9.0",
		"size":    512,
	}
	result := filterFields(data, []string{"name"})

	if result["message"] != "Package info" {
		t.Errorf("message should be preserved, got %v", result["message"])
	}
	if result["name"] != "vim" {
		t.Errorf("expected name=vim, got %v", result["name"])
	}
	if _, ok := result["size"]; ok {
		t.Error("size should be filtered out")
	}
}

func TestFilterFields_NestedDotNotation(t *testing.T) {
	data := map[string]interface{}{
		"name": "firefox",
		"appStream": map[string]interface{}{
			"id":          "org.mozilla.firefox",
			"summary":     "Web browser",
			"description": "Full description here",
		},
	}
	result := filterFields(data, []string{"name", "appStream.id"})

	if result["name"] != "firefox" {
		t.Errorf("expected name=firefox, got %v", result["name"])
	}
	as, ok := result["appStream"].(map[string]interface{})
	if !ok {
		t.Fatal("appStream should be a map")
	}
	if as["id"] != "org.mozilla.firefox" {
		t.Errorf("expected appStream.id, got %v", as["id"])
	}
	if _, ok := as["summary"]; ok {
		t.Error("appStream.summary should be filtered out")
	}
}

func TestFilterFields_SingleWrapperUnwrap(t *testing.T) {
	// Когда на верхнем уровне один ключ с вложенной map — фильтрация внутри обёртки
	data := map[string]interface{}{
		"packageInfo": map[string]interface{}{
			"name":    "gcc",
			"version": "13.2",
			"size":    5000,
		},
	}
	result := filterFields(data, []string{"name", "version"})

	pi, ok := result["packageInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("packageInfo wrapper should be preserved")
	}
	if pi["name"] != "gcc" {
		t.Errorf("expected name=gcc, got %v", pi["name"])
	}
	if pi["version"] != "13.2" {
		t.Errorf("expected version=13.2, got %v", pi["version"])
	}
	if _, ok = pi["size"]; ok {
		t.Error("size should be filtered out")
	}
}

func TestFilterFields_ArrayElements(t *testing.T) {
	// Фильтрация внутри массивов — реальный сценарий apm system search
	data := map[string]interface{}{
		"packages": []interface{}{
			map[string]interface{}{"name": "vim", "version": "9.0", "size": 100},
			map[string]interface{}{"name": "nano", "version": "7.0", "size": 50},
		},
	}
	result := filterFields(data, []string{"name", "version"})

	pkgs, ok := result["packages"].([]interface{})
	if !ok {
		t.Fatal("packages should be a slice")
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}
	first := pkgs[0].(map[string]interface{})
	if first["name"] != "vim" {
		t.Errorf("expected name=vim, got %v", first["name"])
	}
	if _, ok := first["size"]; ok {
		t.Error("size should be filtered from array elements")
	}
}

func TestNormalizeValue_Struct(t *testing.T) {
	type Pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	result := normalizeValue(Pkg{Name: "bash", Version: "5.2"})

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["name"] != "bash" {
		t.Errorf("expected name=bash, got %v", m["name"])
	}
}

func TestNormalizeValue_TypedSlice(t *testing.T) {
	type Item struct {
		Name string `json:"name"`
	}
	input := []Item{{Name: "a"}, {Name: "b"}}
	result := normalizeValue(input)

	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 items, got %d", len(arr))
	}
}

func TestNormalizeValue_MapSlice(t *testing.T) {
	input := []map[string]interface{}{
		{"name": "x"},
		{"name": "y"},
	}
	result := normalizeValue(input)

	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 items, got %d", len(arr))
	}
}

func TestSortedKeys_ExcludesMessage(t *testing.T) {
	data := map[string]interface{}{
		"message": "hello",
		"name":    "pkg",
		"version": "1.0",
	}
	keys := sortedKeys(data)

	for _, k := range keys {
		if k == "message" {
			t.Error("sortedKeys should exclude 'message'")
		}
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
	if keys[0] != "name" || keys[1] != "version" {
		t.Errorf("keys should be sorted: got %v", keys)
	}
}

func TestIsScalarSlice(t *testing.T) {
	if !isScalarSlice([]interface{}{"a", "b", "c"}) {
		t.Error("string slice should be scalar")
	}
	if !isScalarSlice([]interface{}{1, 2, 3}) {
		t.Error("int slice should be scalar")
	}
	if !isScalarSlice([]interface{}{}) {
		t.Error("empty slice should be scalar")
	}
	if isScalarSlice([]interface{}{map[string]interface{}{"x": 1}}) {
		t.Error("slice with maps should not be scalar")
	}
	if isScalarSlice([]interface{}{"a", []interface{}{1}}) {
		t.Error("slice with nested slice should not be scalar")
	}
}

func TestToDataMap_FromMap(t *testing.T) {
	input := map[string]interface{}{"name": "test"}
	result := toDataMap(input)
	if result["name"] != "test" {
		t.Errorf("expected name=test, got %v", result["name"])
	}
}

func TestToDataMap_FromStruct(t *testing.T) {
	type Info struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	result := toDataMap(Info{Name: "pkg", Version: "1.0"})
	if result == nil {
		t.Fatal("toDataMap should convert struct to map")
	}
	if result["name"] != "pkg" {
		t.Errorf("expected name=pkg, got %v", result["name"])
	}
}

func TestToDataMap_NilOnInvalid(t *testing.T) {
	result := toDataMap("just a string")
	if result != nil {
		t.Error("toDataMap should return nil for non-marshalable-to-map values")
	}
}

func TestRenderPlain_MessageFirst(t *testing.T) {
	r := NewRendererFromColors(app.GetDefaultColors())
	data := map[string]interface{}{
		"message": "Package installed",
		"name":    "vim",
	}
	result := r.renderPlain(data)

	lines := strings.Split(result, "\n")
	if len(lines) < 1 {
		t.Fatal("expected at least one line")
	}
	if !strings.Contains(lines[0], "Package installed") {
		t.Errorf("first line should be message, got: %s", lines[0])
	}
}

func TestRenderPlain_ScalarFields(t *testing.T) {
	r := NewRendererFromColors(app.GetDefaultColors())
	data := map[string]interface{}{
		"name":    "bash",
		"version": "5.2",
	}
	result := r.renderPlain(data)

	if !strings.Contains(result, "bash") {
		t.Error("output should contain package name")
	}
	if !strings.Contains(result, "5.2") {
		t.Error("output should contain version")
	}
}

func TestRenderPlain_NestedMap(t *testing.T) {
	r := NewRendererFromColors(app.GetDefaultColors())
	data := map[string]interface{}{
		"info": map[string]interface{}{
			"name":    "gcc",
			"version": "13",
		},
	}
	result := r.renderPlain(data)

	// При одном ключе верхнего уровня — разворачивает внутрь
	if !strings.Contains(result, "gcc") {
		t.Error("output should contain nested name")
	}
}

func TestRenderPlain_ScalarList(t *testing.T) {
	r := NewRendererFromColors(app.GetDefaultColors())
	data := map[string]interface{}{
		"aliases": []interface{}{"vi", "vim.basic"},
	}
	result := r.renderPlain(data)

	if !strings.Contains(result, "vi") || !strings.Contains(result, "vim.basic") {
		t.Errorf("scalar list should be joined: got %s", result)
	}
}

func TestRenderPlain_EmptySlice(t *testing.T) {
	r := NewRendererFromColors(app.GetDefaultColors())
	data := map[string]interface{}{
		"depends": []interface{}{},
	}
	result := r.renderPlain(data)
	if strings.Contains(result, "depends") {
		t.Error("empty slice should produce no output")
	}
}

func TestRenderPlain_ObjectList(t *testing.T) {
	r := NewRendererFromColors(app.GetDefaultColors())
	data := map[string]interface{}{
		"packages": []interface{}{
			map[string]interface{}{"name": "vim", "version": "9.0"},
			map[string]interface{}{"name": "nano", "version": "7.0"},
		},
	}
	result := r.renderPlain(data)

	if !strings.Contains(result, "vim") || !strings.Contains(result, "nano") {
		t.Errorf("should contain all package names: got %s", result)
	}
	// Нумерованные элементы
	if !strings.Contains(result, ".1.") || !strings.Contains(result, ".2.") {
		t.Errorf("should contain numbered items: got %s", result)
	}
}

func TestOK_JSON(t *testing.T) {
	resp := OK(map[string]interface{}{"name": "test"})
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err = json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed["error"] != nil {
		t.Error("OK response should have null error")
	}
	data := parsed["data"].(map[string]interface{})
	if data["name"] != "test" {
		t.Errorf("expected name=test, got %v", data["name"])
	}
}

func TestErrorResponseFromError_PlainError(t *testing.T) {
	resp := ErrorResponseFromError(fmt.Errorf("something went wrong"))

	if resp.Error == nil {
		t.Fatal("error response should have error")
	}
	if resp.Error.ErrorCode != "" {
		t.Error("plain error should not have error code")
	}
	if resp.Error.Message != "something went wrong" {
		t.Errorf("expected error message, got %q", resp.Error.Message)
	}
}

func TestErrorResponseFromError_APMError(t *testing.T) {
	apmErr := apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf("invalid package name"))
	resp := ErrorResponseFromError(apmErr)

	if resp.Error == nil {
		t.Fatal("error response should have error")
	}
	if resp.Error.ErrorCode != apmerr.ErrorTypeValidation {
		t.Errorf("expected error code VALIDATION, got %q", resp.Error.ErrorCode)
	}
	if resp.Error.Message != "invalid package name" {
		t.Errorf("expected error message, got %q", resp.Error.Message)
	}
}

func TestErrorResponseFromError_WrappedAPMError(t *testing.T) {
	apmErr := apmerr.New(apmerr.ErrorTypeNotFound, fmt.Errorf("package foo not found"))
	wrapped := fmt.Errorf("operation failed: %w", apmErr)
	resp := ErrorResponseFromError(wrapped)

	if resp.Error == nil {
		t.Fatal("error response should have error")
	}
	if resp.Error.ErrorCode != apmerr.ErrorTypeNotFound {
		t.Errorf("expected error code NOT_FOUND, got %q", resp.Error.ErrorCode)
	}
}

func TestRenderText_TreeFormat(t *testing.T) {
	r := NewRendererFromColors(app.GetDefaultColors())
	data := map[string]interface{}{
		"message": "Test message",
		"name":    "test-package",
		"version": "1.0.0",
	}

	result := r.RenderText(data, app.FormatTypeTree, false)
	if result == "" {
		t.Fatal("RenderText tree returned empty string")
	}
	if !strings.Contains(result, "Test message") {
		t.Error("tree output should contain message")
	}
	if !strings.Contains(result, "1.0.0") {
		t.Error("tree output should contain version")
	}
}

func TestRenderText_PlainFormat(t *testing.T) {
	r := NewRendererFromColors(app.GetDefaultColors())
	data := map[string]interface{}{
		"message": "Test message",
		"name":    "test-package",
		"version": "1.0.0",
	}

	result := r.RenderText(data, app.FormatTypePlain, false)
	if result == "" {
		t.Fatal("RenderText plain returned empty string")
	}
	if !strings.Contains(result, "Test message") {
		t.Error("plain output should contain message")
	}
	if !strings.Contains(result, "1.0.0") {
		t.Error("plain output should contain version")
	}
}

func TestRenderText_ErrorFlag(t *testing.T) {
	r := NewRendererFromColors(app.GetDefaultColors())
	data := map[string]interface{}{
		"message": "Something went wrong",
	}

	resultOk := r.RenderText(data, app.FormatTypeTree, false)
	resultErr := r.RenderText(data, app.FormatTypeTree, true)

	if resultOk == resultErr {
		t.Error("error and success renders should differ (different styling)")
	}
}

func TestRenderText_DefaultsToTree(t *testing.T) {
	r := NewRendererFromColors(app.GetDefaultColors())
	data := map[string]interface{}{
		"message": "hello",
		"key":     "value",
	}

	treeResult := r.RenderText(data, app.FormatTypeTree, false)
	unknown := r.RenderText(data, "unknown_format", false)

	if treeResult != unknown {
		t.Error("unknown format type should fall back to tree")
	}
}

func TestRenderText_NestedData(t *testing.T) {
	r := NewRendererFromColors(app.GetDefaultColors())
	data := map[string]interface{}{
		"message": "Package info",
		"details": map[string]interface{}{
			"name":    "pkg",
			"version": "2.0",
		},
	}

	for _, ft := range []string{app.FormatTypeTree, app.FormatTypePlain} {
		result := r.RenderText(data, ft, false)
		if !strings.Contains(result, "2.0") {
			t.Errorf("format %s: nested version not found in output", ft)
		}
	}
}

func TestRenderText_ListData(t *testing.T) {
	r := NewRendererFromColors(app.GetDefaultColors())
	data := map[string]interface{}{
		"packages": []interface{}{
			map[string]interface{}{"name": "aaa", "version": "1.0"},
			map[string]interface{}{"name": "bbb", "version": "2.0"},
		},
	}

	for _, ft := range []string{app.FormatTypeTree, app.FormatTypePlain} {
		result := r.RenderText(data, ft, false)
		if !strings.Contains(result, "aaa") || !strings.Contains(result, "bbb") {
			t.Errorf("format %s: list items not found in output", ft)
		}
	}
}
