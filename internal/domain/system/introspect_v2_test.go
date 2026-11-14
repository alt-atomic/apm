package system

import (
	"reflect"
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
)

func TestPackagesIntrospectionV2MatchesAPI(t *testing.T) {
	if err := dbusv2.VerifyIntrospection(packagesIntrospectionV2, &PackagesV2{}); err != nil {
		t.Fatal(err)
	}
}

func TestPackageOperationOptionsDecodeAptConfig(t *testing.T) {
	var options struct {
		aptOperationOptions
		NoUpdate bool `json:"noUpdate"`
	}
	err := wire.DecodeOptions(`{"aptConfig":{"Acquire::Retries":"3"},"noUpdate":true}`, &options)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Acquire::Retries": "3"}
	if !reflect.DeepEqual(options.AptConfig, want) || !options.NoUpdate {
		t.Fatalf("decoded options = %+v, want aptConfig=%v noUpdate=true", options, want)
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
