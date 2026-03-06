package osutils

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestIsURL(t *testing.T) {
	cases := []struct {
		input    string
		expected bool
	}{
		{"https://example.com/file.yaml", true},
		{"http://localhost:8080/api", true},
		{"ftp://files.example.com/data", true},
		{"/etc/apm/config.yml", false},
		{"./relative/path.yml", false},
		{"just-a-string", false},
		{"", false},
	}

	for _, c := range cases {
		got := IsURL(c.input)
		if got != c.expected {
			t.Errorf("IsURL(%q) = %v, want %v", c.input, got, c.expected)
		}
	}
}

func TestStringToFileMode_Standard(t *testing.T) {
	cases := []struct {
		input    string
		expected os.FileMode
	}{
		{"rwxrwxrwx", 0777},
		{"rwxr-xr-x", 0755},
		{"rw-r--r--", 0644},
		{"rw-------", 0600},
		{"---------", 0000},
		{"rwx------", 0700},
	}

	for _, c := range cases {
		got, err := StringToFileMode(c.input)
		if err != nil {
			t.Errorf("StringToFileMode(%q) error: %v", c.input, err)
			continue
		}
		if got != c.expected {
			t.Errorf("StringToFileMode(%q) = %04o, want %04o", c.input, got, c.expected)
		}
	}
}

func TestStringToFileMode_Setuid(t *testing.T) {
	mode, err := StringToFileMode("rwsr-xr-x")
	if err != nil {
		t.Fatal(err)
	}
	if mode&04000 == 0 {
		t.Error("expected setuid bit to be set")
	}
}

func TestStringToFileMode_Setgid(t *testing.T) {
	mode, err := StringToFileMode("rwxr-sr-x")
	if err != nil {
		t.Fatal(err)
	}
	if mode&02000 == 0 {
		t.Error("expected setgid bit to be set")
	}
}

func TestStringToFileMode_Sticky(t *testing.T) {
	mode, err := StringToFileMode("rwxr-xr-t")
	if err != nil {
		t.Fatal(err)
	}
	if mode&01000 == 0 {
		t.Error("expected sticky bit to be set")
	}
}

func TestStringToFileMode_InvalidLength(t *testing.T) {
	_, err := StringToFileMode("rwx")
	if err == nil {
		t.Error("wrong length should fail")
	}
}

func TestCapitalize(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"hello", "Hello"},
		{"Hello", "Hello"},
		{"a", "A"},
		{"", ""},
		{"привет", "Привет"},
	}

	for _, c := range cases {
		got := Capitalize(c.input)
		if got != c.expected {
			t.Errorf("Capitalize(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestWriter_BeforeDivider(t *testing.T) {
	var main, output bytes.Buffer
	w := &Writer{
		RealWriter:       &main,
		RealOutputWriter: &output,
		Divider:          "---SPLIT---",
	}

	w.Write([]byte("before divider"))

	if main.String() != "before divider" {
		t.Errorf("main should get data before divider, got %q", main.String())
	}
	if output.String() != "" {
		t.Error("output should be empty before divider")
	}
}

func TestWriter_AfterDivider(t *testing.T) {
	var main, output bytes.Buffer
	w := &Writer{
		RealWriter:       &main,
		RealOutputWriter: &output,
		Divider:          "---SPLIT---",
	}

	w.Write([]byte("before---SPLIT---after"))
	w.Write([]byte(" more output"))

	if main.String() != "before" {
		t.Errorf("main should get data before divider, got %q", main.String())
	}
	if output.String() != "after more output" {
		t.Errorf("output should get data after divider, got %q", output.String())
	}
}

func TestWriter_DividerInSeparateWrite(t *testing.T) {
	var main, output bytes.Buffer
	w := &Writer{
		RealWriter:       &main,
		RealOutputWriter: &output,
		Divider:          "DONE",
	}

	w.Write([]byte("line1\n"))
	w.Write([]byte("line2\nDONEresult"))

	if main.String() != "line1\nline2\n" {
		t.Errorf("main should accumulate before divider, got %q", main.String())
	}
	if output.String() != "result" {
		t.Errorf("output should get data after divider, got %q", output.String())
	}
}

func TestCopy_File(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "dest.txt")

	os.WriteFile(src, []byte("hello world"), 0644)

	err := Copy(src, dst, false)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(data))
	}
}

func TestCopy_Dir(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	os.MkdirAll(filepath.Join(srcDir, "sub"), 0755)
	os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("bbb"), 0644)

	dstDir := filepath.Join(dir, "dst")
	err := Copy(srcDir, dstDir, false)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dstDir, "sub", "b.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "bbb" {
		t.Errorf("expected 'bbb', got %q", string(data))
	}
}

func TestMove_RemovesSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "dest.txt")

	os.WriteFile(src, []byte("move me"), 0644)

	err := Move(src, dst, false)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = os.Stat(src); !os.IsNotExist(err) {
		t.Error("source should be removed after move")
	}

	data, _ := os.ReadFile(dst)
	if string(data) != "move me" {
		t.Errorf("dest should have moved content, got %q", string(data))
	}
}

func TestClean_File(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("content"), 0644)

	err := Clean(f)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(f)
	if len(data) != 0 {
		t.Errorf("file should be empty after clean, got %q", string(data))
	}
}

func TestClean_Dir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644)

	err := Clean(dir)
	if err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("dir should be empty after clean, got %d entries", len(entries))
	}
}

func TestClean_NonExistent(t *testing.T) {
	err := Clean("/tmp/nonexistent_apm_test_file_12345")
	if err != nil {
		t.Errorf("cleaning non-existent path should not error: %v", err)
	}
}

func TestAppendFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "append.txt")
	dst := filepath.Join(dir, "target.txt")

	os.WriteFile(dst, []byte("existing\n"), 0644)
	os.WriteFile(src, []byte("appended\n"), 0644)

	err := AppendFile(src, dst, 0644)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(dst)
	if string(data) != "existing\nappended\n" {
		t.Errorf("expected 'existing\\nappended\\n', got %q", string(data))
	}
}

func TestPrependFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "prepend.txt")
	dst := filepath.Join(dir, "target.txt")

	os.WriteFile(dst, []byte("existing\n"), 0644)
	os.WriteFile(src, []byte("prepended\n"), 0644)

	err := PrependFile(src, dst, 0644)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(dst)
	if string(data) != "prepended\nexisting\n" {
		t.Errorf("expected 'prepended\\nexisting\\n', got %q", string(data))
	}
}

func TestPrependFile_DestNotExists(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "prepend.txt")
	dst := filepath.Join(dir, "new_target.txt")

	os.WriteFile(src, []byte("content\n"), 0644)

	err := PrependFile(src, dst, 0644)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(dst)
	if string(data) != "content\n" {
		t.Errorf("expected 'content\\n', got %q", string(data))
	}
}
