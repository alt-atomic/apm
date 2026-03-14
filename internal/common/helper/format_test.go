package helper

import (
	"testing"
)

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		input uint64
		want  string
	}{
		{0, ""},
		{500, "500 B/s"},
		{1023, "1023 B/s"},
		{1024, "1 KB/s"},
		{1536, "2 KB/s"},
		{10240, "10 KB/s"},
		{1048576, "1.0 MB/s"},
		{1572864, "1.5 MB/s"},
		{10485760, "10.0 MB/s"},
	}

	for _, tt := range tests {
		got := FormatSpeed(tt.input)
		if got != tt.want {
			t.Errorf("FormatSpeed(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0", "1.0", 0},
		{"1.1", "1.0", 1},
		{"1.0", "1.1", -1},
		{"2.0", "1.9", 1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.0", "1.0.0", 0},
		{"1.0.0", "1.0", 0},
		{"10.0", "9.0", 1},
		{"1.2.3.4", "1.2.3.4", 0},
		{"1.2.3.4", "1.2.3.5", -1},
		{"", "", 0},
		{"1", "2", -1},
		{"1.0", "1.0.0.0.0", 0},
	}

	for _, tt := range tests {
		got := CompareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
