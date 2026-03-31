package altfiles

import (
	"strings"
	"testing"
)

func TestPatchNsswitch(t *testing.T) {
	input := `passwd:     files systemd
shadow:     tcb files systemd
group:      files [SUCCESS=merge] systemd role
gshadow:    files systemd`

	result := string(PatchNsswitch([]byte(input)))

	if !strings.Contains(result, "passwd:     files altfiles systemd") {
		t.Errorf("passwd line not patched correctly:\n%s", result)
	}

	if !strings.Contains(result, "files [SUCCESS=merge] altfiles [SUCCESS=merge] systemd") {
		t.Errorf("group line not patched correctly:\n%s", result)
	}

	if strings.Contains(result, "shadow:") && strings.Contains(result, "shadow:     tcb files altfiles") {
		t.Error("shadow line should not be modified")
	}
}

func TestPatchNsswitchIdempotent(t *testing.T) {
	input := `passwd:     files altfiles systemd
group:      files [SUCCESS=merge] altfiles [SUCCESS=merge] systemd`

	result := string(PatchNsswitch([]byte(input)))

	if strings.Count(result, "altfiles") != 2 {
		t.Errorf("expected 2 altfiles occurrences, got %d:\n%s", strings.Count(result, "altfiles"), result)
	}
}
