package metainfo

import "testing"

func TestCurrentDistroComponentID(t *testing.T) {
	id, err := CurrentOSComponentID()
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("libappstream returned an empty OS component ID")
	}
}

func TestOSComponentMissing(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	found, err := OSComponentExists("org.example.apm-test-nonexistent-system-component")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("unexpected operating-system component")
	}
}
