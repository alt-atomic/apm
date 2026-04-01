package lint

import (
	"testing"
)

func TestParseSysusersLine_User(t *testing.T) {
	tests := []struct {
		line   string
		name   string
		hasUID bool
		uid    uint32
		hasGID bool
		gid    uint32
		gecos  string
		home   string
		shell  string
	}{
		{"u root 0:0 root /root /bin/bash", "root", true, 0, true, 0, "root", "/root", "/bin/bash"},
		{"u bin 1:1 bin / -", "bin", true, 1, true, 1, "bin", "/", ""},
		{`u apache 96:96 "Apache web server" /var/www -`, "apache", true, 96, true, 96, "Apache web server", "/var/www", ""},
		{"u qemu 107:qemu - - -", "qemu", true, 107, false, 0, "", "", ""},
		{"u vboxadd -:1 - - -", "vboxadd", false, 0, true, 1, "", "", ""},
		{"u pathuser /some/file - - -", "pathuser", false, 0, false, 0, "", "", ""},
		{"u! stronguser 500 - - -", "stronguser", true, 500, false, 0, "", "", ""},
	}

	for _, tt := range tests {
		entry, err := parseSysusersLine(tt.line)
		if err != nil {
			t.Errorf("parseSysusersLine(%q) error: %v", tt.line, err)
			continue
		}
		if entry == nil {
			t.Errorf("parseSysusersLine(%q) = nil", tt.line)
			continue
		}
		if entry.Type != sysusersUser {
			t.Errorf("parseSysusersLine(%q).Type = %d, want sysusersUser", tt.line, entry.Type)
		}
		if entry.Name != tt.name {
			t.Errorf("parseSysusersLine(%q).Name = %q, want %q", tt.line, entry.Name, tt.name)
		}
		if tt.hasUID {
			if entry.UID == nil || *entry.UID != tt.uid {
				t.Errorf("parseSysusersLine(%q).UID = %v, want %d", tt.line, entry.UID, tt.uid)
			}
		} else if entry.UID != nil {
			t.Errorf("parseSysusersLine(%q).UID = %v, want nil", tt.line, *entry.UID)
		}
		if tt.hasGID {
			if entry.GID == nil || *entry.GID != tt.gid {
				t.Errorf("parseSysusersLine(%q).GID = %v, want %d", tt.line, entry.GID, tt.gid)
			}
		} else if entry.GID != nil {
			t.Errorf("parseSysusersLine(%q).GID = %v, want nil", tt.line, *entry.GID)
		}
		if entry.Gecos != tt.gecos {
			t.Errorf("parseSysusersLine(%q).Gecos = %q, want %q", tt.line, entry.Gecos, tt.gecos)
		}
		if entry.Home != tt.home {
			t.Errorf("parseSysusersLine(%q).Home = %q, want %q", tt.line, entry.Home, tt.home)
		}
		if entry.Shell != tt.shell {
			t.Errorf("parseSysusersLine(%q).Shell = %q, want %q", tt.line, entry.Shell, tt.shell)
		}
	}
}

func TestParseSysusersLine_Group(t *testing.T) {
	tests := []struct {
		line   string
		name   string
		hasGID bool
		gid    uint32
	}{
		{"g root 0", "root", true, 0},
		{"g wheel 10", "wheel", true, 10},
		{"g nogroup -", "nogroup", false, 0},
		{"g pathgroup /some/path", "pathgroup", false, 0},
	}

	for _, tt := range tests {
		entry, err := parseSysusersLine(tt.line)
		if err != nil {
			t.Errorf("parseSysusersLine(%q) error: %v", tt.line, err)
			continue
		}
		if entry.Type != sysusersGroup {
			t.Errorf("parseSysusersLine(%q).Type = %d, want sysusersGroup", tt.line, entry.Type)
		}
		if entry.Name != tt.name {
			t.Errorf("parseSysusersLine(%q).Name = %q, want %q", tt.line, entry.Name, tt.name)
		}
		if tt.hasGID {
			if entry.GID == nil || *entry.GID != tt.gid {
				t.Errorf("parseSysusersLine(%q).GID = %v, want %d", tt.line, entry.GID, tt.gid)
			}
		} else if entry.GID != nil {
			t.Errorf("parseSysusersLine(%q).GID = %v, want nil", tt.line, *entry.GID)
		}
	}
}

func TestParseSysusersLine_Range(t *testing.T) {
	entry, err := parseSysusersLine("r - 500-999")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Type != sysusersRange {
		t.Errorf("Type = %d, want sysusersRange", entry.Type)
	}
	if entry.RangeStart != 500 || entry.RangeEnd != 999 {
		t.Errorf("Range = %d-%d, want 500-999", entry.RangeStart, entry.RangeEnd)
	}
}

func TestParseSysusersLine_Unknown(t *testing.T) {
	entry, err := parseSysusersLine("m user group")
	if err != nil {
		t.Fatal(err)
	}
	if entry != nil {
		t.Errorf("expected nil for unknown type, got %+v", entry)
	}
}
