package distrobox

import (
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
)

func TestDistroboxIntrospectionV2MatchesAPI(t *testing.T) {
	if err := dbusv2.VerifyIntrospection(distroboxIntrospectionV2, &DBusV2{}); err != nil {
		t.Fatal(err)
	}
}

func TestIconsIntrospectionV2MatchesAPI(t *testing.T) {
	if err := dbusv2.VerifyIntrospection(iconsIntrospectionV2, &IconsV2{}); err != nil {
		t.Fatal(err)
	}
}
