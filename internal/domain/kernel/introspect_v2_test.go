package kernel

import (
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
)

func TestKernelIntrospectionV2MatchesAPI(t *testing.T) {
	if err := dbusv2.VerifyIntrospection(kernelIntrospectionV2, &DBusV2{}); err != nil {
		t.Fatal(err)
	}
}
