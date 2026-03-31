package lint

import (
	"apm/internal/common/app"
	"apm/internal/common/testutil"
	"context"
)

func testContext() context.Context {
	cfg := testutil.DefaultAppConfig()
	return context.WithValue(context.Background(), app.AppConfigKey, cfg)
}
