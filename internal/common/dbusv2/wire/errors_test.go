package wire

import (
	"errors"
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
)

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		dbusErr string
	}{
		{"nil", nil, ""},
		{"apt", apmerr.New(apmerr.ErrorTypeApt, errors.New("x")), "org.altlinux.APM2.Error.Apt"},
		{"no operation", apmerr.New(apmerr.ErrorTypeNoOperation, errors.New("x")), "org.altlinux.APM2.Error.NoOperation"},
		{"permission is standard AccessDenied", apmerr.New(apmerr.ErrorTypePermission, errors.New("x")), "org.freedesktop.DBus.Error.AccessDenied"},
		{"plain error", errors.New("x"), "org.altlinux.APM2.Error.Failed"},
		{"wrapped apm error", errors.Join(errors.New("ctx"), apmerr.New(apmerr.ErrorTypeNotFound, errors.New("x"))), "org.altlinux.APM2.Error.NotFound"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Error(tt.err)
			if tt.dbusErr == "" {
				if got != nil {
					t.Fatalf("Error(nil) = %v", got)
				}
				return
			}
			if got == nil || got.Name != tt.dbusErr {
				t.Errorf("Error() = %v, want name %s", got, tt.dbusErr)
			}
		})
	}
}
