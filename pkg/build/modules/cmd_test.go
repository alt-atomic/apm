package modules

import (
	"reflect"
	"testing"
)

func TestGetEnvFromOutput(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want map[string]string
	}{
		// multiline env output
		{"multiline", "FOO=bar\nBAZ=qux", map[string]string{"FOO": "bar", "BAZ": "qux"}},
		// lines without '=' are ignored
		{"skips lines without eq", "FOO=bar\njustnoise\nBAZ=qux", map[string]string{"FOO": "bar", "BAZ": "qux"}},
		// value may contain '=' (SplitN=2)
		{"value with eq", "URL=a=b=c", map[string]string{"URL": "a=b=c"}},
		// empty key for a "=value" line
		{"empty key", "=value", map[string]string{"": "value"}},
		// empty input yields empty map
		{"empty", "", map[string]string{}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := GetEnvFromOutput(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("GetEnvFromOutput(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
