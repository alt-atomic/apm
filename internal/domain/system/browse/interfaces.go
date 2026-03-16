package browse

import (
	_package "apm/internal/common/apt/package"
	"apm/internal/common/filter"
	"context"
)

type aptDatabaseService interface {
	QueryHostImagePackages(ctx context.Context, filters []filter.Filter, sortField, sortOrder string, limit, offset int) ([]_package.Package, error)
	CountHostImagePackages(ctx context.Context, filters []filter.Filter) (int64, error)
}
