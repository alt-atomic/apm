package repository

import (
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
)

func TestRepoIntrospectionV2MatchesAPI(t *testing.T) {
	if err := dbusv2.VerifyIntrospection(repoIntrospectionV2, &DBusV2{}); err != nil {
		t.Fatal(err)
	}
}
