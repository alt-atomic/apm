package build

import (
	"apm/internal/common/build/core"
	"apm/internal/common/build/models"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEscapeSingleQuotes(t *testing.T) {
	got := escapeSingleQuotes(`echo 'hi'`)
	want := `echo '\''hi'\''`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCombineIf(t *testing.T) {
	tests := []struct {
		parent, child, want string
	}{
		{"", "", ""},
		{"A", "", "A"},
		{"", "B", "B"},
		{"A", "B", "(A) && (B)"},
	}
	for _, tt := range tests {
		if got := combineIf(tt.parent, tt.child); got != tt.want {
			t.Errorf("combineIf(%q,%q)=%q, want %q", tt.parent, tt.child, got, tt.want)
		}
	}
}

func TestMergeEnv(t *testing.T) {
	if got := mergeEnv(nil, nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
	got := mergeEnv(map[string]string{"A": "1", "B": "2"}, map[string]string{"B": "3"})
	if got["A"] != "1" || got["B"] != "3" {
		t.Fatalf("child must win: %v", got)
	}
}

func TestConvertImageRef(t *testing.T) {
	ref, args := convertImageRef("${{ Env.IMAGE }}")
	if ref != "${IMAGE}" || len(args) != 1 || args[0] != "IMAGE" {
		t.Fatalf("got ref=%q args=%v", ref, args)
	}

	ref, args = convertImageRef("altlinux:sisyphus")
	if ref != "altlinux:sisyphus" || len(args) != 0 {
		t.Fatalf("literal image broken: ref=%q args=%v", ref, args)
	}
}

func TestFlattenModulesInclude(t *testing.T) {
	dir := t.TempDir()
	child := `modules:
  - name: child-shell
    type: shell
    body:
      command: echo child
`
	if err := os.WriteFile(filepath.Join(dir, "child.yml"), []byte(child), 0644); err != nil {
		t.Fatal(err)
	}

	modules := []core.Module{
		{Type: core.TypeInclude, If: `Env.BUILD == "x"`, Body: &models.IncludeBody{Targets: []string{"child.yml"}}},
		{Name: "top", Type: core.TypeShell, Body: &models.ShellBody{Command: "echo top"}},
	}

	steps, err := FlattenModules(modules, dir, "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 flat steps, got %d", len(steps))
	}
	if steps[0].Module.Type != core.TypeShell || steps[0].Module.Name != "child-shell" {
		t.Fatalf("first step should be child shell: %+v", steps[0].Module)
	}
	if steps[0].Module.If != `Env.BUILD == "x"` {
		t.Fatalf("parent if must propagate to child, got %q", steps[0].Module.If)
	}
	if steps[1].Module.Name != "top" {
		t.Fatalf("second step should be top module")
	}
}

func TestFlattenModulesNestedRemoteExpand(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// a.yml — remote, внутри remote-include на b.yml.
	mux.HandleFunc("/a.yml", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "modules:\n  - type: include\n    body:\n      targets:\n        - %s/b.yml\n", srv.URL)
	})
	// b.yml — remote, конечный shell-модуль.
	mux.HandleFunc("/b.yml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "modules:\n  - name: from-b\n    type: shell\n    body:\n      command: echo b\n")
	})

	modules := []core.Module{
		{Type: core.TypeInclude, Body: &models.IncludeBody{Targets: []string{srv.URL + "/a.yml"}}},
	}

	steps, err := FlattenModules(modules, t.TempDir(), "", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step from nested remote, got %d: %+v", len(steps), steps)
	}
	if steps[0].Module.Type != core.TypeShell || steps[0].Module.Name != "from-b" {
		t.Fatalf("nested remote include not expanded to b's step: %+v", steps[0].Module)
	}
}

func TestFlattenModulesRemoteNotExpanded(t *testing.T) {
	modules := []core.Module{
		{Type: core.TypeInclude, Body: &models.IncludeBody{Targets: []string{"https://example.com/x.yml"}}},
	}
	steps, err := FlattenModules(modules, t.TempDir(), "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0].Module.Type != core.TypeInclude {
		t.Fatalf("remote include must stay a single include step, got %+v", steps)
	}
}

func TestDetectCrossState(t *testing.T) {
	clean := []FlatStep{
		{Module: core.Module{Type: core.TypeShell, Body: &models.ShellBody{Command: "echo ok"}}},
	}
	if bad, err := DetectCrossState(clean); err != nil || len(bad) != 0 {
		t.Fatalf("clean config flagged: %v (err %v)", bad, err)
	}

	// literal "Modules." вне ${{ }} не должен ловиться (регекс ловил бы).
	literal := []FlatStep{
		{Module: core.Module{Type: core.TypeShell, Body: &models.ShellBody{Command: "echo Modules.foo"}}},
	}
	if bad, err := DetectCrossState(literal); err != nil || len(bad) != 0 {
		t.Fatalf("literal Modules. in string flagged: %v (err %v)", bad, err)
	}

	refIf := []FlatStep{
		{Module: core.Module{Name: "bad", Type: core.TypeShell, If: "Modules.foo.Output.x", Body: &models.ShellBody{Command: "echo ok"}}},
	}
	if bad, err := DetectCrossState(refIf); err != nil || len(bad) != 1 {
		t.Fatalf("cross-ref in if not detected: %v (err %v)", bad, err)
	}

	refBody := []FlatStep{
		{Module: core.Module{Type: core.TypeShell, Body: &models.ShellBody{Command: "echo ${{ Modules.foo.Output.x }}"}}},
	}
	if bad, err := DetectCrossState(refBody); err != nil || len(bad) != 1 {
		t.Fatalf("cross-ref in body not detected: %v (err %v)", bad, err)
	}
}

func TestCacheHashDeterministicAndSensitive(t *testing.T) {
	a := &models.ShellBody{Command: "echo a"}
	h1, err := a.CacheHash("")
	if err != nil {
		t.Fatal(err)
	}
	h1b, _ := a.CacheHash("")
	if h1 != h1b {
		t.Fatal("hash not deterministic")
	}

	b := &models.ShellBody{Command: "echo b"}
	h2, _ := b.CacheHash("")
	if h1 == h2 {
		t.Fatal("hash must change when module changes")
	}
}

func TestCacheHashResourceContent(t *testing.T) {
	dir := t.TempDir()
	res := filepath.Join(dir, "root")
	if err := os.WriteFile(res, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}

	body := &models.CopyBody{Source: "root", Destination: "/root"}
	h1, _ := body.CacheHash(dir)

	if err := os.WriteFile(res, []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}
	h2, _ := body.CacheHash(dir)
	if h1 == h2 {
		t.Fatal("hash must change when resource content changes")
	}
}

func TestRenderBlock(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "root"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	modules := []core.Module{
		{Name: "pkgs", Type: core.TypePackages, Body: &models.PackagesBody{Install: []string{"hello"}}},
		{Name: "cp", Type: core.TypeCopy, Body: &models.CopyBody{Source: "root", Destination: "/"}},
	}

	block, err := RenderBlock(modules, dir, "1.2.3", map[string]string{"FOO": "bar"}, GenerateOptions{ResourcesSource: "./src/resources"})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(block, `ENV FOO="bar"`) {
		t.Errorf("config env must be emitted as ENV:\n%s", block)
	}

	checks := []string{
		"ENV APM_GENERATED_VERSION=1.2.3",
		"RUN apm system image build --phase pre-modules",
		"RUN apm system image build --phase post-modules",
		"--step-inline '",
		"APM_STEP=",
	}
	for _, c := range checks {
		if !strings.Contains(block, c) {
			t.Errorf("block missing %q\n%s", c, block)
		}
	}

	// Ресурсы есть → mount на каждом шаге-модуле (их 2).
	if strings.Count(block, "--mount=type=bind,source=./src/resources") != 2 {
		t.Errorf("resources bind expected on every module step, got:\n%s", block)
	}

	// --mount обязан идти сразу после RUN, до APM_STEP.
	if !strings.Contains(block, "RUN --mount=type=bind,source=./src/resources,target=/etc/apm/resources") {
		t.Errorf("mount flag must come right after RUN:\n%s", block)
	}
}

func TestRenderBlockCacheBust(t *testing.T) {
	modules := []core.Module{
		{Name: "pkgs", Type: core.TypePackages, Body: &models.PackagesBody{Install: []string{"hello"}}},
	}
	dir := filepath.Join(t.TempDir(), "nores")

	with, err := RenderBlock(modules, dir, "1.0.0", nil, GenerateOptions{CacheBust: "20260628"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(with, `ENV APM_CACHE_BUST="20260628"`) {
		t.Errorf("cache-bust token must be emitted as ENV:\n%s", with)
	}

	without, err := RenderBlock(modules, dir, "1.0.0", nil, GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(without, "APM_CACHE_BUST") {
		t.Errorf("no cache-bust ENV expected without token:\n%s", without)
	}
}

func TestRenderBlockNoResources(t *testing.T) {
	modules := []core.Module{
		{Name: "pkgs", Type: core.TypePackages, Body: &models.PackagesBody{Install: []string{"hello"}}},
	}

	// Несуществующий каталог ресурсов → без mount.
	block, err := RenderBlock(modules, filepath.Join(t.TempDir(), "nope"), "1.0.0", nil, GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(block, "--mount") {
		t.Errorf("no mount expected without resources dir:\n%s", block)
	}
	if !strings.Contains(block, "RUN APM_STEP=") {
		t.Errorf("step must be RUN APM_STEP= without mount:\n%s", block)
	}
}

func TestRenderBlockCrossStateError(t *testing.T) {
	modules := []core.Module{
		{Name: "bad", Type: core.TypeShell, If: "Modules.x.Output.y", Body: &models.ShellBody{Command: "echo hi"}},
	}
	if _, err := RenderBlock(modules, t.TempDir(), "1.0.0", nil, GenerateOptions{}); err == nil {
		t.Fatal("expected cross-state error")
	}
}

func TestWriteWithMarkersReplaceRegion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Containerfile")
	original := "FROM base\n" + markerStart + "\nOLD\n" + markerEnd + "\nCMD [\"/sbin/init\"]\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := WriteWithMarkers(path, "NEWBLOCK", "base")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(content, "OLD") {
		t.Fatal("old block not replaced")
	}
	if !strings.Contains(content, "NEWBLOCK") {
		t.Fatal("new block missing")
	}
	if !strings.HasPrefix(content, "FROM base\n") || !strings.Contains(content, "CMD [\"/sbin/init\"]") {
		t.Fatalf("surrounding content not preserved:\n%s", content)
	}
}

func TestWriteWithMarkersSkeleton(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Containerfile")

	content, err := WriteWithMarkers(path, "BLOCK", "${{ Env.IMAGE }}")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"ARG IMAGE", "FROM ${IMAGE}", markerStart, "BLOCK", markerEnd} {
		if !strings.Contains(content, c) {
			t.Errorf("skeleton missing %q\n%s", c, content)
		}
	}
}
