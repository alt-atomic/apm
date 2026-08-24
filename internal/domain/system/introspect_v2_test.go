package system

import (
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
)

func TestPackagesIntrospectionV2MatchesAPI(t *testing.T) {
	if err := dbusv2.VerifyIntrospection(packagesIntrospectionV2, &PackagesV2{}); err != nil {
		t.Fatal(err)
	}
}

func TestImageIntrospectionV2MatchesAPI(t *testing.T) {
	if err := dbusv2.VerifyIntrospection(imageIntrospectionV2, &ImageV2{}); err != nil {
		t.Fatal(err)
	}
}

func TestApplicationsIntrospectionV2MatchesAPI(t *testing.T) {
	if err := dbusv2.VerifyIntrospection(applicationsIntrospectionV2, &ApplicationsV2{}); err != nil {
		t.Fatal(err)
	}
}
