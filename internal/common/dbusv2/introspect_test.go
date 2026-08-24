package dbusv2

import "testing"

func TestJobsIntrospectionMatchesAPI(t *testing.T) {
	if err := VerifyIntrospection(jobsIntrospection, &JobsAPI{}); err != nil {
		t.Fatal(err)
	}
}
